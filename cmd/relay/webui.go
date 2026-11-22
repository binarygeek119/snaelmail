package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"snailmail/internal/page"
	"snailmail/internal/relayreg"
	"snailmail/internal/render"
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

func lookupRelayIDByHostname(ctx context.Context, client *http.Client, mainSite, hostname string) string {
	// try open-post-offices listing
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(mainSite, "/")+"/open-post-offices", nil)
	resp, err := client.Do(req)
	if err == nil {
		defer resp.Body.Close()
		var data struct { Relays []map[string]any `json:"relays"` }
		if json.NewDecoder(resp.Body).Decode(&data) == nil {
			for _, r := range data.Relays {
				h, _ := r["hostname"].(string)
				if h == "" { if v,ok := r["Hostname"].(string); ok { h=v } }
				if strings.EqualFold(h, hostname) {
					if id,ok := r["id"].(string); ok { return id }
					if id,ok := r["ID"].(string); ok { return id }
				}
			}
		}
	}
	return ""
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
	s.MainSite = "https://snaelmail.org"
	s.Hostname = strings.TrimSpace(req.Hostname)
	s.City = strings.TrimSpace(req.City)
	s.Region = strings.TrimSpace(req.Region)
	s.CC = strings.TrimSpace(req.CC)
	s.Lat = req.Lat
	s.Lon = req.Lon
	if req.TZ != "" { s.TZ = req.TZ }
	if s.Pubkey == "" {
		if b, err := os.ReadFile(relayPubPath()); err == nil { s.Pubkey = strings.TrimSpace(string(b)) }
	}
	if s.Pubkey == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "generate a keypair first"})
		return
	}
	client := &http.Client{Timeout: 15 * time.Second}

	// If already registered, update profile
	if s.RelayID != "" {
		up := map[string]any{"city":s.City,"region":s.Region,"cc":s.CC,"lat":s.Lat,"lon":s.Lon,"tz":s.TZ}
		ub, _ := json.Marshal(up)
		ureq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(s.MainSite,"/")+"/api/relay/"+s.RelayID+"/update", bytes.NewReader(ub))
		ureq.Header.Set("Content-Type","application/json")
		ureq.Header.Set("X-Relay-Version", Version)
		uresp, err := client.Do(ureq)
		if err == nil {
			urb, _ := io.ReadAll(uresp.Body); uresp.Body.Close()
			if uresp.StatusCode >= 200 && uresp.StatusCode < 300 {
				s.Status = "registered"; s.LastError = ""
				_ = saveSetupState(s)
				writeJSON(w, http.StatusOK, map[string]any{"state":s, "message":"Relay info updated on main site."})
				return
			}
			// if 404, fall through to register
			if uresp.StatusCode != 404 {
				s.Status = "error"; s.LastError = string(urb); _ = saveSetupState(s)
				w.Header().Set("Content-Type","application/json"); w.WriteHeader(uresp.StatusCode); _,_ = w.Write(urb); return
			}
		}
	}

	// Register new relay
	payload := map[string]any{"hostname":s.Hostname,"city":s.City,"region":s.Region,"cc":s.CC,"lat":s.Lat,"lon":s.Lon,"tz":s.TZ,"pubkey":s.Pubkey,"stamp_svg":req.StampSVG}
	body, _ := json.Marshal(payload)
	hreq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(s.MainSite,"/")+"/api/relays", bytes.NewReader(body))
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("X-Relay-Version", Version)
	resp, err := client.Do(hreq)
	if err != nil {
		s.Status = "error"; s.LastError = err.Error(); _ = saveSetupState(s)
		writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()}); return
	}
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 {
		if resp.StatusCode == http.StatusConflict {
			if id := lookupRelayIDByHostname(r.Context(), client, s.MainSite, s.Hostname); id != "" {
				s.RelayID = id; s.Status = "registered"; s.LastError = ""; _ = saveSetupState(s)
				// try update after linking
				up := map[string]any{"city":s.City,"region":s.Region,"cc":s.CC,"lat":s.Lat,"lon":s.Lon,"tz":s.TZ}
				ub,_ := json.Marshal(up)
				ureq,_ := http.NewRequestWithContext(r.Context(), http.MethodPost, strings.TrimRight(s.MainSite,"/")+"/api/relay/"+id+"/update", bytes.NewReader(ub))
				ureq.Header.Set("Content-Type","application/json"); ureq.Header.Set("X-Relay-Version", Version)
				if uresp,err := client.Do(ureq); err == nil { io.Copy(io.Discard, uresp.Body); uresp.Body.Close() }
				writeJSON(w, http.StatusOK, map[string]any{"state":s, "message":"Relay linked and info updated."}); return
			}
		}
		s.Status = "error"; s.LastError = string(rb); _ = saveSetupState(s)
		w.Header().Set("Content-Type","application/json"); w.WriteHeader(resp.StatusCode); _,_ = w.Write(rb); return
	}
	var out map[string]any
	_ = json.Unmarshal(rb, &out)
	if id, ok := out["id"].(string); ok { s.RelayID = id }
	if id, ok := out["relay_id"].(string); ok { s.RelayID = id }
	s.Status = "registered"; s.LastError = ""; _ = saveSetupState(s)
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
input,textarea,select{width:100%;padding:.6rem;border:1px solid #d1d5db;border-radius:8px;box-sizing:border-box;margin-top:.25rem}
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
    <div class="row"><button class="btn" onclick="keygen()">Generate Keypair</button><button class="btn secondary" onclick="exportIdentity()">Export as TXT</button></div>
    <label>Public Key (base64)</label><textarea id="pubkey" rows="2" readonly></textarea>
    <label>Fingerprint</label><input id="fingerprint" readonly>
  </div>
  <div class="card">
    <h2>2. Register with Main Site</h2>
    <div class="grid">
      <input id="main_site" type="hidden" value="https://snaelmail.org">
      <label>City<input id="city" placeholder="Des Moines" oninput="updateHostname()"></label>
      <label>Country Code<select id="cc" onchange="onCountryChange()"></select></label>
      <label>Region / State<select id="region" onchange="updateHostname()"></select></label>
      <label>Hostname<input id="hostname" placeholder="po-desmoines-us" readonly style="background:#f3f4f6;"></label>
      <label>Timezone<select id="tz"></select></label>
      <label>Latitude<input id="lat" type="number" step="any" placeholder="41.5868"></label>
      <label>Longitude<input id="lon" type="number" step="any" placeholder="-93.6250"></label>
    </div>
    <p class="muted" style="margin-top:.25rem;">Find your coordinates at <a href="https://www.latlong.net/" target="_blank" rel="noopener">latlong.net</a>.</p>
    <label style="display:block;margin:.75rem 0 .25rem;font-weight:600;">Post Office Stamp (SVG or PNG)</label>
    <input id="stampFile" type="file" accept=".svg,image/svg+xml,image/png" style="width:100%;">
    <div class="row" style="margin-top:.5rem;">
      <button class="btn secondary" type="button" onclick="uploadStamp()">Upload Stamp</button>
      <span id="stampMsg" class="muted"></span>
    </div>
    <div class="row"><button class="btn" onclick="registerRelay()">Register / Open Post Office</button><button class="btn secondary" onclick="updateRelayInfo()">Update Info</button></div>
    <div id="result" style="margin-top:.75rem;"></div>
  </div>
  <div class="card">
    <h2>3. City Images</h2>
    <p class="muted">Upload images for your post office public page. Saved to <code>/config/images</code> and synced to Snaelmail on publish.</p>
    <input id="imageFiles" type="file" accept="image/*" multiple>
    <div class="row">
      <button class="btn" onclick="uploadImages()">Upload Images</button>
      <button class="btn secondary" onclick="publishImages()">Sync to Main Site</button>
      <button class="btn secondary" onclick="copyCentral()">Copy from Central Post Office</button>
      <a class="btn secondary" href="/preview" target="_blank" style="text-decoration:none;display:inline-block;">Preview Public Site</a>
    </div>
    <div id="imgResult" style="margin-top:.75rem;"></div>
    <div id="imgList" style="display:flex;flex-wrap:wrap;gap:.75rem;margin-top:.75rem;"></div>
  </div>

</div>
<div id="verBadge" style="position:fixed;bottom:10px;right:10px;background:rgba(79,70,229,.9);color:#fff;padding:8px 12px;border-radius:6px;font-size:.85rem;font-weight:500;box-shadow:0 2px 8px rgba(0,0,0,.15);z-index:1000;">vdev</div>
<script>
const TZ_LIST = ["UTC","America/New_York","America/Chicago","America/Denver","America/Los_Angeles","America/Anchorage","Pacific/Honolulu","America/Toronto","America/Vancouver","America/Sao_Paulo","Europe/London","Europe/Paris","Europe/Berlin","Europe/Madrid","Europe/Rome","Europe/Amsterdam","Europe/Stockholm","Europe/Warsaw","Europe/Athens","Europe/Helsinki","Africa/Cairo","Africa/Johannesburg","Asia/Dubai","Asia/Kolkata","Asia/Singapore","Asia/Tokyo","Asia/Seoul","Asia/Shanghai","Asia/Hong_Kong","Australia/Sydney","Australia/Melbourne","Pacific/Auckland"];
const COUNTRIES = {
  "US": {name:"United States", regions:["Alabama","Alaska","Arizona","Arkansas","California","Colorado","Connecticut","Delaware","Florida","Georgia","Hawaii","Idaho","Illinois","Indiana","Iowa","Kansas","Kentucky","Louisiana","Maine","Maryland","Massachusetts","Michigan","Minnesota","Mississippi","Missouri","Montana","Nebraska","Nevada","New Hampshire","New Jersey","New Mexico","New York","North Carolina","North Dakota","Ohio","Oklahoma","Oregon","Pennsylvania","Rhode Island","South Carolina","South Dakota","Tennessee","Texas","Utah","Vermont","Virginia","Washington","West Virginia","Wisconsin","Wyoming","District of Columbia"]},
  "CA": {name:"Canada", regions:["Alberta","British Columbia","Manitoba","New Brunswick","Newfoundland and Labrador","Nova Scotia","Ontario","Prince Edward Island","Quebec","Saskatchewan","Northwest Territories","Nunavut","Yukon"]},
  "GB": {name:"United Kingdom", regions:["England","Scotland","Wales","Northern Ireland"]},
  "AU": {name:"Australia", regions:["New South Wales","Victoria","Queensland","Western Australia","South Australia","Tasmania","Australian Capital Territory","Northern Territory"]},
  "DE": {name:"Germany", regions:["Baden-Württemberg","Bavaria","Berlin","Brandenburg","Bremen","Hamburg","Hesse","Lower Saxony","Mecklenburg-Vorpommern","North Rhine-Westphalia","Rhineland-Palatinate","Saarland","Saxony","Saxony-Anhalt","Schleswig-Holstein","Thuringia"]},
  "FR": {name:"France", regions:["Île-de-France","Auvergne-Rhône-Alpes","Nouvelle-Aquitaine","Occitanie","Hauts-de-France","Grand Est","Provence-Alpes-Côte d'Azur","Pays de la Loire","Brittany","Normandy","Bourgogne-Franche-Comté","Centre-Val de Loire","Corsica"]},
  "NL": {name:"Netherlands", regions:["North Holland","South Holland","Utrecht","Gelderland","North Brabant","Limburg","Overijssel","Groningen","Flevoland","Drenthe","Friesland","Zeeland"]},
  "SE": {name:"Sweden", regions:["Stockholm","Västra Götaland","Skåne","Uppsala","Östergötland"]},
  "NO": {name:"Norway", regions:["Oslo","Viken","Vestland","Rogaland","Trøndelag"]},
  "DK": {name:"Denmark", regions:["Capital Region","Zealand","Central Denmark","Southern Denmark","North Denmark"]},
  "FI": {name:"Finland", regions:["Uusimaa","Southwest Finland","Pirkanmaa","North Ostrobothnia","Central Finland"]},
  "IE": {name:"Ireland", regions:["Leinster","Munster","Connacht","Ulster"]},
  "ES": {name:"Spain", regions:["Madrid","Catalonia","Andalusia","Valencia","Galicia","Castile and León","Basque Country"]},
  "IT": {name:"Italy", regions:["Lombardy","Lazio","Campania","Sicily","Veneto","Piedmont","Emilia-Romagna","Tuscany"]},
  "PL": {name:"Poland", regions:["Masovia","Lesser Poland","Greater Poland","Silesia","Pomerania"]},
  "BR": {name:"Brazil", regions:["São Paulo","Rio de Janeiro","Minas Gerais","Bahia","Paraná","Rio Grande do Sul"]},
  "MX": {name:"Mexico", regions:["Mexico City","Jalisco","Nuevo León","Puebla","Guanajuato"]},
  "IN": {name:"India", regions:["Maharashtra","Karnataka","Tamil Nadu","Delhi","Telangana","Uttar Pradesh","Gujarat","West Bengal"]},
  "JP": {name:"Japan", regions:["Tokyo","Osaka","Kanagawa","Aichi","Hokkaido","Fukuoka"]},
  "SG": {name:"Singapore", regions:["Singapore"]},
  "NZ": {name:"New Zealand", regions:["Auckland","Wellington","Canterbury","Waikato","Otago"]},
  "ZA": {name:"South Africa", regions:["Gauteng","Western Cape","KwaZulu-Natal","Eastern Cape"]},
  "CH": {name:"Switzerland", regions:["Zurich","Geneva","Bern","Vaud","Basel-Stadt"]},
  "AT": {name:"Austria", regions:["Vienna","Lower Austria","Upper Austria","Styria","Tyrol"]},
  "BE": {name:"Belgium", regions:["Flanders","Wallonia","Brussels"]}
};
function initLocationDropdowns(){
  const tz=document.getElementById('tz');
  tz.innerHTML = TZ_LIST.map(z=>'<option value="'+z+'">'+z+'</option>').join('');
  const cc=document.getElementById('cc');
  cc.innerHTML = '<option value="">Select country</option>' + Object.keys(COUNTRIES).map(c=>'<option value="'+c+'">'+c+' - '+COUNTRIES[c].name+'</option>').join('');
  onCountryChange();
}
function onCountryChange(){
  const cc=document.getElementById('cc').value;
  const region=document.getElementById('region');
  const cur=region.value;
  const list=(COUNTRIES[cc]&&COUNTRIES[cc].regions)||[];
  region.innerHTML = '<option value="">Select region/state</option>' + list.map(r=>'<option value="'+r+'">'+r+'</option>').join('');
  if (list.includes(cur)) region.value=cur;
  updateHostname();
}
function setSelectValue(id,val){
  const el=document.getElementById(id); if(!el) return;
  if(val && !Array.from(el.options).some(o=>o.value===val)){ const o=document.createElement('option'); o.value=val; o.textContent=val; el.appendChild(o); }
  if(val!=null) el.value=val;
}
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
let dropdownsReady=false;
async function loadStatus(){
  if(!dropdownsReady){ initLocationDropdowns(); dropdownsReady=true; }
  const r = await fetch('/api/setup/status'); const d = await r.json();
  const s = d.state || {};
  document.getElementById('status').innerHTML = 'Relay version: <b>'+(d.version||'dev')+'</b> · Status: <b>'+(s.status||'not configured')+'</b>';
  document.getElementById('verBadge').textContent = (d.version||'dev');
  document.getElementById('main_site').value = 'https://snaelmail.org';
  if (s.city!=null) document.getElementById('city').value=s.city;
  setSelectValue('tz', s.tz || 'UTC');
  setSelectValue('cc', s.cc || '');
  onCountryChange();
  setSelectValue('region', s.region || '');
  if (s.hostname!=null) document.getElementById('hostname').value=s.hostname;
  if (s.pubkey!=null) document.getElementById('pubkey').value=s.pubkey;
  if (s.fingerprint!=null) document.getElementById('fingerprint').value=s.fingerprint;
  if (s.lat!=null) document.getElementById('lat').value=s.lat;
  if (s.lon!=null) document.getElementById('lon').value=s.lon;
  updateHostname();
  await loadImages();
}
async function keygen(){
  const r = await fetch('/api/setup/keygen',{method:'POST'}); const d = await r.json();
  if(!r.ok){ document.getElementById('result').innerHTML='<div class="err">'+(d.error||r.status)+'</div>'; return; }
  await loadStatus();
  document.getElementById('result').innerHTML='<div class="ok">Keypair generated.</div>';
}
function exportIdentity(){
  const pubkey=document.getElementById('pubkey').value||'';
  const fp=document.getElementById('fingerprint').value||'';
  const host=document.getElementById('hostname').value||'';
  const txt='Snaelmail Relay Identity\n'
    +'Hostname: '+host+'\n'
    +'Fingerprint: '+fp+'\n'
    +'Public Key (base64): '+pubkey+'\n';
  const blob=new Blob([txt],{type:'text/plain'});
  const a=document.createElement('a');
  a.href=URL.createObjectURL(blob);
  a.download=(host||'relay-identity')+'.txt';
  document.body.appendChild(a); a.click(); a.remove();
  URL.revokeObjectURL(a.href);
}
async function loadImages(){
  const r = await fetch('/api/setup/images'); const d = await r.json();
  const list = d.images || [];
  const el = document.getElementById('imgList');
  if(!list.length){ el.innerHTML='<div class="muted">No images uploaded.</div>'; return; }
  el.innerHTML = list.map(function(img){ 
    var hero = img.hero ? '<span style="background:#667eea;color:white;padding:2px 8px;border-radius:999px;font-size:.75rem;">Hero</span>' : '<button class="btn secondary" style="padding:.25rem .5rem;font-size:.8rem;" onclick="setHeroImage(\''+img.name+'\')">Set as Hero</button>';
    return '<div style="width:180px;border:1px solid #ddd;border-radius:8px;padding:.5rem;background:#fff;">'
    + '<img src="'+img.url+'" style="width:100%;height:110px;object-fit:cover;border-radius:6px;background:#f3f4f6;">'
    + '<div style="font-size:.8rem;color:#555;margin:.35rem 0;word-break:break-word;">'+img.name+'</div>'
    + '<div style="display:flex;gap:.35rem;flex-wrap:wrap;align-items:center;">'+hero+'<button class="btn secondary" style="padding:.25rem .5rem;font-size:.8rem;" onclick="deleteImage(\''+img.name+'\')">Remove</button></div>'
    + '</div>'; }).join('');
}
async function uploadImages(){
  const files = document.getElementById('imageFiles').files;
  if(!files.length){ document.getElementById('imgResult').innerHTML='<div class="err">Choose image files first.</div>'; return; }
  const fd = new FormData();
  for(const f of files) fd.append('images', f);
  const r = await fetch('/api/setup/images',{method:'POST',body:fd});
  const d = await r.json().catch(()=>({}));
  if(!r.ok){ document.getElementById('imgResult').innerHTML='<div class="err">Upload error: '+(d.error||r.status)+'</div>'; return; }
  document.getElementById('imgResult').innerHTML='<div class="ok">Saved '+(d.saved||[]).length+' image(s) to /config/images.</div>';
  document.getElementById('imageFiles').value='';
  await loadImages();
}
async function deleteImage(name){
  await fetch('/api/setup/images/'+encodeURIComponent(name),{method:'DELETE'});
  await loadImages();
}
async function setHeroImage(name){
  await fetch('/api/setup/images/hero',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name})});
  await loadImages();
}
async function publishImages(){
  document.getElementById('imgResult').innerHTML='<div class="muted">Syncing to main site...</div>';
  const r = await fetch('/api/setup/publish',{method:'POST'});
  const d = await r.json().catch(()=>({}));
  if(!r.ok){ document.getElementById('imgResult').innerHTML='<div class="err">Sync error: '+(d.error||JSON.stringify(d))+'</div>'; return; }
  document.getElementById('imgResult').innerHTML='<div class="ok">Synced to main site. '+(d.message||'')+'</div>';
}
async function copyCentral(){
  document.getElementById('imgResult').innerHTML='<div class="muted">Copying from Central Post Office...</div>';
  const r = await fetch('/api/setup/copy-central',{method:'POST'});
  const d = await r.json().catch(()=>({}));
  if(!r.ok){ document.getElementById('imgResult').innerHTML='<div class="err">Copy error: '+(d.error||JSON.stringify(d))+'</div>'; return; }
  document.getElementById('imgResult').innerHTML='<div class="ok">Copied '+(d.copied_photos||0)+' image(s) from Central Post Office.</div>';
  await loadImages();
}
async function uploadStamp(){
  const f = (document.getElementById('stampFile').files||[])[0];
  const msg = document.getElementById('stampMsg');
  if(!f){ msg.style.color='#b91c1c'; msg.textContent='Choose a stamp file first.'; return; }
  msg.style.color='#555'; msg.textContent='Uploading...';
  const isSVG = f.type === 'image/svg+xml' || f.name.toLowerCase().endsWith('.svg');
  const body = {stamp_svg:'', stamp_png_base64:''};
  if(isSVG){ body.stamp_svg = await f.text(); }
  else { body.stamp_png_base64 = await new Promise((res,rej)=>{ const r=new FileReader(); r.onload=()=>res(String(r.result).split(',')[1]||''); r.onerror=rej; r.readAsDataURL(f); }); }
  const r = await fetch('/api/setup/stamp',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  const d = await r.json().catch(()=>({}));
  if(r.ok){ msg.style.color='#16a34a'; msg.textContent='Upload successful. Stamp saved.'; document.getElementById('stampFile').value=''; }
  else { msg.style.color='#b91c1c'; msg.textContent='Error: '+(d.error||r.status); }
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
    lon: parseFloat(document.getElementById('lon').value)||0
  };
  const r = await fetch('/api/setup/register',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  const d = await r.json().catch(()=>({}));
  if(!r.ok){ document.getElementById('result').innerHTML='<div class="err">Error: '+(d.error||JSON.stringify(d))+'</div>'; await loadStatus(); return; }
  document.getElementById('result').innerHTML='<div class="ok">'+(d.message||'Registered with main site. Relay ID: '+((d.state&&d.state.relay_id)||''))+'</div>';
  await loadStatus();
  try { await publishImages(); } catch(e) {}
}
async function updateRelayInfo(){
  const body = {
    main_site: document.getElementById('main_site').value,
    hostname: document.getElementById('hostname').value,
    city: document.getElementById('city').value,
    region: document.getElementById('region').value,
    cc: document.getElementById('cc').value,
    tz: document.getElementById('tz').value,
    lat: parseFloat(document.getElementById('lat').value)||0,
    lon: parseFloat(document.getElementById('lon').value)||0
  };
  document.getElementById('result').innerHTML='<div class="muted">Updating info...</div>';
  const r = await fetch('/api/setup/register',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});
  const d = await r.json().catch(()=>({}));
  if(!r.ok){ document.getElementById('result').innerHTML='<div class="err">Error: '+(d.error||JSON.stringify(d))+'</div>'; await loadStatus(); return; }
  document.getElementById('result').innerHTML='<div class="ok">'+(d.message||'Info updated on main site.')+'</div>';
  await loadStatus();
}
loadStatus();
</script></body></html>`

func relayImagesDir() string { return filepath.Join(relayDataDir(), "images") }
func relayImagesOrderPath() string { return filepath.Join(relayImagesDir(), "order.json") }

func loadImageOrder() []string {
	b, err := os.ReadFile(relayImagesOrderPath())
	if err != nil { return nil }
	var o []string
	if json.Unmarshal(b, &o) != nil { return nil }
	return o
}
func saveImageOrder(order []string) {
	_ = os.MkdirAll(relayImagesDir(), 0755)
	b, _ := json.Marshal(order)
	_ = os.WriteFile(relayImagesOrderPath(), b, 0644)
}
func orderedImageNames() []string {
	entries, err := os.ReadDir(relayImagesDir())
	if err != nil { return nil }
	exists := map[string]bool{}
	for _, e := range entries {
		if e.IsDir() { continue }
		n := e.Name(); ln := strings.ToLower(n)
		if strings.HasSuffix(ln,".png")||strings.HasSuffix(ln,".jpg")||strings.HasSuffix(ln,".jpeg")||strings.HasSuffix(ln,".webp")||strings.HasSuffix(ln,".svg") {
			exists[n]=true
		}
	}
	order := loadImageOrder()
	var out []string
	seen := map[string]bool{}
	for _, n := range order { if exists[n] && !seen[n] { out=append(out,n); seen[n]=true } }
	for n := range exists { if !seen[n] { out=append(out,n) } }
	// stable-ish: keep order file order, append unknowns in dir order
	if len(out) > 1 {
		// keep existing prefix, sort unknowns
		unknown := []string{}
		known := map[string]bool{}
		for _, n := range order { if exists[n] { known[n]=true } }
		for _, n := range out { if !known[n] { unknown = append(unknown,n) } }
		// rebuild: known in order + sorted unknown
		out2 := []string{}
		for _, n := range order { if exists[n] { out2 = append(out2,n) } }
		sortStrings(unknown)
		out2 = append(out2, unknown...)
		out = out2
	}
	return out
}
func sortStrings(a []string) { for i:=0;i<len(a);i++{ for j:=i+1;j<len(a);j++{ if a[j]<a[i]{ a[i],a[j]=a[j],a[i] } } } }

func (app *relayApp) setupListImagesHandler(w http.ResponseWriter, r *http.Request) {
	names := orderedImageNames()
	type img struct { Name string `json:"name"`; Size int64 `json:"size"`; URL string `json:"url"`; Hero bool `json:"hero"` }
	var out []img
	for i, n := range names {
		info, err := os.Stat(filepath.Join(relayImagesDir(), n))
		if err != nil { continue }
		out = append(out, img{Name: n, Size: info.Size(), URL: "/api/setup/images/" + n, Hero: i==0})
	}
	writeJSON(w, http.StatusOK, map[string]any{"images": out})
}

func (app *relayApp) setupUploadImageHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseMultipartForm(25 << 20); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid multipart form"})
		return
	}
	files := r.MultipartForm.File["images"]
	if len(files) == 0 { files = r.MultipartForm.File["file"] }
	if len(files) == 0 { writeJSON(w, http.StatusBadRequest, map[string]string{"error": "no files"}); return }
	if err := os.MkdirAll(relayImagesDir(), 0755); err != nil { writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()}); return }
	var saved []string
	for _, fh := range files {
		f, err := fh.Open(); if err != nil { continue }
		data, err := io.ReadAll(io.LimitReader(f, 10<<20))
		f.Close()
		if err != nil || len(data) == 0 { continue }
		name := filepath.Base(fh.Filename)
		name = strings.ReplaceAll(name, " ", "-")
		// sanitize
		safe := strings.Map(func(r rune) rune {
			if (r>='a'&&r<='z')||(r>='A'&&r<='Z')||(r>='0'&&r<='9')||r=='.'||r=='-'||r=='_' { return r }
			return '-'
		}, name)
		if safe == "" { safe = "image" }
		ln := strings.ToLower(safe)
		if !(strings.HasSuffix(ln, ".png")||strings.HasSuffix(ln, ".jpg")||strings.HasSuffix(ln, ".jpeg")||strings.HasSuffix(ln, ".webp")||strings.HasSuffix(ln, ".svg")) {
			safe = safe + ".png"
		}
		outPath := filepath.Join(relayImagesDir(), safe)
		if err := os.WriteFile(outPath, data, 0644); err != nil { continue }
		saved = append(saved, safe)
	}
	if len(saved) > 0 {
		order := orderedImageNames()
		seen := map[string]bool{}
		for _, n := range order { seen[n]=true }
		for _, n := range saved { if !seen[n] { order = append(order, n) } }
		saveImageOrder(order)
	}
	writeJSON(w, http.StatusOK, map[string]any{"saved": saved})
}

func (app *relayApp) setupGetImageHandler(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("name"))
	p := filepath.Join(relayImagesDir(), name)
	b, err := os.ReadFile(p)
	if err != nil { http.NotFound(w, r); return }
	ct := "application/octet-stream"
	ln := strings.ToLower(name)
	switch {
	case strings.HasSuffix(ln, ".png"): ct = "image/png"
	case strings.HasSuffix(ln, ".jpg"), strings.HasSuffix(ln, ".jpeg"): ct = "image/jpeg"
	case strings.HasSuffix(ln, ".webp"): ct = "image/webp"
	case strings.HasSuffix(ln, ".svg"): ct = "image/svg+xml"
	}
	w.Header().Set("Content-Type", ct)
	_, _ = w.Write(b)
}

func (app *relayApp) setupDeleteImageHandler(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("name"))
	p := filepath.Join(relayImagesDir(), name)
	if err := os.Remove(p); err != nil && !os.IsNotExist(err) { writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()}); return }
	order := orderedImageNames()
	var out []string
	for _, n := range order { if n != name { out = append(out, n) } }
	saveImageOrder(out)
	writeJSON(w, http.StatusOK, map[string]string{"status":"ok"})
}

func (app *relayApp) setupImageHeroHandler(w http.ResponseWriter, r *http.Request) {
	var req struct{ Name string `json:"name"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w,400,map[string]string{"error":"invalid JSON"}); return }
	name := filepath.Base(req.Name)
	order := orderedImageNames()
	found := false
	var out []string
	for _, n := range order { if n == name { found = true } else { out = append(out,n) } }
	if !found { writeJSON(w,404,map[string]string{"error":"image not found"}); return }
	out = append([]string{name}, out...)
	saveImageOrder(out)
	writeJSON(w, http.StatusOK, map[string]string{"status":"ok"})
}

