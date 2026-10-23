# JD 技术提及：远端运行与验收指导

本指南只适用于 CVM 的受控部署边界。真实 JD、命中片段、原始响应和运行时凭据均不得复制到本地、Git、CI 输出或工单评论。

## 受控入口

- 日常同步：`mrolescope-operations` 的固定操作 `initialize-sync`、`enable-sync-timer`、`disable-sync-timer`。
- 基线回填：`mrolescope-operations backfill-versioned-content`。该操作只能启动 root-owned 的 `mrolescope-backfill-versioned-content.service`，它只能用已验证的 worker 镜像执行固定 `backfill-versioned-content` 子命令。
- 待处理分析：`mrolescope-operations run-pending-analysis`。该操作只能启动 root-owned 的 `mrolescope-analysis.service`，它只能执行 worker 的固定 `analyze` 子命令。
- 候选词典：`seed-v0.1-dictionary`、`approve-v0.1-rule` 和 `prepare-v0.1-analysis` 都只能转交给 root-owned 的 `mrolescope-dictionary`。它只接受固定的 v0.1 种子、受限格式的规则键和 GitHub 已认证操作者身份；不能接收正文、SQL、镜像、路径、数据库 URL 或自定义词典内容。
- 隔离候选分析：`run-v0.1-candidate-analysis` 只能启动独立的 `mrolescope-candidate-analysis.service`。它只消费固定的未发布 v0.1 候选配置；日常 `run-pending-analysis` 仍只消费已发布配置。
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

1. **导入候选，不执行。** 选择 `seed-v0.1-dictionary`，确认文本为 `SEED_V01_DICTIONARY`。它把仓库内固定的 `technology-keyword-alias-map-v0.1` 导入为候选/条件规则；重复运行不会重复插入。此步不启用任何规则、不创建分析结果，也不入队。
2. **逐条人工批准确定性规则。** 选择 `approve-v0.1-rule`，输入一个小写 v0.1 `rule_key`，确认文本为 `APPROVE_V01_RULE`。这是对**一条**候选规则的明确决定，重复批准同一条规则是幂等的。条件规则（例如需要阅读上下文才能判断的短词）会被拒绝，不能用这条入口绕过上下文策略。不要因为词典已导入就批量批准规则。
3. **准备候选配置。** 至少有一条确定性规则已获批准后，选择 `prepare-v0.1-analysis`，确认文本为 `PREPARE_V01_ANALYSIS`。它创建或复用固定的“解析器版本＋词典版本”候选配置，并把全部历史正文版本加入该候选配置的队列；不会抓取来源网站，不会发布结果。
4. **运行隔离的候选分析。** 选择 `run-v0.1-candidate-analysis`，确认文本为 `RUN_V01_CANDIDATE_ANALYSIS`。它不触碰已发布配置，也不改变每日同步定时器。工作流输出只应包含状态和计数，绝不能含正文、命中片段、原始响应或凭据。
5. **候选追赶与复核。** 分析期间如果产生新的正文版本，发布前必须重新检查并补齐候选队列；不得将候选结果与旧发布结果混为“当前结果”。每一条实际命中的启用规则至少复核一条远端证据，另抽检高风险边界、多处命中和无命中正文。复核结论为“正确 / 错误 / 暂不确定”，不会覆盖自动结果。
6. **发布门禁尚未开放。** 当前没有“发布候选词典/配置”的 GitHub 操作，也没有可接受任意 SQL 或正文读取的运维入口。不要通过数据库直连、临时脚本或 `run-pending-analysis` 绕过复核。受控远端复核界面与“全部门禁通过后原子发布”的入口将在后续独立实现；旧结果和旧复核记录届时仍须保留，词典覆盖变化不得标注成岗位需求变化。

## 恢复与回退

- 单个分析工作失败：保留为 `pending` 并记录固定错误码。修复原因后执行 `run-pending-analysis`，无需重新抓取来源或重传 COS。
- 候选分析工作失败：候选配置和已完成的候选结果保留，修复后只重试 `run-v0.1-candidate-analysis`；不得把候选配置当成已发布配置来运行普通待处理分析。
- 候选配置失败：标记候选失败，旧发布指针保持不变；不要删除旧结果或证据。
- 需要回退展示：把发布指针切回已完成且验收过的旧配置；不删除候选配置及其结果。
- 怀疑泄露：停止分析 helper，保留最小审计标识，按凭据轮换流程处理；不要通过日志搜寻或复制正文来排障。

## 远端验收输出边界

允许输出：配置/运行/工作项的稳定 ID、状态、计数、版本标签、规则 ID、字段名、坐标、哈希、错误类别与容量汇总。

禁止输出：JD 正文或片段、原始 API 响应、COS 对象内容、数据库 URL、访问密钥、token、Cookie 和未脱敏上游错误。
