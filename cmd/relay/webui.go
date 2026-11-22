package main

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"snailmail/internal/relayreg"
)

type setupState struct {
	MainSite   string `json:"main_site"`
	Hostname   string `json:"hostname"`
	City       string `json:"city"`
	Region     string `json:"region"`
	CC         string `json:"cc"`
	Lat        float64 `json:"lat"`
	Lon        float64 `json:"lon"`
	TZ         string `json:"tz"`
	Pubkey     string `json:"pubkey"`
	Fingerprint string `json:"fingerprint"`
	RelayID    string `json:"relay_id"`
	Status     string `json:"status"`
	LastError  string `json:"last_error"`
}

func relayDataDir() string {
	if d := os.Getenv("SMAIL_RELAY_DIR"); d != "" {
		return d
	}
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return ".snailmail-relay"
	}
	return filepath.Join(home, ".snailmail-relay")
}

func relayStatePath() string { return filepath.Join(relayDataDir(), "setup.json") }
func relayKeyPath() string   { return filepath.Join(relayDataDir(), "relay.key") }
func relayPubPath() string   { return filepath.Join(relayDataDir(), "relay.pub") }

func loadSetupState() setupState {
	s := loadSetupStateDB()
	// migrate from legacy JSON if DB is empty
	if s.Hostname == "" && s.Pubkey == "" && s.RelayID == "" {
		b, err := os.ReadFile(relayStatePath())
		if err == nil {
			var js setupState
			if json.Unmarshal(b, &js) == nil && (js.Hostname != "" || js.Pubkey != "" || js.RelayID != "") {
				if js.MainSite == "" { js.MainSite = "https://snaelmail.org" }
				if js.TZ == "" { js.TZ = "UTC" }
				_ = saveSetupStateDB(js)
				return js
			}
		}
	}
	return s
}

func saveSetupState(s setupState) error {
	if err := saveSetupStateDB(s); err != nil { return err }
	// keep legacy JSON in sync for compatibility
	if err := os.MkdirAll(relayDataDir(), 0700); err == nil {
		b, _ := json.MarshalIndent(s, "", "  ")
		_ = os.WriteFile(relayStatePath(), b, 0600)
	}
	return nil
}

func (app *relayApp) setupUIHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(relaySetupHTML))
}

func (app *relayApp) setupStatusHandler(w http.ResponseWriter, r *http.Request) {
	s := loadSetupState()
	s.LastError = ""
	writeJSON(w, http.StatusOK, map[string]any{"state": s, "version": Version})
}

func (app *relayApp) setupKeygenHandler(w http.ResponseWriter, r *http.Request) {
	s := loadSetupState()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.MkdirAll(relayDataDir(), 0700); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	privHex := fmt.Sprintf("%x", priv.Seed())
	pubB64 := base64.StdEncoding.EncodeToString(pub)
	if err := os.WriteFile(relayKeyPath(), []byte(privHex+"\n"), 0600); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if err := os.WriteFile(relayPubPath(), []byte(pubB64+"\n"), 0644); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	s.Pubkey = pubB64
	s.Fingerprint = fmt.Sprintf("%x", relayreg.KeyFingerprint(pub))
	s.Status = "key_generated"
	_ = saveSetupState(s)
	writeJSON(w, http.StatusOK, map[string]any{"state": s})
}

