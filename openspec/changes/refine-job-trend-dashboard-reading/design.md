# Design

## Subject and reading job

This is a recruiting analyst's historical observation surface. Its single job
is to let that reader configure a few formal conditions and immediately tell
which demand curve is higher, changing, or unavailable—without confusing a
historical fact gap for a zero.

## Visual direction: the observation ledger

The existing Ocean Breeze shell remains the quiet field notebook. The one
signature element is an **observation colour marker**: every configured curve
owns one stable colour that appears as its rail marker, ECharts line, and
legend icon. This is a functional identity system, not decorative colouring.
It lets a reader follow one condition across the page while comparing up to
eight lines.

| Token | Value | Use |
| --- | --- | --- |
| Deep ink | `#14313d` | primary page reading text |
| Harbour blue | `#007cab` | primary action and first curve |
| Current teal | `#0b9a8c` | active status and second curve |
| Sea foam | `#f8fcfd` | quiet configuration-card surface |
| Mist rule | `#dce8ed` | light structural boundary |
| Signal coral | `#a8424d` | actionable curve error |

The dark variants use local custom properties for muted text and rules rather
than leaving light-mode greys on a dark configuration card. No new typeface or
ornament is introduced: the data and colour markers carry the personality.

## Interaction composition

```text
desktop
┌──────── navigation ───────┬────────── reading canvas ───────────┬── rail ──┐
│                            │ title · range · contextual gesture  │ ● Go     │
│                            │                                     │ latest 3 │
│                            │       coloured historical lines     │ ● Python │
│                            │       concise matching legend       │ latest 1 │
└────────────────────────────┴─────────────────────────────────────┴──────────┘

narrow
┌──────────────────────────── chart ───────────────────────────────┐
│ title · range · line chart                                         │
└──────────────────────────────────────────────────────────────────┘
                         [ 配置曲线（N） ]
                    bottom-sheet observation ledger
```

### First curve

With no rows, the canvas has one action: `开始配置首条曲线`. It calls the
existing bounded `addCurve` transition. On narrow screens it also opens the
configuration panel; desktop already exposes that rail. The rail footer and
the floating mobile configuration trigger are hidden until at least one curve
exists. Removing the last row closes the narrow panel so the page returns to
the same single-action state.

### Curve identity and labels

`trendCurveColor(row)` derives a palette slot from the existing stable row ID
instead of the rendered-series array position. Therefore a pending, failed,
hidden, or removed sibling cannot recolour an existing curve. The rail receives
the same colour as a compact marker.

The series keeps its full condition name for tooltips and unambiguous state
mapping. ECharts' legend formatter receives a compact, display-safe form such
as `技术 Go`, `学历 ≥ 本科`, or `岗位 前端 +副`; this prevents the eight-line
legend from becoming a second verbose configuration panel.

### Latest observation summary

`trendCurveObservationSummary(row)` reads only the already returned ordered
points. It finds the final numeric count and the preceding numeric count,
skipping `null` gaps. It returns no summary when no numeric point exists and
does not invent a value for an unavailable day. A configured rail row presents
the full label, then `最近观测 N` and, when possible, a signed `较前一有效日 ±D`.
Zero remains numeric and can produce a negative, zero, or positive delta.

### Chart controls

The full-history range shows a short desktop-only instruction adjacent to the
range controls: `滚轮缩放时间 · Shift+滚轮缩放岗位数 · 拖拽平移`. The existing
`复位视图` remains the recovery control. Fixed natural ranges do not show
zoom/pan guidance. Point clicks no longer announce a false "focused" state;
the existing hover tooltip remains the only point inspection affordance until
a separately approved information layer exists.

### Responsive and dark details

- The terse subtitle becomes `每点为当日匹配岗位数。`, preventing a stranded
  `数。` on a 390px viewport.
- Refresh is hidden until a rendered curve exists. The mobile trigger accounts
  for the safe area and reserves page space so it cannot cover the chart.
- The narrow bottom sheet remains the only scrollable configuration surface.
- Dark rail/card labels, states, and rules use dark-specific tokens with the
  same hierarchy as light mode.

## Confirmed TDD seams

The product owner's approval covers these public seams:

1. **Router seam** — resolving the previously shared `/trends` address must
   lead to the canonical dashboard route. It does not test browser history
   implementation.
2. **Chart-presentation seam** — exported chart helpers and option factory
   must give a persisted curve a stable colour, a concise legend label, and an
   honest latest-observation summary. It does not test canvas pixels.
3. **Dashboard-view seam** — mounting the view with the dashboard state must
   expose one initial action, make it create the first draft, avoid duplicate
   mobile entry points, and keep refresh unavailable until a rendered line
   exists. It does not test Naive UI internals.
4. **Responsive style seam** — the scoped CSS must retain the fixed desktop
   rail, safe-area-aware narrow trigger, and explicit dark tokens. It does not
   reproduce each browser's layout engine.

## Risks

The summary is intentionally descriptive, not a statistical trend claim: it
compares only adjacent available observations and calls the value "最近观测".
This avoids representing the last recoverable historical point as a current
publication. Colour allocation is restricted to the existing maximum of eight
curve rows and reuses the established palette, so it adds no dependency or
unbounded state.
