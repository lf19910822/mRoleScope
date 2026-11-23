# Proposal

## Why

本机真实评测入口已经可以调用 Qwen，但 CVM 上的生产服务才拥有受控运行时配置和更接近生产的网络位置。当前 CVM 没有安全、固定、可审计的 Benchmark 执行入口，不能直接利用服务内部配置进行真实评测。

## What Changes

- 增加 CVM 内部固定 trend/Agent Benchmark 一次性运行入口。
- 复用 worker 镜像中的评测二进制、Benchmark 和固定快照。
- 由 root-owned helper 读取现有运行时 secret，并只注入对应评测容器。
- 评测报告写入专用 root-owned 目录，不写入 Serving、用户看板或日常审计。
- 增加 Compose、部署资产和安全边界检查。

## Capabilities

### New Capabilities

- `cvm-real-ai-evaluation`: 受控、隔离、可脱敏的 CVM 内部真实 Benchmark 执行。

### Modified Capabilities

无。

## Impact

- 影响 `docker/Dockerfile.worker`、CVM Compose、root-owned evaluation helper 和部署静态检查。
- 不改变生产 API 和 Serving 查询接口。
- 真实评测会产生 Qwen 模型费用，只能通过固定 allowlist 手动启动。
