# Proposal

## Why

趋势 AI 当前把每个用户输入都当成曲线规划问题，像“你好”、功能咨询、无法理解的问题也会触发正式值枚举、趋势摘要和空曲线候选。这既浪费模型与 Serving 资源，也让用户看到与问题无关的结果。与此同时，已有的趋势回复只有短说明，无法在工具工作完成后以流式方式完整回答用户。

本变更把趋势规划和 Agent 交互分开：AI 先判断本次请求是趋势、指导还是两者兼有；趋势协议继续保持现有曲线 Intent，指导和解释通过独立的流式回复通道返回。评测同步覆盖现有 30 道异常题，并增加独立的 10 道 Agent 题集。

## What Changes

- 新增外层 `ai-trend-agent-v1` 响应包装，使用可并存的 `trend`、`guidance` 响应模式；内部趋势 Intent 继续使用 `ai-trend-draft-v1`。
- 固定基础 system prompt 在每次模型回合传递；agent 上下文的详细文案、示例和优先级另行设计，首期只固定角色、能力、边界和回复规则的结构。
- 纯指导请求不调用趋势工具、不生成 Draft，但成功生成回复时以 `done.success` 结束；部分可理解请求保留有效曲线并进入 `trend + guidance`。
- 新增 `assistant_message` 流式事件；工具操作继续按现有 NDJSON 事件流式显示，Draft 在混合请求中先展示，随后流式输出解释。
- 允许独立的流式指导/解释回复，普通解释和指导回复上限统一为 1000 个 Unicode 字符；结束后检查质量并记录 warning，不回滚已显示内容、不影响合法 Draft、不触发重试。
- 保留停止、后台失败和非法参数的既有职责边界：用户停止保留已显示回复和 Draft，合法后台失败不触发模型重试，纯指导误调用只记 Agent 边界分项失败。
- 更新现有 30 道趋势 Benchmark 的 Agent 响应判定，并新增 `AI趋势智能体AgentBenchmark-v1.md`，包含 10 道 Agent 题（4 道纯指导、3 道无法理解或超出范围、3 道混合题）。两套评测分别统计。
- **BREAKING** 对 AI 趋势流式事件和模型响应增加 Agent 事件/包装；现有 Draft 和应用协议保持兼容，旧客户端需忽略或升级以支持新增回复事件。

## Capabilities

### New Capabilities

- `ai-trend-agent-interaction`: 趋势 AI 的 Agent 模式判断、外层响应协议、指导/趋势/混合分支、流式自然语言回复、停止与质量状态处理。
- `ai-trend-agent-evaluation`: 现有趋势评测的 Agent 判定扩展，以及独立的 10 道 Agent 交互评测题源、执行和统计边界。

### Modified Capabilities

- 无。当前仓库没有已注册的 durable capability spec；本变更新增上述两个能力。

## Impact

- 后端：`internal/aitrend` 的 Agent 外层合同、Qwen 适配器、模型编排、流式回复、事件校验、审计和 token/成本统计；`internal/httpserver` 的 NDJSON 转发和安全过滤。
- 前端：`web/src/api`、AI 趋势草稿状态和趋势页 AI 对话窗口，增加回复增量、纯指导无 Draft、混合 Draft 先展示等状态。
- 评测：现有 `internal/aitrend/evaluation`、30 题 Markdown、独立 Agent Benchmark、报告与 trace；固定快照仍只用于评测，不进入日常拉取或生产查询。
- 不修改 Serving 趋势点位接口、数据库 schema、已应用曲线语义或发布流程；本变更不包含部署。
