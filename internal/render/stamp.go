package render

import (
	"fmt"
	"html"
	"math"
	"strings"
)

// DefaultStampSVG returns a vintage-style post office stamp template with local info.
func DefaultStampSVG(city, region, cc string) string {
	city = strings.TrimSpace(city)
	region = strings.TrimSpace(region)
	cc = strings.TrimSpace(cc)
	if city == "" { city = "Snaelmail" }
	loc := strings.TrimSpace(strings.Join([]string{region, cc}, " "))
	if loc == "" { loc = "Post Office" }
	cityE := html.EscapeString(strings.ToUpper(city))
	locE := html.EscapeString(strings.ToUpper(loc))

	// serrated outer edge
	points := make([]string, 0, 120)
	outerR, innerR := 56.0, 52.0
	teeth := 48
	for i := 0; i < teeth*2; i++ {
		ang := (float64(i) / float64(teeth*2)) * 2 * math.Pi
		r := outerR
		if i%2 == 1 { r = innerR }
		x := 60 + r*math.Cos(ang-math.Pi/2)
		y := 60 + r*math.Sin(ang-math.Pi/2)
		points = append(points, fmt.Sprintf("%.2f,%.2f", x, y))
	}
	edge := strings.Join(points, " ")

	return fmt.Sprintf(`<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 120 120'>
  <defs>
    <filter id='ink' x='-5%%' y='-5%%' width='110%%' height='110%%'>
      <feTurbulence type='fractalNoise' baseFrequency='0.8' numOctaves='1' result='noise'/>
      <feDisplacementMap in='SourceGraphic' in2='noise' scale='0.6'/>
    </filter>
  </defs>
  <g filter='url(#ink)' fill='none' stroke='#4b3f72' stroke-linecap='round'>
    <polygon points='%s' fill='#f8f5ef' stroke='#4b3f72' stroke-width='1.5'/>
    <circle cx='60' cy='60' r='45' stroke-width='2.2'/>
    <circle cx='60' cy='60' r='41' stroke-width='1'/>
    <path d='M20 60 L100 60' stroke-width='0.8' opacity='0.35'/>
    <text x='60' y='38' text-anchor='middle' font-size='7.5' font-family='Georgia,serif' fill='#4b3f72' stroke='none' font-weight='700' letter-spacing='1'>SNAELMAIL</text>
    <text x='60' y='54' text-anchor='middle' font-size='8.5' font-family='Georgia,serif' fill='#3b2d5c' stroke='none' font-weight='700'>%s</text>
    <text x='60' y='66' text-anchor='middle' font-size='5.8' font-family='Georgia,serif' fill='#4b3f72' stroke='none'>%s</text>
    <text x='60' y='78' text-anchor='middle' font-size='6.5' font-family='Georgia,serif' fill='#4b3f72' stroke='none' font-weight='700' letter-spacing='0.5'>POST OFFICE</text>
  </g>
</svg>`, edge, cityE, locE)
}
