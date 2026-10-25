# JD 技术提及：远端运行与验收指导

本指南只适用于 CVM 的受控部署边界。真实 JD、命中片段、原始响应和运行时凭据均不得复制到本地、Git、CI 输出或工单评论。

## 受控入口

- 日常同步：`mrolescope-operations` 的固定操作 `initialize-sync`、`enable-sync-timer`、`disable-sync-timer`。
- 基线回填：`mrolescope-operations backfill-versioned-content`。该操作只能启动 root-owned 的 `mrolescope-backfill-versioned-content.service`，它只能用已验证的 worker 镜像执行固定 `backfill-versioned-content` 子命令。
- 待处理分析：`mrolescope-operations run-pending-analysis`。该操作只能启动 root-owned 的 `mrolescope-analysis.service`，它只能执行 worker 的固定 `analyze` 子命令。
- 候选词典：当前使用 `seed-v0.2-dictionary`、`approve-v0.2-reviewed-rules`、`approve-v0.2-rule` 和 `prepare-v0.2-analysis`，都只能转交给 root-owned 的 `mrolescope-dictionary`。批量批准入口不接受规则列表，只遍历仓库内人工审阅白名单并在单一事务中逐条记录决定；单条入口仅用于恢复或逐条审计。仓储层会拒绝不在人工审阅白名单内的 v0.2 规则。它不能接收正文、SQL、镜像、路径、数据库 URL 或自定义词典内容。v0.1 入口保留仅供历史候选记录审计，不用于当前分析。
- 隔离候选分析：`run-v0.2-candidate-analysis` 只能启动独立的 `mrolescope-candidate-analysis-v0.2.service`。它只消费固定的未发布 v0.2 候选配置；日常 `run-pending-analysis` 仍只消费已发布配置。
- 首版发布与撤回：`publish-v0.2-baseline`、`withdraw-v0.2-material-result-quality` 和 `withdraw-v0.2-security-privacy` 只能转交给 root-owned 的 `mrolescope-release`。它只使用部署记录中已验证的 worker digest，固定运行 v0.2 的发布门禁或两种撤回类别之一；不接收配置 ID、SQL、路径、镜像、词典内容、正文或自由文本理由。
- 回填与分析容器仅在 `backend` 网络，未映射端口、未挂载采集目录，也不接收 COS 凭据或 edge 网络。
- 同步成功后会 best-effort 启动分析服务。分析失败不会回滚同步、COS 归档或岗位可见期；待处理项保持在数据库中，稍后可重试固定分析操作。

不要在 GitHub Actions 输入中传入镜像、命令、路径、数据库 URL、配置 ID 或密钥。部署记录中的不可变 worker digest 是唯一允许的镜像来源。

## 首次部署顺序

1. 通过受控部署工作流发布包含数据库迁移、worker 与 root-owned helper 的同一修订。
2. 部署工作流完成迁移后，在 CVM 受控操作中运行 `backfill-versioned-content`，确认文本为 `BACKFILL_VERSIONED_CONTENT`。该操作与同步/部署共用排他锁，因此不会与它们并发执行。
3. 验收只读取行数、版本引用数、正文期数与安全错误码；不显示正文。重复运行是幂等的，第二次不应创建新的版本引用或正文期。
4. 确认日常同步仍为独立的 02:00 定时任务。分析服务既不启用新 timer，也不改变同步的重试策略。
5. 词典初始状态保持候选/条件状态。未经人工批准的规则不得设为 `enabled`，也不得生成统计。

## 候选词典与初次分析

每一步均在 GitHub 的 `Remote development operations` 工作流中选择，且只可在受保护的 `main` 上运行。工作流以 `github.actor` 作为操作者/审核人，不接受自行填写的审核人名。

