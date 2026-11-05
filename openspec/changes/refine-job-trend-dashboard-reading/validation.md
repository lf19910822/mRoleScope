# 趋势看板阅读层验证

验证日期：2026-11-05。

## TDD 与自动化

以下命令在本地通过：

```bash
cd web && npm run test:unit -- --run src/router/__tests__/router.spec.ts
cd web && npm run test:unit -- --run src/lib/__tests__/job-trend-chart.spec.ts
cd web && npm run test:unit -- --run src/views/__tests__/JobTrendDashboardReading.spec.ts src/views/__tests__/JobTrendView.spec.ts
cd web && npm run test:unit -- --run
cd web && npm run type-check
cd web && npm run build
openspec validate refine-job-trend-dashboard-reading --strict
git diff --check
```

结果：23 个前端测试文件、121 个测试通过；类型检查、构建、严格 OpenSpec
校验和差异空白检查均通过。红测覆盖 `/trends` 别名、首条曲线单一入口、稳定
色标、零值/缺口摘要、全部历史手势、移动安全区、暗色令牌和点位不产生虚假
反馈。

## 5175 浏览器验收

固定入口为 `http://127.0.0.1:5175/job-trends`。本地 Vite 通过一次临时、仅绑定
回环地址的 SSH 隧道读取 CVM API；没有暴露 API 或数据库端口。验证使用的真实
趋势响应只包含 Go 曲线的日期与聚合岗位数，没有读取岗位 ID、正文或凭据。

- 空态桌面和 390px 截图确认：标题、时间范围、唯一“开始配置首条曲线”动作和稳定
  图表区域均不跳动；空态不显示刷新按钮，也不显示移动配置入口。
- 配置态桌面截图确认：全部历史提示、复位/刷新操作、折线、紧凑图例、配置栏色标
  和“最近观测 40 · 较前一有效日 0”摘要同时可读；配置栏顶端与页面对齐并固定在右侧。
- 配置态 390px 截图确认：图表使用可用宽度，标题不截断，固定“配置曲线（1）”
  入口位于安全区上方；窄屏配置面板不会覆盖图表内容。
- 暗色主题截图确认：卡片、配置栏、边框、标题和摘要使用同一组局部暗色令牌；
  ECharts 暗色主题与页面背景连续，未保留浅色卡片或浅色边框。
- 八条曲线的独立图例、零值与 `null` 缺口由 `job-trend-chart` 自动化测试覆盖；
  稳定色标由配置栏挂载测试和图表选项测试共同覆盖。

当前工作树的 Vite 预览没有将临时隧道配置写入项目文件；交付部署仍使用既有
5175/Web Edge 入口和现有 Serving API 合同。
