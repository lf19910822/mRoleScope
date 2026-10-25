# Design

## Context

旧估算器将 JSON 输入、系统提示词和 Schema 的 `len` 直接相加，实际计算的是 UTF-8 字节而非 Qwen Token。这在中文内容上显著高估，导致远端试跑在网络前全部失败。动机见 proposal.md。

## Goals / Non-Goals

**Goals:**

- 在 CVM 内使用固定 Qwen BPE 词表完成可复现、无网络的保守预检。
- 保持 10,000 输入 / 2,500 输出上限、不截断和零外呼失败语义。
- 使预检规则成为配置身份的一部分，保护已失败的历史记录。

**Non-Goals:**

- 不改变提示词、Schema、span、正文传递范围、模型、地域、价格或输出上限。
- 不自动触发新试跑、Flash、扩展、复核或发布。
- 不把生产 JD、请求或响应写入测试、Git、日志或文档。

## Decisions

### 固定并校验 Qwen BPE 词表

将 Qwen `qwen.tiktoken` 词表以压缩受控资产嵌入 worker，运行时校验解压后的 SHA-256 和预期条目数。实现最低 rank BPE 合并计数，不依赖 CVM 上的 Python、模型文件或请求期网络。词表摘要为 `b2b1b8dfb5cc5f024bafc373121c6aba3f66f9a5a0269e243470a1de16a33186`。

当前实现的估算器版本为 `qwen-bpe-conservative-v2`：它在每个已选分段内逐 UTF-8 字节初始化 BPE，再合并最低 rank token。此前的 `v1` 实现错误地按 rune 起点初始化，可能低估少数多 Token 的 Unicode 码点；即使其余词表、分段和 envelope 未变，此语义修复仍必须改变输入契约摘要，避免复用任何由 `v1` 创建的候选。

备选方案是字符/字节比例换算或调用远端计数 API；前者不能证明与 Qwen 兼容，后者会在预算判定前再次外发正文且增加可用性边界，因此拒绝。

### 保守切分与固定 envelope

计数器允许 ASCII 字母串和单个 Unicode code point 内部执行 BPE 合并，但在其余边界额外切分。额外边界只能阻止合并、不会降低估算，因此在固定词表下形成上界；继续保留既有 512 Token chat envelope 余量以覆盖 provider chat 包装。系统提示词、用户 JSON 和响应 Schema 均单独计入。

### 估算契约冻结进现有输入摘要

不新增 schema 列；将估算器版本及摘要并入已有 `input_contract_sha256`。因此现有的唯一 `configuration_sha256` 自然变化，准备操作创建新的候选而非修改旧行。旧试跑的 `failed_safely/input_too_large` 记录保持为审计历史。

## Risks / Trade-offs

- [Qwen 服务端 tokenizer 未来改变] → 固定本地词表、摘要和版本；任何认可的契约更新以新候选重新试跑。
- [保守上界仍使极少数接近上限的 JD 停止] → 保留安全失败，不截断；以后若要改变分段或余量必须另起候选验证。
- [嵌入词表增加镜像体积与初始化工作] → 压缩资产、只在首次预检时载入，并以测试覆盖常规和真超限路径。

## Migration Plan

1. 部署受控词表和新计数器；数据库迁移不变。
2. `prepare-ai-trial` 在当前输入契约摘要下创建新 Plus/Flash 候选，旧候选保留。
3. 仅在单独授权后执行新的 Plus 试跑；通过前不执行 Flash、扩展或发布。
4. 回退时停用新的 AI 操作；不删除任一配置、工作或结果。
