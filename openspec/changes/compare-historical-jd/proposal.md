# Proposal: Compare Historical JD Changes

## Why

Historical trend drill-down can restore the JD that was visible on a node date,
but a reader cannot quickly tell what changed before opening the latest JD.
The detail response needs a display-safe comparison that makes changed
dimensions and changed body text explicit without exposing SCD2 or parser
internals.

## What Changes

- Compare the node-day historical JD with the request-time latest JD in
  Serving, using one read-only transaction for both versions.
- Return display-safe dimension changes and deterministic body diff fragments
  for responsibilities, requirements, and supplemental notes.
- Render independent collapsed change tags, deep-red deleted historical text,
  and deep-green latest inserted text in the existing single-column detail
  reader, with text/ARIA semantics and light/dark tokens.
- Preserve complete historical reading when comparison is unavailable; allow
  local retry without refreshing the trend chart or node list.

## Non-Goals

- No snapshot table, browser persistence, or change to SCD2 storage.
- No comparison for confirmed-closed historical jobs or latest-detail mode.
- No client-side diff algorithm, side-by-side reader, or truncation of diff
  fragments.
