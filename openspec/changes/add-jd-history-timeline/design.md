# Design

## Context and constraints

The source of truth is the existing immutable SCD2/Serving model:

- published collection runs and their Shanghai business dates;
- per-run job version references;
- source metadata and body content versions;
- confirmed job visibility periods and reopen boundaries;
- Serving memberships and persisted deterministic analysis results.

The feature must not introduce a second historical fact store. The service
owns all business-date and lifecycle decisions; the Vue client renders the
already-derived event stream.

The current trend dashboard already has the relevant comparison categories,
body-section diff implementation, detail container, loading/error states,
short-lived cache, and responsive interaction patterns. The new seam should
reuse those components and algorithms rather than fork them.

## Event derivation

For one public job identifier and one observation scope:

1. Select the final `published` run for each `Asia/Shanghai` business day by
   `published_at DESC, id DESC`. A failed or incomplete attempt never replaces
   a successful observation on that day.
2. Keep only the job's observation from that daily final run. If a day has no
   successful run, it has no observation; it does not produce a warning node.
3. Compare each observed day with the nearest earlier successful business day
   for the same visible lifecycle segment. This preserves changes across a
   missing-day gap without inventing a missing-day event.
4. Use the existing trend/detail comparison logic to classify visible source
   metadata changes as dimension changes and the three body sections as body
   changes. Parser, dictionary, or normalizer result changes alone never create
   an event.
5. Use confirmed `job_presence_periods` boundaries for lifecycle events. A
   missing list item alone is not a close. A new open presence after a confirmed
   close creates `reopened`; the close-to-reopen interval is not connected by a
   line.
6. The first observed version is `discovered`. If it is also the latest daily
   version, merge the semantics into one node with a `latest` tag. On a later
   daily change, the primary type is `modified`; if it is also the latest day,
   add a `latest` tag rather than drawing a second point.
7. A confirmed-closed latest state is represented by the close node and its
   latest-state tag; there is no empty latest point. A reopened day may carry
   `reopened` plus dimension/body change tags and compares against the last
   version before the close.

The server returns nodes in business-date order. A node is retained even when
its historical snapshot cannot be served; the summary marks it as not
viewable and the detail endpoint returns a stable business-unavailable state.

## Public contracts

### History summary

`GET /api/v1/jobs/{public_id}/history`

The response contains only safe display fields:

```json
{
  "nodes": [
    {
      "date": "2026-11-02",
      "type": "modified",
      "tags": ["body_responsibilities"],
      "summary": "岗位职责",
      "can_view_historical": true
    }
  ]
}
```

`type` is one of `discovered`, `modified`, `confirmed_closed`, `reopened`, or
`latest`. Tags and summaries are display-safe Chinese business labels. No run,
job, source, membership, parser, dictionary, or version IDs cross the public
boundary.

### Node detail

`GET /api/v1/jobs/{public_id}/history/detail?date=YYYY-MM-DD&type=modified`

The service resolves the node from the published facts, then maps to the
existing safe JD detail DTO:

- `modified`: compare the nearest earlier successful business day with the
  selected day;
- `reopened`: compare the last pre-close version with the reopened version;
- `discovered`, `confirmed_closed`, `latest`: return one snapshot without a
  comparison object.

The comparison response reuses existing display-safe dimension changes and
ordered body fragments (`unchanged`, `deleted`, `inserted`). Deleted historical
text is deep red and inserted current text is deep green. No latest-JD fallback
is allowed when the requested historical snapshot itself is unavailable.

## Serving and parser behavior

Deterministic parsing remains asynchronous and persisted. The request path does
not parse JD text. While a newly published parser/dictionary identity is still
catching up, the existing Serving selection may return a same-job completed
fallback; this is accepted as best-effort service and is not exposed as a
warning. Once reconciliation catches up, the same historical node naturally
shows the newer interpretation.

Modification facts remain source-based: visible canonical metadata values and
normalized body content are compared, not parser outputs or parser identity.
Fields not currently visible in the job detail page do not create standalone
history events.

## Frontend composition

The `岗位历史` section is a compact fixed-baseline timeline:

- X axis is Shanghai business date;
- Y axis has no meaningful numeric scale, so labels/grid are hidden;
- nodes use stable shape and semantic colors for discovered, modified,
  confirmed-closed, reopened, and latest;
- event tags are concise and appear in the node tooltip/summary;
- all summary nodes load at once, with zoom and horizontal pan for long history;
- node details reuse the trend dashboard detail container and Git diff reader.

The module has independent loading, empty, unavailable, error, and retry
states. Cached content remains visible on failed refreshes. The client cache is
ten minutes and is invalidated by page refresh/re-application according to the
existing trend dashboard semantics. Desktop and 390px views use the same data;
the narrow view uses horizontal interaction and the existing bottom detail
panel without obscuring the surrounding job detail content.

Keyboard focus, Enter/Space activation, non-color node cues, zoom reset,
Escape close, and focus return are part of the component contract.

## Testing and rollout

Use real PostgreSQL integration fixtures for daily final-run selection,
same-day retries, missing days, repair replacement, confirmed close/reopen,
snapshot-unavailable nodes, and parser-only reanalysis. Unit tests cover pure
date aggregation and label mapping. HTTP/OpenAPI tests assert safe fields and
stable error semantics. Component/browser tests cover desktop and 390px state
matrices, cache/retry behavior, timeline interaction, and detail reuse.

No deployment or production data mutation is part of this change until the
OpenSpec tasks and measured verification pass.
