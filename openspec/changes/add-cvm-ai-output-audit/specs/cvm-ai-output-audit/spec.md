# Spec Delta

## Purpose

在新的 Qwen Plus 候选 AI 试跑期间，将模型原始输出短期限制于 CVM root 专属审计区，并让 `FAN` 通过受控终端定位严格契约不一致，而不向普通可观测渠道扩散真实文本。既有对照数据是历史，不增加新的 Flash 捕获或查看流程。

## ADDED Requirements

### Requirement: Plus-candidate-only raw-output capture

系统 SHALL 仅为新的、冻结 `ai-jd-result-v4` / `ai-semantic-validator-v4` 的 `qwen-plus` candidate 配置的、从模型成功取得的原始输出创建 CVM 审计工件，并在严格 schema、证据或语义校验前完成原子保存。工件 SHALL 绑定配置、工作项、尝试号、不可变正文版本和输出摘要；不保存 API Key、原始请求或供应商错误正文。已发布配置不得创建该工件；既有 Flash comparison 工件保持历史但不得触发新的捕获。

#### Scenario: Trial response is later rejected by a contract validator

- **WHEN** Qwen Plus candidate 试跑取得模型输出，但后续严格校验拒绝该输出
- **THEN** 对应原始输出可在 CVM 审计区供受控审计，工作仍以准确的安全错误类别失败

#### Scenario: Audit capture cannot be committed

- **WHEN** 模型已返回原始输出但 CVM 审计工件无法原子写入
- **THEN** 系统 SHALL 以固定的审计捕获失败类别安全停止该工作，不持久化语义结果，且不得为恢复该输出而自动重发模型请求

#### Scenario: Published analysis executes

- **WHEN** 已发布 AI 配置处理一个正文版本
- **THEN** 系统不得创建原始模型输出审计工件

### Requirement: Root-only bounded retention

审计工件 SHALL 仅位于 CVM 的 root-owned 审计目录；目录权限必须为 `0700`，工件权限必须为 `0600`。系统 SHALL 在写入前和受控清理时删除已超过 7 天的工件，并在未删除仍处于保留期的工件前提下，将总量限制为 64 MiB。

#### Scenario: Retention period expires

- **WHEN** 审计工件已达到 7 天保留期限
- **THEN** 下一次受控清理 SHALL 删除该工件，且普通日志、数据库和状态输出不得包含其原始内容

#### Scenario: Capacity is exhausted by retained evidence

- **WHEN** 删除到期工件后，保存下一份输出会超过 64 MiB 硬上限
- **THEN** 系统 SHALL 以审计捕获失败类别安全停止该工作，且不得删除未到期工件或继续外发请求

### Requirement: Fixed terminal contract audit

系统 SHALL 只允许 `FAN` 在直接连接 CVM 的 TTY 中运行固定的 Qwen Plus 候选契约审计会话。会话 SHALL 由程序选择一个已捕获且严格契约失败的输出，显示经终端控制字符净化的原始输出及由不可变正文版本和配置重建的受控输入上下文，并只接受固定的审计结论类别。

#### Scenario: Failed trial output is audited

- **WHEN** `FAN` 在直连 CVM TTY 开始固定 Plus 契约审计
- **THEN** 系统显示下一个未审计的已捕获契约失败输出，并将固定审计结论绑定到该配置、工作与尝试，而不接受任意文件路径、ID、SQL、提示词、模型或自由文本理由

#### Scenario: Audit is invoked outside a direct terminal

- **WHEN** 审计操作没有直接连接的 TTY，或经 GitHub Actions、普通状态接口或日志入口调用
- **THEN** 系统 SHALL 拒绝操作且不得输出正文、受控输入上下文或模型原始输出

#### Scenario: Safe audit status is requested

- **WHEN** CVM root helper invokes the fixed non-interactive `audit-status` operation
- **THEN** 系统 SHALL 仅报告捕获数、待审计数、容量和固定失败类别，且不得输出路径、摘要、正文、重建输入或模型输出

### Requirement: Rollback-only policy failure diagnosis

