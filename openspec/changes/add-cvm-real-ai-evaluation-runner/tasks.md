# Tasks

## 1. CVM helper contract

- [x] 1.1 为 root-owned CVM helper 写 allowlist、无参数和 secret 不打印的失败测试。
- [x] 1.2 实现固定 `run-trend` / `run-agent` 操作，加载 verified release 和 runtime env，不接受任意用户输入。

## 2. Compose and image isolation

- [x] 2.1 为 Compose trend/Agent one-shot 服务写静态边界测试：固定 worker image、固定 Benchmark 路径、仅 ai-edge 出网、无端口和无生产数据挂载。
- [x] 2.2 将评测二进制、Benchmark 和固定快照纳入 worker image，并验证容器只写专用报告目录。
- [x] 2.3 更新部署 bootstrap/checker，确保 helper、Compose 服务和报告目录按 root-owned 边界安装。

## 3. Verification

- [x] 3.1 运行 Go 全量测试、前端检查、部署静态检查、`go vet`、`git diff --check` 和 OpenSpec 严格校验。
- [ ] 3.2 在 CVM 内执行一次真实 trend smoke，确认脱敏报告和模型调用；不自动运行完整 30+10 题集。
