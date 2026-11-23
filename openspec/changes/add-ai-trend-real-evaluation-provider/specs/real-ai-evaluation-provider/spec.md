# Spec: Real AI Evaluation Provider

## ADDED Requirements

### Requirement: Provider mode is explicit

评测命令 SHALL 默认使用离线 fixture。只有用户显式选择真实 provider 时，命令才允许创建真实 Qwen provider 并发送模型请求。

#### Scenario: Default evaluation stays offline

- **WHEN** 用户未指定 provider
- **THEN** 评测使用 fixture provider，不读取或发送 Qwen 凭据

#### Scenario: Real provider is selected explicitly

- **WHEN** 用户指定真实 Qwen provider
- **THEN** 命令创建生产 Qwen 适配器，并保留评测固定快照和独立报告边界

### Requirement: Credential handling is bounded

真实 provider SHALL 只从运行时环境读取 Qwen API key 和 workspace ID；缺少任一配置时 SHALL 在发送请求前失败。凭据 SHALL 不出现在命令参数、日志、trace、report 或错误消息中。

#### Scenario: Missing credential fails before network access

- **WHEN** 真实 provider 缺少 API key 或 workspace ID
- **THEN** 命令返回配置错误且 HTTP client 不收到请求

### Requirement: Real evaluation reuses production seams and isolated data

真实评测 SHALL 复用生产 Qwen provider、TrendRunner、工具协议、Draft 校验和 assistant reply 流；Serving 数据 SHALL 继续来自固定评测快照，评测 SHALL 不写入生产 Serving、用户看板或日常审计。

#### Scenario: Real provider runs against the evaluation snapshot

- **WHEN** 本地模拟 Qwen 返回合法趋势响应和流式说明
- **THEN** Runner 使用固定快照完成 Draft、回复事件和统计汇总

### Requirement: Real usage and latency are reported

真实评测 SHALL 记录每次运行的 provider、模型、工具次数、输入/输出 token、首事件延迟、总耗时、成本和终态；原始回复、凭据和完整工具 payload 不得写入评测产物。

#### Scenario: Provider usage is included in the trace

- **WHEN** 模拟 Qwen 返回 usage 和多个流式回复片段
- **THEN** trace/report 汇总 usage、延迟和成本，并保留脱敏边界
