# Proposal

## Why

The released trend dashboard correctly reads formal historical curves, but the
first-use state duplicates its main action, configured curves do not retain an
obvious visual identity between the rail and chart, and a reader must infer
the latest count and change solely from the plotted line. The historical data
and API are already adequate; the reading layer needs to make comparison
faster and more deliberate.

## What Changes

- Make `/job-trends` the canonical route and redirect the previously shared
  `/trends` path to it.
- Give the no-curve state exactly one first-curve action. It creates the first
  draft condition and opens the narrow-screen configuration panel when needed.
- Establish a stable observation colour for each configured curve across the
  rail, line, and legend; use concise legend labels while retaining a full
  unambiguous condition in the configuration rail.
- Show each applied curve's latest complete count and change from the previous
  complete observation, preserving zero and unavailable-day semantics.
- Explain all-history chart gestures in-context, remove the non-actionable
  point-click acknowledgement, and tune mobile/dark presentation.

## Non-Goals

- No new trend endpoint, query parameter, storage, historical calculation, or
  change to the zero-versus-gap data contract.
- No JD drill-down, date information panel, node navigation, or snapshot
  payload.
- No new frontend component library or change to the existing Vue, Naive UI,
  ECharts, Ocean Breeze theme, or 5175 preview boundary.
