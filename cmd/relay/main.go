package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"snailmail/internal/blocker"
	"snailmail/internal/config"
	"snailmail/internal/sse"
	"snailmail/internal/store"
)

var Version = "0.5.0"

type relayApp struct {
	cfg            *config.Config
	store          *store.Store
	broker         *sse.Broker
	blockerRegistry *blocker.Registry
}

func main() {
	// Check for subcommands
	if len(os.Args) > 1 {
		switch os.Args[1] {
		case "keygen":
			if err := keygenCmd(os.Args[2:]); err != nil {
				log.Fatal(err)
			}
			return
		case "login":
			if err := loginCmd(os.Args[2:]); err != nil {
				log.Fatal(err)
			}
			return
		}
	}

	// Otherwise, run the relay daemon
	cfgPath := flag.String("config", "config.toml", "path to config.toml")
	dsn := flag.String("dsn", "", "postgres DSN (default: env SMAIL_DSN or postgres://localhost/snailmail)")
	addr := flag.String("addr", ":8477", "HTTP listen address (relay daemon)")
	baseURL := flag.String("base-url", "http://localhost:8080", "site base URL for blocker fix links")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cfg, err := config.LoadOrDefault(*cfgPath)
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	log.Printf("snailmail/relay: domain=%s", cfg.MailDomain)

	dsnVal := *dsn
	if dsnVal == "" {
		dsnVal = os.Getenv("SMAIL_DSN")
	}
	if dsnVal == "" {
		dsnVal = "postgres://localhost/snailmail"
	}

	st, err := store.New(ctx, dsnVal)
	if err != nil {
		log.Printf("warning: store unavailable (setup UI still works): %v", err)
		st = nil
	} else {
		defer st.Close()
	}

	app := &relayApp{
		cfg:            cfg,
		store:          st,
		broker:         sse.NewBroker(),
		blockerRegistry: blocker.NewRegistry(*baseURL),
	}
	defer app.broker.Close()

	mux := http.NewServeMux()
	app.setupRoutes(mux)

	// Start background services
	app.startBackgroundServices(ctx)

	srv := &http.Server{
		Addr:              *addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		log.Printf("snailmail/relay: listening on %s", *addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("snailmail/relay: shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(ctx, 10*time.Second)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}
