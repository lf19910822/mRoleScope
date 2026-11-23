# Tasks

## 1. Provider factory and CLI contract

- [x] 1.1 为评测 CLI 增加显式 `-provider fixture|qwen` 参数，默认 fixture；先写参数解析和默认行为失败测试。
- [x] 1.2 为真实 provider 增加环境变量读取和缺失配置错误；先写“不发 HTTP 请求、不泄露凭据”的失败测试。
- [x] 1.3 接入 `aitrend.NewQwenIntentProvider`，按 suite 选择趋势或 Agent Benchmark，并将 provider/model 元数据写入版本；先写元数据测试。

## 2. Real provider evaluation seam

- [x] 2.1 用 httptest 完成 Qwen structured Intent/tool-turn provider contract fixture，覆盖 usage 和安全错误。
- [x] 2.2 用 httptest 完成 Qwen assistant SSE fixture，覆盖多个 delta、结束事件、usage、取消和上游错误。
- [x] 2.3 让 evaluation Runner 通过真实 provider + 固定 Snapshot 完成一条 trend 和一条 Agent 端到端 fixture；验证 Draft、assistant_message、工具次数和 token 汇总。

## 3. Reporting and safety

- [x] 3.1 记录真实 provider 的模型、token、成本、首事件/总耗时和终态，保持原始回复、凭据和完整 payload 脱敏。
- [x] 3.2 增加 CLI 帮助和评测文档，明确真实模式会产生模型费用，且两套 Benchmark 仍分开统计。
- [x] 3.3 运行 Go 全量测试、真实 provider fixture、两套 fixture Benchmark、前端检查、`go vet`、`git diff --check` 和 OpenSpec 严格校验；不在本任务中自动执行真实 Qwen。
