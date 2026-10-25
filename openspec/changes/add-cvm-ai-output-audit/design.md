# Design

## Context

见 [proposal.md](proposal.md) 与 `cvm-ai-output-audit` spec。现有 worker 在 `QwenClient` 成功返回后仅将 `response.Content` 保存在内存，随后记录安全尝试、执行严格映射并丢弃原始字节。现有 CVM 终端复核仅能查看已持久化的有效语义结论，不能查看失败响应；普通服务使用 read-only 根文件系统，AI 试跑 systemd 单元仅获准写入 releases 目录。

## Goals / Non-Goals

**Goals:**

- 在模型成功返回与任何本地语义校验之间，可靠捕获可诊断的原始输出。
- 让审计者仅在直连 CVM 终端内，将失败输出与精确、可验证地重建的受控输入上下文一起查看。
- 使短期留存、容量、安全失败、候选身份和部署隔离可测试、可部署且无正文可观测。

**Non-Goals:**

- 不保存完整 HTTP 交换、原始请求、供应商错误体、API Key 或 Workspace ID。
- 不为已发布配置或正常日常分析启用输出留存，不备份到 COS，不允许下载或 GitHub Actions 审计。
- 不恢复旧配置已经丢弃的 29 份输出，不原地重跑其失败工作，也不在本变更中修订提示词或响应 schema。

## Decisions

### 1. 将原始响应作为 CVM-only artifact，而非数据库字段

在 `/var/lib/mrolescope/ai-output-audit` 创建由 UUID 配置、工作项和尝试号组成的固定层级。每个响应写入独立原始字节文件及不含正文的 metadata sidecar；sidecar 包含 artifact 版本、配置/工作/尝试/正文版本标识、输出 SHA-256、创建时间及固定审计策略摘要。两个文件均经同目录临时文件、`fsync`、原子 rename 后才视为已捕获。若主机恰在两次文件提交之间中断，缺少配对 response 或 sidecar 的文件不是可审计工件；下一次受控 reservation 清理会删除它，既不让它阻塞后续审计，也不以重发模型请求来补偿。

原始字节不进入 PostgreSQL；仅目录内的安全 sidecar 保存其摘要和绑定关系。审计器从数据库读取允许的失败工作与不可变配置，只按严格 UUID 和固定尝试号构造路径，再重新计算输出摘要，因而不接受任意路径或 ID。

备选方案是 PostgreSQL bytea、加密对象存储或普通本地日志。前者扩大备份和 SQL 访问面，后两者违背 CVM-only/普通可观测隔离，均拒绝。

### 2. 先预留固定捕获槽位，再发起模型请求

现有 Qwen HTTP 解码已限制响应 envelope 至 1 MiB。每次可能外呼前，artifact store 先清理到期文件，并为最多 1 MiB 输出和 4 KiB metadata 创建受控 reservation；所有保留中 artifact 加 reservation 不得超过 64 MiB。成功返回后 reservation 只能结算为同一 artifact，实际较小内容释放余量。

reservation 无法取得时，runner 在网络前以 `output_audit_capture_failed` 安全停止。若模型成功返回后的原子写入仍失败（磁盘/权限/硬件错误），工作以同一安全错误停止、不写语义结果、不自动重发。进程或主机在返回和落盘之间崩溃是无法恢复的窄窗口；恢复逻辑记录安全失败而不是靠重发猜测输出。

备选方案是收到响应后再检查空间或淘汰未到期文件。前者会丢失已经付费的诊断证据，后者违反 7 天承诺，均拒绝。

### 3. 捕获只适用于新的候选/对照配置

定义 `cvm-ai-output-audit-v1` 策略摘要，内容包括 artifact 格式、最大响应槽位、7 天留存和 64 MiB 上限，并折入 AI 配置已有的输入/审计契约摘要。准备操作因此为新策略生成不同 Plus candidate 与 Flash comparison；旧配置继续保留历史。另有不可变的 `five-body / Plus-only` 诊断候选身份，供在已有候选失败后最小化地获取新的可审计回复；它不入队 Flash。只有 `candidate` 和 `comparison` worker 实例构造 artifact store；任何 `published` worker 不构造或挂载它。

不新增一个“临时打开审计”的运行时开关，以避免同一配置在不同调用间产生不可比较的留存边界。也不改变模型、提示词、schema、Token 限额或输入正文范围。

### 4. 单独建立固定的终端契约审计会话

