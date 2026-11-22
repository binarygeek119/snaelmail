// Package render implements page rendering from manifests.
package render

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"snailmail/internal/page"
)

// RelayInfo is the relay metadata passed to the template.
type RelayInfo struct {
	Hostname     string
	City         string
	Region       string
	CC           string
	Lat          float64
	Lon          float64
	OpenedAt     *time.Time
	LettersCount int64
	StampSVG     template.HTML
}

// RenderContext is the data passed to the page template.
type RenderContext struct {
	Relay    RelayInfo
	Manifest page.Manifest
	Pages    []int // photo ordinals
}

// pageTemplate is the HTML template for public pages.
const pageTemplate = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>{{.Relay.City}} Post Office</title>
  <style>
    :root {
      --primary: {{.Manifest.Accent.Primary}};
      --secondary: {{.Manifest.Accent.Secondary}};
      --tertiary: {{.Manifest.Accent.Tertiary}};
      --highlight: {{.Manifest.Accent.Highlight}};
      --muted: {{.Manifest.Accent.Muted}};
    }
    body {
      font-family: system-ui, -apple-system, sans-serif;
      line-height: 1.6;
      color: var(--primary);
      max-width: 1200px;
      margin: 0 auto;
      padding: 2rem;
    }
    header {
      border-bottom: 3px solid var(--secondary);
      padding-bottom: 1rem;
      margin-bottom: 2rem;
    }
    h1 {
      color: var(--secondary);
      margin: 0;
    }
    .meta {
      color: var(--muted);
      font-size: 0.9rem;
    }
    .stamp {
      margin: 1.5rem 0;
      display: flex;
      align-items: center;
      gap: 1rem;
      flex-wrap: wrap;
    }
    .stamp-mark {
      width: 140px;
      height: 140px;
      border: 2px solid var(--muted);
      border-radius: 12px;
      display: flex;
      align-items: center;
      justify-content: center;
      background: white;
      padding: 8px;
    }
    .stamp-mark svg, .stamp-mark img { max-width: 100%; max-height: 100%; }
    .photos {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(300px, 1fr));
      gap: 1rem;
      margin: 2rem 0;
    }
    .photo {
      position: relative;
      aspect-ratio: 16/10;
      overflow: hidden;
      border-radius: 4px;
    }
    .photo img {
      width: 100%;
      height: 100%;
      object-fit: cover;
    }
    .caption {
      position: absolute;
      bottom: 0;
      left: 0;
      right: 0;
      background: rgba(0,0,0,0.7);
      color: white;
      padding: 0.5rem;
      font-size: 0.9rem;
    }
    .about {
      background: var(--highlight);
      padding: 1.5rem;
      border-radius: 4px;
      margin: 2rem 0;
    }
    .facts {
      display: grid;
      grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
      gap: 1rem;
      margin: 2rem 0;
    }
    .fact {
      background: var(--muted);
      padding: 1rem;
      border-radius: 4px;
    }
    .fact-key {
      font-weight: bold;
      color: var(--secondary);
    }
    .postmaster {
      background: var(--secondary);
      color: white;
      padding: 1.5rem;
      border-radius: 4px;
      margin: 2rem 0;
    }
    .dispatch {
      background: var(--tertiary);
      padding: 1rem;
      border-radius: 4px;
      margin: 1rem 0;
    }
    footer {
      margin-top: 3rem;
      padding-top: 1rem;
      border-top: 1px solid var(--muted);
      font-size: 0.9rem;
      color: var(--muted);
    }
  </style>
