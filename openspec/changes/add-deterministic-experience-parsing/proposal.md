# Proposal

## Why

Formal JD experience data is currently an AI semantic projection even though
the full stored JD history predominantly uses a bounded set of explicit
year-expression forms. A read-only scan of all 172 stored body versions found
99 Arabic-numeral year expressions and 8 Chinese-numeral expressions; the
dominant form is `N 年以上/及以上`. The product needs a reproducible,
provider-free baseline that never treats a preferred condition, a
degree-conditioned branch, or a non-candidate tenure as a universal work
experience requirement.

## What Changes

- Add a deterministic, versioned parser for candidate work-experience lower
  bounds in normalized JD qualification fields. It recognizes approved Arabic
  and Chinese number forms, ranges, and explicit minimum markers.
- Make one `base_year` the formal result only when it is supported by an
  unconditional, required candidate-experience clause. Where several
  independent required experience lower bounds apply, use their maximum.
- Classify local evidence as `required`, `preferred`, or `uncertain` using
  deterministic clause boundaries and a reviewed priority-marker lexicon.
  Preferred experience must not raise `base_year`.
- Exclude education-conditioned experience branches from the formal overall
  work-experience result. They may be retained as evidence classified as
  conditional, but this change does not model them as a separate product
  dimension.
- Persist immutable parser/lexicon provenance and Unicode source coordinates;
  process unseen JD body versions once during catch-up and incrementally after
  successful daily synchronization, without any Qwen call.
- Preserve existing AI experience projections and audit records unchanged;
  switch the formal experience read boundary to deterministic results without
  an AI fallback.

## Capabilities

### New Capabilities

- `deterministic-experience-requirements`: Derive, persist, process, and
  expose a provider-free formal candidate work-experience lower bound from an
  immutable JD body version.

### Modified Capabilities

- None.

## Impact

- New additive PostgreSQL result and evidence tables, repository APIs, parser
  lexicon, and incremental worker stage in `internal/jdanalysis`.
- The fixed `worker analyze` lifecycle gains an independent deterministic
  experience stage and safe aggregate observability.
- Product-facing formal experience reads move away from AI semantic tables;
  AI prompt, request/response contract, trial policy, and historical results
  are not changed.
- JD modeling objectives and operating guidance gain the formal meaning and
  non-blocking statuses for deterministic experience processing.
