# Proposal

## Why

两套 Benchmark 当前只能通过 `fixtureProvider` 验证编排和评分链路，无法衡量真实 Qwen 的意图识别、工具选择、回复延迟和 token 成本。线上业务已经有 Qwen 适配器，但评测 CLI 没有可控的真实 provider 入口。

## What Changes

- 为评测 CLI 增加显式的真实 provider 模式，默认仍保持离线 fixture，避免误发模型请求。
- 真实模式从运行环境读取 Qwen 凭据，不在参数、日志或评测产物中保存凭据。
- 真实 provider 继续使用评测固定快照，不写入生产 Serving、用户看板或日常审计。
- 记录真实模型的工具次数、token、首事件延迟、总延迟、成本和失败边界。
- 用本地 HTTP 模拟 Qwen 完成 provider 契约和 runner 集成测试；不在自动化测试中访问真实模型。

## Capabilities

### New Capabilities

- `real-ai-evaluation-provider`: 为两套 Benchmark 提供受控、显式、可审计的真实模型评测入口。

### Modified Capabilities

无。

## Impact

- 影响 `cmd/ai-evaluation`、`internal/aitrend/evaluation` 和现有 Qwen provider 接口。
- 增加评测 CLI 参数和运行时环境变量读取，不改变生产 HTTP API。
- 真实评测仍需操作者显式启动并承担模型费用；fixture 模式行为保持不变。
