# Design

基础流式回复提示词不再包含示例正文。`assistantReplyPromptForModes` 仅在 `response_modes` 恰好为 `guidance` 时追加示例；包含 `trend` 的回复明确禁止示例、无关维度推荐和虚构下一步问题。
