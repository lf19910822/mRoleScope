# Tasks

## Schema compatibility

- [x] 1.1 增加红测，证明最终 Agent Schema 不能包含 Qwen 不兼容的数组关键字。
- [x] 1.2 删除 `uniqueItems`，保留 Go 侧重复模式校验。

## Verification

- [x] 2.1 运行相关 Go 测试、全量测试、格式检查和 `git diff --check`。
- [ ] 2.2 经授权部署后，在 CVM 用真实业务问题复验最终模型回合。
