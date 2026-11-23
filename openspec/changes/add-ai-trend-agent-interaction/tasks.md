# Tasks

## 1. Agent 外层合同与事件协议

- [x] 1.1 定义 `ai-trend-agent-v1` 外层响应、`response_modes`、趋势 Intent 引用和 guidance-only 终态，并生成严格 Schema；用纯指导、趋势、混合和非法模式单元测试验证边界。
- [x] 1.2 保持 `ai-trend-draft-v1` Intent/Draft 合同不变，增加 Agent 外层到现有趋势编排的适配器；运行现有 Intent、Draft、应用前校验测试确认旧协议行为不变。
- [x] 1.3 增加 `assistant_message` 的增量、完成和停止事件合同，定义 `running`、`completed`、`user_stopped` 和失败状态；用事件解码测试验证缺字段、超长和非法状态被拒绝。
- [x] 1.4 增加版本化基础 agent 上下文装载接口，只固定角色、能力、边界、回复规则和无记忆约束，不固定后续详细文案；测试每次请求重新装载且不读取上一请求状态。

## 2. Prompt 与模型适配器

- [x] 2.1 更新基础 system prompt，使模型能选择 `trend`、`guidance` 或两者，明确纯指导不应调用趋势工具、未知限制不得扩大为 `*`，并保留现有范围、时间、排名和错误职责规则；用 prompt 契约测试覆盖 51 条决策涉及的核心规则。
- [x] 2.2 扩展 Qwen 工具会话解析，使最终返回外层 Agent 响应、内层严格 Intent 或受控工具调用；验证 guidance-only、trend、mixed、非法外层字段和旧工具修正路径。
- [x] 2.3 为指导和 `explain_draft` 增加独立纯文本流请求，解析上游 SSE/分块响应、结束标记、usage、超时和取消；用 provider contract test 覆盖多个 delta、空 delta、上游错误和中断。
- [x] 2.4 将回复阶段上下文限制为原问题、固定 agent 上下文或已校验 Draft 安全摘要与 warnings；测试 SQL、凭据、JD、完整每日点位、内部错误和其他会话结果不会进入请求。
- [x] 2.5 将回复长度上限统一为 1000 Unicode 字符，并在流结束后执行中文、长度、完整性检查；测试质量 warning 不回滚已展示文本、不影响合法 Draft、不触发回复重试。

## 3. 编排器与运行器分支

- [x] 3.1 增加 guidance-only 分支：不调用趋势 Serving 工具、不生成 Draft，成功回复后发送 `done.success`；用“你好”“怎么使用”和完全超出范围测试无空 Draft 行为。
- [x] 3.2 增加 trend 与 trend+guidance 分支，继续复用现有工具循环、20 次上限、非法参数修正、合法后台失败最大努力和 `ai-trend-draft-v1` Draft 校验；用明确值、类别、全范围、部分失败和多子问题测试回归。
- [x] 3.3 处理 guidance-only 前的误工具调用：不使用结果、不重试、不生成 Draft，但保留 operation 事件和审计记录；测试 Agent 工具边界分项可失败而用户终态仍为 success。
- [x] 3.4 调整混合请求顺序，在 Draft 通过校验后先发送 Draft，再启动流式 `explain_draft` 和 `assistant_message`，最后发送 done；用事件序列测试 Draft 先于回复增量。
- [x] 3.5 保留 Draft、已显示回复和已应用曲线的停止语义；测试用户停止指导流、解释流和工具流分别得到 `user_stopped`，不等待剩余文本、不重试且旧应用曲线不变。
- [x] 3.6 将指导/解释阶段的 prompt token、completion token、工具调用、耗时、取消和质量 warning 汇总进现有审计与统计；测试单请求各阶段计数相加且不保存原始回复或凭据。

## 4. HTTP 流与前端交互

- [x] 4.1 扩展 HTTP NDJSON 白名单、参数过滤和事件校验，转发 `assistant_message` 增量但不暴露上游原始响应；用 HTTP 测试验证安全字段过滤、事件顺序和断开取消。
- [x] 4.2 扩展前端 API 类型和 AI 草稿状态，累积回复 delta、处理完成/停止/质量 warning，并保持操作记录、Draft 和旧请求替换规则；运行 TypeScript 单元测试覆盖请求隔离和成功/partial/stopped 状态。
- [x] 4.3 更新趋势页 AI 对话窗口：纯指导只显示回复和操作记录，趋势/混合显示 Draft，混合请求 Draft 先出现后继续显示回复；用组件测试验证不显示“0/8 条曲线”和不自动应用 Draft。
- [x] 4.4 更新停止、重新生成、旧草稿保留、回复失败和后端部分失败交互；用前端测试验证已应用曲线不被新请求或停止覆盖，Draft 与 assistant_message 状态不会串请求。

## 5. 现有 30 道趋势 Benchmark 扩展

- [x] 5.1 扩展趋势评测 Case/Dataset 字段，支持 `expected_response_modes`、`expected_terminal_status`、`expected_no_trend_tools`、`expected_draft`、`reply_required`、回复语言和长度约束；用生成器测试验证旧 30 题字段兼容。
- [x] 5.2 更新 `AI趋势智能体Benchmark-v1.md` 的 U01/U02/U03/U04/U05：部分支持题要求 `trend + guidance`，U04 要求 guidance-only、无 Draft、无趋势工具且 success，U05 保留非法工具参数修正；重新运行 Markdown 完整性和快照正式值校验。
- [x] 5.3 扩展确定性评分器：响应模式、Draft 是否存在、工具边界、回复是否存在和有效曲线作为硬分项；中文、长度、完整性和限制说明作为 advisory；用 U01-U05 反例测试每个边界。
- [x] 5.4 保持现有 30 题的固定快照、三次串行运行、20 次调用、基础设施补跑、成本/延迟和脱敏 trace 规则；运行原趋势 Benchmark 回归并确认 Agent warning 不改变历史趋势主分数。