func (app *relayApp) setupPublishHandler(w http.ResponseWriter, r *http.Request) {
	s := loadSetupState()
	if s.MainSite == "" || s.RelayID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error":"relay must be registered first (missing main_site or relay_id)"})
		return
	}
	// build images from /config/images in hero-first order
	names := orderedImageNames()
	var images []map[string]string
	var photos []map[string]any
	ord := 0
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(relayImagesDir(), n)); if err != nil { continue }
		images = append(images, map[string]string{"filename": n, "data": base64.StdEncoding.EncodeToString(b)})
		photos = append(photos, map[string]any{"ord": ord, "filename": n, "caption": strings.TrimSuffix(n, filepath.Ext(n))})
		ord++
		if ord >= 6 { break }
	}
	manifest := map[string]any{
		"photos": photos,
		"about": fmt.Sprintf("Welcome to the %s post office in %s, %s. This relay is part of the Snaelmail network, delivering messages with care and collecting postmarks along the way.", s.City, s.Region, s.CC),
		"facts": []map[string]string{{"key":"City","value":s.City},{"key":"Region","value":s.Region},{"key":"Country","value":s.CC}},
		"postmaster": map[string]string{"name":"Postmaster","role":"Relay Operator","bio":"Operating this Snaelmail post office."},
		"dispatch_times": []string{},
		"classes": []string{"standard"},
		"accent": map[string]string{"primary":"#333333","secondary":"#667eea","tertiary":"#764ba2","highlight":"#f5f3ff","muted":"#cccccc"},
	}
	payload := map[string]any{"manifest": manifest, "images": images}
	body, _ := json.Marshal(payload)
	url := strings.TrimRight(s.MainSite, "/") + "/api/relay/" + s.RelayID + "/publish"
	hreq, _ := http.NewRequestWithContext(r.Context(), http.MethodPost, url, bytes.NewReader(body))
	hreq.Header.Set("Content-Type", "application/json")
	hreq.Header.Set("X-Relay-Version", Version)
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(hreq)
	if err != nil { writeJSON(w, http.StatusBadGateway, map[string]string{"error": err.Error()}); return }
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	_, _ = w.Write(rb)
}

