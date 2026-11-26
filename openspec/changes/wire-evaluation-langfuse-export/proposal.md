# Proposal

## Why

The real Trend evaluation runner already creates case-level Langfuse Sessions, but the deployed evaluation Compose services do not receive the Langfuse enable flag or credentials. As a result, a CVM evaluation can complete locally while producing no Langfuse traces for review.

## What Changes

- Pass the explicit evaluation-export enable flag and the existing Langfuse endpoint, project, public key, and secret key to the isolated Trend and Agent evaluation services.
- Keep evaluation export opt-in and disabled unless the operator explicitly enables it.
- Extend the remote evaluation safety checks to require the reviewed Langfuse environment wiring without printing secret values.
- Update the operator documentation with the preflight checks and the exact stop point before starting a run.
- Preserve the existing case-level Session identity and metadata; do not change benchmark cases, scoring, or evaluation execution behavior.

## Capabilities

### New Capabilities

- `evaluation-langfuse-export`: Controlled propagation of Langfuse configuration to remote evaluation runs.

### Modified Capabilities

None.

## Impact

The change affects the remote-development Compose definition, the fixed root-owned evaluation helper checks, and evaluation operator documentation. It does not alter the public API, production AI Debug behavior, benchmark data, Langfuse credentials, or the evaluation CLI's Session implementation. No evaluation is started by this change.
