# Proposal

## Why

真实趋势请求在工具调用成功后，Qwen 对 Agent 最终响应的 `response_format` 返回 HTTP 400。原因是 Agent Schema 在数组字段上使用了 Qwen 当前不接受的 `uniqueItems` 关键字，导致最终 Intent 无法生成，已获得的候选也无法形成草稿。

## What Changes

- 移除发送给 Qwen 的 Agent Schema 中不兼容的数组约束关键字。
- 保留 Go 侧对 `response_modes` 重复值的契约校验。
- 增加 provider 请求边界回归测试，防止不兼容关键字再次进入 Qwen JSON Schema。

## Impact

影响 `internal/aitrend` 的 Agent 最终响应请求 Schema；不改变 Agent 响应结构、Serving 查询、Draft Schema 或用户可见字段。
