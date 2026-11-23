# Spec: CVM Real AI Evaluation

## ADDED Requirements

### Requirement: CVM evaluation entry is controlled and isolated

CVM SHALL provide only fixed allowlisted evaluation operations for the trend and Agent Benchmark. The evaluator SHALL run as a short-lived container from the reviewed worker image, receive only the Qwen runtime credentials and evaluation snapshot, and write reports only below a root-owned evaluation artifact directory. It SHALL not expose a port, mount production raw data, write Serving tables, or accept arbitrary command, prompt, path, image, or secret arguments.

#### Scenario: CVM runs a fixed benchmark operation

- **WHEN** the root-owned evaluation helper receives the fixed trend or Agent operation
- **THEN** it starts the corresponding one-shot Compose service with the reviewed image and fixed in-image Benchmark path

#### Scenario: Unapproved CVM operation is rejected

- **WHEN** a caller supplies an arbitrary operation or extra argument
- **THEN** the helper exits before Docker execution and reports only a generic allowlist error

### Requirement: CVM evaluator reuses the real provider and isolated snapshot

The CVM evaluator SHALL invoke the real Qwen provider through the same evaluation binary and SHALL use the versioned fixed snapshot bundled in the reviewed image. It SHALL not query production Serving or use the API service as a hidden proxy.

#### Scenario: Trend and Agent suites stay separate

- **WHEN** the fixed trend or Agent operation is selected
- **THEN** only its corresponding Benchmark suite and report directory are used

### Requirement: Runtime secrets and reports remain bounded

The CVM helper SHALL load the existing root-owned runtime configuration without printing its values. The evaluation container SHALL receive only the provider variables required by the model call; report output SHALL contain no credentials, raw replies, SQL, JD text, or complete tool payloads.

#### Scenario: Secret values never appear in helper output

- **WHEN** a fixed evaluation operation starts or fails
- **THEN** stdout/stderr contains only operation status and redacted error information
