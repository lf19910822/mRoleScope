# Langfuse session grouping

## ADDED Requirements

### Requirement: Sessions are optional and explicit

系统 SHALL 为每次 trace 生成独立 trace ID。只有调用方显式提供合法 session ID 时，才 SHALL 写入 Langfuse session 属性；未提供时不得根据 query、trace ID 或用户输入自动拼接 session。

#### Scenario: Independent Debug trace

- **WHEN** Debug 请求未提供 session ID
- **THEN** trace SHALL 上传成功且不带 session 归组属性

#### Scenario: Explicit Debug session

- **WHEN** Debug 请求提供少于 200 个 ASCII 字符的 session ID
- **THEN** root trace SHALL 带有该 session ID，所有子 observations SHALL 随 trace 归入该 Session

### Requirement: Evaluation sessions isolate one case within one run

评测导出 SHALL 使用 `eval:<run_id>:<suite>:<case_id>` 作为 session ID。同一运行中同一道题的 repetition 和 retry SHALL 共享该 Session；不同 case 或不同 run SHALL 使用不同 Session。

#### Scenario: Repetitions share a case session

- **WHEN** 同一评测 run 的 E01 执行多个 repetition 或 retry
- **THEN** 所有对应 traces SHALL 进入同一个 E01 Session，并通过 metadata 区分 repetition 和 retry

#### Scenario: Different runs remain separate

- **WHEN** E01 在两个不同 run_id 中执行
- **THEN** 两组 traces SHALL 不共享 Session

### Requirement: Evaluation metadata is queryable

评测 trace SHALL 写入稳定的 metadata，包括 `evaluation_run_id`、`evaluation_suite`、`evaluation_case_id`、`evaluation_version_id`、`evaluation_repetition` 和 `evaluation_retry`；metadata 不得包含凭据。

#### Scenario: Trace metadata identifies an attempt

- **WHEN** 评测 attempt 导出 trace
- **THEN** Langfuse trace metadata SHALL 能区分 run、suite、case、版本、repetition 和 retry
