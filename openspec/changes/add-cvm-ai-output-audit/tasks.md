# Tasks

## 1. CVM 审计工件与不可变身份

- [x] 1.1 先编写失败测试，定义 root-only artifact store 的原子写入、SHA-256 sidecar、`0700/0600` 权限、1 MiB + 4 KiB reservation、7 天清理及 64 MiB 容量行为；实现后以临时目录和合成输出通过单元测试。
- [x] 1.2 先编写 processor/runner 红测，证明 candidate/comparison 成功响应在任何验证前捕获、published 不捕获、reservation 不足零外呼、写入失败不重发且以 `output_audit_capture_failed` 安全失败；实现并运行相关 Go 单元测试。
- [x] 1.3 将 `cvm-ai-output-audit-v1` 策略摘要纳入配置身份，先以 PostgreSQL 集成红测证明新 Plus/Flash 配置不复用旧候选且历史不变；实现后运行迁移与集成回归。

## 2. 受控契约审计会话

- [x] 2.1 新增仅保存安全绑定标识、输出摘要、`FAN` 与固定结论类别的不可变审计记录；以 PostgreSQL 集成测试验证不存储模型/JD 文本、不能篡改或重复覆盖结论。
- [x] 2.2 先编写终端审计红测，覆盖只选择已捕获的 schema/证据/policy 失败、验证重建输入摘要、净化控制字符、拒绝任意路径/ID/SQL/自由文本和无 TTY 调用；实现固定 `audit-ai plus|flash` 会话并通过测试。
- [x] 2.3 扩展安全状态汇总，以合成数据库/工件测试验证只显示捕获数、待审计数、容量和固定类别，且输出不含路径、摘要、正文、模型文本、Workspace ID 或凭据。

## 3. 部署隔离与运行指导

- [x] 3.1 更新 Compose、bootstrap、root helper 与 systemd 资产，添加固定审计服务并限定两条常规试跑及固定五条 Plus 诊断写挂载、审计/状态只读挂载和最小 `ReadWritePaths`；以部署静态测试验证 GitHub Actions 与其它普通 operation 均无审计目录访问。
- [x] 3.2 更新远端运行/验收指导，明确 7 天/64 MiB、`FAN` 直连 TTY、无 GitHub Actions 审计、回退与历史输出不可恢复；以文档/脚本静态检查验证不含真实 JD、模型输出或密钥。

## 4. 集成交付与受控诊断

- [x] 4.1 运行 Go 单元与 PostgreSQL 集成测试、部署静态检查及 `openspec validate add-cvm-ai-output-audit --strict`；确认测试和 CI 不访问真实 CVM、百炼或真实 JD。
- [ ] 4.2 在代码审查、CI、受控部署完成后，仅在单独授权下提供 CVM agent prompt：准备新候选并对 5 个冻结诊断样本执行 Plus；由 `FAN` 在直连终端审计，不运行 Flash、扩展或发布。

## Workflow follow-up

- 在受控诊断、契约修订及相关 AI 变更的复核完成后归档本变更。
