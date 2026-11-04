# 趋势看板交付验证

验证日期：2026-11-04。

## 本地与 CI

以下本地命令均通过：

```bash
sqlc generate && git diff --exit-code -- internal/database/sqlc
go test ./...
cd web && npm run test:unit -- --run
cd web && npm run type-check
cd web && npm run build
openspec validate add-job-trend-dashboard --strict
make deploy-config-check
git diff --check
```

前端单元测试为 22 个文件、112 个断言。开发机没有独立的本地集成数据库；合并前后由
GitHub Actions 的 PostgreSQL 服务执行了迁移、SCD2 和趋势相关集成检查，均通过。

PR #54 的首次 CI 发现新增趋势迁移漏提交 SQLC 自动生成模型。已补齐生成的
`internal/database/sqlc/models.go`，并再次通过 SQLC 差异检查、全量 Go 测试和 CI。

## 发布与真实历史性能

- PR #54 已以 rebase 合并到受保护的 `main`；其后续 CVM 发布工作流成功完成。
- CVM 上通过已有固定运维入口执行了待处理确定性分析及趋势正式版本发布门槛。该入口只在
  Compose 私网访问 PostgreSQL，未读取或输出凭据、JD 正文或岗位标识。
- 使用 API 的私网地址验证了正式趋势路由：全历史学历条件返回 HTTP 200；仅记录到约
  49ms 的响应时间与 638 字节响应大小，未保存响应正文。
- 以看板的八条独立曲线并发方式测量“全部历史”：八个请求均返回 HTTP 200，整体为
  149ms，低于 2 秒门槛。因此保留当前按需、非物化的 Serving 查询，不新增索引、每日
  聚合表或后台预计算任务。

## 前端人工布局检查

已在固定本地入口 `http://127.0.0.1:5175/job-trends` 用无头浏览器检查 1440px 与 390px
视口：桌面端左导航、中央稳定图表区和右侧配置栏可见；390px 端标题未截断且“配置曲线”
入口可达。该现有 Vite 进程没有配置 API 代理，因此本次本地人工检查只覆盖布局和空态，
不将其误记为真实 Serving 数据验收；真实路由验收已在 CVM 私网完成。

节点下钻到岗位筛选功能仍按已确认边界搁置，本期未实现或预留跳转载荷。
