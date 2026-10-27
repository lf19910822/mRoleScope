# Spec Delta

## Purpose

该能力从远端版本化 JD 正文生成可审计的 AI 语义结果，同时将真实文本、模型请求和原始响应限制在 CVM 与经批准的外部模型服务之间。

## ADDED Requirements

### Requirement: Immutable AI analysis configurations
The system SHALL create AI semantic results only under an immutable candidate or published configuration. A configuration SHALL freeze the provider/model request profile, prompt digest, response-schema version, input/segment contract, validator version, and referenced published technology-analysis configuration. A semantic change SHALL create a new candidate configuration and SHALL NOT overwrite prior results.

#### Scenario: Reuse under an identical configuration
- **WHEN** the same body-content version is requested under the same AI configuration
- **THEN** the system reuses its existing valid result and does not make another external model request

#### Scenario: Prompt change creates a candidate
- **WHEN** the prompt content or response-schema version changes
- **THEN** the system creates a distinct candidate configuration and preserves all results from the previous configuration

### Requirement: Versioned and bounded AI input
The system SHALL send only normalized `description`, `jobRequire`, and `addition` text from a remote body-content version, its CVM-generated evidence span identifiers, and the body’s verified technology mentions. The system SHALL NOT send title, `jobSummary`, delivery data, user data, COS objects, source pages, credentials, or a full technology dictionary.

#### Scenario: Metadata-only source change
- **WHEN** title, location, or source labels change without a body-content version change
- **THEN** the system does not create a new AI analysis request

#### Scenario: Oversized body input
- **WHEN** a trial input exceeds the configured token limit
- **THEN** the system does not truncate or send it, records a safe `input_too_large` outcome, and represents the semantic result as not analyzed

### Requirement: Current controlled response contract
The current controlled candidate SHALL freeze `ai-jd-result-v4` and `ai-semantic-validator-v4`. Older versions remain readable immutable history and SHALL NOT be selected for new work.

#### Scenario: New candidate uses the current contract
- **WHEN** a fixed Plus trial is prepared for a published deterministic baseline
- **THEN** its immutable configuration records the V4 prompt, schema and validator digests

### Requirement: Strict structured model response
The system SHALL send the frozen strict JSON Schema and a prompt requiring compact JSON without prose or Markdown. It SHALL accept a normalized result only when one complete object conforms to the schema and its schema version and `request_id` bind to the requested body. The hard gate SHALL NOT evaluate business correctness.

#### Scenario: Valid structured response
- **WHEN** the response matches the schema and all required opaque request identifiers
- **THEN** the system persists every independently representable conclusion after local reference sanitation

#### Scenario: Invalid or mixed response
- **WHEN** the response contains explanatory text, schema violations, unknown fields, or a mismatched request identifier
- **THEN** the system does not invent or persist a normalized semantic result, stores only a safe error category, and does not block other work; a candidate’s raw output may only be retained under the separate CVM output-audit policy

#### Scenario: Output limit interrupts an object
- **WHEN** the provider reports a completion-limit signal or the returned JSON object is incomplete
- **THEN** the system does not repair or persist the partial conclusion, records a bounded safe outcome, and preserves the other work items' independent execution

### Requirement: Program-owned semantic evidence locations
The system SHALL map every persistable model-referenced span identifier and verified technology-evidence identifier to the requested body’s source field, normalization version, field hash, and Unicode code-point half-open interval. It SHALL never persist an unsupported, unknown, cross-body, or invalid reference. Invalid references SHALL be removed locally without reinterpreting or discarding other independently representable model conclusions.

#### Scenario: Model references a valid span
- **WHEN** a semantic conclusion references a span generated for its input body version
- **THEN** the persisted evidence uses the CVM-mapped field/version/hash/interval rather than a model-calculated coordinate

#### Scenario: Model references an unknown span
- **WHEN** a response references a span identifier absent from the input contract
- **THEN** the system removes that reference, persists any remaining representable conclusion without it, and records no evidence that could point outside the requested body

### Requirement: Structurally valid semantic projection
After the hard response gate, the system SHALL treat experience, education, qualifications, roles, necessity, relation, proficiency and model-supplied state as a model projection. It SHALL keep `not_analyzed`, `not_configured`, and `uncertain` distinct from execution state, but SHALL NOT reject, rewrite, or block a projection for semantic disagreement. A model-supplied `base_year` MAY have an empty item list.

#### Scenario: No explicit experience requirement
- **WHEN** a completed analysis finds no explicit candidate experience lower bound or finds an explicit no-experience condition
- **THEN** it stores the model’s structured `not_configured` conclusion and does not convert it to a zero-year requirement

