# AI 调试模式

AI 调试模式只用于受控的本机排障，默认关闭，不属于网页端正式服务。

启用前必须在 API 进程的运行时环境中显式设置：

```bash
MROLESCOPE_AI_DEBUG_ENABLED=true
MROLESCOPE_AI_DEBUG_MAX_EVENTS=256
MROLESCOPE_AI_DEBUG_MAX_BYTES=8388608
MROLESCOPE_AI_DEBUG_TIMEOUT=2m
MROLESCOPE_LANGFUSE_BASE_URL=https://jp.cloud.langfuse.com
MROLESCOPE_LANGFUSE_PROJECT_ID=<Langfuse project ID>
MROLESCOPE_LANGFUSE_PUBLIC_KEY=<Langfuse public key>
MROLESCOPE_LANGFUSE_SECRET_KEY=<Langfuse secret key>
# 仅评测 CLI 使用；默认 false，避免评测数据意外外发
MROLESCOPE_LANGFUSE_EVALUATION_ENABLED=false
```

该模式在 `APP_ENV=production` 下会被配置校验拒绝。启用后，仅可从回环地址调用内部入口：

```bash
curl --noproxy '*' --silent --no-buffer \
  -H 'Content-Type: application/json' \
  --data '{"query":"哪些数据库正在被使用？"}' \
  http://127.0.0.1:8080/internal/ai-debug
```

可选的 `session_id`、`trace_name`、`user_id`、`metadata` 和 `tags` 只用于受控 Langfuse 归组。例如连续多轮排障可以显式传入同一个 `session_id`：

```json
{
  "query": "哪些数据库正在被使用？",
  "session_id": "debug:database-investigation",
  "trace_name": "mrolescope.ai_debug",
  "metadata": {"investigation": "category-ranking"},
  "tags": ["debug", "trend"]
}
```

Langfuse 官方要求 `sessionId` 是少于 200 个 ASCII 字符的应用侧标识；相同 session 下的 trace 会在 Session 页面归组回放。普通 Debug 请求不传 `session_id`，每次保持独立 trace，系统不会根据 query 自动归组。

评测导出使用题目级 Session：

```text
eval:<run_id>:<suite>:<case_id>
```

同一运行中同一道题的 repetition 和 retry 共享 Session，通过 `evaluation_repetition`、`evaluation_retry` 等 trace metadata 区分；不同题目或不同 `run_id` 不共享 Session。评测默认不外发 Langfuse，只有显式设置 `MROLESCOPE_LANGFUSE_EVALUATION_ENABLED=true` 且配置完整 Cloud 凭据时才启用。

入口不使用浏览器邀请码会话，也不挂载到前端路由。请求必须来自 loopback，查询体和 trace 都有大小与时长上限。

每行是一个带 `sequence` 的 NDJSON 调试事件，可能包含模型请求/响应 body、工具调用与返回、校验修正、最终 Intent、Draft 和终态。启用完整 Langfuse 配置后，末尾还会出现 `trace_link`，其中包含 trace ID、Japan Cloud trace URL 和上传状态。该运行不写入普通 `ai_trend_request_audits`，因此不会产生 Intent hash。

调试输出可能包含用户查询和模型上下文，只能写入受控本地文件，排障结束后删除。系统会递归替换 `authorization`、`api_key`、`token`、`password`、`secret`、`database_url`、`invite_code` 等凭据字段；不要把输出复制到公开日志、工单或聊天中。API 请求头、环境变量、数据库连接串不会作为调试事件记录。

完成排障后关闭 `MROLESCOPE_AI_DEBUG_ENABLED` 并重启 API。不要把该入口暴露到公网，也不要在生产环境启用。

Langfuse 使用 Hobby 计划时，平台默认提供最近 30 天的数据访问；本项目不宣称实现 7 天自动删除。上传失败或 20 秒 flush 超时不会阻断本地 Debug 结果，NDJSON 会记录失败状态。
