# qwen-token-budget-estimation Specification

## Purpose

确保受控 Qwen AI 试跑按照可复现的 Token 计数而非 UTF-8 字节数执行本地预算预检，并使计数契约变化产生新的不可变候选配置。

## ADDED Requirements

### Requirement: Pinned conservative Qwen input-token preflight

系统 SHALL 使用内置、固定版本且校验摘要的 Qwen BPE 词表，估算系统提示词、序列化模型输入和严格响应 Schema 的输入 Token 上界，并加上固定的 chat envelope 余量。估算失败 SHALL 在模型调用前以安全配置错误停止。系统 SHALL NOT 将 UTF-8 字节数当作 Token 数，也 SHALL NOT 为通过预算而截断或省略已批准输入。

#### Scenario: Normal Chinese JD reaches the provider seam

- **WHEN** 一个正常长度、包含受控 span 与已验证技术提及的中文 JD 的保守估算低于固定 10,000 输入 Token 上限
- **THEN** 预算门 SHALL 放行该工作到受控 provider seam

#### Scenario: Truly oversized input remains local-only

- **WHEN** 一个合成输入的保守估算超过固定 10,000 输入 Token 上限
- **THEN** 系统 SHALL 以 `input_too_large` 安全失败、保留原输入且不执行 provider 调用或物理尝试

### Requirement: Token estimator participates in candidate identity

系统 SHALL 将 Token 估算器的版本、词表摘要及保守切分契约摘要纳入受控 AI 输入契约摘要。估算器契约的任一变化 SHALL 创建新的 Plus candidate 和 Flash comparison 配置，且 SHALL NOT 覆写或复用旧配置的工作项、尝试、结果或安全失败记录。

#### Scenario: Legacy byte-estimator trial remains historical

- **WHEN** 已存在使用旧 UTF-8 字节估算契约的受控试跑配置
- **THEN** 准备当前受控试跑 SHALL 创建不同的 Plus/Flash 配置，并保留旧配置不变
