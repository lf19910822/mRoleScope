# AI 评测产物格式

评测题目、判分规则和汇总报告可以版本化保存在本目录。每次运行的详细轨迹使用独立评测产物目录，默认是 `var/ai-evaluation/`，不写入 Serving 数据、日常审计表或普通用户响应。

每个 Trace JSON 至少包含：

- `case_id`、`repetition`、`retry`、`evaluation_version_id`；
- 模型、提供商、提示词、工具 Schema、快照、代码和判分器版本；
- 结构化 Intent、工具名、工具参数安全摘要、状态和安全错误码；
- Draft、Schema/范围/排名/草稿/解释判分结果；
- 输入/输出 token、估算成本、首个事件延迟、总耗时和工具调用计数。

轨迹禁止保存逐字思维链、凭据、无关 JD 正文和真实用户看板状态。`var/ai-evaluation/` 是本地或构建产物目录，不是 Serving API 的静态资源目录。
