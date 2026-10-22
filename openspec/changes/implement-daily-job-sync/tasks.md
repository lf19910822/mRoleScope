# Tasks

## 1. 数据模型、迁移与查询边界

- [x] 1.1 新增向前兼容的 PostgreSQL 迁移，建立 `observation_scopes`、`job_presence_periods`、`job_content_periods` 和 `collection_run_audit_events`，并扩展 `collection_runs` 的范围、数据日、触发方式、尝试次数与归档摘要字段；以约束和部分唯一索引保证同一范围只有一个开放可见期和一个活动内容期，验证在空库执行全部迁移成功且迁移测试断言这些约束生效。
- [x] 1.2 调整现有运行互斥和发布状态的数据库模型，使活跃运行、最近可用基线及锁定按观察范围隔离，且不写入现有 `change_events`；生成对应 sqlc 查询和类型，验证 `sqlc generate` 无差异并通过查询编译测试。
- [x] 1.3 实现运行摘要、追加式审计事件、岗位/版本、逐运行观察与可见期/内容期的事务查询，覆盖初始化、当前状态读取及失败写入，验证 PostgreSQL 集成测试可检查行数、外键和审计排序。
- [x] 1.4 为新增表和按范围、运行、当前可见状态、近期失败读取的访问路径添加必要索引，并在迁移设计说明中记录 `TIMESTAMPTZ`、`data_date` 和左闭右开内容期口径，验证使用代表性查询的 `EXPLAIN` 不需要扫描 JD 正文或完整 JSON。

## 2. 采集解释、内容规范化与缺失复核

- [x] 2.1 在 Go 同步域中定义米哈游岗位的规范化业务对象和确定性 SHA-256：包含会展示/分析的业务字段，规范化空值、空白、换行、HTML 表示和键顺序，排除传输及易变技术字段；以单元测试验证等价输入 hash 相同、标题/地点/JD 改变 hash 不同、技术字段改变不产生版本。
- [x] 2.2 将经过 manifest 校验的 collector 原始详情响应解析为稳定岗位身份、规范化内容和单次观察，保持 Python collector 的完整目录协议；用既有离线 fixture 测试列表/详情不完整、ID 不匹配和非法文件时绝不产生可发布观察。
- [x] 2.3 为“列表未出现”的岗位增加低并发详情复核路径与 `available`、`definitively_unavailable`、`indeterminate` 三态结果；先以受控内存请求和 fixture 证明来源的明确不可用语义，验证超时、429、5xx、未知结构均为 `indeterminate`，若语义无法证明则该分类器保持禁用且测试锁定该保守行为。
- [x] 2.4 记录列表遗漏但详情仍可用的安全审计信息，并确保它不会被当作当前列表观察；验证 fixture 场景不会关闭原可见期，且审计内容不含完整 JD 或原始响应。

## 3. 同步状态机、SCD2 与审计

- [x] 3.1 将 `cmd/worker` 从“校验后退出”演进为一次性同步命令：创建运行记录，按 collecting、validating、archiving、persisting、published/failed 阶段推进，并在每个边界写审计事件；验证离线成功和失败路径均留下可追溯的运行摘要与有序事件。
- [x] 3.2 实现按观察范围取得 PostgreSQL advisory lock 的发布协调，并让本地调用获得清晰的争用结果而不改写岗位历史；验证两个并发 worker 中只有一个能进入状态变更事务。
- [x] 3.3 在单个发布事务中写入岗位身份、去重的 `jd_versions`、`run_jobs`、可见期和内容期：首次成功只建基线，未变内容复用版本，内容改变仅切换内容期，重现新开可见期；用 PostgreSQL 集成测试逐一验证这些状态转换。
- [x] 3.4 在同一发布事务中处理缺失复核：仅确定不可用时关闭可见期及其活动内容期，详情可用或不确定时保持开放；验证不完整批次、失败批次与不同观察范围都不会关闭或比较既有岗位。
- [x] 3.5 为运行重试和重复输入实现幂等性，确保不会产生重复版本、可见期、内容期、运行观察或错误的基线推进；验证相同 fixture/已归档批次重复执行后的业务状态不变，并保留合法的尝试审计。
- [x] 3.6 集中实现审计和日志的脱敏、限长与稳定错误代码，禁止密码、Token、DSN、完整请求头、原始 JSON 和 JD 正文进入数据库或日志；验证包含模拟密钥和正文的失败会被安全摘要替代。

## 4. COS 原始证据归档

