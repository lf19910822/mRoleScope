# Spec Delta

## Purpose

为趋势工具调用提供可区分、可审计且能驱动模型修正的安全错误反馈，使模型参数问题不会被误当成后端故障或静默扩大为全范围查询。

## ADDED Requirements

### Requirement: Tool errors preserve their corrective category

趋势工具 MUST distinguish model-correctable parameter errors from unavailable results and backend failures. Unknown formal values MUST use `invalid_value`; fixed enum violations MUST use `invalid_enum`; malformed JSON, missing fields, invalid combinations, limits, unavailable results, and backend failures MUST retain their respective codes.

#### Scenario: Unknown technology category

- **WHEN** a tool receives `category: "database"` and the formal catalog contains `databases-and-storage` but not `database`
- **THEN** the tool returns `invalid_value` with field `category` and a safe correction message

#### Scenario: Backend failure

- **WHEN** a trend summary reader or Serving boundary is unavailable
- **THEN** the tool returns `backend_failure` and does not classify the event as an invalid model value

### Requirement: Correctable tool errors are fed back for repair

When a tool returns a model-correctable error, the next model turn MUST receive the structured safe error and an explicit instruction to repair the tool parameters and retry a suitable discovery or formal-value tool. The feedback MUST not contain SQL, credentials, JD text, or internal stack traces.

#### Scenario: Model retries an invalid category

- **WHEN** `list_category_values` rejects `database` as an invalid formal category
- **THEN** the next model request contains the error code and field plus a repair instruction, and the model can call the category discovery/value tool again

#### Scenario: Backend failure is not parameter repair

- **WHEN** a tool returns `backend_failure`
- **THEN** the next model request does not tell the model to repair a parameter, and the orchestration retains maximum-effort partial-result behavior

### Requirement: Tool feedback remains bounded

Corrective tool feedback MUST remain within the existing tool-call and model-turn budgets. Repeated invalid parameters MUST terminate safely when a budget is exhausted rather than retrying without bound.

#### Scenario: Repeated invalid values

- **WHEN** the model repeatedly submits invalid formal values until the tool budget is exhausted
- **THEN** the request terminates with a bounded safe error and does not issue an unbounded series of model calls
