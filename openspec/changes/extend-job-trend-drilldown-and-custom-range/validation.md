# 自定义范围与历史节点下钻交付验证

验证日期：2026-11-06。

## 实现与 TDD 对照

- 自定义日期范围已由 Serving resolver、HTTP 参数校验、OpenAPI 与看板状态共同处理；起止日期为
  包含式 Asia/Shanghai 自然日，缓存键包含自定义范围。
- 历史节点列表经 `TrendDrilldownReader` 从节点当日最终成功发布的 membership 重建。它使用正式解释
  release 解释条件，区分可信 `0` 与不可用 `null`，并以 SCD2/current inventory 映射“仍有效／有改动／确认下线”。
- 历史详情通过节点日期选择 SCD2 版本，不能恢复时安全失败，不回退到当前 JD；HTTP 与 OpenAPI 只暴露安全 DTO。
- 看板在同一公开状态 seam 上覆盖空值不下钻、零值空态、同节点关闭、换节点复用、300ms 搜索防抖、翻页重置、
  重试、10 分钟会话缓存与旧响应隔离。桌面使用节点锚定气泡，390px 使用锁定背景滚动的 bottom sheet。
- 本次补充了组件回归：三种状态分别只显示其允许操作，且 opaque 岗位 ID 不进入可见 DOM。

完整迁移夹具使用一次性 PostgreSQL 执行；没有因缺少 `TEST_DATABASE_URL` 而跳过下钻历史、状态映射或详情恢复测试。

## 本地质量门

以下检查均通过：

```bash
make remote-release-verify
GOCACHE="$(mktemp -d /tmp/mrolescope-go-build.XXXXXX)" \
  go test ./internal/httpserver ./internal/jdserving -count=1
cd web && npm run test:unit -- --run \
  src/api/__tests__/jd-api.spec.ts \
  src/lib/__tests__/job-trend-chart.spec.ts \
  src/lib/__tests__/job-trend-dashboard-state.spec.ts \
  src/lib/__tests__/job-trend-drilldown-state.spec.ts \
  src/views/__tests__/JobTrendDashboardReading.spec.ts \
  src/views/__tests__/JobTrendView.spec.ts
cd web && npm run test:unit -- --run src/views/__tests__/JobTrendDashboardReading.spec.ts
cd web && npm run type-check
cd web && npm run build
openspec validate extend-job-trend-drilldown-and-custom-range --strict
git diff --check
```

结果：完整隔离验证涵盖 SQLC 一致性、Go/Python、临时 PostgreSQL migration/SCD2 集成、前端类型/单元/构建、
Web Edge 与部署资产检查；最终聚焦前端集共 51 项通过，其中 `JobTrendDashboardReading` 为 8 项通过。临时数据库
和容器由验证脚本自动清理，未触及 CVM 数据库。

## CVM 私网性能采样

使用 CVM Docker 的动态 API 容器发现，在宿主机私网内对保留历史数据执行 20 次只读采样。探针没有输出或保存
岗位标题、ID、JD 正文、连接串或其他凭据；详情响应仅在内存中计时后丢弃。

| 操作 | 样本 | p95 | 验收预算 |
| --- | ---: | ---: | ---: |
| 自定义范围聚合（15 个数值点） | 20 | 8.1ms | 不阻塞列表读取 |
| 一个历史节点列表 | 20 | 14.2ms | ≤ 1s |
| 允许读取的详情 | 20 | 7.2ms | ≤ 1.5s |

列表与详情均显著低于首期预算；前端在请求开始时同步保留固定尺寸骨架，因此满足 100ms 内出现加载容器的体验目标，
无需新增索引、快照表或聚合任务。

## 固定本地入口验收

复用已运行、同仓库且仅绑定回环的 Vite 进程：`http://127.0.0.1:5175/job-trends`。该进程的筛选选项和趋势代理
均返回 HTTP 200；本轮临时创建的 API 隧道也只绑定在本机 `127.0.0.1`，没有公开 API 或数据库。

- 1440px：左侧导航、居中固定图表区、右侧配置栏与底部操作栏完整可见；空态只保留“开始配置首条曲线”，没有
  与此重复的主入口或布局跳动。
- 390px：标题、范围按钮和唯一空态操作完整可见，无横向溢出；桌面侧栏没有挤压图表。
- 节点锚定、视口约束、历史详情关闭后返回同一列表、可信 `0` 的移动抽屉/滚动恢复、`null` 不打开、搜索、分页、
  重试与三种状态操作由上述组件/状态与迁移夹具测试覆盖。真实数据采样不伪造不存在的状态组合。

## 发布事实

趋势下钻与自定义范围已包含在 `origin/main` 的 `f0c5a76`，CVM 私网 API 对本变更的趋势、下钻和详情读取均返回
成功。最终验证阶段未再次构建、推送或部署镜像。今后的发布不再使用 GitHub CI/CD；必须遵循
[`自主镜像发布运行手册`](../../../Docs/开发指导/部署/自主镜像发布运行手册.md) 和受控本机发布脚本。
