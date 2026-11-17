# Validation Record

## Browser environment

- Date: 2026-11-16
- Preview: `http://127.0.0.1:5175/`
- Browser: Chrome 151 via local CDP `9227`
- Method: `scripts/verify-trend-responsive-layout.py`

## Geometry matrix

| Viewport | Chart width | Horizontal overflow | Configuration state |
| ---: | ---: | ---: | --- |
| 1440px | 750px | 0px | Persistent rail visible |
| 1280px | 646px | 0px | Standard persistent rail visible |
| 1100px | 798px | 0px | Compact trigger visible; rail hidden by default |
| 900px | 598px | 0px | Compact trigger visible; rail hidden by default |
| 390px | 356px | 0px | Mobile trigger and bottom sheet available |

## Interaction checks

- Compact desktop configuration opens within the viewport at 900px.
- Curve list retains independent scrolling and the footer remains reachable.
- Closing the compact drawer removes its backdrop and restores the trigger focus.
- An open compact drawer settles into the standard persistent rail when the viewport changes to 1280px.
- At 390px, the event drilldown stays inside the viewport and the岗位详情层 opens at the viewport top instead of remaining in its off-screen enter position.
- 390px screenshot review confirms mobile navigation trigger, chart, event table entry point, and configuration trigger remain visible without horizontal overflow.

## Commands

- `npm run test:unit -- --run src/views/__tests__/JobTrendView.spec.ts src/views/__tests__/JobTrendDashboardReading.spec.ts src/components/__tests__/AppShell.spec.ts`
- `npm run type-check`
- `npm run build-only`
- `python3 scripts/verify-trend-responsive-layout.py`
- `git diff --check`
- `openspec validate improve-desktop-responsive-layout --strict`

## Limitations

- Native browser accessibility-tree inspection is not available in the root Chromium environment; geometry and interaction were verified through Chrome CDP and focused Vue tests.
- The build retains the existing Vite chunk-size warning; it does not block this layout change.
