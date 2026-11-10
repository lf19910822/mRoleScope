# Proposal: Sort Jobs by Latest Modification

## Why

Readers cannot currently find recently changed jobs in the list or see when
each job last changed. The existing history timeline already establishes the
source-version and lifecycle facts needed for an honest observation date.

## What Changes

- Show a latest modification date on every job card, or `-` when no qualifying
  history is known, using the existing card layout.
- Add ascending and descending latest-modification sort rules to the existing
  ordered multi-rule list interface, with unavailable dates last.
- Reuse daily-final history selection and source-version change semantics;
  count a reopen as a modification for this date, but not a close or a first
  observation. Preserve all other filter and sorting behavior.

## Capabilities

### New Capabilities

- `job-last-modified`: Displays and sorts by the latest qualifying observed
  modification date without exposing internal provenance.

### Modified Capabilities

None; existing related capabilities remain in unarchived changes.

## Impact

- Serving list query, HTTP contract, OpenAPI, route state, job cards and tests.
- Read-only derivation from existing history; no change to ODS or DWD facts.
