# Proposal

## Why

首轮远端 Plus 试跑的 30 个工作项在任何模型请求发出前均以 `input_too_large` 安全失败。诊断证实其原因是将 UTF-8 字节数直接当作 Token 数，正常长度的中文 JD 因系统提示词和 Schema 的字节数被错误拦截。

## What Changes

- 以固定、校验过摘要的 Qwen BPE 词表计算系统提示词、序列化输入和响应 Schema 的保守 Token 上界；保留既有 512 Token chat envelope 余量。
- 不截断或删减任何正文、span、技术提及、提示词或 Schema；真正超过 10,000 输入 Token 的工作仍在模型调用前安全失败。
- 将估算器版本、词表摘要和保守切分规则摘要纳入输入契约摘要，令原先的字节估算配置不可复用，并为 Plus/Flash 各创建新的不可变候选配置。
- 保留旧候选、工作项和安全失败历史；不自动重跑、不启动 Flash、不扩展或发布。

## Capabilities

### New Capabilities

- `qwen-token-budget-estimation`: 对 Qwen 受控请求执行可复现、保守且可版本化的输入 Token 预检，并将其纳入不可变候选配置身份。

### Modified Capabilities

- None.

## Impact

影响 `internal/jdanalysis` 的输入预算估算和受控试跑配置构造；新增受控的压缩 Qwen BPE 词表资产与合成单元/数据库集成回归。不会修改数据库 schema、CVM 秘密、远端 JD 或已存在的配置/结果。
