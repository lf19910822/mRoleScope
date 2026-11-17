## ADDED Requirements

### Requirement: Deployment uses the CVM Docker daemon as the network truth

The state-changing local release command MUST perform one bounded preflight against the CVM Docker daemon before building or pushing application images. The preflight MUST pull the exact pinned PostgreSQL image reference used by the reviewed remote Compose definition. It MUST NOT use a direct HTTP request to an official registry endpoint as a release gate.

#### Scenario: The configured Docker pull path is available

- **WHEN** an operator runs `scripts/release-remote-development.sh --deploy` from a valid reviewed `main`
- **THEN** the command MUST report the CVM Docker daemon registry mirror context, successfully pull or confirm the exact pinned PostgreSQL image, and only then continue to local verification, image build, and push

#### Scenario: The configured Docker pull path is unavailable

- **WHEN** the exact pinned PostgreSQL image cannot be pulled by the CVM Docker daemon within the bounded preflight
- **THEN** the command MUST stop before building or pushing application images and report that the actual Docker daemon pull failed, including safe mirror context without claiming that an unrelated direct registry probe is the cause

#### Scenario: Dry-run is requested

- **WHEN** an operator runs `scripts/release-remote-development.sh --dry-run`
- **THEN** the command MUST remain free of CVM SSH, Docker pull, image build, image push, and other external state-changing operations

### Requirement: CVM pull failures are diagnosable at the actual boundary

The root-controlled deployment helper MUST execute the existing Compose pull once and, if it fails, MUST report the actual pull phase, configured Docker registry mirror context when available, and the fixed Compose image-pull operation. It MUST NOT issue a second blind pull or use direct registry HTTP probing to classify the failure.

#### Scenario: Compose image pull fails

- **WHEN** `compose --profile web pull postgres migrate api worker web-edge` returns a non-zero status
- **THEN** the helper MUST preserve the original pull output, print safe Docker daemon mirror diagnostics, record failure at the `pull-images` phase, and stop before migration or service startup

#### Scenario: Compose image pull succeeds

- **WHEN** the fixed Compose image pull completes successfully
- **THEN** the helper MUST continue its existing revision-label validation, migration, health checks, release recording, rollback, and cleanup behavior without changing the application deployment boundary

### Requirement: Release documentation distinguishes network paths

The autonomous release runbook MUST state that Docker daemon/Compose pull results are authoritative for release readiness, while direct registry endpoint probes are not sufficient evidence of a release failure. It MUST document the preflight stop point and the safe diagnostic output.

#### Scenario: An operator investigates a pull failure

- **WHEN** a release stops during the network preflight or `pull-images` phase
- **THEN** the runbook MUST direct the operator to use the reported Docker daemon mirror context and image-specific pull error rather than infer failure from an unrelated direct Registry probe
