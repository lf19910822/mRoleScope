# Proposal

## Why

The 2026-11-06 trend node is missing because every collection attempt for that
day failed, while the successful 2026-11-07 publication contains the complete
observation that should represent the missing day. The trend reader also still
rejects custom dates outside the retained publication window, even though its
calendar query already has explicit null semantics for unavailable dates.

## What Changes

- Add a controlled, idempotent Worker repair operation that copies one
  successful published observation into a missing calendar date as a new
  immutable formal run, including run references, Serving memberships, and
  selected formal resolutions.
- Record repair provenance in internal run audit data without exposing it from
  public trend or drill-down APIs.
- Permit any valid inclusive custom date range, including dates before the
  first publication and after the latest publication. Dates outside available
  history remain `null` points.
- Return the real system coverage envelope (earliest to latest successful
  publication) so the dashboard can explain its data span without listing
  intermediate gaps.

## Non-Goals

- Do not rewrite the source 2026-11-07 run, current inventory, or SCD2 facts.
- Do not expose repair markers, run IDs, version IDs, or audit details to users.
- Do not fabricate counts for dates without a formal publication.
