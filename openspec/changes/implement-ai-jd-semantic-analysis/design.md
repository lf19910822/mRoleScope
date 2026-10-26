# Design

## Context

See [proposal.md](proposal.md) and [jd-ai-semantic-analysis spec](specs/jd-ai-semantic-analysis/spec.md). The existing `jdanalysis` package already provides immutable deterministic technology-analysis configurations, per-body work items, run locks, remote evidence coordinates, candidate/published/retired lifecycle, and a published-configuration pointer. It does not call external models or store semantic qualifications. Real normalized body text exists only in remote PostgreSQL/CVM; existing source synchronization must remain successful when downstream analysis is unavailable.

## Goals / Non-Goals

**Goals:**

- Add a separately versioned, remote-only semantic-analysis pipeline which reuses the published deterministic technology results as controlled model context.
- Make every persisted semantic value, relationship and candidate traceable to a body version, immutable AI configuration and CVM-mapped evidence interval.
- Bound external data transfer, per-attempt tokens, total spend, retries, provider failure, logs and review workflow before the first real request.
- Allow a five-body Plus/Flash comparison to validate the exact Plus configuration that will become the first full baseline.

**Non-Goals:**

- Do not alter deterministic mention matching, the daily source collection schedule, COS access, body normalization, or the existing technology publication pointer.
- Do not add project-name extraction, title/`jobSummary` parsing, autonomous model routing, automatic dictionary approval, visitor APIs, UI pages or charts.
- Do not retain raw model prompts/responses, source snippets, secrets or free-form provider errors in PostgreSQL audits, logs, CI, local files or OpenSpec artifacts.

## Decisions

### 1. Introduce a parallel immutable AI configuration and result family

Add a separate AI configuration family rather than adding nullable AI fields to `technology_analysis_configurations`. An AI configuration references exactly one published deterministic technology configuration and freezes the provider/model request profile, prompt SHA-256, strict schema version/digest, input/segment contract version, validator version and non-secret pricing profile. Its status is `candidate`, `comparison`, `published`, `failed` or `retired`; only a `qwen-plus` candidate can be promoted.

Results are unique by `(body_content_version_id, ai_analysis_configuration_id)`. Add durable work items, runs and attempts alongside normalized semantic result tables for experience requirements, one education result, conditional qualifications, requirement groups/options, candidate mentions, semantic evidence, reviews and release events. Store validated structured values and final evidence coordinates, never the input JSON or raw provider response. An attempt keeps only safe IDs, classification, token/charge aggregates and bounded error code.

Alternative considered: extend deterministic result/mention tables. Rejected because a model response has different provenance, error/retry/cost behavior, many non-technology fields, and independent publication eligibility. Alternative considered: persist the entire accepted JSON. Rejected because normalized data supports constraints/querying and avoids retaining an unnecessarily broad response payload.

### 2. Generate evidence spans in CVM and bind AI input to deterministic results

For each body, the worker deterministically segments only the three normalized fields into ordered sentence/clause spans. A span ID is request-local; it maps in memory to source field, normalization version, field SHA-256 and Unicode code-point `[start,end)`. The segmenter version is part of the input contract and configuration digest.

The request also contains only verified mention evidence from the referenced published deterministic configuration: opaque evidence ID, stable `technology_key`, canonical name, field and intersecting span ID. The full dictionary is never sent. The validator resolves every model reference against this exact body/input map and persists mapped coordinates, not model character counts.

Alternative considered: trust model-provided Unicode positions. Rejected because multilingual position counting is not a reliable model task. Alternative considered: submit only body text. Rejected because it repeats deterministic extraction and makes technology relationship identity unstable.

### 3. Treat the prompt and response schema as a strict, model-independent contract

Store a reviewed prompt template and a canonical strict JSON Schema in the worker image/repository, with version and digest recorded in the AI configuration. The prompt uses opaque request IDs, span IDs and verified technology IDs; it prohibits prose, coordinates, unrecognized fields, external knowledge, text quotation, indentation and avoidable whitespace. It requires a compact JSON object so output budget is spent on conclusions rather than formatting. The provider adapter translates the canonical schema to the provider’s structured-output request format, records only a bounded completion/length signal, and verifies strict nested schema constraints with synthetic tests before real use.

The validator runs in this order: transport success; one JSON object; schema/echo validation; ID/reference validation; semantic invariants; final remote coordinate/candidate validation. Failure at any step produces no valid semantic result and no raw-response persistence. The model's discrete review recommendation is accepted only after program rules can raise it for uncertain/invalid conditions.

