# Repair Trend Technology Release Sync

## Problem

Publishing a successor technology dictionary advances the active technology
analysis configuration, but the historical trend reader also requires a
published formal trend release whose technology component points to that same
configuration. The v0.4 operational release did not advance that formal
release, so job filtering used v0.4 while technology trend curves remained
unavailable.

## Scope

- Publish the v0.4 formal trend interpretation as part of the fixed v0.4
  technology release transaction.
- Preserve the existing complete-history gate and immutable trend release
  lifecycle.
- Cover repeated v0.4 release invocation so an already-active v0.4 pointer can
  repair a missing or stale trend formal release without changing dictionary
  facts.
- Add integration coverage proving that the active technology publication and
  published trend technology component remain aligned.

## Non-goals

- Do not rewrite historical JD content or analysis results.
- Do not change trend counting semantics, date coverage, or public DTOs.
- Do not expose dictionary versions or internal release details to users.
