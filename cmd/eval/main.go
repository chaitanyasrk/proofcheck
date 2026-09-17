// Command eval runs a checker over labeled artwork and reports how often it
// agrees with the designer. It exits with status 1 when file accuracy falls
// below -min-accuracy, so it can gate a release in CI.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"proofcheck/internal/proof"
)

type label struct {
	proof.Order
	Expected []proof.Issue `json:"expected"`
	Why      string        `json:"why"`
}

type counts struct{ TP, FP, FN int }

func (c counts) precision() float64 { return ratio(c.TP, c.TP+c.FP) }
func (c counts) recall() float64    { return ratio(c.TP, c.TP+c.FN) }
func (c counts) f1() float64 {
	p, r := c.precision(), c.recall()
	if p+r == 0 {
		return 0
	}
	return 2 * p * r / (p + r)
}

func ratio(a, b int) float64 {
	if b == 0 {
		return 1 // nothing to find and nothing flagged counts as perfect
	}
	return float64(a) / float64(b)
}

type fileResult struct {
	File     string        `json:"file"`
	Expected []proof.Issue `json:"expected"`
	Got      []proof.Issue `json:"got"`
	Pass     bool          `json:"pass"`
	Notes    []string      `json:"notes,omitempty"`
}

type report struct {
	Checker      string                 `json:"checker"`
	Files        int                    `json:"files"`
	FileAccuracy float64                `json:"file_accuracy"`
	PerIssue     map[proof.Issue]counts `json:"per_issue"`
	Results      []fileResult           `json:"results"`
}

func main() {
	dir := flag.String("samples", "samples", "folder with PNGs and labels.json")
	minDPI := flag.Float64("min-dpi", 300, "flag artwork below this DPI")
	margin := flag.Float64("safe-margin", 0.0625, "safe margin for die cut, in inches")
	alpha := flag.Int("alpha-threshold", 0, "pixels at or below this alpha are treated as empty")
	minAcc := flag.Float64("min-accuracy", 0.8, "exit 1 if file accuracy is below this")
	out := flag.String("out", "", "optional path for a JSON report")
	flag.Parse()

	if *alpha < 0 || *alpha > 255 {
		log.Fatalf("alpha-threshold must be 0-255, got %d", *alpha)
	}

	var checker proof.Checker = proof.RuleChecker{
		MinDPI: *minDPI, SafeMarginIn: *margin, AlphaThreshold: uint8(*alpha), OpaqueShare: 0.9,
	}

	labels, err := loadLabels(filepath.Join(*dir, "labels.json"))
	if err != nil {
		log.Fatal(err)
	}

	rep := run(context.Background(), checker, *dir, labels)
	print(rep)

	if *out != "" {
		b, _ := json.MarshalIndent(rep, "", "  ")
		if err := os.WriteFile(*out, b, 0o644); err != nil {
			log.Fatal(err)
		}
	}
	if rep.FileAccuracy < *minAcc {
		fmt.Printf("\nFAIL: accuracy %.1f%% is below the %.1f%% bar\n", rep.FileAccuracy*100, *minAcc*100)
		os.Exit(1)
	}
	fmt.Printf("\nPASS: accuracy %.1f%% meets the %.1f%% bar\n", rep.FileAccuracy*100, *minAcc*100)
}

func loadLabels(path string) ([]label, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read labels: %w", err)
	}
	var ls []label
	if err := json.Unmarshal(b, &ls); err != nil {
		return nil, fmt.Errorf("parse labels: %w", err)
	}
	if len(ls) == 0 {
		return nil, fmt.Errorf("no labels in %s", path)
	}
	return ls, nil
}

func run(ctx context.Context, c proof.Checker, dir string, labels []label) report {
	rep := report{Checker: c.Name(), PerIssue: map[proof.Issue]counts{}}
	passed := 0

	for _, l := range labels {
		fr := fileResult{File: l.File, Expected: l.Expected}
		res, err := checkFile(ctx, c, filepath.Join(dir, l.File), l.Order)
		if err != nil {
			fr.Notes = []string{"error: " + err.Error()}
		} else {
			fr.Got, fr.Notes = res.Issues, res.Notes
		}

		want, got := set(l.Expected), set(fr.Got)
		for _, is := range proof.AllIssues {
			k := rep.PerIssue[is]
			switch {
			case want[is] && got[is]:
				k.TP++
			case !want[is] && got[is]:
				k.FP++
			case want[is] && !got[is]:
				k.FN++
			}
			rep.PerIssue[is] = k
		}

		fr.Pass = err == nil && sameSet(want, got)
		if fr.Pass {
			passed++
		}
		rep.Results = append(rep.Results, fr)
	}

	rep.Files = len(labels)
	rep.FileAccuracy = float64(passed) / float64(len(labels))
	return rep
}

func checkFile(ctx context.Context, c proof.Checker, path string, o proof.Order) (proof.Result, error) {
	f, err := os.Open(path)
	if err != nil {
		return proof.Result{}, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return proof.Result{}, fmt.Errorf("decode %s: %w", filepath.Base(path), err)
	}
	return c.Check(ctx, img, o)
}

func set(is []proof.Issue) map[proof.Issue]bool {
	m := map[proof.Issue]bool{}
	for _, i := range is {
		m[i] = true
	}
	return m
}

func sameSet(a, b map[proof.Issue]bool) bool {
	if len(a) != len(b) {
		return false
	}
	for k := range a {
		if !b[k] {
			return false
		}
	}
	return true
}

func join(is []proof.Issue) string {
	if len(is) == 0 {
		return "-"
	}
	s := make([]string, len(is))
	for i, v := range is {
		s[i] = string(v)
	}
	sort.Strings(s)
	return strings.Join(s, ", ")
}

func print(rep report) {
	fmt.Printf("Checker: %s\n\n", rep.Checker)

	tw := tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "FILE\tEXPECTED\tGOT\tRESULT")
	for _, r := range rep.Results {
		status := "ok"
		if !r.Pass {
			status = "MISS"
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\n", r.File, join(r.Expected), join(r.Got), status)
	}
	tw.Flush()

	fmt.Println()
	tw = tabwriter.NewWriter(os.Stdout, 2, 4, 2, ' ', 0)
	fmt.Fprintln(tw, "ISSUE\tTP\tFP\tFN\tPRECISION\tRECALL\tF1")
	var all counts
	for _, is := range proof.AllIssues {
		k := rep.PerIssue[is]
		all.TP, all.FP, all.FN = all.TP+k.TP, all.FP+k.FP, all.FN+k.FN
		fmt.Fprintf(tw, "%s\t%d\t%d\t%d\t%.2f\t%.2f\t%.2f\n", is, k.TP, k.FP, k.FN, k.precision(), k.recall(), k.f1())
	}
	fmt.Fprintf(tw, "all issues\t%d\t%d\t%d\t%.2f\t%.2f\t%.2f\n", all.TP, all.FP, all.FN, all.precision(), all.recall(), all.f1())
	tw.Flush()

	passed := int(rep.FileAccuracy*float64(rep.Files) + 0.5)
	fmt.Printf("\nFile accuracy: %d/%d = %.1f%%\n", passed, rep.Files, rep.FileAccuracy*100)
}
