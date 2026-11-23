# Design

## Context

真实 Benchmark CLI 已支持 `-provider qwen`，但当前只存在于开发机。CVM 的部署链已经有 root-owned Compose、runtime env 和受控 AI helper；worker 镜像是远端唯一允许运行一次性分析任务的应用镜像。

## Decisions

1. **复用 worker 镜像**：在 worker 镜像中构建 `mrolescope-ai-evaluation`，并复制两份 Benchmark 与固定快照到固定只读路径，避免 CVM build 源码或挂载开发机工作树。
2. **两个固定服务**：Compose 增加 `ai-trend-evaluation` 和 `ai-agent-evaluation` 两个 one-shot 服务，均使用 `ai-trend-evaluation` profile、无端口、只加入 `ai-edge`，报告目录是唯一写入挂载。
3. **root-owned helper**：新增 helper 只接受 `run-trend`、`run-agent`，不接受路径、query、命令、镜像或 secret 参数；它复用现有 verified release 和 secrets 文件，调用固定 Compose service。
4. **评测隔离**：服务不连接 PostgreSQL、不加入 backend、不写生产审计；快照和 Benchmark 在镜像内只读，报告写入 `/var/lib/mrolescope/ai-evaluation`。
5. **发布边界**：部署资产刷新由现有自主发布流程负责；静态 checker 必须检查 helper allowlist、Compose 网络/挂载/端口、镜像路径和 secret 注入边界。

## TDD Seams

- Helper shell seam：allowlist、无参数、配置缺失、secret 不打印。
- Compose checker seam：服务 profile、网络、端口、挂载、命令和 env allowlist。
- Worker image seam：评测二进制和固定输入文件存在，运行时不需要源码。
- Existing CLI seam：CVM service 调用与本机真实 provider CLI 使用同一参数和报告格式。

## Non-Goals

- 不在本变更中自动运行完整 40 题真实评测。
- 不读取、打印或复制 CVM secret 到开发机。
- 不把 Benchmark 评测接入生产 API、Serving 查询或日常审计。
