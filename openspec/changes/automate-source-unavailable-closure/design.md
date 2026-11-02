# Design

## Context

The daily synchronization service first validates and archives a complete
source list and its normal details. It then sequentially rechecks every open
presence period absent from that list. The repository already knows how to
atomically close a period when it receives
`RecheckDefinitelyUnavailable`, but the miHoYo rechecker currently never
returns that outcome. It also records `first_missing_at` before it knows
whether detail revalidation proves availability.

## Goals and non-goals

**Goals:**

- Automate source-backed closing without a human confirmation step.
- Avoid closing a job because of a list omission, timeout, rate limit, or
  unknown body.
- Preserve period and content history atomically and retain only safe audit
  codes/metadata.

**Non-goals:**

- Retrying a failed whole collection beyond its existing wrapper policy.
- Inferring unavailability from title, body text, AI output, or elapsed time.
- Persisting raw source recheck bodies or logging JD content.

## Decisions

### Reviewed source semantic and three-attempt quorum

For a known job ID submitted with the fixed detail request shape, a successful
HTTP response whose top-level JSON boolean is exactly `success=false` is a
definitive source-negative response. The classifier is backed by an
in-memory fixture and a controlled live verification: four list-absent,
previously observed IDs produced this semantic in each of three sequential
attempts, while the fifth produced a valid matching detail response in each
attempt.

`MihoyoDetailRechecker` makes at most three sequential attempts per missing
job. It returns `definitively_unavailable` only when all three attempts carry
the reviewed negative semantic. A valid matching detail on any attempt wins
as `available`. Any other result (transport, 429, 5xx, non-2xx, malformed
body, or an unrecognized body) makes the outcome `indeterminate`, even if a
different attempt was negative. The bounded attempt count never creates
parallel upstream traffic.

This quorum protects against a transient or altered source response while
still reaching an automatic determination in one complete synchronization.

### Availability overrides a list omission

When the detail response is valid and matches the missing ID, it is stronger
evidence than the list omission for that run. Publication must leave the
presence period open, clear `first_missing_run_id` and `first_missing_at`, and
append a safe `detail_available` audit event. It must not manufacture a list
observation or content version.

### Atomic automatic closure

When the rechecker returns `definitively_unavailable`, the existing
publication transaction writes first-missing provenance and closes the
presence and active content periods. The stable closure reason remains
`detail_definitively_unavailable`; the safe recheck code identifies the
reviewed source semantic. No operator SQL is used, so chart history remains
replayable from the run and audit evidence.

## Test seams

The user-approved behavior is tested at these public domain boundaries:

1. `DetailRechecker.Recheck`: classifies a fixed request/response sequence;
   catches false closure caused by source or transport ambiguity, but does not
   test persistence.
2. `Repository.Publish`: exposes the durable visible-period lifecycle;
   catches incorrect closure or stale missing markers, but does not test HTTP.
3. Append-only collection audit events: exposes safe lifecycle provenance;
   catches missing/unsafe observability, but does not inspect raw responses.

## Risks and rollout

- A future source contract change could make `success=false` ambiguous. The
  fixture locks the observed semantic; any unrecognized variant remains
  indeterminate.
- Three requests add bounded latency only for list-absent jobs. They remain
  sequential, and normal listed jobs make no extra requests.
- Deploy through the existing image/Actions workflow. The next normal
  published sync performs the lifecycle transition. Rollback deploys the
  preceding worker; already closed periods remain historical facts and are
  reopened only by a later observed source row.