Alternative considered: parse Markdown/JSON heuristically. Rejected because it increases cost and makes failures/non-redaction unpredictable. Alternative considered: model-specific prompt behavior embedded in application code without a frozen digest. Rejected because it prevents cache and release reproducibility.

### 4. Preserve semantic meaning conservatively and normalize only validated conclusions

The validator maps the approved v0.1 contract into normalized tables:

- `configured`, `not_configured` and `uncertain` remain distinct from work execution state (`pending`, `running`, `complete`, `failed_safely`); only a completed, validated `not_configured` becomes the established zero-value display mapping.
- Experience stores explicit positive lower bounds and a calculated `base_year`; a valid summary may retain `base_year` with no detailed items. Explicit education/experience variants are separately preserved without inferring absent branches. A single, source-exact, non-alternative domain may be retained with the overall item; alternative or composite domain lists retain only the overall item until a later domain-relation contract exists.
- Education stores one configured minimum level, or a separate non-configured/uncertain state. Explicit incompatible qualification conditions follow the first-release `jobRequire` precedence rule.
- Requirement groups retain role, necessity, `at_least`, minimum satisfaction count and only verified technology options. Separately stated positive qualifications become independent single-option groups; only an explicit connector permits them to be combined. Responsibilities/mentions do not enter candidate matching. Explicit proficiency maps to P1–P4; omitted proficiency remains null.
- The model ranks no more than three external candidates. Only source-exact technical names or abbreviations from explicit required/preferred qualifications enter pending dictionary governance. Invalid, generic, over-limit, responsibility, workflow and ordinary-mention candidates are ignored locally without rejecting independent results. A source-exact term behind an unsupported model normalization is retained as a distinct pending-review artifact, never as a technology candidate, domain or matching input.

Alternative considered: compact all conclusions into one JSON column. Rejected because it cannot enforce option counts, foreign keys, evidence validity, candidate governance or queryable review state. Alternative considered: infer unstated proficiency/relations from domain knowledge. Rejected by the agreed conservative extraction policy.

### 5. Use a staged five-body comparison before expanding the Plus candidate

A controlled preparation operation creates two immutable configurations with the same prompt/input/validator contract: a `qwen-plus` candidate and a `qwen-flash` comparison configuration. It deterministically selects and freezes five remote body IDs using a versioned, non-text stratification policy based on body-length bands, field completeness and verified-mention density. It creates no work outside that frozen trial set.

Each logical task has at most three physical attempts. Both trial configurations use the identical 10,000-input/5,000-output request profile; 10 logical tasks, 30 attempts and 2.50 RMB are hard aggregate limits. The monetary ceiling remains deliberately unchanged as a defence in depth; the smaller frozen sample and attempt limits independently bound trial capacity. The controlled budget precheck uses the frozen pricing profile before each physical request and incorporates returned usage when available.

All valid trial items are reviewed remotely. Only after the Plus trial passes mechanical checks and has zero major semantic errors may a separate fixed operation enqueue the remaining eligible bodies under the **same** Plus candidate configuration. Flash remains comparison-only. The final release gate requires complete Plus work, required formal sampling/reviews, and the reviewed trial attestation; promotion makes Plus the sole default for new/changed body versions.

Alternative considered: change token limits after the trial in the same configuration. Rejected because the reviewed sample would no longer test the released request profile. Alternative considered: automatically route each body to Flash when cheaper. Rejected because it makes result provenance and comparison unstable.

### 6. Keep external calls and operations narrow, idempotent and redacted

Use a provider interface with a Qwen implementation that receives a prebuilt in-memory request and returns a bounded response/usage record. Every provider request derives its origin exclusively from a validated Beijing Workspace ID as `https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com/compatible-mode/v1`; it cannot use the legacy DashScope domain or accept an arbitrary host. The non-secret Workspace ID and API credential are supplied only from the CVM’s root-owned runtime configuration to AI one-shot service environments; the API credential is not an operation parameter, SQL value, container label or log field. The endpoint origin is part of the request-profile digest, so changing Workspace creates a new immutable candidate. The service has no source/COS fetch path, no edge port and no raw-data mount.

Retry only temporary network failures, provider 429 and provider 5xx with exponential backoff/`Retry-After`; authentication, schema, evidence, policy and configuration failures are terminal for automatic execution. A pre-send budget reservation is durable before an external request; recovery reconciles a reservation without a durable attempt as an unknown-outcome timeout under the same deterministic attempt number, so it cannot be resent for free. If that reconciliation exhausts the third attempt, it closes the work and interrupted run as a safe failure rather than leaving an unclaimable pending item. The existing sync transaction only enqueues newly eligible work for the published AI configuration and never waits on external AI.

