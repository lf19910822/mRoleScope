# Tasks

## 1. Qwen Token 预检

- [x] 1.1 以红测证明正常长度的合成中文 JD 不应被 `input_too_large` 误拦截；实现固定、摘要校验的 Qwen BPE 保守计数并验证 provider seam 被调用。
- [x] 1.2 以合成超大中文输入验证超过 10,000 Token 时仍保持 `input_too_large`、零 provider 调用和零物理尝试。

## 2. 不可变配置身份

- [x] 2.1 以 PostgreSQL 集成红测证明旧字节估算试跑不可被新契约复用；将估算器版本/摘要并入输入契约摘要并验证新 Plus/Flash candidates 被创建。
- [x] 2.2 以 PostgreSQL 集成回归证明已存在的 `qwen-bpe-conservative-v1` 候选不会被逐字节 BPE 的 `v2` 契约复用。

## 3. 交付验证

- [x] 3.1 运行 Go 单元与 PostgreSQL 集成测试、部署静态检查和 OpenSpec 验证；确认输出不含真实 JD 或凭据。
- [ ] 3.2 完成代码审查、CI、受控部署后，仅在单独确认下提供 CVM agent prompt 以创建新候选并重跑 Plus；Plus 通过前不运行 Flash。

## Workflow follow-up

- 在代码审查、CI、受控 Plus 试跑和人工复核满足原 AI 变更门禁后，再归档本变更及原 AI 语义分析变更。