1. **导入候选，不执行。** 选择 `seed-v0.2-dictionary`，确认文本为 `SEED_V02_DICTIONARY`。它把仓库内固定的[技术关键词归一化映射表 v0.2](technical-keyword-alias-map-v0.2.md)导入为候选/条件规则；重复运行不会重复插入。此步不启用任何规则、不创建分析结果，也不入队。
2. **批准完整的已审阅规则集。** 常规路径选择 `approve-v0.2-reviewed-rules`，确认文本为 `APPROVE_V02_REVIEWED_RULES`。它不接受规则列表，而是原子地批准人工审阅表 A、B 已确认的全部固定规则，并为每条新启用规则保留独立审核记录；重试只复用既有决定，不重复写入。`approve-v0.2-rule`（确认文本 `APPROVE_V02_RULE`）保留给恢复或逐条审计，且仍只接受一个小写 v0.2 `rule_key`。两条路径都不能启用条件、歧义或未审阅规则。
3. **准备候选配置。** 至少有一条确定性规则已获批准后，选择 `prepare-v0.2-analysis`，确认文本为 `PREPARE_V02_ANALYSIS`。它创建或复用固定的“解析器版本＋词典版本”候选配置，并把全部历史正文版本加入该候选配置的队列；不会抓取来源网站，不会发布结果。
4. **运行隔离的候选分析。** 选择 `run-v0.2-candidate-analysis`，确认文本为 `RUN_V02_CANDIDATE_ANALYSIS`。它不触碰已发布配置，也不改变每日同步定时器。工作流输出只应包含状态和计数，绝不能含正文、命中片段、原始响应或凭据。
5. **候选追赶与复核。** 分析期间如果产生新的正文版本，发布前必须重新检查并补齐候选队列；不得将候选结果与旧发布结果混为“当前结果”。对于首期的确定性精确词面，项目负责人在本地审核词典身份、词面、边界和排除政策，记录在[技术词典 v0.1 人工确认记录](技术词典v0.1人工确认记录.md)和[人工审阅表](技术词典v0.1人工审阅表.md)；CVM 对全部证据做字段哈希、坐标、别名、正式边界和冻结规则集的确定性重放校验，并对多命中与无命中正文做安全抽样。这个组合替代了为首期精确提及而显示/复制真实 JD 的人工阅览。模糊匹配、上下文消歧、年限/学历等语义提取不适用该替代门禁，仍必须另设远端人工文本复核。复核结论为“正确 / 错误 / 暂不确定”，不会覆盖自动结果。
6. **执行受控发布。** 完成第 5 步并确认候选分析无待处理/失败项后，选择 `publish-v0.2-baseline`，确认文本为 `PUBLISH_V02_BASELINE`。操作会在与来源发布互斥的事务内再次将所有正文版本加入候选队列并复核：候选队列和全部正文结果完整、冻结规则集仍等于当前已批准规则、每条证据的字段哈希/规范化版本/Unicode 坐标/别名/`[A-Za-z0-9_-]` 边界/冻结规则成员均有效。任何门禁失败都会保持词典、配置和发布指针不变，只返回安全计数或错误类别。
7. **确认原子发布结果。** 门禁通过时，v0.2 词典和其冻结分析配置在同一事务中一起标为已发布，并写入不可修改的发布证明：配置 ID、词典 ID、规则集哈希、固定门禁版本、发布人和时间。发布人优先使用 GitHub Actions 已认证的 `github.actor`；该身份不可用时受控代码固定记录为 `FAN`，不能手工填写。重复发布同一版本只返回既有证明，不重复写入。发布后词典规则、别名、边界和批准状态均冻结；任何调整都必须创建 v0.3 候选并对历史正文完整重算，不能把词典覆盖增长解释成岗位需求增长。

## 恢复与回退

- 单个分析工作失败：保留为 `pending` 并记录固定错误码。修复原因后执行 `run-pending-analysis`，无需重新抓取来源或重传 COS。
- 候选分析工作失败：候选配置和已完成的候选结果保留，修复后只重试 `run-v0.2-candidate-analysis`；不得把候选配置当成已发布配置来运行普通待处理分析。
- 候选配置失败：标记候选失败，旧发布指针保持不变；不要删除旧结果或证据。
- **首版已发布后发现重大结果质量缺陷：** 只有确认问题会实质影响结果质量时，才选择 `withdraw-v0.2-material-result-quality`，确认文本为 `WITHDRAW_V02_MATERIAL_RESULT_QUALITY_DEFECT`。它不删除任何词典、规则、结果、证据、复核、来源历史或 COS 归档；只原子地退休这套首版词典/配置、清空发布指针并追加撤回证明。
- **首版已发布后发生安全或隐私事件：** 只有确认该类别事件时，才选择 `withdraw-v0.2-security-privacy`，确认文本为 `WITHDRAW_V02_SECURITY_PRIVACY_INCIDENT`。同样保留所有历史事实；随后按凭据轮换和事件响应流程处置，不要通过日志搜寻或复制正文来排障。
- **不要把普通分析失败当撤回理由。** 短暂容器/数据库/队列故障只保留受影响工作以便恢复，已发布首版继续有效。撤回入口只有上述两个固定类别，不存在自由文本理由、任意目标版本或“切回旧配置”功能。
- **没有发布指针时的安全行为。** 撤回后，日常来源同步、COS 归档和岗位历史仍正常运行；普通 `run-pending-analysis` 会安全地报告“没有已发布分析配置”，不会自行采用候选结果。必须先建立新的候选词典、完成完整历史重算和门禁，再发布替代版本。

