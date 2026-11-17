# Spec Delta

## Purpose

让主导航和趋势配置在所有屏幕尺寸都成为按需打开的辅助面板，释放图表与岗位内容的阅读空间，同时保持可访问的入口、关闭方式和现有业务操作。

## ADDED Requirements

### Requirement: Desktop navigation is on demand

应用 SHALL 在桌面视口默认不为主导航保留固定内容列，并提供始终可见的“展开导航”入口；用户展开后 SHALL 能通过关闭按钮收起导航。

#### Scenario: User expands and collapses desktop navigation

- **WHEN** 用户点击桌面侧边的展开导航入口
- **THEN** 主导航面板显示，且用户可点击关闭按钮将其收起，主内容恢复全宽

#### Scenario: Navigation route remains available

- **WHEN** 用户在展开的主导航中切换岗位筛选或趋势看板
- **THEN** 路由正常切换，导航面板按既有规则关闭，目标页面保持可用

### Requirement: Trend configuration is on demand

趋势看板 SHALL 在所有视口提供曲线配置展开入口；配置栏关闭时 SHALL 不在右侧保留固定空间，打开时 SHALL 复用现有曲线编辑器和批量操作。

#### Scenario: User opens and closes curve configuration

- **WHEN** 用户点击趋势看板的配置入口
- **THEN** 曲线配置面板显示，且用户可点击关闭按钮将其收起，图表工作区恢复可用宽度

#### Scenario: User starts the first curve

- **WHEN** 趋势看板没有曲线且用户点击“开始配置首条曲线”
- **THEN** 配置面板打开，用户可以继续添加和应用第一条曲线

### Requirement: Panel controls preserve keyboard access

两个面板 SHALL 提供可识别的按钮名称；面板关闭后焦点 SHALL 返回对应的展开入口，移动端现有遮罩和 Escape 关闭行为 SHALL 保持有效。

#### Scenario: Focus returns after closing

- **WHEN** 用户通过键盘或按钮关闭主导航或曲线配置面板
- **THEN** 焦点返回对应的展开入口

#### Scenario: Mobile behavior remains usable

- **WHEN** 用户在 390px 视口打开导航或曲线配置
- **THEN** 面板不产生横向溢出，且现有移动端抽屉/底部面板可以关闭和再次打开
