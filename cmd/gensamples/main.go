// Command gensamples writes synthetic artwork files plus labels.json.
//
// The labels are what a human designer would decide for each file, not what
// the rules would output. Some files are built to trip the rules on purpose,
// so the eval shows real misses instead of a perfect score.
package main

import (
	"encoding/json"
	"flag"
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
	"path/filepath"

	"proofcheck/internal/proof"
)

type label struct {
	proof.Order
	Expected []proof.Issue `json:"expected"`
	Why      string        `json:"why"`
}

var (
	ink   = color.NRGBA{20, 90, 160, 255}
	white = color.NRGBA{255, 255, 255, 255}
)

func canvas(w, h int) *image.NRGBA { return image.NewNRGBA(image.Rect(0, 0, w, h)) }

func fill(img *image.NRGBA, c color.NRGBA) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.SetNRGBA(x, y, c)
		}
	}
}

func circle(img *image.NRGBA, cx, cy, r int, c color.NRGBA) {
	for y := cy - r; y <= cy+r; y++ {
		for x := cx - r; x <= cx+r; x++ {
			if (x-cx)*(x-cx)+(y-cy)*(y-cy) <= r*r && image.Pt(x, y).In(img.Bounds()) {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

func rect(img *image.NRGBA, x0, y0, x1, y1 int, c color.NRGBA) {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if image.Pt(x, y).In(img.Bounds()) {
				img.SetNRGBA(x, y, c)
			}
		}
	}
}

// centered draws a circle that leaves a comfortable margin.
func centered(w, h int) *image.NRGBA {
	img := canvas(w, h)
	r := min(w, h) * 4 / 10
	circle(img, w/2, h/2, r, ink)
	return img
}

func main() {
	dir := flag.String("out", "samples", "output directory")
	flag.Parse()
	if err := os.MkdirAll(*dir, 0o755); err != nil {
		log.Fatal(err)
	}

	var labels []label
	add := func(name string, img *image.NRGBA, wIn, hIn float64, cut string, why string, expected ...proof.Issue) {
		f, err := os.Create(filepath.Join(*dir, name))
		if err != nil {
			log.Fatal(err)
		}
		if err := png.Encode(f, img); err != nil {
			log.Fatal(err)
		}
		f.Close()
		if expected == nil {
			expected = []proof.Issue{}
		}
		labels = append(labels, label{
			Order:    proof.Order{File: name, WidthIn: wIn, HeightIn: hIn, Cut: cut},
			Expected: expected, Why: why,
		})
	}

	add("01_clean_diecut.png", centered(900, 900), 3, 3, "die_cut", "300 DPI, transparent, centered")
	add("02_lowres_diecut.png", centered(450, 450), 3, 3, "die_cut", "150 DPI will print soft", proof.LowResolution)

	img := canvas(900, 900)
	circle(img, 450, 450, 448, ink)
	add("03_art_touches_edge.png", img, 3, 3, "die_cut", "circle runs into the cut line", proof.UnsafeMargin)

	img = canvas(900, 900)
	fill(img, white)
	circle(img, 450, 450, 300, ink)
	add("04_white_box.png", img, 3, 3, "die_cut", "white box will print around the design", proof.OpaqueBackground)

	img = canvas(300, 300)
	circle(img, 150, 150, 149, ink)
	add("05_lowres_and_edge.png", img, 2, 2, "die_cut", "150 DPI and art at the edge", proof.LowResolution, proof.UnsafeMargin)

	img = canvas(900, 600)
	fill(img, ink)
	add("06_square_full_bleed.png", img, 3, 2, "square", "full bleed is fine for square cut")

	img = canvas(450, 300)
	fill(img, ink)
	add("07_square_lowres.png", img, 3, 2, "square", "150 DPI", proof.LowResolution)

	// Faint glow reaching the edge. Invisible once printed, so a designer
	// would pass it. A zero alpha threshold will flag it.
	img = canvas(900, 900)
	circle(img, 450, 450, 449, color.NRGBA{20, 90, 160, 6})
	circle(img, 450, 450, 320, ink)
	add("08_faint_glow.png", img, 3, 3, "die_cut", "glow at 2% opacity will not show in print")

	// Soft shadow box at 78% opacity. A designer sees a visible box, but the
	// edge pixels are below the 250 alpha cutoff.
	img = canvas(900, 900)
	fill(img, color.NRGBA{0, 0, 0, 200})
	circle(img, 450, 450, 300, ink)
	add("09_shadow_box.png", img, 3, 3, "die_cut", "semi-transparent box still prints as a box", proof.OpaqueBackground)

	add("10_280dpi.png", centered(840, 840), 3, 3, "die_cut", "280 DPI looks fine on a 3in sticker")
	add("11_exact_300dpi.png", centered(1200, 1200), 4, 4, "die_cut", "exactly 300 DPI")
	add("12_tall_lowres.png", centered(400, 800), 2, 4, "die_cut", "200 DPI", proof.LowResolution)

	img = centered(900, 900)
	rect(img, 440, 4, 460, 12, ink)
	add("13_small_mark_at_top.png", img, 3, 3, "die_cut", "small mark will be cut through", proof.UnsafeMargin)

	img = canvas(600, 900)
	rect(img, 60, 60, 540, 840, ink)
	add("14_clean_rectangle.png", img, 2, 3, "die_cut", "rectangle with safe margin")

	img = canvas(900, 900)
	fill(img, color.NRGBA{250, 200, 40, 255})
	circle(img, 450, 450, 300, ink)
	add("15_color_box.png", img, 3, 3, "die_cut", "yellow box around the design", proof.OpaqueBackground)

	out, err := json.MarshalIndent(labels, "", "  ")
	if err != nil {
		log.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(*dir, "labels.json"), out, 0o644); err != nil {
		log.Fatal(err)
	}
	log.Printf("wrote %d samples to %s", len(labels), *dir)
}