## 远端验收输出边界

允许输出：配置/运行/工作项的稳定 ID、状态、计数、版本标签、规则 ID、字段名、坐标、哈希、错误类别与容量汇总。

禁止输出：JD 正文或片段、原始 API 响应、COS 对象内容、数据库 URL、访问密钥、token、Cookie 和未脱敏上游错误。

## 首期 AI 语义解析：受控试跑与发布

本节适用于已部署 AI 迁移和新版 root-owned 资产后。它只处理已经版本化的
`description`、`jobRequire`、`addition`，技术词典仍由确定性分析配置提供；不接受
任意模型、提示词、正文、配置 ID、SQL、路径、价格或批次参数。

### 密钥与部署前提

- `MROLESCOPE_QWEN_WORKSPACE_ID` 与 `MROLESCOPE_QWEN_API_KEY` 只能由 CVM
  操作者手工加入既有的 `/etc/mrolescope/secrets/remote-development.env`。前者是已确认的
  北京业务空间标识，worker 只用它构造
  `https://{WorkspaceId}.cn-beijing.maas.aliyuncs.com/compatible-mode/v1`；后者是按量付费
  百炼 API Key。该文件必须始终是 `root:root`、`0600`；密钥值不得进入 Git、GitHub
  Actions 输入/输出、聊天、`.env`、镜像、Compose 文件或普通日志。缺失或非法 WorkspaceId
  会在连接数据库或发送模型请求前安全失败，绝不退回旧通用端点。
- 只有 `ai-plus-trial` 与 `ai-flash-trial` 两个一次性容器接收该环境变量，并只连接
  `backend` 与专用 `ai-edge` 网络。准备、扩展、发布、状态容器没有模型密钥、COS
  凭据、正文挂载或主机端口。
- 先经受控部署工作流部署包含 `000010` 至 `000018` 迁移、worker 和 bootstrap 资产的
  同一受保护 `main` 修订。随后由 CVM 的 root 操作者以固定提交重新运行 bootstrap，
  使 `mrolescope-ai-analysis` 和五个 `mrolescope-ai-*.service` 单元落位；bootstrap
  不创建或打印密钥，也不启动试跑。
- 在未准备候选前，可以选择 `ai-status` 检查无正文的状态；它不发送模型请求。

### GitHub Actions 固定操作

仅在 GitHub 的 `Remote development operations` 工作流、受保护的 `main` 上运行下列
操作。除 `ai-status`、`ai-logs` 外，确认文本必须逐字匹配。操作没有额外参数输入框；
GitHub actor 也不会成为可选择的模型、提示词或发布人。AI 发布证明的发布人固定为
`FAN`。

| 顺序 | operation | confirmation | 会做什么 | 不会做什么 |
| --- | --- | --- | --- | --- |
| 1 | `prepare-ai-trial` | `PREPARE_AI_TRIAL` | 对固定的 30 份分层正文建立同一输入契约下的 Plus 候选与 Flash 对照，创建共享的 60 逻辑任务/180 尝试/2.00 元预算组。 | 不调用模型，不发送正文，不扩大样本。 |
| 2 | `run-ai-plus-trial` | `RUN_AI_PLUS_TRIAL` | 只执行固定的 `qwen-plus` 非思考试跑工作；每个逻辑任务至多三次，发送前预留预算。 | 不执行 Flash、不改变提示词或 token 上限、不处理扩展队列。 |
| 3 | `run-ai-flash-trial` | `RUN_AI_FLASH_TRIAL` | 只执行同一批样本的固定 `qwen-flash` 成本/质量对照。 | 不成为默认模型或自动路由目标。 |
| 4 | `ai-status` | 无 | 输出配置 ID、工作/尝试/复核状态及 token、金额等安全汇总。 | 不发送模型请求、不显示 JD/片段/原始响应/密钥。 |
| 5 | `ai-logs` | 无 | 只读取当天五个 AI systemd 单元的 journal。 | 不读取容器原始请求/响应；输出仍须按本指南的无正文边界审阅。 |
| 6 | `expand-ai-plus` | `EXPAND_AI_PLUS` | 仅在 30 份 Plus 试跑全部完成、全审且零重大错误后，把其余已完成确定性技术分析的正文**入队**。 | 不调用模型；不绕过生产批次预算；不发布。 |
| 7 | `publish-ai-plus` | `PUBLISH_AI_PLUS` | 在所有已入队工作完成、试跑全审、`required_review` 全审、`spot_check` 20% 和 `routine` 5% 抽审均满足时，写入不可变发布证明并指向固定 Plus 配置。 | 不接受发布人、配置 ID 或任意复核结果；任一门禁失败时不移动发布指针。 |

