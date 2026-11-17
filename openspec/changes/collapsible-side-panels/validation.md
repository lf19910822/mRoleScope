# Validation Record

## Browser environment

- Date: 2026-11-16
- Preview: `http://127.0.0.1:5175/`
- Browser: Chrome 151 via local CDP `9227`
- Method: `scripts/verify-trend-responsive-layout.py`

## Geometry matrix

| Viewport | Chart width | Horizontal overflow | Side-panel state |
| ---: | ---: | ---: | --- |
| 1440px | 1374px | 0px | Navigation and curve configuration collapsed by default |
| 1280px | 1222px | 0px | Navigation and curve configuration collapsed by default |
| 1100px | 1034px | 0px | Both panels use on-demand fixed launchers |
| 900px | 834px | 0px | Configuration opens as a bounded right drawer |
| 390px | 356px | 0px | Existing mobile navigation and bottom configuration panel preserved |

## Interaction checks

- Desktop navigation opens from the left launcher and collapses through its close button without changing the active route.
- Curve configuration opens from the right launcher and closes through its existing close button; the panel retains its curve cards and footer actions.
- Closing the curve configuration returns focus to the right launcher.
- Closed side panels do not reserve fixed left or right content columns.
- No horizontal overflow appears across the 1440px, 1280px, 1100px, 900px, and 390px matrix.
- The mobile drilldown/detail path was conditionally skipped in this run because the local preview API returned no trend fixture data; focused Vue tests continue to cover the existing mobile overlay contract.

## Commands

- `npm run test:unit -- --run src/components/__tests__/AppShell.spec.ts src/views/__tests__/JobTrendView.spec.ts src/views/__tests__/JobTrendDashboardReading.spec.ts`
- `npm run test:unit -- --run`
- `npm run type-check`
- `npm run build-only`
- `python3 scripts/verify-trend-responsive-layout.py`
- `git diff --check`
- `openspec validate collapsible-side-panels --strict`

## Limitations

- The local preview API did not return filter options during browser acceptance, so the visual run used the empty trend state and draft curve cards rather than production trend data.
- The production build retains the existing Vite chunk-size warning; it does not block this side-panel change.
