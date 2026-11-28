# Design

## Context

参见 `proposal.md`。现有 `scripts/release-remote-development.sh --deploy` 会构建四个镜像，并调用只管理 `mrolescope-remote-development` 的 root helper；该路径会启动 PostgreSQL、执行迁移并替换正式 API，不能用于候选版本验证。正式 API 已在 `backend`、`edge` 与 `ai-edge` Compose 网络中运行，数据库连接使用稳定的服务名 `postgres`；本地 Vite 已支持以 `VITE_API_PROXY_TARGET` 指向回环 API 隧道。

## Goals / Non-Goals

**Goals:**

- 从受保护 `main` 构建并以不可变 digest 发布最新 API 镜像，在 CVM 启动独立的测试 API 容器。
- 复用正式数据库和 API 所需的受控运行配置，但隔离容器、Compose 项目、锁、发布记录和宿主机端口。
- 让测试入口只经 `127.0.0.1:18080` 和 SSH 隧道访问，并提供独立健康检查和测试实例回滚。
- 扩充同一份自主镜像发布手册，明确正式发布与测试 API 发布的区别。

**Non-Goals:**

- 不创建第二套 PostgreSQL、数据副本、迁移、worker、Web Edge、域名、TLS 或公网端口。
- 不将测试 API 标为只读，也不承诺其对共享数据库产生的正常应用审计写入会被隔离。
- 不改变正式发布命令的四镜像构建、迁移、正式 API 回滚或 Web Edge 行为。

## Decisions

### 1. 在现有发布脚本中增加受限的测试 API 模式

`scripts/release-remote-development.sh` 增加 `--deploy-test-api`。它沿用 `main` 同步检查、CVM Docker 拉取预检、完整本机质量门和部署资产刷新，但只构建、推送并部署 `mrolescope-api`。正式 `--deploy` 保持原有四镜像路径不变。

选择同一脚本而非手工 SSH 或独立无校验脚本，使测试镜像仍有 revision 标签、digest、质量门和 root-owned helper 的审计边界。备选的“直接 docker run”会绕过这些边界，也容易误连或替换正式容器。

### 2. 测试 API 使用独立 Compose 项目和 root-owned helper

在 `deploy/remote-development/` 增加仅含 `api-test` 服务的 Compose 文件和专用 helper。bootstrap 同时安装它们，并仅允许 `mrolescope-deploy` 通过固定 sudo 命令调用。测试 helper 使用独立的项目名、锁和发布目录，只接受允许命名空间内的 `mrolescope-api@sha256:...`，并验证镜像 revision 标签。

正式与测试部署资产继续由同一个已合入 `main` 的 bootstrap 刷新；刷新资产不启动任何服务。备选的自定义 CVM shell 命令会绕过 root-owned 文件权限、输入校验和可复现的 bootstrap 路径。

### 3. 复用正式运行配置与网络，但不管理其服务

测试 Compose 通过 root-owned `remote-development.env` 获取数据库及 AI 配置，并以 external network 连接既有 `mrolescope-remote-development_backend`、`edge` 和 `ai-edge` 网络。这样 `DATABASE_URL` 中的 `postgres` 服务名继续可解析，AI 调用维持既有出网边界；Compose 文件不声明 PostgreSQL、迁移、worker 或 Web Edge 服务。

这是用户明确选择的共享数据方案。备选的独立数据库会隔离数据但增加初始化、数据同步和密钥管理；使用正式 Compose 项目则会使测试操作影响正式容器。

### 4. 固定回环端口与独立回滚

`api-test` 将 `8080` 固定映射为 `127.0.0.1:18080`。helper 只启动该服务，等待 `/healthz` 与 `/readyz` 对应容器健康检查；成功后记录测试 digest/revision。失败时恢复先前测试 digest，若不存在先前成功版本则停止失败的测试服务。它绝不操作正式发布记录、数据库或正式容器。

选择回环 SSH 隧道而非公网端口，避免安全组、域名、TLS 和正式 Web Edge 的人工门槛。备选的 `0.0.0.0:18080` 会不必要地暴露未经 Web Edge 门禁的 API。

## Risks / Trade-offs

- [共享数据库使测试 API 的正常审计或会话写入可见] → 文档明确这不是数据隔离环境；不运行迁移、worker 或采集，且测试入口仅本机隧道可达。
- [既有 Compose 网络不存在或被外部改名] → helper 在拉取或启动前验证所需 external network；失败时不触碰正式服务。
- [候选 API 健康失败] → 使用独立测试发布记录回滚测试实例；正式 API 和数据库不参与回滚。
- [18080 已被占用] → Compose 启动失败并保持先前测试实例或停止新实例；不自动选择其他端口。

## Migration Plan

1. 将部署资产、helper、测试和运行手册合入 `main`。
2. 从 clean 且与 `origin/main` 同步的本机执行 `scripts/release-remote-development.sh --deploy-test-api`；脚本完成质量门、API 镜像发布、受控资产刷新和测试 helper 调用。
3. 在开发机建立 `ssh -N -L 18080:127.0.0.1:18080 root@118.89.68.77`，再以 `VITE_API_PROXY_TARGET=http://127.0.0.1:18080` 启动本地 Vite 前端。
4. 若候选 API 不健康，helper 自动恢复先前测试版本；正式发布仍仅使用 `--deploy`。