#### Scenario: Education-dependent experience
- **WHEN** a valid span explicitly gives different experience lower bounds for different education levels
- **THEN** the system persists the model-supplied structured base value and conditional branches that fit the normalized shape without inventing data not present in the response

#### Scenario: Uncertain qualification statement
- **WHEN** the model or validator cannot reliably determine a qualification’s meaning
- **THEN** the relevant field remains the model-supplied `uncertain` state rather than being converted to `not_configured`; optional review is non-blocking

#### Scenario: Supported base year without detailed items
- **WHEN** a valid response supplies an overall experience lower bound and valid summary evidence but no `items`
- **THEN** the system persists the configured `base_year` with an empty item list and does not force required review solely for that omission

#### Scenario: Alternative domain experience
- **WHEN** one experience statement names multiple domains connected by an alternative or composite separator and the first-release contract cannot express their relation
- **THEN** the system persists the model’s overall value and every domain item that fits the normalized persistence shape, without treating relation correctness as a failure

### Requirement: Technology requirement semantics
The system SHALL persist the model-supplied `qualification`, `responsibility`, `mention`, or `unspecified` role, necessity, group relation and P1–P4/null proficiency when the normalized shape and technology identity allow it. Matching/statistical consumers SHALL use the stored model role; the program SHALL NOT independently reject an option because it believes the role, necessity, connector or proficiency is semantically wrong.

#### Scenario: Responsibility is not a hard requirement
- **WHEN** a span describes work to be performed with a technology but does not state a candidate qualification
- **THEN** the system stores the model-supplied role if its option is governable; future consumers use that persisted role rather than a second program semantic judgment

#### Scenario: Explicit technology alternatives
- **WHEN** an explicit qualification states that one of several verified technologies is sufficient
- **THEN** the system persists the model-supplied `at_least` relation and minimum when the referenced options are governable

#### Scenario: Ambiguous punctuation-only list
- **WHEN** a multi-technology list has no unambiguous conjunction, alternative, or quantity relation
- **THEN** the system persists the model-supplied relation when it fits the schema; optional review may record a later quality concern but cannot block it

#### Scenario: Separate positive requirements
- **WHEN** two verified technologies occur in separate positive qualification statements without an alternative connector
- **THEN** the system persists the model-supplied group arrangement; it does not independently combine, split, or reject it for semantic reasons

### Requirement: Governed candidate technology mentions
The frozen response schema SHALL contain at most three `candidate_mentions`. The system SHALL admit a structurally valid candidate text to pending dictionary governance independently of the model’s asserted semantic role or evidence-role correctness, subject to a transactionally enforced global limit of 500 distinct `pending_review` candidate terms. A candidate SHALL NOT create a formal technology identifier, active alias, statistic, or valid requirement option.

#### Scenario: Verified external candidate
- **WHEN** a model returns a structurally valid candidate within the schema’s three-item limit and the candidate is not already governed
- **THEN** the system creates or reuses its remote pending dictionary candidate if the 500 pending-term capacity permits it

#### Scenario: Candidate is invalid or beyond the cap
- **WHEN** a candidate cannot be represented safely, is a schema-invalid fourth item, or would create a new pending term after the 500-term capacity is full
- **THEN** the system ignores only that candidate without modifying dictionary state or invalidating the JD's independently valid semantic conclusions

### Requirement: Candidate pending-capacity lifecycle
The system SHALL count only distinct candidate terms whose current governance state is `pending_review` toward the 500-term capacity. Approval or rejection SHALL retain the historical record but remove it from the pending count. Concurrent candidate writes SHALL not exceed the limit.

#### Scenario: Pending candidate is resolved
- **WHEN** a pending candidate term is approved or rejected through its existing governance workflow
- **THEN** its historical record remains available and one pending-capacity slot becomes available for a later distinct candidate

#### Scenario: Concurrent candidate capacity race
- **WHEN** concurrent structurally valid responses try to create distinct pending candidates near the 500-term limit
- **THEN** the transactionally selected requests at or below the limit persist their candidates and later requests ignore only their new candidate mentions

### Requirement: Bounded external execution and safe failure
The system SHALL run AI work independently of source synchronization. A trial logical task SHALL allow at most three physical attempts, retrying only transient transport, rate-limit, or provider-server failures. Every attempt SHALL count toward request, token, and monetary limits; non-transient failures and budget exhaustion SHALL stop safely without retrying or exposing text.

#### Scenario: Transient provider failure
- **WHEN** a model attempt fails with a transient timeout, 429, or 5xx outcome and budget remains
- **THEN** the system performs no more than two delayed retries for that logical task