func (app *relayApp) localPreviewManifest(s setupState) page.Manifest {
	names := orderedImageNames()
	var photos []page.Photo
	for ord, n := range names {
		if ord >= 6 { break }
		photos = append(photos, page.Photo{Ord: ord, Filename: n, Caption: strings.TrimSuffix(n, filepath.Ext(n))})
	}
	if s.City == "" { s.City = "Your City" }
	if s.Region == "" { s.Region = "Region" }
	if s.CC == "" { s.CC = "US" }
	return page.Manifest{
		Photos: photos,
		About: fmt.Sprintf("Welcome to the %s post office in %s, %s. This relay is part of the Snaelmail network, delivering messages with care and collecting postmarks along the way.", s.City, s.Region, s.CC),
		Facts: []page.Fact{{Key:"City",Value:s.City},{Key:"Region",Value:s.Region},{Key:"Country",Value:s.CC}},
		Postmaster: page.Postmaster{Name:"Postmaster", Role:"Relay Operator", Bio:"Operating this Snaelmail post office."},
		Classes: []string{"standard"},
		Accent: page.Accent{Primary:"#333333",Secondary:"#667eea",Tertiary:"#764ba2",Highlight:"#f5f3ff",Muted:"#cccccc"},
	}
}

func (app *relayApp) setupPreviewHandler(w http.ResponseWriter, r *http.Request) {
	s := loadSetupState()
	m := app.localPreviewManifest(s)
	relayInfo := render.RelayInfo{
		Hostname: s.Hostname, City: s.City, Region: s.Region, CC: s.CC, Lat: s.Lat, Lon: s.Lon,
	}
	if relayInfo.City == "" { relayInfo.City = "Your City" }
	if relayInfo.Region == "" { relayInfo.Region = "Region" }
	if relayInfo.CC == "" { relayInfo.CC = "US" }
	// local stamp if present, else default
	if b, err := os.ReadFile(filepath.Join(relayDataDir(), "stamp.svg")); err == nil {
		relayInfo.StampSVG = template.HTML(string(b))
	} else if b, err := os.ReadFile(filepath.Join(relayDataDir(), "stamp.png")); err == nil {
		relayInfo.StampSVG = template.HTML(fmt.Sprintf(`<img src="data:image/png;base64,%s" alt="Stamp">`, base64.StdEncoding.EncodeToString(b)))
	} else {
		relayInfo.StampSVG = template.HTML(render.DefaultStampSVG(relayInfo.City, relayInfo.Region, relayInfo.CC))
	}
	html, err := render.RenderPage(m, relayInfo)
	if err != nil { http.Error(w, err.Error(), 500); return }
	// rewrite image URLs to local preview
	out := strings.ReplaceAll(string(html), "/open-post-offices/"+s.Hostname+"/images/", "/preview/images/")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(out))
}

