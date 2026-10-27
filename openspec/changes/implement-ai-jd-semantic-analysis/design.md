# Design

## Context

See [proposal.md](proposal.md) and [jd-ai-semantic-analysis spec](specs/jd-ai-semantic-analysis/spec.md). The existing `jdanalysis` package already provides immutable deterministic technology-analysis configurations, per-body work items, run locks, remote evidence coordinates, candidate/published/retired lifecycle, and a published-configuration pointer. It does not call external models or store semantic qualifications. Real normalized body text exists only in remote PostgreSQL/CVM; existing source synchronization must remain successful when downstream analysis is unavailable.

## Goals / Non-Goals

**Goals:**

- Add a separately versioned, remote-only semantic-analysis pipeline which reuses the published deterministic technology results as controlled model context.
- Make every persisted semantic value, relationship and candidate traceable to a body version, immutable AI configuration and CVM-mapped evidence interval.
- Bound external data transfer, per-attempt tokens, total spend, retries, provider failure, logs and review workflow before the first real request.
- Use one fixed five-body Qwen Plus trial to validate the request path and structural contract of the immutable configuration that will become the first full baseline.

**Non-Goals:**

- Do not alter deterministic mention matching, the daily source collection schedule, COS access, body normalization, or the existing technology publication pointer.
- Do not add project-name extraction, title/`jobSummary` parsing, autonomous model routing, automatic dictionary approval, visitor APIs, UI pages or charts.
- Do not retain raw model prompts/responses, source snippets, secrets or free-form provider errors in PostgreSQL audits, logs, CI, ordinary local files or OpenSpec artifacts. The separately scoped CVM root-only candidate-output audit is the sole exception for raw responses.

## Decisions

### 1. Introduce a parallel immutable AI configuration and result family

Add a separate AI configuration family rather than adding nullable AI fields to `technology_analysis_configurations`. An AI configuration references exactly one published deterministic technology configuration and freezes the provider/model request profile, prompt SHA-256, strict schema version/digest, input/segment contract version, validator version and non-secret pricing profile. New configurations use `candidate`, `published`, `failed` or `retired`; only a `qwen-plus` candidate can be promoted. Existing historical `comparison` rows remain readable immutable history, but no future operation creates or selects them.

Results are unique by `(body_content_version_id, ai_analysis_configuration_id)`. Add durable work items, runs and attempts alongside normalized semantic result tables for experience requirements, one education result, conditional qualifications, requirement groups/options, candidate mentions, semantic evidence, reviews and release events. Store validated structured values and final evidence coordinates, never the input JSON or raw provider response. An attempt keeps only safe IDs, classification, token/charge aggregates and bounded error code.

Alternative considered: extend deterministic result/mention tables. Rejected because a model response has different provenance, error/retry/cost behavior, many non-technology fields, and independent publication eligibility. Alternative considered: persist the entire accepted JSON. Rejected because normalized data supports constraints/querying and avoids retaining an unnecessarily broad response payload.

### 2. Generate evidence spans in CVM and bind AI input to deterministic results

For each body, the worker deterministically segments only the three normalized fields into ordered sentence/clause spans. A span ID is request-local; it maps in memory to source field, normalization version, field SHA-256 and Unicode code-point `[start,end)`. The segmenter version is part of the input contract and configuration digest.

The request also contains only verified mention evidence from the referenced published deterministic configuration: opaque evidence ID, stable `technology_key`, canonical name, field and intersecting span ID. The full dictionary is never sent. The processor resolves every persistable reference against this exact body/input map and persists mapped coordinates, not model character counts; an unresolvable reference is locally removed rather than becoming cross-body evidence or rejecting unrelated data.

Alternative considered: trust model-provided Unicode positions. Rejected because multilingual position counting is not a reliable model task. Alternative considered: submit only body text. Rejected because it repeats deterministic extraction and makes technology relationship identity unstable.

### 3. Treat the prompt and response schema as a strict, model-independent contract

Store a reviewed prompt template and a canonical strict JSON Schema in the worker image/repository, with version and digest recorded in the AI configuration. The current candidate freezes `ai-jd-result-v4` and `ai-semantic-validator-v4`; V1–V3 remain readable only for immutable historical records and audit reconstruction. The prompt uses opaque request IDs, span IDs and verified technology IDs; it prohibits prose, coordinates, unrecognized fields, external knowledge, text quotation, indentation and avoidable whitespace. It requires a compact JSON object so output budget is spent on conclusions rather than formatting. The provider adapter translates the canonical schema to the provider’s structured-output request format, records only a bounded completion/length signal, and verifies strict nested schema constraints with synthetic tests before real use.

