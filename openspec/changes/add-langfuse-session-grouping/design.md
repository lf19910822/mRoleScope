# Design

## Decisions

1. `LangfuseTraceContext` 作为 exporter 输入，包含 `SessionID`、`Name`、`UserID`、`Metadata` 和 `Tags`；root span 映射为 `langfuse.session.id`、`langfuse.trace.name`、`langfuse.user.id` 及 `langfuse.trace.metadata.*`。
2. Session ID 只接受 ASCII、非空、长度小于 200；metadata key/value 使用短 ASCII key 和有界 UTF-8 value。非法值使受控导出禁用/失败，不影响本地 Debug 结果。
3. HTTP Debug 请求通过 context 注入可选的 trace context，避免把会话字段扩展到普通业务 API；请求未传 session 时保持当前行为。
4. 评测 runner 显式接收 Langfuse 配置和 trace context。CLI 只有在 `MROLESCOPE_LANGFUSE_EVALUATION_ENABLED=true` 且完整 Langfuse 配置存在时导出，避免 fixture/普通评测意外外发数据。
5. 评测 session ID 由 run、suite、case 组成；repetition、retry 和版本放 metadata，不放入 session ID，便于 Session 回放和单次 attempt 筛选。

## Boundaries

- Langfuse Session 只归组 trace，不改变 trace 的父子 observation 关系。
- 不为普通 Debug 根据 query 自动归组，避免不同请求意外合并。
- 不修改生产审计、Intent hash 或公开页面。
