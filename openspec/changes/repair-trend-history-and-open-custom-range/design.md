# Design

The repair operation lives in `internal/jobsync` because it mutates collection
history. It resolves the fixed Mihoyo technical observation scope, locks the
scope/target date, selects the latest successful published run for the source
Shanghai date, and rejects a target date that already has a successful run.
One transaction creates a `published` run with `trigger_kind=repair`, copies
the source run's immutable references and Serving membership projection, and
appends an audit event containing only internal provenance. A unique partial
index makes duplicate target repairs impossible; a repeated command returns
the existing repair result without writing a second snapshot.

The target publication timestamp is the source publication timestamp shifted
by the calendar-day delta. This makes existing daily selection, historical
detail, and drill-down queries see the repair under the requested local date,
while preserving the source observation facts. The current scope publication
pointer is intentionally unchanged.

Trend custom ranges remove retention/today bounds and keep only date validity
and ordering validation. Each response includes `coverage.first_date` and
`coverage.last_date`, computed from successful published runs independent of
the requested range. The frontend renders this envelope only near the custom
range controls.
