# Proposal

## Why

当前技术关键词词典仍将 Unreal、Unity、React/ReAct 等重要词面保留为条件或存在大小写混淆，导致真实 JD 可能漏选、React 与 ReAct 可能误归类，也无法表达 `UE4/UE5 -> Unreal Engine + 版本身份` 的继承关系。现在需要发布不可变的 v0.4 词典并全量重算历史正文，使公开展示只依赖一套可复演、可审计的最新分析结果。

## What Changes

- 新增不可变 `technology-keyword-alias-map-v0.4`，纳入 Unreal、Unity、React/ReAct 修正及审计报告第一批高置信度候选；下列五组关键词均为 v0.4 首期必须纳入的正式词典范围，不得仅停留在候选记录：
  - Unreal 子系统：`UObject`、`World Partition`、`WorldPartition`、`StateTree`、`MassEntity`、`GameThread`、`BuildGraph`；
  - 游戏工具：`Wwise`、`FMOD`、`FMOD Studio`、`CRIWARE`、`Criware`、`Maya`；
  - GPU/性能：`DCGM`、`Nsight Systems`、`Nsight Compute`、`NVSwitch`、`CANN`、`NUMA`、`PCIe`、`RoCE`、`ROCE`；
  - Kubernetes 组件：`kubelet`、`kube-scheduler`、`controller-manager`、`KubeFed`、`controller-runtime`、`client-go`、`KEDA`；
  - 基础设施：`Elasticsearch`、`Ceph`、`OpenStack`、`QEMU`、`libvirt`、`cloud-init`。
- 将 `UE`、`UE4`、`UE5`、`UE5.4` 的基础身份与版本身份关系显式建模；命中版本词面时同时产生 `Unreal Engine` 和对应版本身份。
- 将 `Unity`、`unity`、`Unity 3D`、`Unity3D` 归入 `Unity`，不把同义写法拆成多个展示身份。
- 将 `React`、`react` 与精确大小写的 `ReAct` 分离，禁止全局大小写不敏感规则混淆二者。
- 明确技术词面默认直接命中；排除规则只收录真实 JD 或审计样本中观察到、可由固定字符串规则复演的普通语言反例。
- 将词面规则、排除规则、身份展开关系及其指纹纳入不可变分析配置，保证全量重算和证据校验可复演。
- 对全部历史 JD 正文版本执行 v0.4 重分析，并在完整结果、证据坐标和发布门禁通过后原子切换 Serving 发布指针。
- **BREAKING**：v0.4 发布后，岗位筛选、技术关键词选项、趋势统计和前端展示不再回退读取 v0.3；v0.3 结果保留用于审计和回溯，但不再参与当前产品展示。
- **BREAKING**：Serving 只允许读取 active publication 对应的 v0.4 结果；候选分析期间继续使用 v0.3，不能提前暴露部分 v0.4 结果。

## Capabilities

### New Capabilities

- `jd-technology-dictionary`: 提供版本化技术身份、精确词面、边界、排除规则和父子身份展开的确定性词典能力。
- `jd-technology-analysis-publication`: 提供按新词典全量重算、证据复演、发布门禁、active publication 切换及 Serving 只读最新结果的生命周期能力。

### Modified Capabilities

当前项目没有 durable main specs；相关 v0.1–v0.3 行为位于既有 change-local specs 中。因此本变更以新的 durable capability 记录 v0.4 的完整行为契约，不直接改写历史 change-local spec。

## Impact

- **后端与数据层**：`internal/jdanalysis` matcher、词典种子、规则快照、证据校验、发布流程；新增 PostgreSQL 迁移以保存排除规则和身份展开关系的不可变输入。
- **分析运行**：worker 的候选配置、全量历史正文队列、重分析完成门禁、v0.4 发布与 v0.3 superseded 记录。
- **Serving**：技术结果、过滤选项、趋势统计和岗位详情必须约束到 active publication 对应配置；不得混入旧版本结果。
- **测试**：Go 单元/集成测试覆盖 Unreal 版本继承、Unity 独立命中、React/ReAct 大小写、明确反例抑制、全量重算和原子发布；仅使用合成正文。
- **运维与文档**：增加 v0.4 受控 seed/approve/prepare/analyze/publish 操作及审计文档；不读取或输出真实 JD 正文、凭据或外部来源内容。
- **不改变**：来源采集接口、每日同步调度、ODS 原始事实、历史岗位正文、公开 API 的无关字段及前端技术筛选的基本交互模型。
