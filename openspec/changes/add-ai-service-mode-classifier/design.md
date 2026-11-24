# Design

## Context

当前 `TrendRunner` 通过 Qwen 工具会话完成意图规划、工具调用、Draft 构造和 Agent 模式判断。`response_modes` 目前由同一个模型回合返回，HTTP 流只转发操作状态，评测器从最终 Agent 响应推断模式。新增能力需要在首个趋势规划回合前完成轻量分类，同时保持现有 `ai-trend-agent-v1` 和 `ai-trend-draft-v1` 的下游兼容。

## Goals / Non-Goals

**Goals:**

- 在同一次请求中先得到经过本地校验的 `service_tags`。
- 将服务类别判断与趋势工具规划、Draft 构造、自然语言回复分离。
- 保留现有 Agent 外层响应和 HTTP NDJSON 事件合同的兼容映射。
- 让分类失败安全降级为 guidance，并记录分类 usage、耗时和状态。
- 让评测器能够独立判定分类结果，并让题卡与快照候选保持一致。

**Non-Goals:**

- 不在本变更中增加未来服务类别的业务处理。
- 不让分类节点读取 Serving、调用趋势工具或生成 Draft。
- 不建立 TAG 组合白名单；不同已知 TAG 可以自由组合，组合适用性由后续服务端工作流处理。
- 不改变现有曲线、Serving 查询和用户显式应用 Draft 的语义。

## Decisions

### 1. 新增独立 Provider 分类方法

在现有 Qwen Provider 增加分类专用调用边界，使用独立短 system prompt、严格 JSON Schema、无 tools、关闭 thinking 和较小 `max_tokens`。分类输入只包含用户原始问题；输出只包含 `contract_version` 和 `service_tags`。

选择同一 Qwen 适配器而不是新模型：复用凭据、HTTP 超时、usage 解析和审计路径，避免引入第二个模型配置。选择独立方法而不是复用 `AnalyzeIntentTurn`：防止工具协议、趋势 Intent Schema 和服务分类互相污染。

### 2. 分类结果作为服务端权威状态

Orchestrator 在工具会话前调用分类方法，并把规范化后的 TAG 放入编排结果。服务端按固定顺序去重后映射当前模式：`trend` 映射趋势，`guidance` 映射指导，二者同时存在映射混合模式。后续规划模型只返回趋势 Intent 或工具调用，不再返回或修改 `response_modes`；服务端在最终 Agent 响应包装阶段写入映射后的模式。

这样可以保留现有 Agent 外层协议，同时消除模型在初始回合和最终回合重复判断模式造成的漂移。

### 3. 分类失败的降级路径

Provider 错误、超时、空列表、未知 TAG、重复 TAG、非法 JSON 和 Schema 校验失败统一归类为分类失败。Orchestrator 将服务类别降级为 guidance-only，跳过趋势工具和 Draft，继续进入已有 `assistant_message` 指导回复路径。分类失败仍发出失败操作事件并进入审计；指导回复成功后业务终态为 success。

不把分类失败当作趋势工具失败：分类阶段尚未产生趋势数据，继续调用趋势工具只会把错误扩散到 Draft。

### 4. 流式操作与调用预算

分类阶段发出 `classify_service_modes` operation running/completed/failed 事件。HTTP 层将其加入安全工具白名单，但操作计数和现有 20 次 Serving 工具预算分开统计。分类节点不展示原始 TAG 判断文本；`service_tags` 进入内部状态、审计和评测 trace。

### 5. 回复提示词装载

回复阶段根据服务端生成的 `response_modes` 选择提示词片段：只有 guidance-only 注入样例问题；trend-only 和 mixed 模式不注入样例，并且只回答趋势结果和用户明确提出的指导内容。该路由在服务端完成，回复模型不再自行决定是否给出样例。

### 6. 评测与快照

题卡新增 `expected_service_tags`，scorer 在工具、范围、Draft 和回复质量之外增加分类硬分项。现有 30 道 Trend 和 10 道 Agent 题需要逐题审阅：显式值不强制额外类别调用，类别结果允许使用正式值表达，候选数量不能超过固定快照能力，未知值和混合问题必须与实际错误事件及服务 TAG 对齐。快照版本变更时生成新的 snapshot version，旧报告继续引用旧版本。

### 7. 前端和审计兼容

前端沿用现有 operation 记录组件，只新增可显示的 `classify_service_modes` 名称；不新增用户可见分类内容。审计增加分类 TAG、分类状态、分类 token 和分类耗时字段，现有工具调用、Draft 和回复统计继续保持独立，便于比较分类阶段的额外成本。

## Risks / Trade-offs

- **增加一次模型调用和首事件延迟** → 分类请求使用短提示词、无工具和小输出上限；流立即发送 operation running，审计单独记录 P50/P95。
- **分类失败导致趋势请求降级为指导** → 采用 guidance fallback，保证用户收到可读回复；评测单独记录分类失败，不伪造 Draft。
- **TAG 组合未来出现工作流不适用** → 不提前建立组合矩阵；服务端在实际处理点生成结构化限制反馈，再交给 AI 回复。
- **服务端映射与旧 Agent 响应不一致** → 保留 `response_modes` 外层字段，但禁止规划模型覆盖，增加协议和 scorer 测试。
- **旧 Benchmark 题卡与新分类节点不一致** → 在真实付费评测前完成 30+10 题全量审阅、快照校验和 fixture 回归。

## Migration Plan

1. 先实现分类协议、Provider seam、服务端映射、失败降级和操作事件测试；旧趋势工具会话测试继续通过。
2. 接入 Runner/Orchestrator 和回复提示词路由，补充分类 usage、审计和 HTTP/前端状态测试。
3. 重构 Trend/Agent Benchmark 题卡与 scorer，生成与候选容量匹配的新评测快照，先运行离线 fixture 全量回归。
4. 在 CVM 部署后先执行 Trend 6 类与 Agent 3 类抽样真实评测，确认分类失败、混合 TAG 和成本记录，再决定全量评测。
5. 回滚时停用分类节点并恢复旧 `response_modes` 路径；保留新增审计字段和评测产物，不删除历史报告。
