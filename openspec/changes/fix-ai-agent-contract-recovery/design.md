# Design

## Provider recovery

The Qwen adapter will keep tool calls when content is present instead of returning `invalid_combination`. On the initial no-history turn, non-JSON or unsupported Agent-shaped content becomes a bounded guidance Agent response. Once tool history exists, strict final Intent validation remains unchanged.

## Orchestrator recovery

If the initial tool session still returns a contract error without any tool history, the orchestrator may return a guidance-only Agent response. This is limited to the initial turn so malformed final trend Intents after real tool evidence remain visible as semantic failures.

## TDD seams

- Qwen provider test for tool call plus incidental content.
- Qwen provider test for initial plain-text guidance fallback.
- Orchestrator test for invalid initial Agent JSON fallback.
