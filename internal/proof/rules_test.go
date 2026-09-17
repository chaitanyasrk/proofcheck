package proof

import (
	"context"
	"image"
	"image/color"
	"testing"
)

func baseChecker() RuleChecker {
	return RuleChecker{MinDPI: 300, SafeMarginIn: 0.0625, OpaqueShare: 0.9}
}

func has(r Result, i Issue) bool {
	for _, got := range r.Issues {
		if got == i {
			return true
		}
	}
	return false
}

func TestLowResolution(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 300, 300)) // 100 DPI at 3in
	r, err := baseChecker().Check(context.Background(), img, Order{WidthIn: 3, HeightIn: 3, Cut: "square"})
	if err != nil {
		t.Fatal(err)
	}
	if !has(r, LowResolution) {
		t.Fatalf("want low_resolution, got %v", r.Issues)
	}
}

func TestInkInMargin(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 900, 900))
	img.Set(450, 3, color.NRGBA{0, 0, 0, 255})
	r, _ := baseChecker().Check(context.Background(), img, Order{WidthIn: 3, HeightIn: 3, Cut: "die_cut"})
	if !has(r, UnsafeMargin) {
		t.Fatalf("want unsafe_margin, got %v", r.Issues)
	}
}

func TestInkOnRightEdge(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 900, 900))
	img.Set(897, 450, color.NRGBA{0, 0, 0, 255})
	r, _ := baseChecker().Check(context.Background(), img, Order{WidthIn: 3, HeightIn: 3, Cut: "die_cut"})
	if !has(r, UnsafeMargin) {
		t.Fatalf("want unsafe_margin, got %v", r.Issues)
	}
}

func TestSolidBackgroundSkipsMargin(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 900, 900))
	for y := 0; y < 900; y++ {
		for x := 0; x < 900; x++ {
			img.Set(x, y, color.NRGBA{255, 255, 255, 255})
		}
	}
	r, _ := baseChecker().Check(context.Background(), img, Order{WidthIn: 3, HeightIn: 3, Cut: "die_cut"})
	if !has(r, OpaqueBackground) || has(r, UnsafeMargin) {
		t.Fatalf("want only opaque_background, got %v", r.Issues)
	}
}

func TestTinyImageDoesNotHang(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 4, 4))
	if _, err := baseChecker().Check(context.Background(), img, Order{WidthIn: 0.01, HeightIn: 0.01, Cut: "die_cut"}); err != nil {
		t.Fatal(err)
	}
}
