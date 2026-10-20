# 架构说明

## 架构原则

本项目是模块化单体：同一 Go 模块提供 API、worker 和共享领域逻辑，以独立进程运行。它避免过早拆分服务，同时以明确边界隔离采集、归档、解析、聚合和展示。

```
云主机定时器
    │
    ▼
Go worker ──受控 argv──> Python collector ──> 公开招聘 API
    │                            │
    │                            └──> task directory + manifest
    ▼
验证 → 原文归档 → 解析 → 聚合 → 短事务发布
    │          │          │
    │          │          └────────── PostgreSQL
    │          └───────────────────── 对象存储（生产）/本地（开发）
    ▼
Go Gin REST API ───────────────> Vue 3 浏览器前端
```

## 组件职责

| 组件 | 职责 | 不负责 |
| --- | --- | --- |
| Python collector | 请求公开 API、限速重试、保存原始响应、输出带版本 manifest 的任务目录 | 业务解析、聚合、发布网站数据 |
| Go worker | 任务租约、启动 collector、校验、归档、解析、聚合、幂等恢复和发布切换 | 通过 shell 拼接运行外部命令 |
| Go API | 匿名只读 REST API、健康与就绪检查、OpenAPI 契约 | 管理后台或直接暴露数据库 |
| PostgreSQL | 批次元数据、版本化正文、批次映射、解析结果、变化事件、聚合快照 | 原始归档的唯一副本 |
| Archive store | 成功批次的不可变压缩原始归档 | 浏览器直写或凭据分发 |
| Vue frontend | 展示、筛选、加载/错误/空态、确定性 diff 的渲染 | 抓取、聚合或判定岗位关闭 |

## 运行与发布状态

`queued → collecting → validating → archiving → parsing → aggregating → published`。

失败记录阶段与错误，不修改 `published_run_id`。采集器退出、归档上传和数据库写入跨系统不构成一个事务，因此 worker 必须逐阶段记录可恢复状态并检查孤立产物；只有所有校验完成后才在短数据库事务内切换当前发布批次。

## 本地与生产

本地的 Docker Compose 只启动 PostgreSQL；对象归档由本地目录适配器承担。生产使用单台个人云主机的反向代理、API、worker 和 PostgreSQL，归档接入经验证的目标对象存储 SDK。具体云厂商、区域、域名和备案要求均未决定，当前不创建或购买任何资源。

## 安全边界

- 仅 HTTPS 入口对公网开放。
- `DATABASE_URL`、对象存储密钥和管理触发凭据只存在部署环境与个人 CI/CD 密钥管理中。
- collector 只保存公开业务响应，绝不归档 Cookie、Authorization 或其他敏感头。
- JD 一律按不可信文本处理：不执行其中内容，前端必须安全转义。

## M0 模块边界

- `cmd/api`：Gin 路由与进程生命周期。
- `cmd/worker`：受控 collector 启动入口，后续承载状态机。
- `internal/config`：环境配置和默认值。
- `internal/collector`：manifest schema 校验和受控 Python 执行。
- `migrations`、`db/query`、`sqlc.yaml`：显式 SQL 与生成查询的边界。
- `collector`：零第三方依赖的 Python 采集器与离线 fixture。
