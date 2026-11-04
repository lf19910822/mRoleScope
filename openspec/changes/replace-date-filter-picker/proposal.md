# Proposal

## Why

The two editable first-discovery date boundaries currently use the browser's
native date input. Its visual treatment varies by browser and does not match
the Ocean Breeze query console, making a core filter feel disconnected from
the rest of the workbench.

## What Changes

- Replace each editable first-discovery date input with one shared,
  button-triggered calendar popover, using the shadcn-vue Date Picker's
  trigger-and-calendar interaction as the reference.
- Render the calendar with the application's existing Naive UI dependency and
  Ocean Breeze tokens rather than introduce a second component system.
- Render exactly one visible calendar surface. The month/year quick selector
  must remain opaque and legible above the date grid.
- Preserve the existing `YYYY-MM-DD` draft values, route query parameters,
  filtering semantics, pagination, and Serving request contract.

## Non-Goals

- No new API, date-range validation rule, persisted field, query parameter,
  or backend change.
- No change to dates that are display-only on role cards or detail pages.
