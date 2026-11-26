# Benchmark contract alignment

## ADDED Requirements

### Requirement: Benchmark fields stay within production contracts

评测题卡的 Intent 指标和数量 SHALL 能被生产 Schema 表达；用户原始但不支持的指标 SHALL 使用独立请求指标字段记录，不得伪装成合法 `ranking_metric`。

#### Scenario: Unsupported share metric is recorded separately

- **WHEN** 用户询问岗位占比
- **THEN** 题卡 SHALL 使用合法回退指标，并单独记录原始不支持指标及预期 `unsupported_metric` 错误

### Requirement: Fault injection produces observable correction evidence

`invalid_tool_params_once` SHALL 使工具调用失败进入模型校验修正事件，并在后续调用成功时被 scorer 识别为 `invalid_tool_params_then_corrected`。

#### Scenario: Invalid tool parameters are corrected

- **WHEN** 评测注入一次缺少必填字段的工具参数
- **THEN** 评测事件 SHALL 同时包含校验失败和后续成功工具调用

### Requirement: Guidance-only tool scoring is not a false failure

当题卡要求 guidance-only 且禁止趋势工具时，无工具操作 SHALL 通过 ToolCalls 子评分；有任何趋势工具调用 SHALL 失败。

#### Scenario: Guidance avoids trend tools

- **WHEN** 纯指导题只返回说明
- **THEN** 工具子评分 SHALL 通过

### Requirement: Fixtures cover capacity and batch boundaries

默认评测快照 SHALL 提供超过 8 个项目和超过 8 个数据库候选，并包含至少一个零岗位数据库值；容量题和类别题 SHALL 实际经过服务端截断与跨批次摘要路径。

#### Scenario: Category crosses a batch boundary

- **WHEN** 数据库类别包含 10 个正式值
- **THEN** 评测 SHALL 能观察到跨批次摘要并验证最终最多 8 条曲线
