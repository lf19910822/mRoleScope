# AI 趋势智能体 Agent Benchmark v1

本题集只评测 Agent 的交互边界，与 30 道趋势曲线题分开执行、分开统计。它像一组“先判断该不该画图”的口试题：模型要先判断用户是在咨询功能、提出趋势问题，还是两者都有。

## 题目格式

每题使用以下 Agent 专用字段：

| 字段 | 含义 | 例子 |
| --- | --- | --- |
| `id` | 题目编号 | `G01` |
| `category` | Agent 分组；纯指导 4 题、无法理解或超出范围 3 题、混合 3 题 | `pure-guidance` |
| `query` | 用户原话 | `你好` |
| `expected_response_modes` | 预期模式；`guidance` 只回答问题，`trend` 生成曲线，二者都出现表示混合问题 | `["guidance"]` |
| `expected_terminal_status` | 预期请求终态；纯指导正常结束也是 `success` | `success` |
| `expected_no_trend_tools` | 是否禁止调用趋势工具；纯指导题为 `true` | `true` |
| `expected_draft` | 是否应有曲线 Draft；纯指导为 `false` | `false` |
| `reply_required` | 是否必须返回用户可读说明 | `true` |
| `reply_language` | 回复语言 | `zh-CN` |
| `reply_max_runes` | 回复 Unicode 字符上限 | `1000` |

模式、工具边界、回复是否存在和 Draft 是否存在是硬判定；具体措辞、中文完整性和限制说明只作为质量记录，不会把合法业务结果回滚。

## 纯指导 / 能力咨询（4 题）

```json
{"id":"G01","category":"pure-guidance","query":"你好","expected_response_modes":["guidance"],"expected_terminal_status":"success","expected_no_trend_tools":true,"expected_draft":false,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

```json
{"id":"G02","category":"pure-guidance","query":"怎么使用这个趋势看板？","expected_response_modes":["guidance"],"expected_terminal_status":"success","expected_no_trend_tools":true,"expected_draft":false,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

```json
{"id":"G03","category":"pure-guidance","query":"你能分析哪些维度和指标？","expected_response_modes":["guidance"],"expected_terminal_status":"success","expected_no_trend_tools":true,"expected_draft":false,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

```json
{"id":"G04","category":"pure-guidance","query":"我想先了解一下，曲线草稿生成后还能修改吗？","expected_response_modes":["guidance"],"expected_terminal_status":"success","expected_no_trend_tools":true,"expected_draft":false,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

## 无法理解或超出范围（3 题）

```json
{"id":"O01","category":"unclear-or-out-of-scope","query":"帮我看看这份简历适不适合做产品经理","expected_response_modes":["guidance"],"expected_terminal_status":"success","expected_no_trend_tools":true,"expected_draft":false,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

```json
{"id":"O02","category":"unclear-or-out-of-scope","query":"请根据公司内部面试记录给出录用建议","expected_response_modes":["guidance"],"expected_terminal_status":"success","expected_no_trend_tools":true,"expected_draft":false,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

```json
{"id":"O03","category":"unclear-or-out-of-scope","query":"随便分析一下现在的就业市场","expected_response_modes":["guidance"],"expected_terminal_status":"success","expected_no_trend_tools":true,"expected_draft":false,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

## 趋势与指导混合（3 题）

```json
{"id":"M01","category":"mixed-trend-guidance","query":"哪些项目的岗位最多？另外告诉我生成的曲线还能怎么修改。","dimension":"project","expressions":["*"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":["*"],"expected_scope":"global","max_curves":8,"expected_status":"complete","expected_response_modes":["trend","guidance"],"expected_terminal_status":"success","expected_draft":true,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

```json
{"id":"M02","category":"mixed-trend-guidance","query":"比较 Go 和 Python 的岗位数量，并解释你为什么选择这些曲线。","dimension":"technology","expressions":["go","python"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"none","limit":2,"expected_values":["go","python"],"expected_scope":"explicit","max_curves":8,"expected_status":"complete","expected_response_modes":["trend","guidance"],"expected_terminal_status":"success","expected_draft":true,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```

```json
{"id":"M03","category":"mixed-trend-guidance","query":"哪些数据库正在使用？尤其关注 MySQL，并说明看板现在支持什么图形。","dimension":"technology","expected_category":"databases-and-storage","expressions":["databases-and-storage"],"ranking_metric":"avg_daily_active_jobs","ranking_order":"desc","limit":8,"expected_values":[],"required_values":["mysql"],"expected_scope":"category-with-required-value","max_curves":8,"expected_status":"complete","expected_response_modes":["trend","guidance"],"expected_terminal_status":"success","expected_draft":true,"reply_required":true,"reply_language":"zh-CN","reply_max_runes":1000}
```
