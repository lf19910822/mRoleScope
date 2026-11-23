# Design

## Context

现有趋势链路由 `TrendRunner`、受限 Qwen 工具会话、Serving 工具、确定性 Draft 构造和 HTTP NDJSON 流组成。`Intent` 使用严格的 `ai-trend-draft-v1` 合同，要求至少一个子查询；运行器在模型完成后无条件规划趋势工具、构造 Draft、校验 Draft 并调用 `explain_draft`。前端 AI 状态也以 Draft explanation 作为主要用户回复。

本变更在不改变 Serving 点位、Draft 应用和已应用曲线语义的前提下增加 Agent 外层。现有 Qwen 适配器使用非流式 HTTP 完整读取响应；现有公开流支持 operation、validation、draft 和 done 事件，但没有普通回复增量事件。评测基线已有固定快照、30 道趋势题、串行三次运行和确定性判分，本变更需要在其上扩展而不能污染历史统计。

## Goals / Non-Goals

**Goals:**

- 让一次请求能够明确表达趋势、指导或两者并存，并为纯指导请求提供成功的无 Draft 路径。
- 保留 `ai-trend-draft-v1` 作为内层趋势协议，避免把聊天字段混入曲线合同。
- 通过真正的模型流式回复让用户在 Draft 出现后立即看到解释或指导文字。
- 保持单次请求无跨请求记忆、20 次工具上限、最大努力交付、用户停止和错误职责边界。
- 让现有 30 题和新增 10 题 Agent 评测覆盖响应模式、工具边界、Draft 保留和回复质量。

**Non-Goals:**

- 不在本变更中设计 agent 上下文的最终长文案、示例库或版本化优先级；只提供结构化装载点。
- 不增加连续多轮记忆、会话存储或跨请求上下文。
- 不让 Agent 自动应用 Draft、修改看板或替代 Serving 统计。
- 不在流式过程中做内容拦截、改写或回滚；不把回复质量 warning 变成业务失败。
- 不改变评测发布门槛、固定快照的日常隔离或现有 30 题的历史统计口径。

## Decisions

### 1. 外层 Agent 合同与内层趋势合同分离

新增 `ai-trend-agent-v1` 外层对象，至少表达 `response_modes` 和可选的内层趋势 Intent；趋势 Intent 和最终 Draft 继续使用 `ai-trend-draft-v1`。纯指导没有内层 Intent，趋势和混合模式必须有经本地验证的趋势 Intent。

直接把 `response_modes` 和聊天字段塞入 `ai-trend-draft-v2` 会让曲线合同承担客服语义，并迫使所有旧 Intent、测试和评测一起迁移，因此不采用。外层包装多一层解析，但可以保留旧曲线验证和应用接口。

### 2. 模型回合与响应模式

固定基础 system prompt 每轮发送，包含角色、能力、边界、无记忆原则和响应模式规则；agent 上下文的具体文案另行设计。模型在现有受限工具会话中自主判断：纯指导直接返回外层 guidance 结果且不应调用趋势工具；趋势或混合问题才继续现有工具循环。若模型误调用后最终判为 guidance-only，服务端不使用这些结果、不重试、不生成 Draft，但保留操作记录。

`response_modes` 是数组而非互斥枚举。这样“比较 Go 和 Python，并告诉我如何修改曲线”可以同时表达 `trend` 和 `guidance`，而不会把一部分请求丢弃。模式、内层 Intent 和终态都在服务端本地验证，模型不能通过模式字段绕过 Draft 校验。

### 3. 回复阶段与真正流式传输

严格 Intent/Draft JSON 不做增量解析；自然语言回复单独使用纯文本流。纯指导在模式判断后新增一次回复模型阶段；趋势请求复用已有 `explain_draft` 阶段但改为流式，因此趋势请求不增加新的模型阶段，只改变响应传输。混合请求在 Draft 通过校验后启动 explain 流。

Qwen 上游使用支持流式的兼容 HTTP 响应（例如 SSE 或等价分块），Provider 逐段读取文本并通过回调交给 Runner；Runner 将其转为公开 NDJSON `assistant_message` 增量事件。公开事件使用 `running + delta` 和一个 `completed` 事件，不把上游 JSON、思维链或 Provider 原始消息暴露给浏览器。上游流结束后才做中文、长度和完整性检查；检查只记录内部 warning，不回滚已展示内容、不改变合法 Draft、不触发回复重试。

模型上下文和回复流的 token、耗时、取消和错误仍进入同一请求审计汇总。Provider 请求取消时立即停止上游读取；用户停止保留已展示回复和 Draft，最终发送 `user_stopped`。

### 4. Draft 与回复的事件顺序

现有运行器在 Draft 后调用 explanation provider，改为：工具结果和 Draft 校验完成后先发送 Draft 事件，再发送 `explain_draft` operation running 和 `assistant_message` 增量，最后发送回复 completed 与 done。这样用户能在说明生成期间先编辑草稿；正式图表仍需显式应用。

