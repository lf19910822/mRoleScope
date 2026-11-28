# Tasks

## 1. 受控测试 API 资产

- [x] 1.1 新增只含 `api-test` 的测试 Compose 资产：复用正式 external networks 与运行配置、固定 `127.0.0.1:18080:8080`，且不声明 PostgreSQL、迁移、worker 或 Web Edge；用静态配置检查验证边界。
- [x] 1.2 实现 root-owned 测试 API helper：校验调用者、文件权限、允许的 immutable API digest 与 revision，记录独立发布状态，健康失败时只回滚测试 API；用本地 mock Docker 回归测试验证成功、拒绝和回滚。
- [x] 1.3 扩充 bootstrap 与 sudoers，使已审核 main commit 能安装测试 Compose/helper 并创建独立发布目录；用现有 bootstrap/部署资产测试验证安装和受限调用边界。

## 2. 发布入口与运行指导

- [x] 2.1 扩充 `release-remote-development.sh` 的测试 API 模式：从 clean、同步的 main 运行完整质量门，仅构建/推送 API 镜像、刷新受控资产并调用测试 helper；测试证明正式 `--deploy` 路径保持四镜像行为。
- [x] 2.2 更新自主镜像发布运行手册，明确测试 API 与正式环境的容器、数据库、迁移、端口、访问、回滚差异；核对 SSH 隧道和 Vite 代理命令可按文档执行。

## 3. 集成验证与交付

- [x] 3.1 执行部署配置检查、相关 shell 测试、`go test ./...`、前端单测/构建、OpenSpec strict 校验和 `git diff --check`，记录实际结果（2026-11-28：全部通过；Compose v2 本机缺失时已完成静态边界检查，CVM 部署将执行真实 Compose）。
- [ ] 3.2 推送并合入已验证 main 后执行 `--deploy-test-api`，确认 CVM 测试 API 健康、`18080` 仅回环监听、最新 revision 已运行且正式容器未被测试发布替换。