新增 `audit-ai plus` 与 `audit-ai flash` worker 子命令和对应 root helper 固定操作。helper 与 worker 都要求 stdin/stdout 是 TTY；CVM helper 还将 `FAN` 固定绑定为由 `ubuntu` 登录账户经 sudo 发起的调用，拒绝 root 直登或其他 sudo 发起者。不注册为 systemd 服务，也不加入 GitHub Actions allowlist。Plus 的固定候选集合按五条诊断候选、再常规 Plus candidate 的顺序解析；Flash 只解析常规 Flash comparison。程序从这些固定集合选择最早的、尚未由 `FAN` 审计且**仍具有完整捕获 artifact**的 `invalid_response_schema`、`invalid_evidence_reference` 或 `policy_rejected` 工作；清理后的历史行会被跳过，而不阻塞后续工件。

会话先验证配置的 prompt/schema/input-contract/audit-policy 摘要，再用同一不可变正文版本和确定性 request ID 重建模型输入；摘要不匹配则安全停止。它把净化后的 `description`、`jobRequire`、`addition`、重建输入和原始模型输出仅写到连接 TTY。审计者只能输入一个固定结论：`prompt_noncompliance`、`schema_contract_mismatch`、`evidence_contract_mismatch`、`policy_contract_mismatch` 或 `unable_to_classify`。数据库追加不可变的安全审计记录，记录 `FAN`、绑定标识、原始输出 SHA-256 和结论，不记录文本。

备选方案是允许审计者传 attempt ID/路径，或复用已完成语义结果的 review 命令。前者给出绕过选择器的文件读取面，后者不能处理没有有效 result 的失败响应，均拒绝。

### 5. 用最小挂载与 systemd 允许列表维持隔离

bootstrap 创建审计根目录为 `root:root:0700`。两个常规 Plus/Flash 试跑服务与一个固定五条 Plus 诊断服务获得读写 bind mount；诊断服务不具备 Flash、扩展或发布操作。两个 TTY 审计服务与一个固定的非交互状态读取器只读挂载；其余服务（包括 API、同步、普通 worker、prepare、expand、publish 和已有语义复核）没有该挂载。仅现有 Plus/Flash systemd 单元把该目录加入 `ReadWritePaths`；诊断和审计操作经 root helper 直接启动、不会注册为 systemd 或 GitHub Actions 操作。

`audit-status` 仅从无正文 metadata、文件属性和数据库安全字段报告 artifact 数、待审计失败数、已用/可用容量及安全类别聚合；不输出文件名、路径、正文或摘要值。日志中只允许 `output_audit_capture_failed` 与固定结论类别。

## Risks / Trade-offs

- [Raw model output可能含 JD 原文或模型复述] → root 目录、最小挂载、短期保留、无备份及 TTY-only 展示。
- [宿主机或进程在供应商返回和磁盘提交间崩溃] → 预留槽位、原子写入、恢复时安全失败；不以重发来伪造恢复。
- [64 MiB 使试跑在容量满时停止] → 每次外呼前预留，保留期外自动清理，状态以脱敏计数提示人工处理。
- [模型输出控制字符攻击终端] → 所有 terminal 文本沿用并覆盖控制字符净化；测试 ESC、C0/C1 控制符和超长行。
- [审计查看依赖未来代码仍能构建旧输入] → 以版本化 prompt/schema/input/audit 摘要验证重建；缺少兼容构建器时安全拒绝展示而非显示错误上下文。

## Migration Plan

1. 以 TDD 实现 artifact store、reservation/cleanup、捕获顺序和安全错误码；合成测试不使用真实 JD 或模型输出。
2. 实现固定终端审计会话及不可变、安全的审计结论记录；验证不接受 TTY 外调用、路径或自由文本。
3. 修改 Compose、bootstrap、systemd 资产和静态部署测试，证明只有两个常规试跑 writer、一个固定五条 Plus 诊断 writer、两个 TTY 审计 reader 和一个无正文状态 reader 接触目录。
4. 部署后先运行固定诊断 prepare 产生新候选；仅在单独授权下对 5 个冻结诊断样本运行 Plus，并由 `FAN` 在 CVM 终端审计。不会运行 Flash、扩展或发布。
5. 回退时停用新固定操作并移除新镜像/资产；既有数据库历史和未到期 root 审计 artifact 不被普通服务读取，按既定留存自动清理。
