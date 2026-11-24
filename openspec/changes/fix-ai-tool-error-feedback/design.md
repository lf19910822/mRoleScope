# Design

## Context

`list_category_values` 当前要求模型传入正式类别 ID 或显示标签，但未知类别会返回 `unavailable_value`。Orchestrator 会把错误放进工具历史，却没有在下一轮提示词中明确说明“这是可修正的模型参数错误”。同时，后端故障和模型参数错误共用部分路径，容易误触发或错过重试。

## Decisions

### 1. 错误分类

新增 `invalid_value` 安全错误码，表示模型提供的动态正式值不在当前安全目录中。`invalid_enum` 继续表示固定枚举字段错误；`invalid_combination` 表示字段之间的组合不合法；`invalid_json`、`missing_field`、`limit_exceeded` 保持结构/容量语义。`backend_failure` 表示 Serving、数据库或聚合读取故障；`unavailable_value` 只表示值已通过形式校验但当前结果不可提供。

### 2. 修正反馈

模型工具循环保留完整安全工具结果，并在下一轮用户提示中加入服务端生成的修正指令：上一次工具调用失败、错误字段、错误类别以及下一步应先使用目录或正式值工具核对。提示不包含 SQL、JD 正文、凭据或内部堆栈。

### 3. 重试边界

`invalid_json`、`missing_field`、`invalid_enum`、`invalid_value`、`invalid_combination` 属于模型可修正错误，允许模型继续工具循环；`backend_failure`、`provider_failure`、`configuration_failure` 不触发参数修正。工具总调用上限仍是现有硬上限，后端失败仍遵循最大努力结果。

### 4. 可观察性

修正错误继续作为安全 `StreamEvent.Validation` 或失败工具事件传给前端和审计；对用户只展示安全状态。工具历史向模型提供结构化 `error.code`、`error.field` 和服务端短消息。

## Testing Seams

- `ServingToolService.Registry`：未知类别/正式值、固定枚举和后端错误映射。
- `Orchestrator` tool session：可修正错误进入下一轮且不直接标记 partial；后端错误保持 partial 且不触发修正。
- `QwenIntentProvider.AnalyzeIntentTurn` HTTP contract：下一轮请求含安全错误和明确修正指令，不含内部内容。

## Risks

- 额外模型回合增加少量延迟和成本；受现有工具调用上限约束。
- 模型反复提交无效值时仍可能耗尽预算；达到上限后安全结束，不无限重试。