func (app *relayApp) setupPreviewImageHandler(w http.ResponseWriter, r *http.Request) {
	name := filepath.Base(r.PathValue("name"))
	p := filepath.Join(relayImagesDir(), name)
	b, err := os.ReadFile(p)
	if err != nil { http.NotFound(w, r); return }
	ct := "application/octet-stream"
	ln := strings.ToLower(name)
	switch { case strings.HasSuffix(ln,".png"): ct="image/png"; case strings.HasSuffix(ln,".jpg"),strings.HasSuffix(ln,".jpeg"): ct="image/jpeg"; case strings.HasSuffix(ln,".webp"): ct="image/webp"; case strings.HasSuffix(ln,".svg"): ct="image/svg+xml" }
	w.Header().Set("Content-Type", ct); _, _ = w.Write(b)
}

func (app *relayApp) setupCopyCentralHandler(w http.ResponseWriter, r *http.Request) {
	s := loadSetupState()
	if s.MainSite == "" { s.MainSite = "https://snaelmail.org" }
	// fetch central page manifest
	client := &http.Client{Timeout: 20 * time.Second}
	req, _ := http.NewRequestWithContext(r.Context(), http.MethodGet, strings.TrimRight(s.MainSite, "/")+"/api/public/post-office/central.snaelmail.org/page", nil)
	resp, err := client.Do(req)
	if err != nil { writeJSON(w, 502, map[string]string{"error": err.Error()}); return }
	defer resp.Body.Close()
	rb, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 300 { w.Header().Set("Content-Type","application/json"); w.WriteHeader(resp.StatusCode); _,_ = w.Write(rb); return }
	var data struct {
		Page struct { Manifest page.Manifest `json:"Manifest"` } `json:"page"`
	}
	// also try lowercase manifest
	var raw map[string]any
	_ = json.Unmarshal(rb, &raw)
	_ = json.Unmarshal(rb, &data)
	m := data.Page.Manifest
	if len(m.Photos) == 0 && m.About == "" {
		// try alternate shape
		if p, ok := raw["page"].(map[string]any); ok {
			if mm, ok := p["manifest"].(map[string]any); ok {
				mb, _ := json.Marshal(mm); _ = json.Unmarshal(mb, &m)
			} else if mm, ok := p["Manifest"].(map[string]any); ok {
				mb, _ := json.Marshal(mm); _ = json.Unmarshal(mb, &m)
			}
		}
	}
	// download images
	if err := os.MkdirAll(relayImagesDir(), 0755); err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
	for _, ph := range m.Photos {
		imgURL := strings.TrimRight(s.MainSite, "/") + "/open-post-offices/central.snaelmail.org/images/" + ph.Filename
		ir, err := client.Get(imgURL)
		if err != nil { continue }
		ib, _ := io.ReadAll(io.LimitReader(ir.Body, 10<<20))
		ir.Body.Close()
		if ir.StatusCode == 200 && len(ib) > 0 {
			_ = os.WriteFile(filepath.Join(relayImagesDir(), filepath.Base(ph.Filename)), ib, 0644)
		}
	}
	// save copied manifest info to a local file for reference
	mb, _ := json.MarshalIndent(m, "", "  ")
	_ = os.WriteFile(filepath.Join(relayDataDir(), "central-template.json"), mb, 0600)
	writeJSON(w, http.StatusOK, map[string]any{"status":"ok","copied_photos":len(m.Photos),"about":m.About})
}