## 6. 独立 10 道 Agent Benchmark

- [x] 6.1 创建 `Docs/开发指导/AI评测/AI趋势智能体AgentBenchmark-v1.md`，编写 10 道人工审阅题：纯指导/能力咨询 4 题、无法理解或超出范围 3 题、趋势与指导混合 3 题；每题包含 Agent 专用题卡字段并通过人工可读性检查。
- [x] 6.2 为独立题源增加加载、生成和版本元数据，确保它不进入日常 Serving/拉取任务且与 30 题文件分别校验；用测试验证题数、分组、正式值和无敏感 payload。
- [x] 6.3 为 Agent 题执行三次独立运行并生成独立报告，单独汇总模式、工具边界、Draft、回复质量、token、成本和延迟；用报告测试验证 Agent 结果不改变趋势 Benchmark v1 统计。
- [x] 6.4 为纯指导、超出范围、模糊问题、混合有效/无效条件和误工具调用增加生产编排端到端 fixture；验证无 Draft success、mixed Draft-first、no-trend-tools 分项和最大努力回复。

## 7. 审计、文档与决策覆盖

- [x] 7.1 更新评测产物格式，记录 Agent 模式、assistant_message 状态/质量 warning、Draft 顺序、流式耗时和安全上下文摘要，继续禁止思维链、凭据、SQL、JD 正文和完整每日数据；运行脱敏测试。
- [x] 7.2 更新 `Docs/开发指导/AI评测/AI趋势智能体Benchmark-v1.md` 的字段说明和独立 Agent Benchmark 文档说明，解释硬判定与 advisory 的区别；验证示例 JSON 可由工具加载。
- [x] 7.3 建立决策 1-29、30-43、44-51 到 proposal/spec/design/tasks/测试的覆盖矩阵，逐条检查固定快照隔离、工具上限、错误职责、流式顺序、停止、外层协议和两套评测；覆盖缺口必须在任务或测试中补齐。
- [x] 7.4 在实现阶段维护 `AI评测开发决策.md` 的记录与 OpenSpec 任务一致，运行 `openspec validate add-ai-trend-agent-interaction --type change --strict` 验证 artifacts 和场景完整。

## 8. 集成验证与交付准备

- [x] 8.1 运行 Go 全量测试、前端单元/类型检查、评测命令、双 Benchmark fixture 回归、`go vet` 和 `git diff --check`；记录真实 Qwen/CVM 检查是否可用，不把 fixture 结果冒充线上质量。
- [x] 8.2 运行固定快照趋势评测和 Agent 评测，检查每题三次结果、工具次数、tokens、成本、首事件/总延迟、质量 warning 和代表性 trace；确认两份报告独立且无生产状态写入。
- [x] 8.3 执行本地真实链路冒烟测试：你好、功能咨询、明确趋势、混合请求、未知值、冲突时间和停止；验证 assistant_message 流、Draft-first、应用确认和旧曲线保护。
- [x] 8.4 完成 OpenSpec 严格验证并更新任务状态；不在本变更任务中部署，发布仍需用户另行授权并遵循自主镜像发布运行手册。

## Decision coverage matrix

| 已确认决策 | 主要覆盖任务 | 覆盖内容 |
| --- | --- | --- |
| 1-5 | 5.4, 6.2, 7.1, 8.2 | 固定快照、主/分项评分、工具语义、脱敏轨迹 |
| 6-8 | 5.4, 6.3, 8.2 | 三次运行、题目分组、端到端判定 |
| 9-13 | 5.2, 6.1, 7.1 | Markdown 题源、版本、成本/延迟、独立产物 |
| 14-17 | 3.2, 3.3, 3.5, 8.3 | 最大努力、错误职责、应用前终点、隔离 |
| 18-21 | 5.2, 6.1, 6.3, 8.2 | 人工题目、判分、串行执行、上下文隔离 |
| 22-25 | 3.2, 5.4, 6.3, 8.2 | 工具上限、汇总、基础设施失败和补跑 |
| 26-29 | 1.2, 3.2, 5.4, 7.3 | 生产链路复用、正式值、顺序和八条容量 |
| 30-34 | 1.4, 2.1, 3.1, 3.3, 3.6 | 基础上下文、分阶段回复、响应模式、最大努力和误调用 |
| 35-38 | 3.4, 3.5, 4.1, 4.2, 4.3 | Draft-first、流式质量、停止、状态和前端展示 |
| 39-43 | 1.1, 2.2, 2.4, 4.1, 4.2 | 统一回复、终态、安全摘要、暂定上下文和 Agent 外层 |
| 44-51 | 5.1-5.4, 6.1-6.4, 7.1-7.3 | 30 题异常恢复、10 题 Agent 集、题卡、分层评分和边界失败 |

## Workflow follow-up

- Review the completed planning artifacts before implementation.
- Start `$openspec-apply-change` only in a later user-authorized turn.