系统 SHALL 提供一个仅限 CVM root helper 的固定离线诊断，用于检查尚未审阅、已捕获的
`policy_rejected` 候选回复。诊断 SHALL 使用其不可变配置和正文版本重建输入，并经同一
校验与持久化路径在必定回滚的事务内重放。它不得调用模型、不得带 API Key 或外网、不得持久化
工作项/语义结果/复核/工件变更，且不得输出模型回复、输入、正文、路径、ID、摘要或数据库错误。
输出只能包含已处理总数和固定的持久化阶段类别聚合。

#### Scenario: Captured policy failures are classified without another model call

- **WHEN** `FAN` 通过 root helper 调用固定的 policy diagnostic
- **THEN** 系统 SHALL 仅重放已捕获且未审阅的 `policy_rejected` 回复，以固定类别汇总结果，且原有失败工作和语义结果保持不变

#### Scenario: Policy diagnosis is exposed through a normal operation channel

- **WHEN** GitHub Actions、普通 operations helper 或任意带参数的 worker 命令尝试调用该诊断
- **THEN** 系统 SHALL 拒绝调用，且不得读取或输出审计回复

### Requirement: Field-local candidate-governance fallback

对使用字段级容错校验器的候选或对照配置，系统 SHALL 将“词典外候选词无法写入已发布或已退役
词典版本”处理为候选词字段的不确定状态，而不是整份 JD 的安全失败。系统不得因该降级修改冻结
词典、创建技术身份、创建已验证技术选项，或持久化模型给出的候选词字符串；其余可验证语义
SHALL 继续原子持久化。

#### Scenario: External candidate targets a published dictionary

- **WHEN** V2 配置的模型回复包含已由片段证据绑定的词典外候选词，而依赖词典版本已发布或退役
- **THEN** 系统 SHALL 保留仅含既有证据的 `candidate_technology` 不确定、需复核工件，完成工作并将
  结果标记为 `uncertain`；字典候选表、技术身份和硬性技术条件均不得被该候选词改变

#### Scenario: Historical strict configuration replays the same limitation

- **WHEN** V1 配置遇到相同的冻结词典候选词限制
- **THEN** 系统 SHALL 保持其既有安全拒绝语义，避免历史不可变配置在回放时悄悄改变结果

### Requirement: Raw-output isolation from normal observability

原始模型输出、重建输入上下文、正文片段和候选字符串 SHALL NOT 写入 PostgreSQL、Git、GitHub Actions、CI、systemd journal、普通日志、状态输出、对象存储或本地工作区文件。普通可观测渠道仅可记录允许的标识、计数、版本、用量、安全错误码及固定审计结论。

#### Scenario: Trial capture and audit complete

- **WHEN** 一个模型输出被捕获并由终端审计
- **THEN** 状态和部署验收只报告脱敏的计数与固定类别，且不含模型文本、JD 文本、请求内容、Workspace ID 或凭据

### Requirement: Audit findings are non-blocking semantic feedback

The system SHALL use output audit only to diagnose the prompt, strict schema, request binding, or local reference/persistence boundary. A fixed audit finding or a semantic review SHALL NOT reject a hard-contract-valid response, rewrite its persisted model projection, block Plus expansion, or block manual publication. Hard-contract-invalid responses remain safe failures and SHALL NOT be repaired into structured data.

#### Scenario: Audit identifies a semantic quality concern
- **WHEN** a direct-CVM audit or review records a concern about a model interpretation
- **THEN** the immutable finding remains available for a later contract revision without changing the current result, trial terminal state, expansion eligibility, or manual publication eligibility

### Requirement: Audit policy is part of candidate identity

系统 SHALL 将审计捕获策略版本、7 天保留期和 64 MiB 容量规则纳入 AI 输入/审计契约身份。该身份任一变化 SHALL 创建新的 Plus 候选配置，且不得覆盖、重跑或伪造恢复既有配置已丢弃的输出。

#### Scenario: Audit policy changes after an earlier trial

- **WHEN** 审计策略版本、保留期限或容量规则发生变化
- **THEN** 准备试跑 SHALL 创建不同的不可变配置，保留旧配置、工作、尝试和安全失败历史不变

#### Scenario: Earlier output was discarded under the legacy policy

- **WHEN** 一个历史试跑在启用该能力前已丢弃模型输出
- **THEN** 系统不得声称可恢复该输出，且不得原地重跑其失败队列