The validator has two deliberately separate layers. The hard layer verifies transport framing, exactly one complete JSON object, the frozen schema version, and echo/request-to-body binding; a failure there produces no normalized result because the program must not invent structure. The soft layer resolves known span/evidence IDs, maps final coordinates, and checks persistence prerequisites. An invalid reference is removed locally; a conclusion with remaining representable data is persisted. A semantic disagreement (necessity, relation, proficiency, experience, education, role or evidence-role correctness) is a model conclusion and later audit material, not a rejection reason. A formal technology option whose key cannot resolve to a governed `technology_identity` is the sole local option drop required by the relational model; it cannot create a new identity or invalidate unrelated fields. Candidate terms that cannot be represented safely are similarly ignored locally.

Alternative considered: parse Markdown/JSON heuristically. Rejected because it increases cost and makes failures/non-redaction unpredictable. Alternative considered: model-specific prompt behavior embedded in application code without a frozen digest. Rejected because it prevents cache and release reproducibility.

### 4. Persist structurally valid model projections with local boundary degradation

The validator maps the approved v0.1 contract into normalized tables:

- `configured`, `not_configured` and `uncertain` remain distinct from work execution state (`pending`, `running`, `complete`, `failed_safely`). A structurally valid model value is stored under the state the model supplied; it is not reclassified by a semantic policy gate. `base_year` may legitimately exist with an empty item list.
- Experience, education, condition branches, roles, necessity, alternatives and P1–P4/null are stored as the model’s structured conclusion wherever the normalized shape can represent them. The application does not infer omitted meaning or independently overrule the conclusion. Review records can later identify quality issues without changing or blocking the result.
- Each formal requirement option must still resolve to an existing governed `technology_identity`, because its table has a non-null foreign key and drives matching/statistics. Unknown, obsolete or unmappable option keys are dropped individually. Invalid span/evidence IDs are removed individually, including a bad ID alongside valid IDs; no persisted evidence may point at another body. The remaining option or conclusion stays when its own persistence prerequisites are met.
- The response schema caps `candidate_mentions` at three. Candidate storage uses the candidate text as a pending-governance input, not a formal technology identity or valid option. It does not re-adjudicate whether the model’s claimed role is a responsibility, ordinary mention or qualification. A transactionally counted global maximum of 500 distinct `pending_review` candidate terms applies; approval or rejection ends pending state and frees capacity, while historical rows remain. At capacity, only a new candidate is ignored; all other response data continues.

Alternative considered: compact all conclusions into one JSON column. Rejected because it cannot enforce option counts, foreign keys, evidence isolation, candidate governance or queryable review state. Alternative considered: infer unstated proficiency/relations from domain knowledge. Rejected because the release persists the model’s explicit structured projection rather than adding a second program interpretation.

### 5. Use a staged five-body Plus trial before expanding the candidate

A controlled preparation operation creates one immutable `qwen-plus` candidate configuration. It deterministically selects and freezes five remote body IDs using a versioned, non-text stratification policy based on body-length bands, field completeness and verified-mention density. It creates no work outside that frozen trial set. Historical Flash comparison configurations remain untouched and are not a source for membership or release decisions.

Each logical task has at most three physical attempts. The Plus trial uses a frozen `10,000`-input/`5,000`-output request profile; five logical tasks, 15 attempts and 2.50 RMB are hard aggregate limits. The controlled budget precheck uses the frozen pricing profile before each physical request and incorporates returned usage when available.

Review is available for quality learning and does not gate expansion or publication. After the trial’s work reaches terminal states, a separate fixed operation can enqueue remaining eligible bodies under the **same** Plus candidate configuration. Manual publication requires only mechanical completion: each eligible work item is terminal, every structurally valid response has been persisted, and safe failure counts are attestable. It does not require a zero-error semantic finding, a review percentage, an audit verdict, or a Flash comparison. Promotion makes Plus the sole default for future new/changed body versions.

Alternative considered: change token limits after the trial in the same configuration. Rejected because the frozen sample would no longer test the released request profile. Alternative considered: automatically route each body to Flash when cheaper. Rejected because future Flash routing is out of scope and would make provenance unstable.

### 6. Keep external calls and operations narrow, idempotent and redacted

Use a provider interface with a Qwen implementation that receives a prebuilt in-memory request and returns a bounded response/usage record. Every provider request derives its origin exclusively from a validated Beijing Workspace ID as `https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com/compatible-mode/v1`; it cannot use the legacy DashScope domain or accept an arbitrary host. The non-secret Workspace ID and API credential are supplied only from the CVM’s root-owned runtime configuration to AI one-shot service environments; the API credential is not an operation parameter, SQL value, container label or log field. The endpoint origin is part of the request-profile digest, so changing Workspace creates a new immutable candidate. The service has no source/COS fetch path, no edge port and no raw-data mount.

Retry only temporary network failures, provider 429 and provider 5xx with exponential backoff/`Retry-After`; authentication, malformed structured output/binding, and configuration failures are terminal for automatic execution. Soft semantic/reference defects do not trigger a retry because their independently representable conclusions are locally persisted after local sanitation. A pre-send budget reservation is durable before an external request; recovery reconciles a reservation without a durable attempt as an unknown-outcome timeout under the same deterministic attempt number, so it cannot be resent for free. If that reconciliation exhausts the third attempt, it closes the work and interrupted run as a safe failure rather than leaving an unclaimable pending item. The existing sync transaction only enqueues newly eligible work for the published AI configuration and never waits on external AI.

