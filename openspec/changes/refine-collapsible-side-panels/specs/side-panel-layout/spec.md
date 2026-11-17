# Side Panel Layout

## ADDED Requirements

### Requirement: Desktop side panels reflow the workspace

桌面端必须将左导航栏、主内容和右曲线栏作为同一布局轨道渲染；侧栏展开或收起时，主内容宽度必须同步变化，不得使用浮层覆盖主内容作为默认桌面行为。
The desktop shell MUST render the navigation rail, workspace, and curve rail as one in-flow layout.

#### Scenario: Wide desktop starts with both panels expanded

- **WHEN** a user opens the application at a viewport width of at least `1440px`
- **THEN** the left navigation and right curve configuration rails are expanded
- **AND** the chart/content area is laid out between the two rails

#### Scenario: Compact desktop preserves narrow rails

- **WHEN** a user opens the application at a viewport width between `761px` and `1099px`
- **THEN** both side panels start in narrow icon/marker form
- **AND** the main content remains usable without horizontal overflow

### Requirement: Navigation rail remains usable when collapsed

收起的左导航栏必须保留独立的功能图标、当前路由状态和可访问名称；展开/收起控制必须位于导航栏顶部，不得依赖页面中央的悬浮文字按钮。
The collapsed navigation rail MUST remain operable and identifiable without its text labels.

#### Scenario: User navigates from a collapsed icon

- **WHEN** a user clicks or activates a collapsed navigation icon
- **THEN** the corresponding route opens
- **AND** the active icon receives the navigation active state

### Requirement: Curve rail exposes configured curves

收起的右曲线栏必须用稳定的曲线颜色标记提示已配置曲线；激活标记时必须展开配置栏并定位到对应曲线，而不是触发新的曲线或改变数据口径。
The collapsed curve rail MUST expose configured curves without changing trend data semantics.

#### Scenario: User opens a configured curve from the marker rail

- **WHEN** a user activates a curve color marker in the collapsed right rail
- **THEN** the right configuration rail expands
- **AND** the matching curve card receives focus or visible focus styling

### Requirement: Rail state is transient

侧栏展开/收起状态只能保存在当前页面运行时内，不得写入服务端、浏览器持久化存储、URL 或业务 API。
Rail state MUST remain transient and MUST NOT be persisted to a server, browser storage, URL, or business API.

#### Scenario: Reload resets the responsive default

- **WHEN** a user collapses a side panel and reloads the page
- **THEN** the side panel returns to the responsive default state for the current viewport
- **AND** no side-panel state is read from or written to browser storage

### Requirement: Rail transitions respect motion preferences

桌面侧栏和主内容轨道必须平滑过渡，并在用户偏好减少动画时停止或显著缩短过渡。
Rail transitions MUST respect the user's reduced-motion preference.

#### Scenario: User prefers reduced motion

- **WHEN** `prefers-reduced-motion: reduce` is active
- **THEN** opening and closing side panels does not use a noticeable sliding animation
- **AND** all controls remain operable and visible

### Requirement: Collapsed rails use stable controls and geometry

桌面侧栏收起态必须保持稳定的交互入口和几何尺寸；不能因重复控制、隐式网格拉伸或中等桌面固定宽度导致主内容跳动或右栏被挤压。
Collapsed desktop rails MUST keep stable controls and geometry without duplicate controls or intrinsic-size expansion.

#### Scenario: Navigation rail is reopened from its collapsed brand

- **WHEN** the desktop navigation is collapsed
- **THEN** the rail does not render a second expand arrow
- **AND** activating the collapsed brand icon expands the navigation
- **AND** the collapsed header does not switch to a taller vertical control layout

#### Scenario: Curve rail uses one close control

- **WHEN** the desktop curve configuration rail is expanded
- **THEN** the rail shows one close control
- **AND** the rail does not show a second direction control with the same collapse meaning

#### Scenario: Curve marker rail keeps fixed marker rows

- **WHEN** the desktop curve configuration rail is collapsed
- **THEN** each marker keeps a fixed height of `32px`
- **AND** markers are top-aligned with a fixed gap rather than stretched across the rail height

#### Scenario: Medium desktop preserves the collapsed rail width

- **WHEN** the viewport width is between `1180px` and `1439px` and the curve rail is collapsed
- **THEN** the right rail remains the `48px` collapsed grid track
- **AND** the rail content does not declare an independent `320px` width

### Requirement: Desktop rail content remains mounted during transitions

桌面侧栏的收起态和展开态内容必须同时保留在 DOM 中，通过可见性、透明度和位移切换，避免条件销毁/重建导致闪烁；移动端仍应可通过现有抽屉控制访问配置内容。
Desktop rail states MUST remain mounted during transitions and switch through visibility, opacity, and transform rather than conditional destruction.

#### Scenario: User opens or closes a desktop rail

- **WHEN** a user changes the navigation or curve rail state
- **THEN** the surrounding workspace uses its existing width transition
- **AND** the rail content fades and moves smoothly without an intermediate DOM replacement

#### Scenario: User opens the mobile curve panel

- **WHEN** a user activates the mobile curve configuration entry
- **THEN** the existing bottom panel remains available
- **AND** the panel enters and leaves with a smooth vertical transition unless reduced motion is requested
