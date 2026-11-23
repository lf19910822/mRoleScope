# Design

## Context

`cmd/ai-evaluation` 当前在命令内部构造 `fixtureProvider`，`evaluation.Runner` 已经复用生产 `TrendRunner`，但没有 provider 工厂。生产 Qwen 适配器 `aitrend.QwenIntentProvider` 已支持 Intent 工具会话和 `assistant_message` 流式回复。评测固定快照和报告模型已经存在。

## Decisions

1. **默认离线，真实模式显式开启**：新增 `-provider fixture|qwen`，默认 `fixture`。真实模式不会通过隐式环境变量自动开启，防止普通回归测试产生费用。
2. **凭据只在进程内使用**：真实模式读取 `MROLESCOPE_QWEN_API_KEY` 和 `MROLESCOPE_QWEN_WORKSPACE_ID`，传给 `NewQwenIntentProvider`；错误、trace 和 report 只记录 provider/configuration failure，不记录值。
3. **固定快照继续隔离**：真实模型只负责 Intent、工具调用和回复；Serving reader 仍使用 `evaluation.Snapshot`，从而不访问生产数据库，也不把结果写回生产。
4. **HTTP seam 可替换**：CLI 通过一个 provider 工厂或构造函数注入 `http.Client`，测试用 `httptest.Server` 验证请求、响应、usage、SSE 和错误，不触碰真实网络。
5. **模型元数据进入评测版本**：真实运行的 `VersionInput` 使用 `Model: qwen-plus`、`Provider: qwen`，fixture 继续使用现有标识；报告保持 trend/agent 两套独立入口。

## TDD Seams

- Provider factory：provider 参数、环境变量缺失、真实 provider 构造；捕获误发送和凭据泄露。
- Evaluation runner：真实 provider 与固定 Snapshot 的组合；捕获工具、Draft、回复和统计是否沿用生产路径。
- CLI metadata：真实/fixture provider 和模型写入版本与报告；捕获报告误标。
- HTTP provider contract：本地模拟 Qwen 的 structured response、tool turn、SSE reply、usage 和失败；捕获适配器协议回归。

## Non-Goals

- 不在自动化测试或默认命令中调用真实 Qwen。
- 不把真实评测改成生产数据库查询。
- 不自动读取 `.env` 并把其中内容打印到终端。
