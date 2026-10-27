# Proposal: CVM-Controlled AI Model Output Audit

## Why

首轮 Plus 试跑已经证明百炼能够返回结果，但 29 个返回在本地严格契约校验时被拒绝。旧实现按安全边界立即丢弃原始输出，只留下错误码，因而无法区分提示词、结构化响应、证据引用或语义策略的具体不一致。

需要在不把真实 JD 或模型文本扩散到数据库、日志、Git、GitHub Actions、CI 或聊天的前提下，保留短期、可受控查看的候选试跑证据，辅助定位和修订契约。

## What Changes

- 新增仅限 CVM root 的模型输出审计能力：当前 `ai-jd-result-v4` / `ai-semantic-validator-v4` 的 Qwen Plus 候选试跑中，每个成功取得的模型原始输出在校验前原子保存到受控目录；已发布日常分析不启用该能力。既有 Flash 对照工件仅保留为不可变历史，不再产生新的 Flash 捕获。
- 新增固定的直连 CVM 终端审计会话，只能由 `FAN` 审阅由程序选取的已捕获、契约失败输出；会话按不可变正文版本和配置在内存中重建受控输入上下文，并显示经终端控制字符净化的原始输出。
- 设定 `root:root`、目录 `0700`、文件 `0600`、7 天自动清理和 64 MiB 总量硬上限；审计写入失败安全停止对应工作且不重发外部请求。
- 将审计捕获策略与版本纳入不可变候选配置身份。既有候选、工作项和已经丢弃的输出保持历史原状，不能原地重跑或伪造恢复。
- 继续保持所有普通可观测渠道的脱敏；仅允许安全的捕获/审计计数、固定错误类别和标识进入状态与审计摘要。

## Capabilities

### New Capabilities

- `cvm-ai-output-audit`: 在有限候选试跑期间捕获、保留、清理并通过 root 直连终端审计模型原始输出，同时维持普通渠道的文本隔离。

### Modified Capabilities

- None.

## Impact

- **Worker:** 在 provider 成功返回与响应校验之间增加 CVM 审计工件写入和失败安全处理；增加固定审计终端子命令与合成测试。
- **Runtime:** 新增 `/var/lib/mrolescope/ai-output-audit` root-only 路径、受限 Compose 挂载、systemd 写入路径和 bootstrap 静态校验；普通服务与 CI 不获得该挂载。固定的审计状态读取器只能输出无正文聚合。
- **Configuration:** 审计策略版本进入候选配置身份，确保留存策略、试跑输出和复核结论可追溯地绑定。
- **Operations:** 只增加直连 CVM 终端受控审计；不新增 GitHub Actions 操作，不迁移、备份或向对象存储复制原始输出。
