// Package proof checks customer artwork for common print problems before a
// designer reviews it.
package proof

import (
	"context"
	"image"
)

// Issue is a print problem a designer would send back to the customer.
type Issue string

const (
	LowResolution    Issue = "low_resolution"
	UnsafeMargin     Issue = "unsafe_margin"
	OpaqueBackground Issue = "opaque_background"
)

// AllIssues is the fixed order used in reports.
var AllIssues = []Issue{LowResolution, UnsafeMargin, OpaqueBackground}

// Order holds the details of an order that affect the proof.
type Order struct {
	File     string  `json:"file"`
	WidthIn  float64 `json:"width_in"`
	HeightIn float64 `json:"height_in"`
	Cut      string  `json:"cut"` // "die_cut" or "square"
}

// Result is what a checker found in one file.
type Result struct {
	Issues []Issue
	Notes  []string
}

// Checker is anything that can review artwork. The rule based checker lives in
// rules.go. A model backed checker (Claude, OpenAI, Grok) can implement the
// same interface, so the eval tool can compare them on the same files.
type Checker interface {
	Name() string
	Check(ctx context.Context, img image.Image, o Order) (Result, error)
}
