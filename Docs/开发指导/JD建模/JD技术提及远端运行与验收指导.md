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
