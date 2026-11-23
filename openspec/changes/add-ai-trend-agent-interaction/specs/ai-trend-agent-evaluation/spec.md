# Spec Delta

## Purpose

为趋势 Agent 建立可审阅、可重复、与现有趋势 Benchmark 分离统计的评测能力，既能检查 30 道曲线题新增的 Agent 响应边界，也能覆盖纯指导、超出范围和混合交互。

## ADDED Requirements

### Requirement: Existing trend cases include Agent expectations

现有 30 道趋势题 SHALL 保持原有题意和历史统计边界，并为相关题目增加响应模式、最终终态、是否允许趋势工具、Draft 是否存在和回复要求。U01/U02/U03 等部分交付题 SHALL 预期 `trend + guidance`；U04 SHALL 预期 `guidance-only`、无 Draft、无趋势工具且成功结束。

#### Scenario: Unknown value case keeps partial delivery
- **WHEN** 评测 U01
- **THEN** 判分器同时检查 Redis 曲线、`trend + guidance` 模式和未知值说明边界

#### Scenario: Unsupported chart case includes guidance
- **WHEN** 评测 U03
- **THEN** 判分器检查折线 Draft、`trend + guidance` 模式和不支持图表要求的回复质量记录

#### Scenario: Conflicting time range is successful guidance
- **WHEN** 评测 U04
- **THEN** 判分器要求无趋势工具、无 Draft、`guidance-only` 和 `done.success`

### Requirement: A separate Agent benchmark has ten reviewed cases

系统 SHALL 从独立的 `AI趋势智能体AgentBenchmark-v1.md` 读取 10 道人工审阅题，并将其分为纯指导/能力咨询 4 题、完全无法理解或超出范围 3 题、趋势与指导混合 3 题。Agent 题 SHALL 与 30 道趋势题分开运行和统计。

#### Scenario: Pure guidance coverage
- **WHEN** 执行纯指导题
- **THEN** 题目要求指导模式、回复存在、无趋势工具和无 Draft

#### Scenario: Mixed interaction coverage
- **WHEN** 执行混合题
- **THEN** 题目同时要求趋势模式、有效曲线、回复和必要的限制说明

#### Scenario: Case source is human-reviewable
- **WHEN** 评测题集被生成执行数据
- **THEN** Markdown 题源保留用户原话、响应模式、终态、工具边界、Draft 约束和回复语言/长度约束

### Requirement: Agent case fields are deterministic and layered

Agent 题卡 SHALL 支持 `expected_response_modes`、`expected_terminal_status`、`expected_no_trend_tools`、`expected_draft`、`reply_required`、回复语言和长度等字段；模式、工具边界、回复存在性和 Draft 保留是硬判定，回复具体措辞只做 advisory 质量记录。

#### Scenario: Guidance tool boundary is a hard sub-score
- **WHEN** 纯指导题调用了趋势工具
- **THEN** Agent 工具边界分项失败，但不触发模型重试，也不改变用户请求的 success 终态

#### Scenario: Synonymous guidance remains eligible
- **WHEN** 回复用不同中文措辞表达相同限制
- **THEN** 只要功能模式、工具边界和回复存在性正确，文字质量记录允许通过 advisory 检查

### Requirement: Agent evaluations reuse isolated production seams

Agent 评测 SHALL 复用生产提示词、模型适配器、外层协议、工具编排、流式事件和 Draft 校验，并继续采用固定评测快照、每题 3 次独立运行、20 次工具上限和既有基础设施补跑规则。Agent 轨迹不得写入日常 Serving 或生产审计。

#### Scenario: Independent Agent repetition
- **WHEN** 同一 Agent 题运行三次
- **THEN** 每次使用独立请求和状态，不继承前一次回复、Draft 或工具结果

#### Scenario: Tool and reply trace is bounded
- **WHEN** Agent 评测保存轨迹
- **THEN** 轨迹记录模式、工具事件、回复状态、Draft、token、成本和延迟，但不保存思维链、凭据、SQL 或无关 JD 正文

### Requirement: Reports keep Agent and trend metrics separate

评测报告 SHALL 分别汇总 30 道趋势题和 10 道 Agent 题的通过率、模式/工具/Draft 分项、回复质量 advisory、工具次数、token、成本和延迟。Agent 题结果不得改变趋势 Benchmark v1 的历史通过率。

#### Scenario: Separate report sections
- **WHEN** 同时运行两套评测
- **THEN** 报告包含独立的 trend benchmark 和 agent benchmark 汇总，并能分别查看失败原因

#### Scenario: Quality does not become a hidden hard gate
- **WHEN** 回复在流结束后的中文或长度质量检查中出现 warning
- **THEN** 报告保留 warning 和质量统计，但不因该 warning 回滚合法 Draft 或伪造业务失败
