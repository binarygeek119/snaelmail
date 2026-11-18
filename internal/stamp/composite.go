// Package stamp/composite builds the composite postmark PNG image.
//
// Canvas: 320x400, up to 6 stamp positions. Each position shows the relay's
// plate PNG, plus date, city, and hop index. Rendering is deterministic:
// same inputs produce byte-identical PNG output.
package stamp

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strings"
	"time"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

// DefaultCanvas is the composite size per §4.
var DefaultCanvas = image.Rect(0, 0, 320, 400)

// DefaultPositions are the slot coordinates per §4.
var DefaultPositions = [][2]int{
	{18, 16}, {18, 120}, {18, 224}, {18, 300}, {120, 16}, {120, 120},
}

// Slot describes a single postmark slot in the composite.
type Slot struct {
	// StampPNG is the relay's plate PNG (already rasterized from SVG).
	StampPNG []byte
	// Date is the stamp_at.
	Date time.Time
	// City is the post office city name.
	City string
	// Idx is the hop index (0-based).
	Idx int
	// Total is the total number of hops.
	Total int
	// Verified is false when the stamp failed verification — renders gray.
	Verified bool
}

// Composite renders a composite image with up to len(DefaultPositions) slots.
// Extra slots beyond 6 are silently ignored.
func Composite(slots []Slot) ([]byte, error) {
	img := image.NewRGBA(DefaultCanvas)

	// Background: off-white paper color
	paper := color.RGBA{250, 248, 242, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{paper}, image.Point{}, draw.Src)

	// Draw border
	borderColor := color.RGBA{180, 170, 150, 255}
	drawBorder(img, img.Bounds(), borderColor, 2)

	for i, slot := range slots {
		if i >= len(DefaultPositions) {
			break
		}
		x, y := DefaultPositions[i][0], DefaultPositions[i][1]
		drawSlot(img, x, y, slot)
	}

	var buf bytes.Buffer
	enc := &png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// drawSlot renders a single postmark slot: the stamp image, then text below it.
func drawSlot(dst draw.Image, x, y int, slot Slot) {
	// Decode stamp PNG
	stampImg, err := png.Decode(bytes.NewReader(slot.StampPNG))
	if err != nil {
		// If stamp is invalid, draw a gray placeholder
		drawGrayBox(dst, x, y, 80, 80)
		drawText(dst, x, y+85, "INVALID", true)
		return
	}

	// If not verified, desaturate to gray
	if !slot.Verified {
		stampImg = desaturate(stampImg)
	}

	// Draw stamp
	bounds := stampImg.Bounds()
	draw.Draw(dst, image.Rect(x, y, x+bounds.Dx(), y+bounds.Dy()), stampImg, bounds.Min, draw.Over)

	// Draw text below stamp
	textColor := !slot.Verified
	textY := y + bounds.Dy() + 4

	// Date line: "18 Nov 2026"
	dateStr := slot.Date.UTC().Format("02 Jan 2006")
	drawText(dst, x, textY, dateStr, textColor)

	// City line
	drawText(dst, x, textY+12, slot.City, textColor)

	// Index line: "2/6"
	idxStr := strings.Join([]string{intToStr(slot.Idx + 1), "/", intToStr(slot.Total)}, "")
	drawText(dst, x, textY+24, idxStr, textColor)
}

func intToStr(n int) string {
	if n < 10 {
		return string(rune('0' + n))
	}
	return intToStr(n/10) + string(rune('0'+n%10))
}

// drawText renders a single line of text using the basic bitmap font.
func drawText(dst draw.Image, x, y int, text string, gray bool) {
	ink := color.RGBA{40, 40, 40, 255}
	if gray {
		ink = color.RGBA{160, 160, 160, 255}
	}
	d := &font.Drawer{
		Dst:  dst,
		Src:  &image.Uniform{ink},
		Face: basicfont.Face7x13,
		Dot:  fixed.Point26_6{X: fixed.I(x), Y: fixed.I(y + 11)},
	}
	d.DrawString(text)
}

func drawBorder(dst draw.Image, rect image.Rectangle, c color.Color, thickness int) {
	for i := 0; i < thickness; i++ {
		r := rect.Inset(i)
		// Top and bottom
		for x := r.Min.X; x < r.Max.X; x++ {
			dst.Set(x, r.Min.Y, c)
			dst.Set(x, r.Max.Y-1, c)
		}
		// Left and right
		for y := r.Min.Y; y < r.Max.Y; y++ {
			dst.Set(r.Min.X, y, c)
			dst.Set(r.Max.X-1, y, c)
		}
	}
}

func drawGrayBox(dst draw.Image, x, y, w, h int) {
	gray := color.RGBA{200, 200, 200, 255}
	draw.Draw(dst, image.Rect(x, y, x+w, y+h), &image.Uniform{gray}, image.Point{}, draw.Src)
}

// desaturate converts an image to grayscale.
func desaturate(src image.Image) *image.Gray {
	bounds := src.Bounds()
	gray := image.NewGray(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := src.At(x, y).RGBA()
			// Luma: 0.299*R + 0.587*G + 0.114*B
			lum := uint16(0.299*float64(r>>8) + 0.587*float64(g>>8) + 0.114*float64(b>>8))
			gray.SetGray(x, y, color.Gray{Y: uint8(lum >> 8)})
		}
	}
	return gray
}

// MakeStubStampPNG creates a minimal 80x80 PNG for testing. Deterministic.
func MakeStubStampPNG(label string) []byte {
	img := image.NewRGBA(image.Rect(0, 0, 80, 80))
	// Fill with a fixed color
	base := color.RGBA{200, 60, 60, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{base}, image.Point{}, draw.Src)
	// Draw a border
	drawBorder(img, img.Bounds(), color.RGBA{80, 20, 20, 255}, 2)
	// Draw label
	drawText(img, 4, 35, label, false)
	var buf bytes.Buffer
	png.Encode(&buf, img)
	return buf.Bytes()
}