### CVM 直连人工复核（不属于 GitHub Actions）

真实 JD 只能在 CVM 终端短暂显示，因此人工复核**不能**从 GitHub Actions、`ai-logs`、
`ai-status` 或聊天中执行。已取得 CVM root 操作权限的审核人可在直连终端逐次运行：

```bash
sudo /usr/local/sbin/mrolescope-ai-analysis review-plus
sudo /usr/local/sbin/mrolescope-ai-analysis review-flash
```

每次命令只选择当前固定 Plus 候选或 Flash 对照中的一个未复核结构化结论，显示三段已版本化
正文、持久化的结构化结论和证据位置；文本仅写到这次交互终端，不写 journal、数据库审计、
GitHub 日志或本地文件。终端会提示输入且只接受 `accepted`、`not_reviewable`、`minor_error`
或 `major_error`；后两者还必须从五类固定错误类别中选择一项。裁决人固定记录为 `FAN`，记录
不可修改。输入非枚举值、配置不是当前受控试跑臂或没有待复核项都会安全停止，不会写入裁决。

终端内容属于远端受控查看，不要复制到聊天、Git、Issue、CI 输出或截图。正文中的终端控制字符
会被转义后再显示，避免不可信文本改写终端界面。30 份 Plus 和 30 份 Flash 的有效结论须重复
上述命令直至 `ai-status` 的无正文汇总显示相应复核完成；任一重大错误即停止扩展和发布。

### 试跑验收与扩展停止点

1. 先执行步骤 1，再分别执行步骤 2 与 3；运行任一模型前都必须确认密钥已由 root
   按上述边界配置。缺少密钥时 worker 会在读取数据库配置或构造模型客户端前安全失败。
2. 使用 `ai-status` 只读确认试跑安全汇总。30 份 Plus 与 30 份 Flash 必须保持固定样本；
   任何 `not_analyzed`、预算停止、校验失败或安全失败都不能解释成学历/年限未配置。
3. 在 CVM 的受控人工复核流程中审阅 30 份 Plus 与 30 份 Flash 的全部有效解析项；只记录
   复核结论、ID、计数、错误类别和用量摘要。发现任一重大错误时，不执行扩展或发布；
   修订提示词、输入契约、校验器或模型档案必须创建新的候选配置并重新试跑。
4. `expand-ai-plus` 只建立剩余工作项，特意不具有模型外呼能力。当前共享的 2.00 元预算
   只适用于 30×2 试跑，**不得**用 `run-ai-plus-trial` 消费扩展后的工作项。
5. 在允许任何正式 Plus 外呼之前，必须另行批准生产批次策略：每批正文数、请求/尝试/
   输入 token/输出 token/人民币上限、一次确认覆盖一批还是全部剩余正文，以及每批后的
   暂停与复核规则。在该决定和对应的新受控操作部署前，扩展队列是安全的停止状态，而非
   可执行的正式分析队列。

### 恢复与回退

- 试跑的瞬时故障只可在固定重试上限内恢复；鉴权、请求、JSON、证据或策略错误不自动
  重试。每次物理尝试都持久化并计入共享上限，进程中断后也不会使一次已预留请求消失。
- 候选或试跑未通过时，不删除历史技术提及、正文版本、已完成 AI 结果或审计记录；旧的
  已发布确定性分析基线持续有效。修正语义契约后创建新的 AI 候选，而不是覆盖旧配置。
- 在 AI 配置尚未发布时，回退就是不执行 `publish-ai-plus`；扩展队列及候选结果保留作
  审计，不触发日常同步或默认分析。
