package proof

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"math"
)

// RuleChecker applies fixed print rules. It is fast and cheap, and it sets the
// baseline that any model backed checker has to beat.
type RuleChecker struct {
	MinDPI         float64 // below this, flag low resolution
	SafeMarginIn   float64 // die cut content must stay this far from the edge
	AlphaThreshold uint8   // pixels at or below this alpha count as empty
	OpaqueShare    float64 // share of solid edge pixels needed to call the background opaque
}

func (c RuleChecker) Name() string {
	return fmt.Sprintf("rules(min_dpi=%.0f, alpha>%d)", c.MinDPI, c.AlphaThreshold)
}

func (c RuleChecker) Check(_ context.Context, img image.Image, o Order) (Result, error) {
	var r Result
	if o.WidthIn <= 0 || o.HeightIn <= 0 {
		return r, fmt.Errorf("%s: print size must be positive", o.File)
	}
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()

	dpi := math.Min(float64(w)/o.WidthIn, float64(h)/o.HeightIn)
	if dpi < c.MinDPI {
		r.add(LowResolution, fmt.Sprintf("%.0f DPI at %.1fx%.1f in, needs %.0f", dpi, o.WidthIn, o.HeightIn, c.MinDPI))
	}

	// Margin and background rules only matter when we cut around the artwork.
	if o.Cut != "die_cut" {
		return r, nil
	}

	if share := solidEdgeShare(img); share >= c.OpaqueShare {
		r.add(OpaqueBackground, fmt.Sprintf("%.0f%% of edge pixels are solid", share*100))
		// A solid background fills the whole margin, so a margin warning
		// would only repeat the same problem.
		return r, nil
	}

	margin := int(math.Ceil(c.SafeMarginIn * dpi))
	if x, y, ok := firstInkInMargin(img, margin, c.AlphaThreshold); ok {
		r.add(UnsafeMargin, fmt.Sprintf("ink at (%d,%d), inside the %dpx safe margin", x, y, margin))
	}
	return r, nil
}

func (r *Result) add(i Issue, note string) {
	r.Issues = append(r.Issues, i)
	r.Notes = append(r.Notes, string(i)+": "+note)
}

func alphaAt(img image.Image, x, y int) uint8 {
	return color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA).A
}

// solidEdgeShare returns the share of pixels on the outer 1px ring that are
// close to fully opaque.
func solidEdgeShare(img image.Image) float64 {
	b := img.Bounds()
	total, solid := 0, 0
	count := func(x, y int) {
		total++
		if alphaAt(img, x, y) >= 250 {
			solid++
		}
	}
	for x := b.Min.X; x < b.Max.X; x++ {
		count(x, b.Min.Y)
		count(x, b.Max.Y-1)
	}
	for y := b.Min.Y + 1; y < b.Max.Y-1; y++ {
		count(b.Min.X, y)
		count(b.Max.X-1, y)
	}
	if total == 0 {
		return 0
	}
	return float64(solid) / float64(total)
}

// firstInkInMargin scans only the margin band and returns the first pixel whose
// alpha is above the threshold.
func firstInkInMargin(img image.Image, m int, threshold uint8) (int, int, bool) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		fullRow := y < b.Min.Y+m || y >= b.Max.Y-m
		for x := b.Min.X; x < b.Max.X; x++ {
			if !fullRow && x == b.Min.X+m && b.Max.X-m > x {
				x = b.Max.X - m // skip the middle of the row
			}
			if alphaAt(img, x, y) > threshold {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}
