# Design

## Context

The current normalized minimum-education row belongs to an AI semantic result
and is therefore tied to one immutable Qwen configuration.  Normal daily
analysis is already a provider-free, best-effort worker that runs after a
successful source synchronization, but it currently executes only the
published technology dictionary processor.  See [proposal.md](proposal.md)
for the motivation and the new capability spec for observable behavior.

## Goals / Non-Goals

**Goals:**

- Derive one formal education constraint from an immutable JD body version
  without an AI request, credential, or model configuration.
- Keep rule/lexicon provenance and source coordinates sufficient to replay the
  decision against the same normalized field.
- Preserve lower and upper bounds separately, so the data model does not turn
  `本科及以下` into a false minimum-bachelor requirement.
- Run a one-time historical catch-up followed by idempotent daily processing
  for only unseen body versions.

**Non-Goals:**

- Parsing school names, disciplines, full-time status, foreign-equivalency,
  education/experience exchange conditions, or candidate CVs.
- Removing, rewriting, reinterpreting, or backfilling historic AI semantic
  projections.
- Changing the Qwen request, prompt, response schema, trial size, budget, or
  release state in this change.

## Decisions

### Independent deterministic education result

Introduce `education_requirement_results` and
`education_requirement_evidence`, rather than reuse
`ai_minimum_education_requirements`.  A result is unique on
`(body_content_version_id, parser_version, lexicon_version)` and records a
status plus nullable `lower_bound_level` and `upper_bound_level`.  Evidence
stores the selected rule key, modality, source-field normalization provenance,
and code-point range; it does not store a duplicated JD snippet.

The level enum is independent from the AI enum and has the four ordered
values `associate_or_lower`, `bachelor`, `master`, and `doctor`. The product
display names are `专科及以下`、`本科`、`硕士`、`博士`. This avoids treating the
overlapping natural-language label `本科及以下` as an exclusive minimum level.
An actual `本科及以下` phrase produces an upper bound of `bachelor`; it has no
invented lower bound.

Alternative considered: write deterministic values into the existing AI table.
That would misstate their provenance, make them disappear behind AI candidate
identity, and require changing immutable historic records. It is rejected.

### Immutable embedded lexicon and deterministic parser

The first parser uses a reviewed embedded lexicon asset with a stable version
and SHA-256 digest. Each entry has a unique rule key, canonical level,
case/boundary policy, and literal aliases. The asset contains at least:

| Level | Initial aliases |
| --- | --- |
| `associate_or_lower` | 专科、大专、高职、高专、中专、中职、职高、职校、技校、技工学校、职业高中、高中 |
| `bachelor` | 本科、大学本科、学士、学士学位、统招本科、全日制本科、专升本 |
| `master` | 硕士、硕士研究生、研究生、专业硕士、工程硕士、MBA、EMBA、MPA、MEM、硕博 |
| `doctor` | 博士、博士研究生、博士学位、PhD、Ph.D.、DBA |

The parser applies longest-alias matching before generic terms, preserves
original code-point coordinates, and uses explicit ASCII identifier
boundaries for Latin aliases. `博士研究生` consequently beats `研究生`; the
product decision maps standalone `研究生` to `master`. A standalone `博士后`
or a responsibility phrase such as `研究生项目` is not a candidate education
requirement.

Alternative considered: a general fuzzy matcher or an LLM fallback. Both make
the result non-reproducible and are outside the explicit-vocabulary objective.

### Clause-scoped qualification and modality resolution

The parser splits normalized fields into bounded clauses, then evaluates each
degree occurrence in its clause. `jobRequire` is qualification scope by
default. `description` is eligible only inside an explicit qualification
heading; `addition` can retain preferred evidence but cannot create a hard
bound. Explicit priority markers (`优先`, `加分`, `更佳`, `优先考虑`) make only
their local term preferred. `学历不限`, `不限制学历`, and equivalent expressions
produce no hard bound.

Required lower markers include `及以上`, `以上`, `不低于`, `至少`, and ordinary
unmodified degree requirements in a qualification clause. `及以下`, `以下`,
and `不高于` create an upper bound. Explicit `或`/`/` alternatives select their
lowest permitted required level. Incompatible unscoped hard requirements are
reported as `uncertain`, with all competing evidence retained.

Alternative considered: globally select the first degree word found in a JD.
It misreads `硕士优先，本科及以上`, responsibilities, and upper-bound phrases.

### Daily lifecycle is independent from AI and technology success

Add an `EducationProcessor` to the existing fixed `worker analyze` operation.
It runs after synchronization, has no caller-controlled parser, body, query,
or credential input, and processes any body version missing the current
parser/lexicon result. It runs independently of the technology processor: a
missing technology publication or a technology-processing failure cannot
prevent durable education processing, and the reverse is also true. The
existing best-effort behavior keeps source synchronization successful when
downstream analysis fails.

The first deployed run treats existing versions as unseen and creates their
one-time baseline. Thereafter a changed JD has a different body-content
version and is processed once; unchanged daily observations do not create
another result. Parser or lexicon changes use new immutable versions and
therefore intentionally create a fresh reanalysis result.

### Product read has no AI fallback

Add a formal education read seam that queries the current deterministic parser
and lexicon version for the exact body-content version. The product uses that
seam exclusively. An absent result is reported as unavailable rather than
falling back to an AI projection; this makes deployment/backfill state visible
instead of silently reintroducing model dependence. AI education values remain
available only in their existing historical/audit read paths.

## Risks / Trade-offs

- [Lexicon omissions lower recall] → The lexicon is a reviewed, versioned
  asset; additions produce a new version and a deliberate reanalysis.
- [Qualification wording varies by source] → Restrict v1 to explicit degree
  vocabulary and return `not_configured` or `uncertain` rather than guess.
- [Initial historical baseline adds database work] → The processor is
  idempotent, bounded by existing body versions, and does not call a provider.
- [Two education representations may confuse readers] → Name APIs and product
  fields `deterministic`/`formal`; document AI values as historical projections.

## Migration Plan

1. Add additive tables, enums, provenance constraints, and indexes; no AI
   table or historic result is altered.
2. Ship the parser and processor with red/green unit and PostgreSQL integration
   coverage, then make the standard fixed analysis operation invoke it.
3. Run the normal analysis service once to create the historical deterministic
   baseline, validating aggregate counts only in ordinary operations output.
4. Verify a second run is idempotent and that a newly inserted body version is
   the only newly processed result.
5. Switch the product read to the deterministic seam. Rollback consists of
   deploying the prior reader/worker; additive deterministic rows remain
   historical evidence and AI rows are untouched.
