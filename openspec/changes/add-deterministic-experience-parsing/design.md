# Design

## Context

The fixed `worker analyze` operation already runs independent deterministic
project-tag and education stages before the published technology processor.
Education demonstrates the required lifecycle: an immutable parser/lexicon
identity, result and evidence persistence, one-time catch-up, incremental
selection, and a formal deterministic read with no AI fallback. The current
experience representation instead belongs to `ai_experience_summaries` and
cannot be made formal without conflating model provenance with deterministic
policy. See [proposal.md](proposal.md) and the capability spec for required
behavior.

## Goals / Non-Goals

**Goals:**

- Build a small, replayable parser for explicit candidate-experience lower
  bounds, including local priority and education-condition scope.
- Store only versioned decisions and source coordinates; keep original JD text
  in the existing normalized body version.
- Provide a provider-free baseline and daily incremental lifecycle that remains
  successful for `configured`, `not_configured`, and `uncertain` results.

**Non-Goals:**

- Resolve pronouns or arbitrary cross-clause meaning, infer experience from a
  title, or use fuzzy matching, embeddings, or an LLM fallback.
- Create a product dimension for degree-conditioned experience, project tenure,
  seniority labels, salary, or technology proficiency.
- Alter AI configuration, prompts, model calls, audit rows, or historic AI
  experience projections.

## Decisions

### Independent formal experience result and evidence

Add `experience_requirement_results` and
`experience_requirement_evidence`, following the immutable education-result
pattern rather than changing AI tables. A result is uniquely identified by
`(body_content_version_id, parser_version, lexicon_version, lexicon_sha256)`
and has one of `configured`, `not_configured`, or `uncertain`, plus a nullable
positive `base_year`. Evidence records its extracted lower bound, modality,
rule key, source field, normalization identity, and Unicode code-point range.
It never duplicates a JD snippet.

Evidence modalities are `required`, `preferred`, `conditional`, and
`uncertain`. A result is `configured` only when all retained candidate-year
expressions have deterministic scope and at least one is `required`; its
`base_year` is the maximum required lower bound. `not_configured` represents
no required bound, including preferred-only, conditional-only, and explicit
experience-unrestricted text. `uncertain` represents a candidate-year
expression that cannot be safely assigned a local modality, and has no
`base_year`.

Alternative considered: reuse `ai_experience_summaries` or overwrite an AI
`base_year`. This would falsely imply AI provenance, make identity depend on a
model configuration, and violate immutable audit history.

### Reviewed embedded lexicon and bounded number grammar

Introduce an embedded immutable experience lexicon whose version and SHA-256
are persisted with the result. It contains reviewed number aliases (Arabic
positive integers and Chinese forms such as `一` through `十` and `两`), range
separators, lower-bound markers (`以上`, `及以上`, `至少`, `不少于`, `不低于`),
candidate-experience anchors, unrestricted expressions, priority markers,
and education-condition aliases.

The parser recognizes only an approved number-plus-year expression joined to
an approved candidate-experience anchor in the same local requirement atom.
For `N-M 年`, it emits `N`; for explicit lower-bound forms it emits the stated
number. It never turns calendar years, project/company operation duration,
age, or an unanchored number into candidate experience. Chinese-number parsing
is deliberately bounded in v1 rather than using a general natural-language
number interpreter.

Alternative considered: a broad regular expression for every `N 年`. It would
mistake unrelated tenure for candidate experience and cannot provide stable
rule provenance.

### Clause and atom-local modality resolution

The parser first divides each eligible normalized field into strong clauses at
newlines, list boundaries, periods, and semicolons. Within a strong clause it
forms local atoms around candidate-experience expressions and explicit
priority/condition markers. A trailing priority marker, such as `者优先`,
binds the immediately preceding compatible atom; a leading marker, such as
`优先考虑`, binds the following compatible atom. A marker cannot cross a strong
boundary. If a marker could bind more than one candidate expression in the
same atom, the affected expression is `uncertain` rather than guessed.

An education-conditioned atom is recognized only when a reviewed education
alias and a candidate-year expression participate in the same local branch or
explicit branch list. Thus `本科 3 年、硕士 2 年` is conditional, while an
independent `本科及以上；3 年工作经验` clause preserves the three-year required
baseline. A required technology or domain experience remains eligible; a
preferred one does not.

Alternative considered: treating any `优先` or degree word anywhere in the JD
as global. The full-history scan shows that would wrongly discard many
otherwise explicit requirements.

### Deliberately narrow source-field policy

`jobRequire` is qualification scope by default. `description` is considered
only inside an explicit qualification heading, and `addition` may retain
preferred evidence but cannot establish a required baseline. Titles, source
metadata, AI artifacts, prompts, and model output are not parser inputs. This
mirrors the deterministic education boundary and prevents role descriptions
or source labels from becoming requirements.

Alternative considered: searching all persisted text. That increases recall
by treating responsibilities and delivery instructions as candidate criteria.

### Fixed operation integration and observability

Add `ProcessPendingExperienceRequirements` as another independent stage in
`worker analyze`. It uses durable missing-identity selection, so the first run
catches up existing body versions and later runs process only a new or changed
body version, parser version, or lexicon revision. It accepts no provider or
credential input and runs even if technology configuration is absent. Logs and
operator output report only safe aggregates by status; they do not expose JD
text or evidence snippets.

`ReadFormalExperienceRequirement` reads only the current deterministic
identity. An unavailable result remains unavailable; it never queries
`ai_experience_summaries` as a fallback.

Alternative considered: adding the parser to an AI trial or publishing a
candidate AI configuration. That would violate the provider-free daily path
and couple a deterministic value to model operations.

## Risks / Trade-offs

- [Unseen wording lowers recall] → Return `not_configured` or `uncertain` and
  extend the reviewed lexicon in a new identity; do not guess.
- [Local punctuation is inconsistent] → Limit priority binding to explicit
  local atoms and use `uncertain` for ambiguous attachment rather than a
  global heuristic.
- [Conditional evidence is useful later] → Persist it with modality and
  coordinates, while explicitly not exposing it as a universal `base_year`.
- [Historical catch-up is database work] → Use the idempotent identity query,
  provider-free execution, bounded safe summaries, and a second-run check.
- [Two experience representations coexist] → Name deterministic data
  `formal`; document AI values as historical/audit projections only.

## Migration Plan

1. Add additive enums, immutable result/evidence tables, constraints, indexes,
   and read/publish APIs. Do not alter AI tables or rows.
2. Develop the parser test-first with table-driven coverage for grammar,
   scope, source coordinates, statuses, and exclusions; then add migration
   backed persistence and processor tests.
3. Wire the independent stage into the fixed worker operation and document the
   formal source of truth and safe aggregate observability.
4. Deploy through the existing CI/CD path. Run `worker analyze` once for the
   historical deterministic baseline; verify only aggregates and repeat once
   to prove idempotence. No Qwen operation is part of this procedure.
5. Roll back by deploying the prior worker and reader. Added immutable rows
   remain historical provenance and no AI record needs restoration.
