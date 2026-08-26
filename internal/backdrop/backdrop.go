// Package backdrop measures a background image and says how much of it
// has to be shaded for the text to stay readable.
//
// It advises, it does not decide. Where the text lands on the picture
// depends on the window: the image is scaled to cover, so a wide screen
// crops the top and bottom and a tall one crops the sides. A number
// picked from the whole image is therefore an estimate, and the operator
// is the one looking at the result.
package backdrop

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"image"
	"image/color"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"math"
	"os"
	"sort"
)

// safeLuminance is where the shaded picture has to land for the quiet
// inks to clear 4.5:1 over it. Calibrated against a rendered page rather
// than derived: the shade is a darken blend, and the text carries a
// shadow — both help by an amount no formula predicts. The number comes
// from measuring a page whose worst heading landed at 4.7:1.
const safeLuminance = 0.18

// samples is how many pixels are looked at. A background is thousands of
// times larger than the answer needs.
const samples = 20000

// Recommend returns the shade a picture needs, between 0 and 0.95.
func Recommend(path string) (float64, error) {
	f, err := os.Open(path)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return 0, fmt.Errorf("%s: %w", path, err)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w == 0 || h == 0 {
		return 0, fmt.Errorf("%s: empty image", path)
	}
	step := 1
	if w*h > samples {
		step = int(float64(w*h)/float64(samples)) / 4
		if step < 1 {
			step = 1
		}
	}
	var ls []float64
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bl, _ := img.At(x, y).RGBA()
			ls = append(ls, luminance(float64(r)/65535, float64(g)/65535, float64(bl)/65535))
		}
	}
	if len(ls) == 0 {
		return 0, fmt.Errorf("%s: nothing to measure", path)
	}
	sort.Float64s(ls)
	// The bright tenth, not the mean: the text has to survive the
	// lightest part of the picture, not its average.
	bright := ls[len(ls)*9/10]
	if bright <= safeLuminance {
		return 0, nil
	}
	dim := 1 - safeLuminance/bright
	if dim > 0.95 {
		dim = 0.95
	}
	return dim, nil
}

// Average is the picture's mean colour as a CSS hex string. It is what
// the page is painted with before anything has decoded: black behind a
// photograph is a flash, the photograph's own average is not.
func Average(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	step := b.Dx() / 64
	if step < 1 {
		step = 1
	}
	var rs, gs, bs, n uint64
	for y := b.Min.Y; y < b.Max.Y; y += step {
		for x := b.Min.X; x < b.Max.X; x += step {
			r, g, bb, _ := img.At(x, y).RGBA()
			rs += uint64(r >> 8)
			gs += uint64(g >> 8)
			bs += uint64(bb >> 8)
			n++
		}
	}
	if n == 0 {
		return "", fmt.Errorf("%s: nothing to measure", path)
	}
	return fmt.Sprintf("#%02x%02x%02x", rs/n, gs/n, bs/n), nil
}

// Placeholder is the picture at the size of a postage stamp, as a data
// URI. It goes underneath the real background so the first paint is the
// picture, blurred, instead of a flat colour that jumps to a photograph
// a moment later.
func Placeholder(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return "", err
	}
	b := img.Bounds()
	const width = 32
	height := width * b.Dy() / b.Dx()
	if height < 1 {
		height = 1
	}
	small := image.NewRGBA(image.Rect(0, 0, width, height))
	// A box average, not a resampler: at this size the result is a blur
	// either way, and the blur is the point.
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			var rs, gs, bs, n uint64
			x0, x1 := b.Min.X+x*b.Dx()/width, b.Min.X+(x+1)*b.Dx()/width
			y0, y1 := b.Min.Y+y*b.Dy()/height, b.Min.Y+(y+1)*b.Dy()/height
			for sy := y0; sy < y1; sy++ {
				for sx := x0; sx < x1; sx++ {
					r, g, bb, _ := img.At(sx, sy).RGBA()
					rs += uint64(r >> 8)
					gs += uint64(g >> 8)
					bs += uint64(bb >> 8)
					n++
				}
			}
			if n == 0 {
				continue
			}
			small.Set(x, y, color.RGBA{uint8(rs / n), uint8(gs / n), uint8(bs / n), 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, small, &jpeg.Options{Quality: 60}); err != nil {
		return "", err
	}
	return "data:image/jpeg;base64," + base64.StdEncoding.EncodeToString(buf.Bytes()), nil
}

func luminance(r, g, b float64) float64 {
	f := func(c float64) float64 {
		if c <= 0.03928 {
			return c / 12.92
		}
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return 0.2126*f(r) + 0.7152*f(g) + 0.0722*f(b)
}