Expose fixed root-owned operations for Plus trial preparation, controlled candidate execution, Plus expansion, manual publication, safe status and redacted logs. They resolve the fixed candidate/profile internally and accept only fixed confirmation phrases; no caller can provide model name, prompt, schema, body ID list, price, command, SQL, path, image or secret. A separate direct-CVM terminal review session has the smaller interface `review-plus`; it selects its own next artifact, prints text only to its attached terminal, accepts only a fixed verdict/category vocabulary and records the reviewer as `FAN`. It is deliberately absent from GitHub Actions and journal-backed systemd units. Existing Flash review history remains immutable but no future Flash review is scheduled.

Alternative considered: call the provider from source sync. Rejected because an external failure or spend gate must not delay an otherwise valid source publication. Alternative considered: permit interactive free-form provider commands for experiments. Rejected because this violates the data, budget and reproducibility boundary.

### 7. Keep review immutable but make publication a manual mechanical operation

Review records are immutable and attach to exact semantic artifact/result/configuration versions. The remote review command presents text only in the CVM session and writes bounded verdicts/error categories; safe summaries contain IDs/counts only. They are audit/training material: no verdict, error category, sample coverage, or review recommendation can block an otherwise mechanically eligible configuration.

The manual publication operation verifies terminal work completion, durable structured-result persistence where the response passed the hard contract, fixed configuration identity, and a redacted count/usage/failure attestation. It does not score semantic correctness or require reviews. A malformed/truncated/non-JSON response remains an auditable terminal safe failure, never becomes invented structured data, and does not prevent other bodies, daily synchronization, or eventual publication of a mechanically complete configuration. A later policy/model/prompt/input change creates a new candidate rather than changing the published configuration.

Alternative considered: treat a reviewer’s quality judgment as a first-release gate. Rejected for this delivery stage: model correctness must be visible in the future product and improved iteratively, rather than blocking a usable processing pipeline. Carrying review verdicts into a new configuration remains invalid because a changed prompt/model/input can produce different interpretations.

## Risks / Trade-offs

- [External provider receives approved body text] → Limit it to the three normalized fields, require the approved regional/data boundary, use a root-owned secret, and retain no request/response outside process memory.
- [Pricing/profile changes before execution] → Require a controlled current pricing profile and budget precheck; do not start if it cannot bound cost.
- [A long body exceeds the initial profile] → Mark it safely not analyzed; do not truncate. A later segmentation/profile change needs a new candidate configuration.
- [Model spends its response budget on legal JSON whitespace] → Require compact JSON, retain a bounded completion/length signal, increase the immutable output profile to 5,000 tokens, and still reject incomplete objects rather than repairing them.
- [A structurally valid model conclusion is semantically wrong] → Persist immutable model provenance, retain optional CVM-only review/audit material, and improve a later candidate prompt/contract; do not silently rewrite or block the current delivery pipeline.
- [Semantic tables grow quickly] → Body text remains deduplicated in existing body versions; store only structured values and coordinate evidence, then monitor safe row/count growth before defining retention.
- [CVM/service deployment lacks a valid credential] → Deploy migrations/runtime with AI operations unavailable; source sync and deterministic analysis continue independently.

## Migration Plan

1. Add forward-only schema, sqlc models/queries and redacted repositories for AI configurations, work/runs/attempts, normalized semantic results/evidence/candidates/reviews, usage and release attestations. Keep existing deterministic tables and publication state unchanged.
2. Build the deterministic span generator, canonical prompt/schema, hard structural/binding validator, local soft-boundary degradation and synthetic unit/PostgreSQL integration tests before adding a provider client.
3. Add the Qwen adapter, pricing-profile/budget guard, retry classifier and synthetic transport tests; validate provider structured-output behavior only with synthetic fixtures.
4. Add the no-port worker subcommand, Compose service, root-owned fixed operations, deployment validation and operational guidance. Keep all AI operations disabled until a root-owned credential and provider/profile checks are present.
5. Deploy additive assets; prepare the frozen five-body Plus trial and execute it only after separate confirmation. Any review is optional audit material and does not cause another model call.
6. After all trial work is terminal, expand that same configuration to remaining eligible bodies, then use a separately confirmed manual publication operation after mechanical completion. New/changed body versions then use the released Plus configuration.

Rollback: disable AI-specific helpers/services and prevent new AI operations while leaving source synchronization and deterministic technology analysis running. Preserve all additive configurations, results, attempts, review and release records. A failed/candidate/historical-comparison configuration never changes the AI publication pointer; a published configuration follows the controlled retirement/withdrawal policy introduced with this capability rather than deletion.
