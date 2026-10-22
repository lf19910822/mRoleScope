# Spec Delta

## Purpose

定义米哈游技术类社会招聘岗位的每日采集、证据归档和历史入库行为，使每个可用批次均可追溯、可审计，并为后续业务展示保留可信的版本与可见历史。

## ADDED Requirements

### Requirement: Fixed source scope and single active synchronization
The system SHALL collect only the configured miHoYo technical social-recruitment scope. It SHALL identify that scope by its normalized source filter and SHALL permit at most one active synchronization for it at a time.

#### Scenario: A second invocation arrives while a run is active
- **WHEN** an initialization, scheduled, or retry invocation starts while the configured scope has an active run
- **THEN** the second invocation SHALL not collect or mutate job history
- **AND THEN** it SHALL record or return a safe contention result

#### Scenario: The source filter changes in a future configuration
- **WHEN** a run uses a materially different normalized source filter
- **THEN** the system SHALL treat it as a new observation scope and baseline
- **AND THEN** it SHALL not infer additions, removals, or reappearances by comparing it with the old scope

### Requirement: Complete batches are the only publishable evidence
The system SHALL accept a collector batch for persistence only when its manifest, paths, counts, and file SHA-256 values validate as complete. It SHALL archive the validated raw batch and manifest in private immutable object storage before marking the batch usable.

#### Scenario: A complete collector directory passes validation
- **WHEN** every manifest entry is safe, present, hash-valid, and count-consistent
- **THEN** the system SHALL archive the raw list responses, detail responses, and manifest under the batch identity
- **AND THEN** it SHALL retain the archive URI and archive digest with the run record

#### Scenario: A collector batch is incomplete or invalid
- **WHEN** manifest validation, file validation, or collection completeness fails
- **THEN** the system SHALL record the failed attempt and a safe failure summary
- **AND THEN** it SHALL not publish the batch or alter job visibility/content history

### Requirement: Stable job identity and content-version deduplication
The system SHALL identify a miHoYo job by `(source_company, source_job_id)` only. It SHALL create a JD version only when a canonical hash of normalized business fields changes, and SHALL retain one raw-field payload and normalized display payload for each unique version.

#### Scenario: An observed job has unchanged business content
- **WHEN** a complete run observes an existing job ID with the same canonical content hash
- **THEN** the system SHALL reuse the existing JD version
- **AND THEN** it SHALL not duplicate the version payload

#### Scenario: A job ID returns with changed title or JD
- **WHEN** an existing job ID is observed after absence with different business content
- **THEN** the system SHALL keep the same job identity and create or reuse the matching JD version
- **AND THEN** it SHALL not create a second job identity based on the changed fields

#### Scenario: Only a technical source field changes
- **WHEN** a source response differs only in excluded technical or volatile fields
- **THEN** the system SHALL not create a new JD version

### Requirement: Successful observations create SCD2 history
The system SHALL persist every job observed in a complete usable run. It SHALL model a job's appearance/disappearance history separately from its JD content periods, so a content change does not appear as a disappearance and a reappearance opens a new visible period.

#### Scenario: A job's content changes while it remains observable
- **WHEN** an open visible period observes a different JD version
- **THEN** the system SHALL close the prior content period and open a new content period within the same visible period
- **AND THEN** the job SHALL remain currently visible

#### Scenario: A previously closed job reappears
- **WHEN** a complete run observes a prior job ID with no open visible period
- **THEN** the system SHALL open a new visible period and its initial content period
- **AND THEN** it SHALL preserve all prior visible and content periods

### Requirement: Missing jobs require source-detail revalidation
The system SHALL revalidate every currently visible job absent from a complete current list through the source detail endpoint before closing its visible period. Only a recognized source-level unavailable result SHALL confirm absence; all ambiguous failures SHALL preserve the open period.

#### Scenario: A list-absent job remains available through detail revalidation
- **WHEN** the detail response is structurally valid and matches the missing job ID
- **THEN** the system SHALL keep the visible period open
- **AND THEN** it SHALL retain an audit indication that the job was absent from the list but available by detail

#### Scenario: A list-absent job is explicitly unavailable
- **WHEN** the detail endpoint returns a pre-validated source semantic for a missing or unavailable job
- **THEN** the system SHALL record the first missing evidence and close the visible period in the current run

#### Scenario: A list-absent job cannot be classified
- **WHEN** detail revalidation times out, is rate-limited, returns a server error, or has an unknown response shape
- **THEN** the system SHALL classify the result as indeterminate
- **AND THEN** it SHALL not close the visible period

### Requirement: Runs are auditable and publication is atomic
The system SHALL retain a summary record for every synchronization attempt and append stage-level audit records with safe diagnostic metadata. It SHALL make a successful run the current usable baseline only after object-storage verification and the database history transaction both succeed.

#### Scenario: A run fails during archive or persistence
- **WHEN** a validated collection cannot be archived or its database transaction fails
- **THEN** the system SHALL record the failed stage and a length-limited, redacted error summary
- **AND THEN** it SHALL leave the previously usable baseline and all existing job history unchanged

#### Scenario: An audit event is persisted
- **WHEN** the synchronization changes stage, retries, or encounters an upstream failure
- **THEN** the system SHALL append an audit event linked to the run with time, stage, severity, stable code, attempt number, and safe diagnostics
- **AND THEN** it SHALL not store secrets, full request headers, full raw responses, or JD body text in the audit event

### Requirement: Daily execution is bounded and does not catch up
The system SHALL run the configured synchronization at 02:00 Asia/Shanghai, with an explicit operator initialization run before scheduling is enabled. A failed whole run SHALL have at most two non-overlapping retries with bounded delays; a missed schedule SHALL be silently skipped rather than caught up after recovery.

#### Scenario: The first deployment is initialized
- **WHEN** an operator invokes the initialization run
- **THEN** the system SHALL execute the same synchronization path used by the daily schedule
- **AND THEN** the first successful run SHALL establish a baseline without inferred job changes

#### Scenario: A whole scheduled run fails
- **WHEN** a run cannot complete after its request-level retries
- **THEN** the system SHALL make at most two further whole-run attempts using the configured delays
- **AND THEN** it SHALL stop after the final failed attempt without publishing partial data

#### Scenario: The host misses 02:00
- **WHEN** the scheduler or host is unavailable at the scheduled time and later becomes available
- **THEN** the system SHALL not automatically run a catch-up collection
- **AND THEN** it SHALL wait for the next scheduled time
