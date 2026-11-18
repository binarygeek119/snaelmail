package main

import (
	"context"
	"crypto/ed25519"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"snailmail/internal/config"
	"snailmail/internal/store"
)

type appState struct {
	cfg     *config.Config
	store   *store.Store
	siteKey ed25519.PrivateKey
}

func main() {
	cfgPath := flag.String("config", "config.toml", "path to config.toml")
	dsn := flag.String("dsn", "", "postgres DSN (default: env SMAIL_DSN or postgres://localhost/snailmail)")
	addr := flag.String("addr", ":8080", "HTTP listen address")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.Load(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("snailmail/site: domain=%s hops_max=%d surface_kpd=%d", cfg.MailDomain, cfg.Route.MaxHops, cfg.SpeedKmPerDay.Surface)

	dsnVal := *dsn
	if dsnVal == "" {
		dsnVal = os.Getenv("SMAIL_DSN")
	}
	if dsnVal == "" {
		dsnVal = "postgres://localhost/snailmail"
	}

	store, err := store.New(ctx, dsnVal)
	if err != nil {
		log.Fatalf("store: %v", err)
	}
	defer store.Close()

	// Load site key
	siteKey, err := loadSiteKey(cfg.SiteKey)
	if err != nil {
		log.Fatalf("site key: %v", err)
	}

	app := &appState{
		cfg:     cfg,
		store:   store,
		siteKey: siteKey,
	}

	mux := http.NewServeMux()

	// Serve static files (web UI)
	mux.HandleFunc("GET /{$}", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/site/index.html")
	})
	mux.HandleFunc("GET /login.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/site/login.html")
	})
	mux.HandleFunc("GET /signup.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/site/signup.html")
	})
	mux.HandleFunc("GET /dashboard.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/site/dashboard.html")
	})
	mux.HandleFunc("GET /settings.html", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "web/site/settings.html")
	})

	// Health checks
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		tctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := store.Health(tctx); err != nil {
			http.Error(w, "db down", http.StatusServiceUnavailable)
			return
		}
		fmt.Fprintln(w, "ok")
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, "ready")
	})

	// API routes - users
	mux.HandleFunc("POST /api/signup", app.apiSignup)
	mux.HandleFunc("POST /api/verify", app.apiVerify)
	mux.HandleFunc("POST /api/session", app.apiSession)
	mux.HandleFunc("POST /api/letters", app.apiLetters)
	mux.HandleFunc("GET /api/letters/", app.apiGetLetter)
	mux.HandleFunc("GET /inbox", app.apiInbox)
	mux.HandleFunc("GET /outbox", app.apiOutbox)

	// API routes - relays
	mux.HandleFunc("POST /api/relays", app.apiCreateRelay)
	mux.HandleFunc("GET /api/relays/", app.apiRelayRouter)
	mux.HandleFunc("POST /api/relay/", app.apiRelayActionRouter)
	mux.HandleFunc("GET /open-post-offices", app.apiOpenPostOffices)
	mux.HandleFunc("GET /open-post-offices/", app.apiOpenPostOffice)

	// API routes - admin
	mux.HandleFunc("POST /api/admin/pages/", app.apiAdminPageRouter)

	// API routes - search
	mux.HandleFunc("GET /api/relays/search", app.handleRelaySearch)
	mux.HandleFunc("GET /api/users/search", app.handleUserSearch)
	mux.HandleFunc("GET /api/letters/search", app.handleLetterSearch)

	// API routes - transparency log
	mux.HandleFunc("GET /api/tlog/root", app.handleTLogRoot)
	mux.HandleFunc("GET /api/tlog/proof/", app.handleTLogProof)
	mux.HandleFunc("GET /api/tlog/leaf/", app.handleTLogLeaf)

	// API routes - settings
	mux.HandleFunc("GET /api/settings", app.handleGetSettings)
	mux.HandleFunc("POST /api/settings", app.handleUpdateSettings)

	// API routes - tracking
	mux.HandleFunc("GET /api/tracking/", app.handleGetTracking)
	mux.HandleFunc("POST /api/tracking/", app.handleShareTracking)

	// API routes - notifications
	mux.HandleFunc("GET /api/notifications", app.handleGetNotifications)
	mux.HandleFunc("POST /api/notifications/read", app.handleMarkNotificationRead)

	// Admin panel
	mux.HandleFunc("GET /admin.html", app.handleAdminPanel)
	mux.HandleFunc("GET /api/admin/stats", app.handleAdminStats)
	mux.HandleFunc("GET /api/admin/users", app.handleAdminUsers)
	mux.HandleFunc("GET /api/admin/relays", app.handleAdminRelays)
	mux.HandleFunc("GET /api/admin/letters", app.handleAdminLetters)
	mux.HandleFunc("GET /api/admin/actions", app.handleAdminActions)
	mux.HandleFunc("GET /api/users/me", app.handleUserMe)
	
	// Admin user management
	mux.HandleFunc("POST /api/admin/users/{id}/suspend", app.handleAdminSuspendUser)
	mux.HandleFunc("POST /api/admin/users/{id}/unsuspend", app.handleAdminUnsuspendUser)
	mux.HandleFunc("DELETE /api/admin/users/{id}", app.handleAdminDeleteUser)
	
	// Admin relay management
	mux.HandleFunc("POST /api/admin/relays/{id}/suspend", app.handleAdminSuspendRelay)
	mux.HandleFunc("POST /api/admin/relays/{id}/unsuspend", app.handleAdminUnsuspendRelay)
	mux.HandleFunc("DELETE /api/admin/relays/{id}", app.handleAdminDeleteRelay)

	// Start background workers
	app.startNotificationWorker()
	app.startDeliveryWorker()
	log.Println("snailmail/site: background workers started")

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("snailmail/site: listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("snailmail/site: shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

// loadSiteKey loads the Ed25519 site key from a hex-encoded file.
func loadSiteKey(path string) (ed25519.PrivateKey, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// If file doesn't exist, generate a new key for development
		if os.IsNotExist(err) {
			log.Printf("site key not found at %s, generating new key", path)
			_, priv, err := ed25519.GenerateKey(nil)
			if err != nil {
				return nil, err
			}
			// Save for future use
			hexKey := hex.EncodeToString(priv.Seed())
			if err := os.WriteFile(path, []byte(hexKey+"\n"), 0600); err != nil {
				log.Printf("warning: could not save generated key to %s: %v", path, err)
			}
			return priv, nil
		}
		return nil, err
	}

	// Parse hex-encoded seed
	hexKey := strings.TrimSpace(string(data))
	seed, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("invalid hex key: %w", err)
	}

	if len(seed) != ed25519.SeedSize {
		return nil, fmt.Errorf("invalid key size: got %d bytes, want %d", len(seed), ed25519.SeedSize)
	}

	return ed25519.NewKeyFromSeed(seed), nil
}
