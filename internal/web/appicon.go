package web

import (
	"bytes"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"sync"
)

// The page's own motif drawn as an app icon: a few rows on the same warm
// near-black the page uses. It is generated rather than shipped so there
// is no binary asset in the repository and no second place to keep the
// palette.
var (
	iconOnce sync.Once
	iconPNG  = map[int][]byte{}
)

// FaviconSVG is the same motif as a scalable icon: four rows, the first
// one shorter and quieter, drawn in the page's own inks. An SVG because
// a tab icon is 16 CSS pixels on one screen and 32 on the next, and a
// bitmap has to pick one.
const FaviconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 32 32">` +
	`<rect width="32" height="32" rx="7" fill="#141312"/>` +
	`<g fill="#8a8478"><rect x="7" y="9" width="4" height="2.6" rx="1"/><rect x="13" y="9" width="12" height="2.6" rx="1.3"/></g>` +
	`<g fill="#e8e4da">` +
	`<rect x="7" y="14.2" width="4" height="2.6" rx="1"/><rect x="13" y="14.2" width="9" height="2.6" rx="1.3"/>` +
	`<rect x="7" y="19.4" width="4" height="2.6" rx="1"/><rect x="13" y="19.4" width="12" height="2.6" rx="1.3"/>` +
	`</g></svg>`

// AppIcon returns a square PNG of the given size for the web app manifest
// and for the icon a phone puts on its home screen.
func AppIcon(size int) []byte {
	iconOnce.Do(func() {
		for _, s := range []int{192, 512} {
			iconPNG[s] = drawIcon(s)
		}
	})
	return iconPNG[size]
}

func drawIcon(size int) []byte {
	bg := color.RGBA{0x14, 0x13, 0x12, 0xff}
	ink := color.RGBA{0xe8, 0xe4, 0xda, 0xff}
	dim := color.RGBA{0x8a, 0x84, 0x78, 0xff}

	img := image.NewRGBA(image.Rect(0, 0, size, size))
	draw.Draw(img, img.Bounds(), &image.Uniform{bg}, image.Point{}, draw.Src)

	unit := size / 16
	rows := []struct {
		y, w int
		c    color.RGBA
	}{
		{4, 7, dim}, // a heading, shorter and quieter
		{6, 9, ink},
		{8, 8, ink},
		{10, 10, ink},
	}
	for _, r := range rows {
		// A square for the icon, then the line of text beside it: the row
		// of the page, at the size of a favicon.
		fill(img, unit*3, unit*r.y, unit*2, unit, r.c)
		fill(img, unit*6, unit*r.y, unit*r.w, unit, r.c)
	}

	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil
	}
	return buf.Bytes()
}

func fill(img *image.RGBA, x, y, w, h int, c color.RGBA) {
	draw.Draw(img, image.Rect(x, y, x+w, y+h), &image.Uniform{c}, image.Point{}, draw.Src)
}