func (app *relayApp) setupRegisterHandler(w http.ResponseWriter, r *http.Request) {
	var req struct {
		MainSite string  `json:"main_site"`
		Hostname string  `json:"hostname"`
		City     string  `json:"city"`
		Region   string  `json:"region"`
		CC       string  `json:"cc"`
		Lat      float64 `json:"lat"`
		Lon      float64 `json:"lon"`
		TZ       string  `json:"tz"`
		StampSVG string  `json:"stamp_svg"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid JSON"})
		return
	}
	s := loadSetupState()
	if req.MainSite != "" { s.MainSite = strings.TrimRight(req.MainSite, "/") }
	s.Hostname = strings.TrimSpace(req.Hostname)
	s.City = strings.TrimSpace(req.City)
	s.Region = strings.TrimSpace(req.Region)
	s.CC = strings.TrimSpace(req.CC)
	s.Lat = req.Lat
	s.Lon = req.Lon
	if req.TZ != "" { s.TZ = req.TZ }
	if s.Pubkey == "" {
		// try load existing
		b, err := os.ReadFile(relayPubPath())
		if err == nil { s.Pubkey = strings.TrimSpace(string(b)) }
	}
	if s.Pubkey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "generate a keypair first"})
		return
	}
	payload := map[string]any{
		"hostname": s.Hostname, "city": s.City, "region": s.Region, "cc": s.CC,
		"lat": s.Lat, "lon": s.Lon, "tz": s.TZ, "pubkey": s.Pubkey, "stamp_svg": req.StampSVG,
	}
	body, _ := json.Marshal(payload)
	url := s.MainSite + "/api/relays"
	hreq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("X-Relay-Version", Version)
	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(hreq)
	if err != nil {
		s.Status = "error"
		s.LastError = err.Error()
		_ = saveSetupState(s)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()})
		return
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		s.Status = "error"
		s.LastError = string(rb)
		_ = saveSetupState(s)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(resp.StatusCode)
		_, _ = w.Write(rb)
		return
	}
	var out map[string]any
	_ = json.Unmarshal(rb, &out)
	if id, ok := out["id"].(string); ok { s.RelayID = id }
	s.Status = "registered"
	s.LastError = ""
	_ = saveSetupState(s)
	writeJSON(w, http.StatusOK, map[string]any{"state": s, "response": out})
}

const relaySetupHTML = `<!DOCTYPE html>
<html lang="en"><head><meta charset="UTF-8"><meta name="viewport" content="width=device-width,initial-scale=1">
<title>Snaelmail Relay Setup</title>
<style>
body{font-family:system-ui,-apple-system,sans-serif;background:#f5f7fa;margin:0;color:#1f2937}
.wrap{max-width:860px;margin:0 auto;padding:2rem 1rem}
.card{background:#fff;border-radius:12px;padding:1.25rem;box-shadow:0 2px 10px rgba(0,0,0,.06);margin-bottom:1rem}
h1{margin:0 0 .25rem;color:#4f46e5} p.muted{color:#6b7280;margin-top:0}
label{display:block;font-weight:600;margin:.5rem 0 .25rem}
input,textarea{width:100%;padding:.6rem;border:1px solid #d1d5db;border-radius:8px;box-sizing:border-box}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(220px,1fr));gap:.75rem}
.btn{background:#4f46e5;color:#fff;border:0;padding:.65rem 1rem;border-radius:8px;cursor:pointer;font-weight:600}
.btn:hover{background:#4338ca} .btn.secondary{background:#64748b}
.row{display:flex;gap:.5rem;flex-wrap:wrap;margin-top:.75rem}
pre{background:#0f172a;color:#e2e8f0;padding:1rem;border-radius:8px;overflow:auto}
.ok{color:#16a34a;font-weight:600}.err{color:#dc2626;font-weight:600}
</style></head>
<body><div class="wrap">
  <div class="card">
    <h1>📮 Snaelmail Relay Setup</h1>
    <p class="muted">Configure this post office relay and register it with the main Snaelmail site. Runs on port 8477.</p>
    <div id="status" class="muted">Loading...</div>
  </div>
  <div class="card">
    <h2>1. Relay Identity</h2>
    <p class="muted">Generate an Ed25519 keypair for this relay. Keep the private key safe.</p>
    <div class="row"><button class="btn" onclick="keygen()">Generate Keypair</button></div>
    <label>Public Key (base64)</label><textarea id="pubkey" rows="2" readonly></textarea>
    <label>Fingerprint</label><input id="fingerprint" readonly>
  </div>
  <div class="card">
    <h2>2. Register with Main Site</h2>
    <div class="grid">
      <label>Main Site URL<input id="main_site" value="https://snaelmail.org"></label>
      <label>City<input id="city" placeholder="Des Moines" oninput="updateHostname()"></label>
      <label>Region / State<input id="region" placeholder="Iowa" oninput="updateHostname()"></label>
      <label>Country Code<input id="cc" placeholder="US" oninput="updateHostname()"></label>
      <label>Hostname<input id="hostname" placeholder="po-desmoines-us" readonly style="background:#f3f4f6;"></label>
      <label>Timezone<input id="tz" placeholder="America/Chicago"></label>
      <label>Latitude<input id="lat" type="number" step="any" placeholder="41.5868"></label>
      <label>Longitude<input id="lon" type="number" step="any" placeholder="-93.6250"></label>
    </div>
    <label>Stamp SVG (optional)</label><textarea id="stamp_svg" rows="4" placeholder="<svg ...>...</svg>"></textarea>
    <div class="row"><button class="btn" onclick="registerRelay()">Register / Open Post Office</button></div>
    <div id="result" style="margin-top:.75rem;"></div>
  </div>
  <div class="card">
    <h2>Setup State</h2>
    <pre id="state"></pre>
  </div>
</div>
<script>
function slugPart(s){
  return (s||'').toLowerCase().normalize('NFKD').replace(/[^a-z0-9]+/g,'').slice(0,24);
}
function updateHostname(){
  const city = document.getElementById('city').value;
  const region = document.getElementById('region').value;
  const cc = (document.getElementById('cc').value||'').toLowerCase().replace(/[^a-z]/g,'').slice(0,2);
  let mid = slugPart(city) || slugPart(region) || 'relay';
  if (mid.length < 2) mid = (mid + 'relay').slice(0,24);
  if (mid.length > 24) mid = mid.slice(0,24);
  const host = 'po-' + mid + '-' + (cc || 'xx');
  document.getElementById('hostname').value = host;
}
async function loadStatus(){
  const r = await fetch('/api/setup/status'); const d = await r.json();
  const s = d.state || {};
  document.getElementById('status').innerHTML = 'Relay version: <b>'+(d.version||'dev')+'</b> · Status: <b>'+(s.status||'not configured')+'</b>';
  for (const k of ['main_site','hostname','city','region','cc','tz','pubkey','fingerprint']) { const el=document.getElementById(k); if(el && s[k]!=null) el.value=s[k]; }
  if (s.lat!=null) document.getElementById('lat').value=s.lat;
  if (s.lon!=null) document.getElementById('lon').value=s.lon;
  updateHostname();
  document.getElementById('state').textContent = JSON.stringify(d, null, 2);
}
async function keygen(){
  const r = await fetch('/api/setup/keygen',{method:'POST'}); const d = await r.json();
  if(!r.ok){ document.getElementById('result').innerHTML='<div class="err">'+(d.error||r.status)+'</div>'; return; }
  await loadStatus();
  document.getElementById('result').innerHTML='<div class="ok">Keypair generated.</div>';
}
async function registerRelay(){
  const body = {
    main_site: document.getElementById('main_site').value,
    hostname: document.getElementById('hostname').value,
    city: document.getElementById('city').value,
    region: document.getElementById('region').value,
    cc: document.getElementById('cc').value,
    tz: document.getElementById('tz').value,
    lat: parseFloat(document.getElementById('lat').value)||0,
    lon: parseFloat(document.getElementById('lon').value)||0,
    stamp_svg: document.getElementById('stamp_svg').value
  };
  const r = await fetch('/api/setup/register',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  const d = await r.json().catch(()=>({}));
  if(!r.ok){ document.getElementById('result').innerHTML='<div class="err">Error: '+(d.error||JSON.stringify(d))+'</div>'; await loadStatus(); return; }
  document.getElementById('result').innerHTML='<div class="ok">Registered with main site. Relay ID: '+((d.state&&d.state.relay_id)||'')+'</div>';
  await loadStatus();
}
loadStatus();
</script></body></html>`
