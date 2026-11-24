# Tasks

## 1. 分类协议与 Provider

- [x] 1.1 定义 `service_tags` v1 结构、已知 TAG、去重和固定顺序校验，并用合法、重复、未知、空列表和非法 JSON 单元测试验证分类边界
- [x] 1.2 为 Qwen Provider 增加无工具分类调用，使用短提示词、严格 JSON Schema 和 usage 统计，并用 HTTP contract test 验证请求字段、Schema 和响应解析
- [x] 1.3 增加分类失败错误映射和 guidance fallback 测试，验证超时、服务错误、非法 JSON、未知 TAG 均不会进入趋势工具流程

## 2. 编排与 Agent 路由

- [x] 2.1 在请求首个模型回合前执行分类节点，并把规范化 TAG 放入编排结果；用 orchestrator test 验证分类只执行一次且不占趋势工具预算
- [x] 2.2 根据 TAG 由服务端生成 `response_modes`，禁止后续规划模型覆盖模式；用 trend、guidance 和 mixed 三类回归测试验证最终外层 Agent 响应
- [x] 2.3 将趋势规划模型输入改为已确定的服务模式上下文，验证 trend-only、guidance-only 和 mixed 请求的工具与 Draft 行为
- [x] 2.4 处理服务端发现不适用 TAG 组合的结构化反馈，并用测试验证限制信息可以传给最终回复模型且不泄露内部错误

## 3. 流式、审计与回复提示词

- [x] 3.1 在 HTTP NDJSON 和前端操作记录中增加 `classify_service_modes` 进行中、完成和失败状态，并用 HTTP/前端测试验证原始分类内容不会泄露
- [x] 3.2 让 guidance-only 才注入样例问题提示词，trend-only 和 mixed 禁止注入，并用 Provider prompt contract test 覆盖三种模式
- [x] 3.3 扩展趋势审计与评测 trace 保存 service tags、分类状态、分类 token 和分类耗时，并用审计集成测试验证趋势工具次数与分类调用分开统计

## 4. Benchmark 与固定快照重构

- [x] 4.1 为 Trend/Agent 题卡增加 `expected_service_tags` 并更新题卡校验、解析和 scorer；用分类错误、纯 guidance、trend 和 mixed 题验证硬判分
- [x] 4.2 逐题审阅现有 30 道 Trend 与 10 道 Agent 题，修正不符合当前实现的类别调用、候选数量、错误事件和模式期望，并用固定快照 ValidateCases 验证全部题目可执行
- [x] 4.3 更新评测快照版本与候选容量，保留旧快照供历史报告使用，并用 snapshot tests 验证版本隔离、候选数量和日期范围
- [x] 4.4 修正离线 fixture、真实评测 runner 和报告统计以记录分类阶段，运行 Trend/Agent fixture 全量回归确认无 panic、无非法 trace 和成本统计完整

## 5. 集成验证与发布准备

- [x] 5.1 运行 Go、前端、评测和 OpenSpec 全量质量门，确认新分类协议、旧 Draft/Serving 契约和现有 UI 流事件同时通过
- [x] 5.2 在本地或 CVM 发布后先执行 Trend 6 类与 Agent 3 类抽样真实评测，核对分类失败、mixed TAG、工具调用、成本和延迟，再决定是否运行完整 Benchmark
- [x] 5.3 记录部署后的抽样评测报告和已知语义问题，确认未将分类原始模型内容、凭据或用户原始数据写入报告

## Workflow follow-up

- 评审并确认所有规划 artifacts 后，使用 `$openspec-apply-change` 开始实现。
- 实现和抽样评测通过后，再按自主镜像发布运行手册授权部署。
