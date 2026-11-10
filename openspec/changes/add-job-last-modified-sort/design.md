# Design

## Fact and time semantics

For each job, select the same most recently published current observation scope
as its history timeline, then derive the last qualifying date from that scope's
daily-final published observations and lifecycle event rules as
the JD history timeline. A `modified` node qualifies, as does a `reopened` node
even when its source versions are unchanged. A discovery, close, unchanged latest
observation, parser-only change, or missing date does not qualify. Return the
Shanghai business date, not a source modification timestamp. Repaired daily
observations participate under the same rules as the history timeline.

The current timeline classifies source-version identity changes rather than a
display-only field comparison. The list must match that existing behavior, not
substitute a new notion of visible changes. A reopened event takes precedence
over an ordinary modified event on the same date, but qualifies either way.
Do not invent history before the earliest retained observation.

## List and sorting

Derive the latest date for the full filtered set before pagination. Share or
centralize event selection with the existing history implementation where
practicable; if SQL and Go must express the same rule in different forms, add
cross-implementation fixture tests to prevent semantic drift. Avoid per-card
HTTP requests and N+1 snapshot reads. Preserve scope, daily-final selection,
close/reopen priority, missing-day and deterministic tie-break semantics.

Return nullable `last_modified_date` as an additive safe list field. Support
`last_modified_asc` and `last_modified_desc` in the existing repeated `sort`
interface, place NULL last in either direction, and preserve the four-rule cap,
filtering, URL state, stable tie-breaking, and backward compatibility.

The card uses the established metadata presentation, rendering a date or `-`.
No internal version, audit, processing, or repair information is displayed.

## Validation and performance

Cover shared history semantics with synthetic PostgreSQL fixtures and focused
Go, HTTP, route-state and Vue tests. Include event changes on same day,
cross-gap comparison, close/reopen, no history, scope isolation, sort direction,
multi-rule tie-break and page boundaries. Check the query plan and timings on
representative data; introduce a rebuildable Serving projection or index only
if measurements justify it. Browser-verify the existing fixed loopback preview
at desktop and 390px with clearly identified fixture data if necessary.
