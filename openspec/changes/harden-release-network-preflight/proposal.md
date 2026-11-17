# Proposal

## Why

发布链路曾把 Docker Hub 官方 Registry 直连超时误判为实际 Docker 拉取失败，导致不必要地暂停后续工作并提出离线镜像方案。现在需要让发布流程自身验证并报告 CVM Docker daemon 的真实拉取路径，避免把备用网络路径当成主发布链路。

## What Changes

- 在正式构建和推送前，通过 CVM 的 Docker daemon 对 Compose 使用的固定 PostgreSQL 镜像执行一次受控、单次的真实 `docker pull` 预检。
- 预检输出 Docker daemon 的 registry mirror 配置和明确的实际验证对象，但不执行官方 Registry 直连探测。
- 在 CVM helper 的 Compose 镜像拉取失败时，报告实际 Compose pull 边界、镜像源配置和失败阶段，不把错误归因于未经验证的直连路径。
- 为本地发布编排器和 CVM helper 增加回归测试，并同步更新自主镜像发布运行手册。
- 保持 `--dry-run` 不连接 CVM、不执行远程拉取；保持现有四镜像 immutable digest、回滚和权限边界不变。

## Capabilities

### New Capabilities

- `release-network-preflight`: 发布前验证并解释 CVM Docker daemon 实际镜像拉取链路。

### Modified Capabilities

无。

## Impact

- 修改 `scripts/release-remote-development.sh`、`deploy/remote-development/mrolescope-deploy` 及其 shell 回归测试。
- 更新 `Docs/开发指导/部署/自主镜像发布运行手册.md`。
- 不修改应用 API、数据库结构、运行时密钥、公网端口或镜像发布目标。
