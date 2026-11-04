# Tasks

## 1. Contract and component

- [x] 1.1 Add red component regressions for ISO-value forwarding, selection,
      clear behavior, and the absence of native date inputs.
- [x] 1.2 Implement the shared button-triggered calendar adapter with the
      existing Naive UI popover/calendar primitives.
- [x] 1.3 Add Ocean Breeze DatePicker theme overrides and scoped trigger
      styling for light, dark, desktop, and narrow layouts.

## 2. JD browser integration

- [x] 2.1 Add a red list-page regression showing two shared date pickers and
      preserving their ISO route query values when filters are applied.
- [x] 2.2 Replace both editable first-discovery date inputs without changing
      other filter, pagination, URL, or Serving behavior.

## 3. Acceptance

- [x] 3.1 Verify the fixed `http://127.0.0.1:5175/jobs` entry at desktop and
      390px, including selection and clear behavior.
- [x] 3.2 Run focused and full frontend tests, type check, production build,
      Prettier validation, strict OpenSpec validation, and `git diff --check`.

## 4. Calendar surface repair (TDD)

- [x] 4.1 Add a red rendered-component regression for one visible calendar
      surface and an opaque month/year quick selector above the date grid.
- [x] 4.2 Make the Popover positioning-only and scope base-panel styling to
      the direct date panel; give the nested quick selector its own opaque
      overlay treatment.
- [x] 4.3 Run focused and full frontend checks, strict OpenSpec validation,
      diff check, and visual verification at the fixed local entrypoint.
