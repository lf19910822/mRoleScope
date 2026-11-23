# Proposal

## Why

明确的趋势问题在 `response_modes: ["trend"]` 下仍收到其他问题示例，回复偏离用户请求。原因是指导示例被放在所有流式回复共用的系统提示词中，模型可能在趋势回复中复述它们。

## What Changes

- 趋势或混合回复只接收趋势回复规则，明确禁止提问示例和无关推荐。
- guidance-only 回复才动态注入可复制的问题示例。
- 增加 provider 请求边界测试和提示词契约测试。

## Impact

影响流式 `assistant_message` 的提示词装载，不改变 Agent 响应、Draft 或 Serving 数据。
