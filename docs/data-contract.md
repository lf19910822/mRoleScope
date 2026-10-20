# 数据与接口契约（M0）

## 约定

- 时间字段均为 RFC 3339 UTC 字符串。
- `schema_version` 是正整数；不兼容变更必须升级版本。
- 哈希使用小写十六进制 SHA-256。
- 任何未识别字段都不得改变现有字段的含义。

## 采集器任务目录

Python collector 成功时在 `--output-dir` 产生以下结构：

```
<run-dir>/
  manifest.json
  raw/list/page-001.json
  raw/details/<source_job_id>.json
```

`manifest.json` 的最小契约：

```json
{
  "schema_version": 1,
  "status": "complete",
  "run_id": "uuid-or-external-run-key",
  "source": {
    "company": "mihoyo",
    "list_endpoint": "https://…/job/list",
    "detail_endpoint": "https://…/job/info",
    "filter": { "competencyTypes": ["1"], "hireType": 0, "channelDetailIds": [1] },
    "filter_sha256": "…"
  },
  "started_at": "2026-10-20T00:00:00Z",
  "finished_at": "2026-10-20T00:01:00Z",
  "counts": { "list_reported_total": 1, "list_items": 1, "unique_source_job_ids": 1, "detail_succeeded": 1, "detail_failed": 0 },
  "files": [{ "path": "raw/list/page-001.json", "sha256": "…", "bytes": 42 }]
}
```

状态只能是 `complete` 或 `failed`。只有 `complete` 可交给 worker 的归档/发布阶段；`failed` 应携带 `failure.stage` 和安全的错误摘要。worker 必须在使用前重新计算文件哈希、校验相对路径和计数，不能信任 collector 的自报结果。

## 数据库实体

| 实体 | 不变量 |
| --- | --- |
| `collection_runs` | 一个调度键只有一个活跃执行；`published_at` 仅在完整成功批次设置 |
| `jobs` | `(source_company, source_job_id)` 唯一；未出现在某批次不等于已关闭 |
| `jd_versions` | 正文内容 hash 去重，保留原始字段和展示文本 |
| `run_jobs` | 每个成功批次都完整映射其所有岗位版本，`(run_id, job_id)` 唯一 |
| `parsed_versions` | 原文版本与规则/词典版本共同确定，不覆盖历史结果 |
| `change_events` | 只比较可比的相邻成功批次，定位旧/新版本 |
| `aggregate_snapshots` | 以批次和统计口径版本寻址 |

## REST API v1

M0 只实现运行状况接口；业务端点在各图表周期添加并先更新 OpenAPI。

| 方法与路径 | 成功响应 | 用途 |
| --- | --- | --- |
| `GET /healthz` | `200 {"status":"ok"}` | 进程存活，不连接数据库 |
| `GET /readyz` | `200 {"status":"ready"}` 或 `503` | 数据库可连接性 |
| `GET /api/v1/meta` | 预留 | 当前发布批次、数据范围与口径 |

响应 Content-Type 为 `application/json; charset=utf-8`。错误格式固定为 `{"error":{"code":"…","message":"…"}}`，不泄露内部 DSN、栈或密钥。
