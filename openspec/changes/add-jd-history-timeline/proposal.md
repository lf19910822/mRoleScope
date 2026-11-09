# Proposal: Add JD History Timeline

## Why

The system already preserves versioned job metadata, JD body content, daily
published observations, and confirmed visibility periods. A job detail reader
can therefore recover what was observed over time, but it cannot currently
show when a job was discovered, modified, confirmed closed, reopened, or last
observed. Users also cannot open an adjacent-version diff for a specific
change; the existing historical detail comparison is oriented around a
historical node versus the latest JD.

## What Changes

- Add a compact `岗位历史` timeline below the three JD body sections on the
  job detail page.
- Add a server-owned history summary that selects one final successful
  observation per Asia/Shanghai business day and derives event nodes from
  existing SCD2/Serving facts.
- Use stable node types `discovered`, `modified`, `confirmed_closed`,
  `reopened`, and `latest`, with display-safe tags and summaries.
- Add a node detail seam that returns a single JD snapshot or an adjacent
  version diff depending on node type, reusing the trend dashboard's existing
  change classification and Git-style body diff.
- Preserve best-effort serving: asynchronous parser reanalysis is never run in
  an HTTP request; current Serving fallback results may be shown while a newer
  analysis catches up.
- Reuse the trend timeline interaction, detail container, ten-minute client
  cache, retry behavior, and responsive desktop/390px presentation patterns.

## Non-Goals

- No new JD snapshot table, event table, `change_events` writes, or copied body
  payloads.
- No predictive analytics, AI summaries, or parser/dictionary/version details
  in the public response.
- No modification to the existing trend dashboard response contract.
- No frontend reconstruction of SCD2, daily publication selection, lifecycle
  periods, or parser fallback rules.
- No event generated solely by parser, dictionary, or normalizer changes.
- No implicit inclusion of source fields that are not currently visible in the
  job detail page.
