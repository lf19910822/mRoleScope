# 技术关键词归一化映射表 v0.3

状态：**已审阅的候选词典来源；须经受控导入、全量重算及发布门禁后才对 Serving 生效**。本文件是 v0.3 的不可变来源说明，不含真实 JD、岗位信息、命中片段、原始响应或凭据。

## 版本边界

- v0.3 以 [v0.2 映射表](technical-keyword-alias-map-v0.2.md) 的已审阅规则为基线；v0.1、v0.2 的内容、指纹、规则和历史结果均不修改。
- 解析器仍为 `technology-mentions-v1`，且仅扫描 `description`、`jobRequire`、`addition`。
- 每条规则保留独立 `rule_key`、原始字段、规范化文本版本和 Unicode 码点区间；归一化只统一技术身份，不改写原文或坐标。

## v0.3 已审阅增量

| 技术身份 | 可批准的精确词面 | 匹配政策 | 保留约束 |
| --- | --- | --- | --- |
| Go | `Go`、`Golang` | ASCII 大小写不敏感；ASCII 标识符边界 | `Go` 与 `Golang` 归为同一编程语言身份；不从 `go-zero`、`go_micro`、`gopher` 等较长标识符内部取词，不做模糊拼写纠错。 |

除上表外，v0.2 的全部已审阅政策、完整技术名称和候选/条件项原样延续。`go`、`GO`、`golang`、`GOLANG` 是上述两条显式规则的大小写变体，不另建技术身份或别名规则。

## 受控生命周期

1. 以固定 `seed-v0.3-dictionary` 导入候选词典。
2. 以固定 `approve-v0.3-reviewed-rules` 批准完整审阅规则集；其中新增的 `go.01`、`go.02` 受固定白名单约束。
3. 以固定 `prepare-v0.3-analysis` 创建历史正文全量重算配置。
4. 以隔离的 `run-v0.3-candidate-analysis` 完成候选分析。
5. 仅在完整结果、规则集与证据坐标均通过后，以固定 `publish-v0.3-upgrade` 原子切换 Serving 所用的技术分析配置。

以上操作只在 PostgreSQL 私网中处理规范化正文；不访问来源网站、COS、本地真实 JD 或外部模型。