- 当前首期没有 `withdraw-ai-plus` 这一受控操作。发布后的撤回策略必须在投入生产
  批次前另行设计、测试并作为新的固定 operation 部署；不得通过直接 SQL、删除记录或
  改写发布证明实施“回退”。部署资产本身若需回退，沿用既有受控部署工作流与已验证提交。

### CVM 候选输出契约诊断（仅辅助开发）

当 Plus 已实际返回模型结果、却被 `invalid_response_schema`、
`invalid_evidence_reference` 或 `policy_rejected` 拒绝时，不能从 journal、GitHub
Actions、数据库或聊天中寻找原文。已部署审计能力后，只有下列 CVM root-owned 固定操作可
用于定位契约问题：

1. `prepare-output-audit-diagnostic`：创建或复用一个**独立的**五条冻结样本 Plus 候选；不
   调用模型，不创建 Flash 工作项，不改变原有 30 条 Plus/Flash 试跑及其历史。
2. `run-plus-output-audit-diagnostic`：只执行上述五条 Plus 工作；外呼前仍受既有请求、token、
   金额与尝试限制，并在模型成功返回、严格校验前捕获输出。它没有 Flash、扩展或发布能力。
3. `audit-status`：仅输出当前固定审计候选/对照集合中、仍有工件的待审契约失败数，连同
   捕获工件数、按固定安全错误码的计数和 `64 MiB` 容量的已用/可用汇总；不显示路径、哈希值、
   JD、重建输入、模型输出、WorkspaceId 或凭据。它不会调用模型，也不会重新排队工作。
4. `audit-plus-contract`：只能由 `FAN` 在**直接连接 CVM 的 TTY**运行；本环境将 `FAN` 固定
   映射为从 `ubuntu` 登录并经 `sudo` 发起的会话，root 直登或其他 sudo 发起者会被拒绝。程序自行选择一个
   已捕获、尚未审计的契约失败（优先五条诊断候选；若另行受控准备了常规试跑，再检查其
   Plus 候选），显示净化后的模型输出和从冻结正文/配置重建的输入；只接受
   `prompt_noncompliance`、`schema_contract_mismatch`、
   `evidence_contract_mismatch`、`policy_contract_mismatch`、`unable_to_classify` 五种结论。

`audit-flash-contract` 保持为同样受限、同样要求该 `ubuntu`→`sudo` 操作人链的 TTY 读取器，但只会读取已经由常规 Flash 对照试跑捕获
的工件；本次五条诊断既不创建 Flash 工作也不得运行该操作。任何已过期或不完整的工件均不会
阻塞后续仍完整、可审计的工件；下一次受控清理会移除不成对的响应/sidecar。

这些操作不出现在 GitHub Actions 或普通 operations 助手中。诊断 Plus 容器是唯一额外的固定
外呼写入者：它只获得百炼 Key、北京 WorkspaceId、`backend` 与 `ai-edge`，以及
`/var/lib/mrolescope/ai-output-audit` 的固定写挂载；诊断准备没有 Key 或外网；审计会话和
`audit-status` 只有只读审计挂载且没有 Key、COS、端口或外网。

审计目录必须保持 `root:root:0700`，工件为 `0600`；成功返回的原始输出最多保留 7 天，未到期
工件绝不为腾空间而删除，64 MiB 无法预留时工作安全失败为
`output_audit_capture_failed`，不会重发模型。原始输出、重建输入和正文不得进入 PostgreSQL、
Git、CI、GitHub Actions、journal、普通日志、COS 或本地工作区。此前旧规则下已经丢弃的输出
不能恢复，不能直接重跑旧失败队列。

worker 镜像的默认身份不是 root；凡是获得这一固定审计挂载的三类固定服务（两条常规试跑、五条
诊断调用器、两条 TTY 审计和无文本状态读取器）必须显式使用容器 root 身份，才能访问
`root:root:0700` 目录。它们仍是 `read_only` 根文件系统、固定命令、单一审计挂载，且没有 Docker
socket、任意主机路径、任意参数或普通服务入口；其他服务不获得该挂载。这是维持“仅受控容器可读写
原始输出”所必需的权限映射，不是放宽目录权限。

五条诊断样本只用于定位响应/校验契约，**绝不构成**30 条双模型试跑、人工复核、扩展或发布的
替代门槛。回退时停止使用这四个固定诊断操作并回退已验证部署修订；数据库历史和未到期的
root-only 工件不被普通服务读取，仍按 7 天规则自动清理。
