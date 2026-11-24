# Proposal

## Why

当前 `trend`、`guidance` 的判断由趋势规划模型在同一个工具会话中完成。纯指导题通常可以正确识别，但混合问题、部分支持问题和未知值问题经常漏掉 `guidance`，导致回复缺少必要说明，且指导示例与趋势回复的提示词边界容易互相污染。

现在需要把“用户需要哪些服务”从趋势规划中独立出来，先得到稳定的服务类别，再决定后续工作流和提示词。这样可以让新增服务类别继续扩展，同时保持当前趋势 Intent 和 Agent 回复协议兼容。

## What Changes

- 新增轻量的 `classify_service_modes` 首节点，使用独立短提示词和严格 JSON 输出判断服务类别。
- 新增独立 `service_tags` 协议；首期支持 `trend`、`guidance`，允许不同 TAG 自由组合，同一 TAG 不得重复。
- 服务端对 TAG 做存在性校验、去重和固定顺序规范化，并映射为既有 Agent `response_modes`。
- 服务端直接决定最终 `response_modes`；趋势规划模型不再重新判断服务类别。
- 分类节点失败、返回未知/重复/空 TAG 或非法 JSON 时降级为 `guidance`，继续生成指导回复。
- 在 AI 操作记录中显示分类节点的进行中、完成和失败状态；分类模型内容不直接展示，也不计入趋势工具 20 次调用额度。
- 仅 `guidance` 模式注入样例问题提示词；包含其他 TAG 时不注入样例。
- 重构 Trend 与 Agent Benchmark：题卡增加 `expected_service_tags`，分类错误独立硬判分，并按实际快照候选和 scorer 能力修订旧题。

## Capabilities

### New Capabilities

- `ai-service-mode-classification`: 在趋势 Agent 工作流前识别、校验、规范化和审计本次请求需要的服务类别，并为后续服务路由提供结构化 TAG。
- `ai-trend-agent-mode-routing`: 根据分类节点的 `service_tags` 生成 Agent `response_modes`，并按模式选择趋势、指导和混合回复流程。
- `ai-service-mode-evaluation`: 评测题卡记录预期服务 TAG，并把分类结果纳入 Trend 与 Agent 的确定性硬判分。

### Modified Capabilities

无。当前仓库没有已注册的主规格文件可作为 delta 的修改对象；上述行为以新能力规格记录。

## Impact

影响 `internal/aitrend` 的 Provider、Runner、Orchestrator、HTTP 流事件和 Agent Schema；影响前端操作记录、评测题卡、scorer、固定评测快照和评测 CLI。Serving 查询接口和 Draft 内层协议保持兼容。每次请求增加一次轻量模型调用及对应 token/延迟审计；分类节点不读取 Serving 数据，不调用趋势工具，不直接生成用户回复。
