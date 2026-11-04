# Design

## Context

The JD browser owns two optional, independent date-boundary filters:
`firstDiscoveredFrom` and `firstDiscoveredUntil`. They are draft values until
the visitor applies filters, then serialize to the existing
`first_discovered_from` and `first_discovered_until` route and Serving query
parameters. Both values are ISO calendar dates (`YYYY-MM-DD`).

## Audience and visual direction

The user is a recruiting reader narrowing a live job collection. The control
should read as a deliberate query instrument rather than a browser-owned form
field. Following the shadcn-vue Date Picker reference, a calendar-icon button
opens a compact calendar popover; selection returns the ISO date while the
trigger presents a scannable dotted date. Ocean Breeze supplies the Sea Glass
trigger, Tide Blue focus and selection, and quiet structural borders.

## Decision

`FilterDatePicker` is a small Vue adapter around existing Naive UI `NPopover`
and panel-mode `NDatePicker`:

```text
ISO v-model string
       │
calendar trigger button ── NPopover ── panel calendar + 清除
       │
formatted selection emits the same ISO string
```

The component accepts and emits `string`, with the empty string representing
an unset boundary. It configures Naive UI's formatted-value contract with
`value-format="yyyy-MM-dd"`; this prevents local time-zone conversion from
changing a date. Selecting a day closes the popover. `清除` resets only the
owning boundary and closes it. Each calendar is independent—this change does
not invent a date-range meaning or require the start boundary to precede the
end boundary.

The existing application provider supplies Naive UI's `zhCN` and `dateZhCN`
locales, so calendar months, weekdays, and actions match the Chinese browser
instead of inheriting the package's English defaults.

### Surface layering

The outer `NPopover` is a positioning-only container (`raw`) with no surface
shadow. The adapter's calendar content is therefore the sole visible card.
This avoids a Popover background and padding surrounding a second bordered
calendar card.

Naive UI renders its month/year quick selector as a nested `n-date-panel`.
The quiet base-panel rule intentionally targets only the direct calendar panel;
it must not make nested panels transparent. The nested month selector receives
an opaque Ocean Breeze surface, a restrained shadow, and a higher local
stacking order so that its year and month columns remain readable above the
date grid.

## Compatibility and responsive behavior

- Desktop: the two controls retain their paired query-console placement.
- Narrow viewports: the pair follows the existing single-column filter flow;
  each trigger is full width and its calendar remains anchored to it.
- Light and dark: shared Naive UI DatePicker overrides apply the paired Ocean
  Breeze palette while component CSS owns the trigger affordance.
- The list page still owns apply/reset and its URL state. The component never
  calls the API or router.

## Risks

The reference implementation normally assumes shadcn-vue's Tailwind and Reka
UI stack, which this Naive UI application does not use. Recreating its
trigger/popover/calendar interaction using the installed system avoids a
second visual runtime, duplicated accessibility primitives, and unrelated
bundle expansion while retaining the intended visitor experience.
