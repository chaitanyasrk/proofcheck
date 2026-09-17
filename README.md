# proofcheck

A first pass reviewer for sticker artwork, plus the eval harness that decides whether it is good enough to ship.

Every custom sticker order gets a proof, which means a designer looks at every file. Most files have one of a few boring problems: the resolution is too low, the art runs into the cut line, or there is a solid box behind a die cut design. This project catches those before a person has to, and it measures how often it agrees with the designer.

The checker is the easy part. The eval is the part I care about. An agent that nobody measures ends up running forever whether it helps or not.

## What it checks

| Issue | Rule |
|---|---|
| `low_resolution` | Effective DPI at the ordered print size is below `-min-dpi` |
| `unsafe_margin` | Die cut only. Visible pixels inside the safe margin (default 1/16 in) |
| `opaque_background` | Die cut only. At least 90% of edge pixels are solid |

## Run it

Needs Go 1.22 or newer. No outside dependencies.

```bash
go test ./...
go run ./cmd/gensamples            # writes samples/*.png and samples/labels.json
go run ./cmd/eval                  # v1 settings
go run ./cmd/eval -alpha-threshold 16 -min-dpi 250 -out report.json
```

`eval` exits with status 1 when file accuracy drops below `-min-accuracy` (default 0.8), so it can block a release in CI.

## Results on the 15 sample files

The labels in `labels.json` are what a designer would decide, not what the rules produce. A few files are there to break the rules on purpose.

| Settings | File accuracy | Precision | Recall |
|---|---|---|---|
| v1: `min_dpi=300, alpha>0` | 12/15 (80.0%) | 0.75 | 0.90 |
| v2: `min_dpi=250, alpha>16` | 14/15 (93.3%) | 0.90 | 0.90 |

What v1 got wrong:

- **08_faint_glow**: a glow at about 2% opacity touches the edge. It won't show in print, but alpha > 0 flags it.
- **10_280dpi**: 280 DPI is fine on a 3 inch sticker. A hard 300 cutoff sends it back anyway.
- **09_shadow_box**: a 78% opacity box prints as a visible box. The edge pixels sit below the 250 alpha cutoff, so it gets called a margin problem instead of a background problem.

v2 fixes the first two. The shadow box is still wrong, and I left it that way. Fixing it means deciding how see-through a background has to be before a designer stops caring, and that is a question for the designers, not a number I should guess.

One honest caveat: v2 was tuned on the same 15 files it is scored on, so 93% is optimistic. The next step is a held-out set of real, designer-labeled orders.

## Layout

```
cmd/gensamples     builds synthetic artwork and designer labels
cmd/eval           runs a checker, prints per-file results and per-issue precision/recall
internal/proof     Checker interface and the rule based checker
```

## Next

- A model backed checker (Claude, OpenAI, Grok) behind the same `Checker` interface, so all of them run against the same labels and the cheapest one that clears the bar wins.
- Store eval runs in Postgres so accuracy can be tracked across versions.
- A small GraphQL API so the proof tool can ask for a review.
