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

### Requirement: Strict structured model response
The system SHALL accept an external model response only when it is one JSON object conforming to the frozen strict response schema. The response SHALL contain no prose, Markdown, unrecognized fields, raw text evidence, or model-supplied character coordinates.

#### Scenario: Valid structured response
- **WHEN** the response matches the schema and all required opaque request identifiers
- **THEN** the system continues with remote semantic and evidence validation

#### Scenario: Invalid or mixed response
- **WHEN** the response contains explanatory text, schema violations, unknown fields, or a mismatched request identifier
- **THEN** the system rejects it as a valid result, stores only a safe error category, and does not persist the raw response

### Requirement: Program-owned semantic evidence locations
The system SHALL map model-referenced span identifiers and verified technology-evidence identifiers to the body’s source field, normalization version, field hash, and Unicode code-point half-open interval. The system SHALL reject unsupported, unknown, cross-body, or invalid references.

#### Scenario: Model references a valid span
- **WHEN** a semantic conclusion references a span generated for its input body version
- **THEN** the persisted evidence uses the CVM-mapped field/version/hash/interval rather than a model-calculated coordinate

#### Scenario: Model references an unknown span
- **WHEN** a response references a span identifier absent from the input contract
- **THEN** the associated conclusion is not persisted as valid and is marked for required review or safely rejected according to the configuration policy

### Requirement: Conservative semantic extraction
The system SHALL record experience lower bounds, one minimum-education outcome, optional education/experience condition branches, and technology relationship groups only when they are supported by valid spans. It SHALL keep `not_analyzed`, `not_configured`, and `uncertain` distinct. For irreconcilable explicit qualification conflicts, the first release SHALL use the explicit `jobRequire` qualification condition.

#### Scenario: No explicit experience requirement
- **WHEN** a completed analysis finds no explicit candidate experience lower bound or finds an explicit no-experience condition
- **THEN** it records the experience field as `not_configured` and does not present it as a zero-year requirement

#### Scenario: Education-dependent experience
- **WHEN** a valid span explicitly gives different experience lower bounds for different education levels
- **THEN** the system records the agreed simplified minimum-education/base-year values and preserves the supported conditional branches without inventing unmentioned branches

#### Scenario: Uncertain qualification statement
- **WHEN** the model or validator cannot reliably determine a qualification’s meaning
- **THEN** the relevant field is `uncertain` with required-review status and is not represented as `not_configured`

### Requirement: Technology requirement semantics
The system SHALL distinguish `qualification`, `responsibility`, `mention`, and `unspecified` roles. Only qualification groups SHALL affect candidate requirement matching. It SHALL use `required` for positive non-optional qualifications, `preferred` for explicit preference/bonus conditions, and `unspecified` when necessity cannot be determined. Proficiency SHALL be null unless explicitly supported by the configured P1–P4 mapping.

#### Scenario: Responsibility is not a hard requirement
- **WHEN** a span describes work to be performed with a technology but does not state a candidate qualification
- **THEN** the system stores it only as responsibility or mention and excludes it from candidate hard-requirement statistics

#### Scenario: Explicit technology alternatives
- **WHEN** an explicit qualification states that one of several verified technologies is sufficient
- **THEN** the system records one `at_least` group with `min_satisfied` equal to one

#### Scenario: Ambiguous punctuation-only list
- **WHEN** a multi-technology list has no unambiguous conjunction, alternative, or quantity relation
- **THEN** the system does not infer a usable hard-requirement group and routes the relation to required review

### Requirement: Governed candidate technology mentions
The system SHALL accept a dictionary-external technology candidate only when its candidate text and referenced span can be revalidated against the remote normalized body and it is not an active dictionary alias. Candidates SHALL remain pending review and SHALL NOT create a formal technology identifier, active alias, published statistic, or valid requirement-group option.

#### Scenario: Verified external candidate
- **WHEN** a model returns a candidate string that is exactly recoverable from its valid span and is not an active alias
- **THEN** the system creates or reuses a remote pending dictionary candidate with required-review status

#### Scenario: Hallucinated candidate
- **WHEN** the candidate text cannot be recovered from the referenced span
- **THEN** the system rejects the candidate and does not modify dictionary state

### Requirement: Bounded external execution and safe failure
The system SHALL run AI work independently of source synchronization. A trial logical task SHALL allow at most three physical attempts, retrying only transient transport, rate-limit, or provider-server failures. Every attempt SHALL count toward request, token, and monetary limits; non-transient failures and budget exhaustion SHALL stop safely without retrying or exposing text.

#### Scenario: Transient provider failure
- **WHEN** a model attempt fails with a transient timeout, 429, or 5xx outcome and budget remains
- **THEN** the system performs no more than two delayed retries for that logical task

#### Scenario: Schema validation failure
- **WHEN** a model response fails schema or evidence validation
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
The initial two-model trial SHALL use 30 remotely selected body-content versions, at most 60 logical tasks, at most 180 physical attempts, per-task trial limits of 10,000 input and 2,500 output tokens, and a 2.00 RMB total hard limit. The system SHALL stop before an attempt that would exceed any active limit and SHALL not silently truncate input or output.

#### Scenario: Trial budget precheck
- **WHEN** the estimated worst-case charge for a next attempt would exceed the 2.00 RMB trial limit
- **THEN** the system does not send the attempt and leaves the affected work not analyzed

#### Scenario: Trial price cannot be verified
- **WHEN** current official pricing or sufficient usage information cannot support the configured budget precheck
- **THEN** the system does not start the external trial

#### Scenario: First baseline reuses the trial request profile
- **WHEN** the first `qwen-plus` configuration advances from its reviewed trial bodies to the remaining eligible bodies
- **THEN** it retains the identical 10,000-input and 2,500-output token limits, and any limit or segmentation change requires a new candidate configuration and renewed trial validation

### Requirement: Review-gated AI configuration publication
The system SHALL publish an AI configuration only through a controlled operation after all eligible work is complete and the configured release gate passes. The first `qwen-plus` baseline SHALL require 30 fully reviewed trial bodies, zero major semantic errors, and complete mechanical validation. `qwen-flash` SHALL remain a comparison result and SHALL NOT be selected through automatic routing.

#### Scenario: Terminal-only semantic review
- **WHEN** a human reviews an unreviewed semantic artifact
- **THEN** the system SHALL display the remote body fields and the persisted structured conclusion only in a direct CVM terminal session, accept only a fixed verdict and error-category vocabulary, and append an immutable review bound to that exact artifact/result/configuration; it SHALL NOT expose this session through GitHub Actions, status output, logs, or a normal local file

#### Scenario: Major trial error
- **WHEN** review finds an error that reverses necessity, alternatives, explicit education, or explicit experience meaning
- **THEN** the system refuses publication until a new candidate configuration is validated on the affected trial scope

#### Scenario: Successful first baseline publication
- **WHEN** the candidate has complete eligible work, all trial review requirements, and zero major errors
- **THEN** the controlled operation publishes the immutable configuration with a redacted attestation and makes it the default for future eligible body versions

### Requirement: Redacted AI observability
Logs, audits, CI, operations output, and local files SHALL contain only approved identifiers, versions, counts, usage summaries, and safe error categories. They SHALL exclude JD text, spans, candidate strings, model requests, model responses, and credentials.

#### Scenario: Observability boundary
- **WHEN** an AI task succeeds or fails
- **THEN** its ordinary logs and operation output exclude JD text, spans, candidate strings, model requests, model responses, and credentials