#### Scenario: Hard-contract validation failure
- **WHEN** a model response fails schema or request/body binding validation
- **THEN** the system does not retry automatically and records only the configured safe error category

#### Scenario: Daily source synchronization succeeds while AI is unavailable
- **WHEN** source synchronization successfully publishes a body version but its AI analysis cannot run or fails safely
- **THEN** the source publication remains successful and the AI work remains recoverable without re-collecting the source

### Requirement: Beijing Workspace-bound provider endpoint
The system SHALL construct every Qwen trial request origin solely as `https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com/compatible-mode/v1`, where `WorkspaceId` comes from root-owned runtime configuration and is validated as one DNS label. The endpoint origin SHALL be included in the frozen request-profile digest. The system SHALL NOT use the legacy DashScope domain or accept a caller-provided region, host, scheme, path, or endpoint.

#### Scenario: Missing or malformed Workspace ID
- **WHEN** any fixed AI lifecycle operation starts without a valid configured Beijing Workspace ID
- **THEN** it fails safely before database initialization or provider network activity

#### Scenario: Workspace endpoint changes
- **WHEN** the configured Beijing Workspace ID changes
- **THEN** the derived request-profile digest and immutable candidate configuration identity change; prior results remain unmodified

### Requirement: Trial and production cost gates
The initial Qwen Plus trial SHALL use five remotely selected body-content versions, at most five logical tasks, at most 15 physical attempts, per-task trial limits of 10,000 input and 5,000 output tokens, and a 2.50 RMB total hard limit. The system SHALL stop before an attempt that would exceed any active limit and SHALL not silently truncate input or output. Existing Flash trial data is immutable history and SHALL NOT create future work.

#### Scenario: Frozen five-body trial membership
- **WHEN** a current controlled trial is prepared
- **THEN** it deterministically freezes exactly five body-content versions for Qwen Plus and never queues a sixth body or any new Flash work

#### Scenario: Trial budget precheck
- **WHEN** the estimated worst-case charge for a next attempt would exceed the 2.50 RMB trial limit
- **THEN** the system does not send the attempt and leaves the affected work not analyzed

#### Scenario: Trial price cannot be verified
- **WHEN** current official pricing or sufficient usage information cannot support the configured budget precheck
- **THEN** the system does not start the external trial

#### Scenario: First baseline reuses the trial request profile
- **WHEN** the first `qwen-plus` configuration advances from its frozen five trial bodies to the remaining eligible bodies
- **THEN** it retains the identical 10,000-input and 5,000-output token limits, and any limit or segmentation change requires a new candidate configuration and renewed trial validation

### Requirement: Manual mechanically-complete AI configuration publication
The system SHALL publish an AI configuration only through a controlled manual operation after all eligible work is terminal. The first `qwen-plus` baseline SHALL require fixed identity, persisted hard-contract successes, and a redacted count/usage/failure attestation. Review verdicts, semantic findings, review recommendation, sample percentages and Flash comparison SHALL NOT gate expansion or publication.

#### Scenario: Terminal-only semantic review
- **WHEN** a human reviews an unreviewed semantic artifact
- **THEN** the system SHALL display the remote body fields and the persisted structured conclusion only in a direct CVM terminal session, accept only a fixed verdict and error-category vocabulary, and append an immutable review bound to that exact artifact/result/configuration; it SHALL NOT expose this session through GitHub Actions, status output, logs, or a normal local file

#### Scenario: Semantic quality issue is reviewed
- **WHEN** review finds an error in necessity, alternatives, explicit education, explicit experience, evidence role, or another model interpretation
- **THEN** the immutable review records the issue for future improvement and does not change result persistence, expansion eligibility, or publication eligibility

#### Scenario: Successful first baseline publication
- **WHEN** the candidate has complete eligible terminal work, persisted hard-contract successes, and its mechanical attestation
- **THEN** the controlled operation publishes the immutable configuration with a redacted attestation and makes it the default for future new or changed eligible body versions

### Requirement: Redacted AI observability
Logs, database audits, CI, operations output, Git and ordinary local files SHALL contain only approved identifiers, versions, counts, usage summaries, and safe error categories. They SHALL exclude JD text, spans, candidate strings, model requests, model responses, and credentials. The separately specified CVM root-only raw-output audit directory is the sole exception for a new Plus candidate response.

#### Scenario: Observability boundary
- **WHEN** an AI task succeeds or fails
- **THEN** its ordinary logs and operation output exclude JD text, spans, candidate strings, model requests, model responses, and credentials; any raw candidate output remains accessible only under the separate CVM audit policy
