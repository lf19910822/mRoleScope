# 问题描述
目前我们有了AI，可以凭借AI进行AI作图，以及以后拟定会有更多的AI应用。
但是我们目前没有一个完善的评测体系，benchmark。无法回答一下问题：
1. 我们的某次更新/修改是否有效
2. 不同模型之间差异如何
3. 我们对系统的优化无法量化，无法说明：某某指标提高了百分之几。

如果没有一套评测体系，目前只能算作DEMO。且后续的AI功能优化/扩展/测试都十分困难。

因此，我们需要一个评测体系。

# 开发要点
1. 仔细阅读 Docs/开发指导/AI作图_01/第十二章 智能体性能评估.md 中的相关评测体系，调研其评测方案/指标。重点关注BFCL
2. 首期拟定benchmark 30题。且benchmark要先生成供人员审阅的md文档，人员审阅通过后再编入代码。文件生成在Docs/开发指导/AI评测。
3. 每次评测需要审计，比如对失败的题目，要还原处理，思考过程，帮助后期调优者理解错误原因/trace过程。
4. 注意评估成本。不过30道题应该成本不会太大。

# 真实模型评测入口

Benchmark 默认使用离线 fixture，不会调用模型。需要进行真实评测时必须显式指定 Qwen provider：

```sh
go run ./cmd/ai-evaluation \
  -provider qwen \
  -suite trend \
  -cases Docs/开发指导/AI评测/AI趋势智能体Benchmark-v1.md

go run ./cmd/ai-evaluation \
  -provider qwen \
  -suite agent \
  -cases Docs/开发指导/AI评测/AI趋势智能体AgentBenchmark-v1.md
```

运行前由操作者在进程环境中配置 `MROLESCOPE_QWEN_API_KEY` 和 `MROLESCOPE_QWEN_WORKSPACE_ID`。程序只读取它们，不把值写入命令参数、日志、trace 或 report；缺少配置时会在发送请求前失败。

真实评测的模型调用使用 Qwen，工具和曲线数据仍来自评测固定快照。每道题运行三次，报告记录真实工具次数、输入/输出 token、首事件延迟、总耗时和按评测费率估算的成本。两套题分开统计，不写入生产 Serving、用户看板或日常审计。