- [x] 4.1 增加腾讯云 COS 的 S3 兼容归档适配器和显式运行时配置（私有桶、端点、前缀、区域及最小权限凭据），按公司/过滤器哈希/数据日/运行 ID 上传 manifest 与经校验文件；验证 fake S3 或等价测试服务收到期望对象路径与最小权限配置校验。
- [x] 4.2 为每个对象写入预期 SHA-256 元数据，并在归档完成前核验对象存在、长度和摘要；验证篡改、缺件、元数据不匹配或上传失败都会阻止发布且不会更新岗位历史。
- [x] 4.3 将“验证任务目录 → COS 上传并核验 → 单个数据库发布事务”固化为严格顺序，保存 archive URI 与 manifest digest；验证数据库失败后只留下未发布的可调查 COS 前缀、此前可用基线不变，成功或失败退出都会清除 CVM 临时目录。
- [x] 4.4 在运行时配置与运维文档中说明桶必须私有、无需自动生命周期删除、凭据只能位于 root-only 环境文件；验证示例不包含真实凭据且 worker 的配置错误不会回显敏感值。

## 5. Worker 镜像与受控远端部署

- [x] 5.1 添加包含 Go worker、Python 运行时和现有 collector 的可复现 worker Docker 构建目标，保留 API 正常启动行为；验证镜像可用离线 fixture 完成 worker 流程，且正常 Compose 启动不会自动执行同步。
- [x] 5.2 将 worker 声明为远端 Compose 的无端口、按需 profile/one-shot 服务：通过内部 backend 访问 PostgreSQL，并通过不发布端口的 edge 网络仅作米哈游/COS HTTPS 出站；为其接入临时目录配置，验证 `docker compose config` 显示数据库未暴露公网、worker 没有 published port 且持久 API 栈不拉起 worker。
- [x] 5.3 扩展 Docker Hub 构建与 GitHub Actions 部署工作流，生成、验证并传递不可变 worker 镜像 digest；扩展 root 受控部署入口的 digest allowlist，验证伪造/可变 tag 被拒绝、API/migration 既有部署路径仍可用。
- [x] 5.4 为 worker 的运行时环境变量、timeout、临时目录和重试策略添加严格校验与安全默认值，验证缺失 COS 设置、错误 digest 或不安全路径会在真实采集前失败并给出脱敏诊断。

## 6. CVM 同步入口与定时运行

- [ ] 6.1 添加 root-owned `mrolescope-sync` 入口，使用 `flock` 防止本机重叠、创建并清理私有临时目录，并以 Compose one-shot worker 执行 `initialize`、`scheduled` 与 `retry` 触发；验证非 root 部署用户不能修改密钥或调用越权 Docker 命令，且并发调用不会启动第二个同步容器。
- [ ] 6.2 添加 `systemd` oneshot service 与 timer：`Asia/Shanghai` 每日 02:00 启动、`Persistent=false` 禁止恢复后补跑，整轮失败最多以 15 分钟和 45 分钟非重叠延迟再试两次；验证 unit 文件通过 `systemd-analyze verify`，并用受控失败命令验证尝试上限与不会补跑的配置。
- [ ] 6.3 编写远端部署/运维文档，给出先配置私有 COS 和 root-only 密钥、显式初始化、检查 COS/数据库/审计、再启用 timer 的顺序，以及禁用 timer 的回滚步骤；验证每条命令与实际文件路径、服务名称和镜像 digest 参数一致，且文档不要求开放任何新入站端口。
- [ ] 6.4 记录 CVM GitHub Actions runner 仅构建测试和部署镜像、而非调度生产采集的职责边界，并验证 scheduled service 仅由本机 systemd 触发。

## 7. 集成验收与交付检查

- [x] 7.1 组装 PostgreSQL、对象存储模拟服务与 collector fixture 的端到端测试，验证初始化基线、未变运行、内容变更、消失确认、列表遗漏但详情可用、重现、归档失败与数据库失败的完整可观察结果。
- [x] 7.2 将 Go/Python 测试、迁移检查、sqlc 生成检查、Compose 配置检查和 worker 镜像构建纳入 CI，验证一条干净 CI 流水线可通过且不会访问真实米哈游接口或真实 COS。
- [ ] 7.3 完成上线验收清单：部署后人工执行一次初始化同步，逐项核对 COS manifest 摘要、`collection_runs`/审计/观察/期表、临时目录清理及禁用状态的 timer；验证成功后才允许启用每日 timer，且本任务不引入 API、页面、图表、周期汇总、同比环比或微信告警。

## Workflow follow-up

- 在代码审查、远端初始化同步和每日定时运行验收满足后，归档该 OpenSpec change。
