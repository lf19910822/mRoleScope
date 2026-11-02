# Proposal: Automate definitive source-unavailable closures

## Why

The 2026-10-28 complete miHoYo snapshot contained 166 list rows and 166
successful detail rows, while five previously open periods were absent from
the list. A controlled three-attempt recheck of those five identities found
four consistent source responses with `success=false` and one consistently
available detail response; there were no transport or HTTP failures.

The existing classifier deliberately treats every non-available body as
indeterminate. It therefore leaves explicitly unavailable jobs open and also
retains a missing marker for a job whose detail endpoint proves it is still
available. Operators should not have to make either lifecycle decision by
hand.

## What Changes

- Recognize the reviewed miHoYo detail semantic `success=false` as a
  definitive-unavailable result for a known, previously observed job.
- Recheck a list-absent open job at most three times, sequentially and with a
  bounded attempt count. Only three definitive-unavailable responses confirm closure;
  any available or indeterminate response prevents closure in that run.
- Automatically close a confirmed unavailable presence and its active content
  periods in the normal publication transaction, retaining a safe closure
  reason and audit trail.
- Treat a valid matching detail response as proof that a list omission is not
  a disappearance: keep the period open and clear its missing marker.
- Keep timeouts, rate limits, server failures, malformed responses, and
  unrecognized payloads automatic-but-indeterminate. They receive later
  scheduled rechecks; they never create a manual confirmation queue or close
  a period.

## Capabilities

### New Capabilities

- `source-unavailable-closure`: defines the recognized source-unavailable
  semantic, bounded per-job retries, automatic closure, and list-omission
  recovery for the daily synchronization lifecycle.

## Impact

- `internal/jobsync` recheck classification and publication lifecycle.
- Existing repository integration tests and in-memory source-response tests.
- No schema migration, AI model call, JD parsing change, raw-response audit,
  or manual production SQL update.