</head>
<body>
  <header>
    <h1>{{.Relay.City}} Post Office</h1>
    <div class="meta">
      {{.Relay.Region}}, {{.Relay.CC}} · 
      {{if .Relay.OpenedAt}}Established {{formatDate .Relay.OpenedAt}}{{else}}Coming soon{{end}}
      · {{.Relay.LettersCount}} letters handled
    </div>
  </header>

  {{if .Relay.StampSVG}}
  <section class="stamp">
    <div class="stamp-mark">{{.Relay.StampSVG}}</div>
    <div>
      <h2 style="margin:0;color:var(--secondary);">Post Office Stamp</h2>
      <p class="meta" style="margin:.25rem 0 0 0;">Official stamp of {{.Relay.City}} Post Office</p>
    </div>
  </section>
  {{end}}

  <section class="photos">
    {{range .Manifest.Photos}}
    <div class="photo">
      <img src="/open-post-offices/{{$.Relay.Hostname}}/images/{{.Filename}}" alt="{{.Caption}}">
      {{if .Caption}}<div class="caption">{{.Caption}}</div>{{end}}
    </div>
    {{end}}
  </section>

  <section class="about">
    <h2>About {{.Relay.City}}</h2>
    <p>{{.Manifest.About}}</p>
  </section>

  {{if .Manifest.Facts}}
  <section class="facts">
    {{range .Manifest.Facts}}
    <div class="fact">
      <div class="fact-key">{{.Key}}</div>
      <div>{{.Value}}</div>
    </div>
    {{end}}
  </section>
  {{end}}

  <section class="postmaster">
    <h2>Postmaster: {{.Manifest.Postmaster.Name}}</h2>
    {{if .Manifest.Postmaster.Role}}<p><em>{{.Manifest.Postmaster.Role}}</em></p>{{end}}
    {{if .Manifest.Postmaster.Bio}}<p>{{.Manifest.Postmaster.Bio}}</p>{{end}}
  </section>

  {{if .Manifest.DispatchTimes}}
  <section>
    <h2>Dispatch Times</h2>
    {{range .Manifest.DispatchTimes}}
    <div class="dispatch">{{.}}</div>
    {{end}}
  </section>
  {{end}}

  {{if .Manifest.Classes}}
  <section>
    <h2>Classes Handled</h2>
    <ul>
      {{range .Manifest.Classes}}<li>{{.}}</li>{{end}}
    </ul>
  </section>
  {{end}}

  <footer>
    <p>
      <strong>{{.Relay.Hostname}}</strong><br>
      Coordinates: {{.Relay.Lat}}, {{.Relay.Lon}}<br>
      {{if .Relay.OpenedAt}}Open since {{formatDate .Relay.OpenedAt}}{{end}}
    </p>
  </footer>
</body>
</html>`

// templateFuncs are the functions available in templates.
var templateFuncs = template.FuncMap{
	"formatDate": func(t *time.Time) string {
		if t == nil {
			return "unknown"
		}
		return t.Format("2 January 2006")
	},
}

// RenderPage renders a page from a manifest and relay info.
func RenderPage(m page.Manifest, relay RelayInfo) ([]byte, error) {
	tmpl, err := template.New("page").Funcs(templateFuncs).Parse(pageTemplate)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}

	ctx := RenderContext{
		Relay:    relay,
		Manifest: m,
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, ctx); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}

	return buf.Bytes(), nil
}

// RenderFallback renders a minimal page from relay registration data (no manifest).
func RenderFallback(relay RelayInfo) ([]byte, error) {
	// Create a minimal manifest with just the essentials
	m := page.Manifest{
		About: fmt.Sprintf("Welcome to the %s post office. This branch is currently being set up and will open soon.", relay.City),
		Postmaster: page.Postmaster{
			Name: "Pending Assignment",
		},
		Accent: page.Accent{
			Primary:   "#333333",
			Secondary: "#666666",
			Tertiary:  "#999999",
			Highlight: "#f0f0f0",
			Muted:     "#cccccc",
		},
	}

	return RenderPage(m, relay)
}

// RenderFromPage renders a page from a Page object.
func RenderFromPage(p *page.Page, relay RelayInfo) ([]byte, error) {
	return RenderPage(p.Manifest, relay)
}
