package render

import (
	"fmt"
	"html"
	"strings"
)

// DefaultStampSVG returns a basic post office stamp template with local info.
func DefaultStampSVG(city, region, cc string) string {
	city = strings.TrimSpace(city)
	region = strings.TrimSpace(region)
	cc = strings.TrimSpace(cc)
	if city == "" { city = "Snaelmail" }
	loc := strings.TrimSpace(strings.Join([]string{region, cc}, " "))
	if loc == "" { loc = "Post Office" }
	cityE := html.EscapeString(city)
	locE := html.EscapeString(loc)
	return fmt.Sprintf(`<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 120 120'>
  <defs><linearGradient id='g' x1='0' y1='0' x2='1' y2='1'><stop offset='0%%' stop-color='#667eea'/><stop offset='100%%' stop-color='#764ba2'/></linearGradient></defs>
  <circle cx='60' cy='60' r='54' fill='none' stroke='url(#g)' stroke-width='6'/>
  <circle cx='60' cy='60' r='46' fill='none' stroke='#764ba2' stroke-width='2'/>
  <text x='60' y='44' text-anchor='middle' font-size='10' font-family='Georgia,serif' fill='#3b2d5c' font-weight='700'>%s</text>
  <text x='60' y='58' text-anchor='middle' font-size='8' font-family='Georgia,serif' fill='#5b4b7a'>%s</text>
  <text x='60' y='72' text-anchor='middle' font-size='8' font-family='Georgia,serif' fill='#5b4b7a'>POST OFFICE</text>
  <text x='60' y='84' text-anchor='middle' font-size='7' font-family='Georgia,serif' fill='#5b4b7a'>SNAELMAIL</text>
</svg>`, cityE, locE)
}