func (app *relayApp) setupUploadStampHandler(w http.ResponseWriter, r *http.Request) {
	var req struct { StampSVG string `json:"stamp_svg"`; StampPNGBase64 string `json:"stamp_png_base64"` }
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil { writeJSON(w, 400, map[string]string{"error":"invalid JSON"}); return }
	if err := os.MkdirAll(relayDataDir(), 0700); err != nil { writeJSON(w, 500, map[string]string{"error":err.Error()}); return }
	if req.StampSVG != "" {
		if err := os.WriteFile(filepath.Join(relayDataDir(), "stamp.svg"), []byte(req.StampSVG), 0644); err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
		_ = os.Remove(filepath.Join(relayDataDir(), "stamp.png"))
	}
	if req.StampPNGBase64 != "" {
		b, err := base64.StdEncoding.DecodeString(req.StampPNGBase64)
		if err != nil { writeJSON(w,400,map[string]string{"error":"invalid stamp_png_base64"}); return }
		if err := os.WriteFile(filepath.Join(relayDataDir(), "stamp.png"), b, 0644); err != nil { writeJSON(w,500,map[string]string{"error":err.Error()}); return }
		if req.StampSVG == "" { _ = os.Remove(filepath.Join(relayDataDir(), "stamp.svg")) }
	}
	if req.StampSVG == "" && req.StampPNGBase64 == "" { writeJSON(w,400,map[string]string{"error":"no stamp data"}); return }
	writeJSON(w, 200, map[string]string{"status":"ok"})
}
