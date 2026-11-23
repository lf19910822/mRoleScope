# Spec Delta

## Purpose

为趋势看板提供单次、无记忆的 Agent 交互层，让用户问题可以得到指导、趋势曲线或两者兼有的回复，同时保持现有曲线 Intent、Draft 和应用协议的边界。

## ADDED Requirements

### Requirement: Agent responses use an outer mode envelope

系统 SHALL 使用 `ai-trend-agent-v1` 外层响应表达 `trend`、`guidance` 或两者并存的 `response_modes`，并在需要趋势时携带既有 `ai-trend-draft-v1` Intent。Agent 请求 SHALL 不继承其他请求的对话、Draft 或工具结果。

#### Scenario: Guidance-only envelope
- **WHEN** 用户输入“你好”或“怎么使用趋势看板”
- **THEN** 系统返回 `response_modes: ["guidance"]`，不携带趋势 Intent

#### Scenario: Trend envelope
- **WHEN** 用户输入“哪些项目的岗位最多”
- **THEN** 系统返回包含 `trend` 的模式和经过既有合同校验的趋势 Intent

#### Scenario: Mixed envelope
- **WHEN** 用户要求比较 Go 和 Python 并询问如何修改曲线
- **THEN** 系统返回同时包含 `trend` 和 `guidance` 的模式，并携带趋势 Intent

### Requirement: Guidance-only requests do not generate curves

纯 `guidance` 请求 SHALL 不调用趋势 Serving 工具、不生成 Draft，并 SHALL 通过 `assistant_message` 返回中文指导。回复成功后请求 SHALL 以 `done.success` 结束；没有 Draft 不得被判定为分析失败。

#### Scenario: Greeting receives guidance
- **WHEN** 用户输入“你好”
- **THEN** 系统只返回操作记录、指导回复和成功终态，不返回空 Draft 或“0/8 条曲线”

#### Scenario: Unsupported request receives boundary guidance
- **WHEN** 用户询问股票价格走势或薪资趋势
- **THEN** 系统不调用趋势工具，说明当前能力边界并以成功终态结束

#### Scenario: Unclear request receives actionable guidance
- **WHEN** 用户输入“帮我看看那个东西”且无法识别统计对象
- **THEN** 系统不生成曲线，返回通俗的可复制提问示例

### Requirement: Partial support preserves valid trend results

当问题同时包含可支持和不可支持内容时，系统 SHALL 保留可生成的有效曲线并使用 `trend + guidance` 模式说明限制。未知值、未知类别或不支持条件 SHALL 不得静默扩大为 `*`。

#### Scenario: Unknown value with a valid value
- **WHEN** 用户比较未知技术值和已知 Redis
- **THEN** 系统保留 Redis 曲线，说明未知值不可用，并将请求标为趋势加指导

#### Scenario: Unsupported metric with supported dimension
- **WHEN** 用户询问项目岗位占比最高
- **THEN** 系统按岗位数量生成可用曲线，并在回复中说明占比指标暂不支持

#### Scenario: Unsupported chart request
- **WHEN** 用户要求使用柱状图展示项目岗位
- **THEN** 系统保留折线 Draft，并在回复中说明当前看板使用折线图

### Requirement: Replies stream independently from validated curve drafts

`assistant_message` SHALL 支持一次请求内的增量文本事件和结束事件。混合请求 SHALL 在 Draft 通过校验后先发送 Draft，再流式发送曲线说明和指导，最后发送 `done`；正式图表仍只有用户应用后才更新。

#### Scenario: Draft appears before explanation stream
- **WHEN** 混合请求的工具调用和 Draft 校验完成
- **THEN** 客户端先看到曲线 Draft，随后收到多个 `assistant_message` 增量并最终收到完成事件

#### Scenario: Guidance reply streams without a Draft
- **WHEN** 纯指导请求的模型判断完成
- **THEN** 客户端收到 `assistant_message` 增量和完成事件，不收到 Draft

#### Scenario: Trend-only explanation streams
- **WHEN** 趋势请求只包含 `trend` 模式
- **THEN** 系统仍通过 `assistant_message` 流式说明曲线选择、统计口径、排序依据和必要限制

### Requirement: Reply text has a bounded quality contract

system prompt SHALL 约束回复使用中文、控制在 1000 个 Unicode 字符以内且不暴露内部内容。首期 SHALL 不在流式过程中拦截或修改文本；流结束后检查完整性、中文和长度并记录质量 warning，不回滚已展示内容、不影响合法 Draft、不触发回复重试。

#### Scenario: Reply quality warning does not erase a Draft
- **WHEN** 流结束后的回复检查发现超长、非中文或不完整
- **THEN** 已展示回复和合法 Draft 保留，系统记录内部质量 warning，业务终态仍按 Draft 结果决定

#### Scenario: Reply uses only safe context
- **WHEN** 回复模型生成趋势解释
- **THEN** 它只能使用原始问题、已校验 Draft 安全摘要和必要 warnings，不接收 SQL、凭据、JD 正文、内部错误或完整每日数据

### Requirement: Stop and backend errors preserve maximum-effort results

用户停止回复流时，系统 SHALL 保留已展示的回复和已生成 Draft，以 `user_stopped` 结束且不重试。合法工具或回复后台失败 SHALL 不触发模型参数修正；已应用曲线 SHALL 不因新请求停止或失败而改变。

#### Scenario: User stops during explanation
- **WHEN** 用户在 `explain_draft` 流式回复期间点击停止
- **THEN** 已显示回复和 Draft 保留，终态为 `user_stopped`，不等待剩余文本或重试

#### Scenario: Guidance model accidentally calls a trend tool
- **WHEN** 最终模式为 `guidance-only` 但模型此前误调用趋势工具
- **THEN** 工具结果不参与 Draft 或回复，系统不重试，保留操作记录并仍按指导成功收口

### Requirement: Existing curve application remains unchanged

Agent 交互层 SHALL 不自动应用 Draft，不改变已经应用的曲线，不修改 Serving 点位接口或曲线应用协议。趋势 Draft SHALL 继续使用现有最多 8 条曲线、正式值、校验和用户确认流程。

#### Scenario: Draft is shown but not applied
- **WHEN** 混合请求先展示 Draft
- **THEN** 当前看板曲线保持不变，只有用户点击应用后才更新

#### Scenario: Existing Draft contract remains usable
- **WHEN** 趋势模式携带内层 Intent
- **THEN** 内层继续按 `ai-trend-draft-v1` 校验并交给现有 Draft 构造和应用流程