Expose fixed root-owned operations for trial preparation, controlled candidate execution, Plus expansion, release, safe status and redacted logs. They resolve the fixed candidate/profile internally and accept only fixed confirmation phrases; no caller can provide model name, prompt, schema, body ID list, price, command, SQL, path, image or secret. A separate direct-CVM terminal review session has the smaller interface `review-plus` or `review-flash`; it selects its own next artifact, prints text only to its attached terminal, accepts only a fixed verdict/category vocabulary and records the reviewer as `FAN`. It is deliberately absent from GitHub Actions and journal-backed systemd units.

Alternative considered: call the provider from source sync. Rejected because an external failure or spend gate must not delay an otherwise valid source publication. Alternative considered: permit interactive free-form provider commands for experiments. Rejected because this violates the data, budget and reproducibility boundary.

### 7. Make review and release attestations part of the durable safety gate

Review records are immutable and attach to exact semantic artifact/result/configuration versions. The remote review command presents text only in the CVM session and writes bounded verdicts/error categories; safe summaries contain IDs/counts only. Review policy is versioned separately from semantic configuration so changing an ordinary sample percentage does not mutate results.

The first release gate requires all five Plus trial bodies fully reviewed, zero major semantic errors, complete eligible Plus results, all `required_review` artifacts reviewed, 20% stratified `spot_check` review and 5% stratified `routine` review. A major error reverses necessity, alternatives, explicit education or explicit experience meaning. Release writes an immutable redacted attestation; a later policy/model/prompt/input change creates a new candidate rather than changing the published configuration.

Alternative considered: publish after zero-exit model execution. Rejected because valid JSON is not semantic validation. Alternative considered: carry review verdicts into a new configuration. Rejected because a changed prompt/model/input can produce different interpretations.

## Risks / Trade-offs

- [External provider receives approved body text] → Limit it to the three normalized fields, require the approved regional/data boundary, use a root-owned secret, and retain no request/response outside process memory.
- [Pricing/profile changes before execution] → Require a controlled current pricing profile and budget precheck; do not start if it cannot bound cost.
- [A long body exceeds the initial profile] → Mark it safely not analyzed; do not truncate. A later segmentation/profile change needs a new candidate configuration.
- [Model spends its response budget on legal JSON whitespace] → Require compact JSON, retain a bounded completion/length signal, increase the immutable output profile to 5,000 tokens, and still reject incomplete objects rather than repairing them.
- [Five reviewed bodies miss a systematic issue] → Stratify selection, inspect all trial results, retain major-error stop rules and continue formal review sampling after expansion.
- [Semantic tables grow quickly] → Body text remains deduplicated in existing body versions; store only structured values and coordinate evidence, then monitor safe row/count growth before defining retention.
- [CVM/service deployment lacks a valid credential] → Deploy migrations/runtime with AI operations unavailable; source sync and deterministic analysis continue independently.

## Migration Plan

1. Add forward-only schema, sqlc models/queries and redacted repositories for AI configurations, work/runs/attempts, normalized semantic results/evidence/candidates/reviews, usage and release attestations. Keep existing deterministic tables and publication state unchanged.
2. Build the deterministic span generator, canonical prompt/schema, semantic validator, candidate revalidation and synthetic unit/PostgreSQL integration tests before adding a provider client.
3. Add the Qwen adapter, pricing-profile/budget guard, retry classifier and synthetic transport tests; validate provider structured-output behavior only with synthetic fixtures.
4. Add the no-port worker subcommand, Compose service, root-owned fixed operations, deployment validation and operational guidance. Keep all AI operations disabled until a root-owned credential and provider/profile checks are present.
5. Deploy additive assets; prepare the frozen five-body Plus/Flash trial; execute only after separate confirmation; remotely review all Plus trial results and record safe comparison summaries.
6. On a passing Plus trial, expand that same configuration to remaining eligible bodies, complete formal reviews, and perform a separately confirmed controlled first publication. New published-body work then uses the released Plus configuration.

Rollback: disable AI-specific helpers/services and prevent new AI operations while leaving source synchronization and deterministic technology analysis running. Preserve all additive configurations, results, attempts, review and release records. A failed/candidate/comparison configuration never changes the AI publication pointer; a published configuration follows the controlled retirement/withdrawal policy introduced with this capability rather than deletion.
