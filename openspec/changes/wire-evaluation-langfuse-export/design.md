# Design

## Context

The evaluation CLI already gates Langfuse export with `MROLESCOPE_LANGFUSE_EVALUATION_ENABLED` and constructs case-level Session metadata. The fixed CVM helper loads the root-owned runtime env file and starts isolated one-shot Compose services, while the Compose definition currently passes only Qwen variables to those services.

## Goals / Non-Goals

**Goals:**

- Make the existing CLI opt-in switch and Langfuse credentials available to both remote evaluation services.
- Keep credentials confined to the root-owned env file and container environment; never add them to arguments, artifacts, or logs.
- Add static checks that fail if the service wiring or allowlisted configuration propagation regresses.
- Preserve the existing Session identity, trace payload, benchmark, scorer, and local report behavior.

**Non-Goals:**

- Do not change the Langfuse exporter implementation or Session algorithm.
- Do not enable export automatically on CVM.
- Do not run a benchmark as part of deployment or this change.
- Do not expose Langfuse or evaluation ports or add a persistent evaluation service.

## Decisions

1. **Pass configuration through Compose environment mappings.** The evaluation services already receive Qwen credentials from the same protected env file. Add the four Langfuse values plus the explicit evaluation switch to both services, keeping the one-shot services on the existing `ai-edge` network. This avoids shell interpolation or command-line secrets and matches the API's runtime configuration shape.

2. **Keep the switch default empty/false.** The Compose mapping uses an empty default for the switch and credentials. The CLI's existing `true` check remains the final opt-in gate, so deployment alone cannot export traces.

3. **Extend the existing shell checker and its tests.** The checker will require each evaluation service block to contain the five Langfuse environment keys and reject unsafe additions as before. Tests assert the reviewed mappings textually without loading or printing any secret values.

4. **Document a preflight, then stop before execution.** The operator flow will state that the enable flag and complete keys must be verified before invoking `run-trend`; this implementation task ends after deployment readiness checks and does not invoke that helper.

Alternatives considered: adding a second secrets file was rejected because it duplicates ownership and rotation boundaries; adding Langfuse flags to the fixed helper command was rejected because secrets would become process arguments and the CLI already reads environment variables; enabling export in the API container was rejected because evaluation traces must be isolated from ordinary Debug configuration.

## Risks / Trade-offs

- [An operator enables export with invalid credentials] → The CLI/exporter reports an upload/configuration failure while the local evaluation result remains available; preflight checks verify presence without revealing values.
- [Compose mappings accidentally leak values in diagnostics] → Keep checker output limited to key names and rely on existing secret-file permissions; do not print rendered Compose configuration.
- [The remote helper and deployed Compose file diverge] → Include both assets in the existing release image/bootstrap path and run the repository's remote evaluation and Compose checks before deployment.

## Migration Plan

1. Update Compose mappings, checker assertions, tests, and operator documentation.
2. Run focused tests, strict OpenSpec validation, and the repository quality gate.
3. Deploy through the approved autonomous release path.
4. Verify the deployed helper and environment key presence without printing values.
5. Stop and wait for explicit instruction before invoking `run-trend`.

Rollback is the normal digest-based redeploy of the previously verified release; leaving the enable flag unset also provides an immediate no-export fallback.