纯指导请求只发送 operation、assistant_message 和 done，不发送空 Draft。回复质量 warning 不影响 done 的业务终态：无 Draft 的纯指导成功为 success；趋势 Draft 合法时按 complete/partial；用户停止为 user_stopped；服务或协议失败才为 failure。

### 5. 安全上下文和回复职责

指导回复只接收用户原问题和固定 agent 上下文；趋势/混合回复只接收用户问题、已校验 Draft 的安全摘要、排名指标、时间范围和 warnings。摘要使用正式显示名称和必要聚合值，不包含 SQL、凭据、JD 正文、内部错误、完整每日点位或其他请求历史。

回复不是 Draft 属性：`assistant_message` 是 1000 Unicode 字符上限的正式用户回复载体，Draft 只保存曲线候选和应用所需数据。工具名称和参数仍可作为操作记录展示，但不由普通回复解释内部实现。

### 6. 前端状态和兼容性

API 类型增加 Agent 外层和 assistant_message 增量；AI 状态增加当前回复缓冲、回复质量 warning 和纯指导成功状态。现有 operation 展示继续工作，Draft 组件在没有 Draft 时隐藏；混合请求先渲染 Draft，再追加回复文本。旧客户端如果忽略未知事件，仍不会自动应用曲线；支持完整体验的客户端需处理新事件。

已应用曲线、替换/追加按钮、颜色池、Serving API 和数据库结构不变。新请求仍独立清空待确认状态，失败/停止不替换旧 Draft。

### 7. 评测与基线隔离

现有 30 道趋势题继续从原 Markdown 读取，U01/U02/U03/U04/U05 增加 Agent 响应字段；U04 变为 guidance-only、无趋势工具、无 Draft 但 success。新增 `AI趋势智能体AgentBenchmark-v1.md` 保存 10 道题，按 4/3/3 分组，使用 Agent 专用题卡字段。

两套题各自三次串行运行，沿用固定快照、20 次调用、最多两次基础设施补跑、结构化 trace 脱敏和成本/延迟统计。Agent 模式、工具边界、回复存在性和 Draft 保留是硬分项；中文、长度、完整性和限制说明是 advisory。纯指导误调用导致 Agent 边界分项失败，但不重试、不改变用户请求的 success 终态。Agent 报告单独汇总，不改变趋势 Benchmark v1 历史通过率。

### 8. 决策覆盖检查

- 评测决策 1-29：沿用现有固定快照、30 题 Markdown、三次串行、20 次工具、基础设施补跑、结构化轨迹、确定性判分和应用前终点；任务包含全量回归，禁止把 Agent 题或流式 warning 混入旧趋势主分数。
- 决策 30-34：由基础 system prompt、外层模式、纯指导无 Draft、最大努力 partial、误调用收口和上下文边界任务覆盖。
- 决策 35-43：由 Draft-first 事件顺序、纯文本增量、1000 字回复、结束后质量检查、停止语义、统一 assistant_message、success 终态、安全摘要、暂定 agent 上下文和外层协议任务覆盖。
- 决策 44-51：由 30 题异常恢复扩展、10 题独立题源、Agent 题卡、分层评分、分开报告和误调用硬分项任务覆盖。

## Risks / Trade-offs

- **[模型仍可能在 guidance-only 前误调用工具]** → Prompt 明确禁止；服务端不使用结果、不重试，评测单独记录边界分项，后续可依据数据决定是否增加预检。
- **[上游流式协议或结构化回复不兼容]** → 只对自然语言回复启用纯文本流，Intent/Draft 继续完整严格 JSON；Provider 设有取消、超时和确定性回退。
- **[回复在展示后才发现质量问题]** → 质量检查只用于 warning 和审计，不伪造业务失败；system prompt 先提供正常约束。
- **[回复阶段增加成本]** → 趋势请求复用现有 explain 阶段，只有纯指导增加一次回复阶段；单独记录 token、成本和 P95 延迟。
- **[新事件导致旧客户端体验不完整]** → Draft 和应用协议保持兼容，API 事件向后可忽略；前端升级覆盖纯指导和增量回复。
- **[Agent 评测改变旧基线]** → 单独题源、单独报告和分项，不改趋势 30 题历史主分数。

## Migration Plan

1. 先实现外层 Agent 合同、模式校验、纯指导分支和事件合同测试；旧趋势 Intent/Draft 测试必须继续通过。
2. 扩展 Qwen Provider 的自然语言流式回复和取消路径，接入 Runner 的 Draft-first 顺序、质量 warning、停止和审计 token 统计。
3. 更新 HTTP NDJSON 和前端状态，覆盖纯指导、趋势、混合、停止、回复失败和旧 Draft 应用流程。
4. 更新 30 题异常恢复字段，创建并执行独立 10 题 Agent Benchmark；分别审阅 trace、报告、工具边界和成本/延迟。
5. 运行 Go/前端质量门、OpenSpec 严格校验、固定快照双基线和独立真实链路冒烟；本变更不自动部署，发布仍遵循既有发布流程。

## Open Questions

- 上游 Qwen 兼容端点实际采用 SSE 还是其他分块格式，需要在实现阶段用 provider contract test 固定解析器；不改变本设计的公开 NDJSON 事件。
