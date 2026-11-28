# Proposal

## Why

需要在不替换 CVM 既有正式 API 容器的前提下，用正式数据验证最新 AI 趋势能力。现有远端发布链只管理正式 `remote-development` 栈，不能安全地启动第二个可访问的 API 实例。

## What Changes

- 新增受控的远端测试 API 发布路径：用同一源码 revision 的不可变 API 镜像启动或替换一个测试容器。
- 测试容器复用既有数据库运行配置，但不启动 PostgreSQL、迁移、worker 或 Web Edge，也不更改正式容器、镜像记录和端口。
- 测试 API 仅绑定 CVM 回环地址的固定测试端口，并记录通过 SSH 隧道和本地 Vite 前端访问的方法。
- 补充发布验证和运行手册，明确测试 API 与正式环境的隔离边界、发布入口和回滚行为。

## Capabilities

### New Capabilities

- `remote-test-api-deployment`: 以受控、可验证的方式在 CVM 运行一个与正式容器隔离、复用正式数据库的回环测试 API。

### Modified Capabilities

- 无。

## Impact

- 受影响代码：远端部署资产、发布脚本、部署配置检查和测试。
- 受影响系统：CVM Docker Compose、Docker Hub 不可变 API 镜像发布，以及远端运行指导。
- 不改变公开 API 合同、数据库 schema、正式容器或公网入口。
