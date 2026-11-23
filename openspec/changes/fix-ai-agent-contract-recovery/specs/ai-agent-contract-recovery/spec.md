# AI agent contract recovery

## ADDED Requirements

### Requirement: Recover initial non-structured guidance
When the initial Agent model turn contains no tool calls and cannot be decoded as a valid Agent response, the orchestrator MUST safely treat it as guidance-only and continue to the normal user-facing reply stage.

#### Scenario: Natural-language greeting response
- **WHEN** the initial model turn returns plain text for a guidance query
- **THEN** the request produces a guidance response path instead of ending with an empty semantic failure

### Requirement: Preserve tool calls with incidental text
When a model tool turn contains both function calls and incidental textual content, the provider MUST preserve the function calls and the orchestrator MUST continue the bounded tool session.

#### Scenario: Mixed query emits tool call plus explanation
- **WHEN** the model emits a tool call and a short guidance sentence in the same turn
- **THEN** the tool call is executed and the final Agent response determines the response modes

