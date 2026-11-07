# Proposal: Isolate Detail-Level Synchronization Failures

## Why

Daily synchronization is the system's source of observation dates, so one malformed or temporarily unavailable job detail must not discard the other jobs collected that day. The current collector rejects the whole batch when any detail is incomplete, which creates avoidable gaps in the SCD2 history.

## What Changes

- Treat each detail request and response validation as an independent outcome.
- Publish every valid observation from a list-complete batch and record failed source job IDs in the immutable manifest and run counts.
- Exclude failed IDs from missing-job closure and retry them naturally on the next scheduled run.
- Keep list-level failures and batches with no usable details as whole-run failures.

## Impact

The collector manifest contract, Go parsing/publishing boundary, synchronization audit records, and tests change. No API or UI contract changes.
