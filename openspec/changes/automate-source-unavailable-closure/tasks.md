# Tasks

## 1. Source recheck contract

- [x] 1.1 Add a failing `DetailRechecker` test for three reviewed
  `success=false` responses and verify it remains red under the current
  classifier.
- [x] 1.2 Implement the smallest bounded, sequential three-attempt classifier
  that makes task 1.1 green; retain all non-reviewed source or transport
  responses as indeterminate.
- [x] 1.3 Add regression tests proving that a matching available detail wins
  over negatives and any ambiguous attempt prevents automatic closure.

## 2. Durable lifecycle

- [x] 2.1 Add a failing repository integration test that a detail-available
  list omission clears the missing marker without creating an observation or
  closing the period.
- [x] 2.2 Implement the minimal transactional update and pass task 2.1 while
  preserving the existing definitive-unavailable closure path.
- [x] 2.3 Add and pass a lifecycle/audit regression test for automatic source
  closure and safe stable codes.

## 3. Verification and rollout readiness

- [x] 3.1 Run focused jobsync tests in a permitted test environment, followed
  by relevant integration and full Go regression tests; record the red/green
  evidence.
- [x] 3.2 Run `openspec validate automate-source-unavailable-closure --strict`
  and resolve all findings.
- [x] 3.3 Deploy only through the established immutable-image/GitHub Actions
  workflow; allow the next complete sync to transition verified jobs and
  verify aggregate lifecycle counts without outputting source IDs or JD text.
  Completed by deployment run #29 and one audited 2026-10-28 synchronization:
  four source-confirmed closures, two detail-available list omissions retained,
  and zero remaining open missing markers.
