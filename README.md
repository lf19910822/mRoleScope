# MHY JD Observatory

非官方的米哈游「程序&技术」社招 JD 数据观察项目。它采集公开岗位、永久保存每个成功批次的完整原始归档，并以确定性规则生成可审计的变化、解析与图表数据。

项目仍处于 M0 基础工程阶段：没有生产采集结果、没有真实历史图表，也没有部署到任何云环境。完整范围、边界和待定项见 [docs/PRD.md](docs/PRD.md)。

## 本地开始

```bash
cp .env.example .env
# 修改 POSTGRES_PASSWORD 后：
make db-up
make migrate-up
go run ./cmd/api
curl http://localhost:8080/healthz
```

在另一个终端，运行不会请求官网的 collector 契约检查：

```bash
python3 -m unittest collector.test_collector
```

## 前端初始化状态

前端必须经 Vue 官方脚手架生成，而不是手写替代模板：

```bash
./scripts/bootstrap-web.sh
```

它使用与项目 Node 20.20.2 兼容的官方 `create-vue@3.22.0`，生成 Vue 3 + TypeScript + Vue Router + Vitest + ESLint + Prettier 初始工程，再安装 Naive UI 和 ECharts。脚本以临时 `npm@11.21.1` 处理当前依赖图，不会修改全局 npm。生成并提交 `web/package-lock.json` 后，CI 会自动启用前端类型检查、单测与构建。

Go 的版本在 `go.mod`、Dockerfile 与 CI 中固定为 1.22.5，Gin 和 pgx 也使用精确版本；依赖已通过 `go mod tidy` 锁定并完成测试。前端 Node 版本固定在 `.nvmrc` 和 CI 的 `20.20.2`。运行 `make test` 执行 Go 与 Python 测试，前端测试命令见 `web/package.json`。

## 常用命令

```bash
make test
make collector-fixture
make db-up
make migrate-up
```

采集器、数据库和 API 契约分别见 [collector/README.md](collector/README.md)、[docs/data-contract.md](docs/data-contract.md) 与 [openapi/openapi.yaml](openapi/openapi.yaml)。部署没有被执行；计划和恢复要求见 [docs/deployment.md](docs/deployment.md) 与 [docs/backup-restore.md](docs/backup-restore.md)。
