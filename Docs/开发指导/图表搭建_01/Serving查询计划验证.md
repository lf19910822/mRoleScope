# Serving 查询计划验证

## 范围

列表 API 的本期查询仅读取 `serving_jd_current_inventory_memberships`、
`serving_jd_public_identifiers` 和确定性维度解析关系。它不回读 ODS 正文、
来源元数据、AI 语义结果或审计表。

集成验证使用 200 条当前 Serving 成员的隔离数据库样本；这一规模覆盖了本期
约 171 条可见岗位的部署量级。验证执行两类
`EXPLAIN (ANALYZE, BUFFERS, COSTS OFF)` 查询：

1. 默认的有效岗位、地点筛选和 20 条分页；
2. 固定阈值的标题包含／模糊查询和 20 条分页。

命令：

```bash
TEST_DATABASE_URL=... go test -p 1 ./internal/jobsync \
  -run TestServingListQueryPlansStayInsideServingBoundaryAtDeployedScale -count=1
```

## 验收条件与结论

- 计划中存在 `Limit`，列表页的返回量受页面容量限制。
- 计划不得出现 `job_source_metadata_versions`、
  `job_body_content_versions`、`jd_versions`、`ai_semantic_results` 或原始字段表。
- 已验证的索引均属于 DWD/Serving：当前成员查找、标题 trigram、地点 GIN 和
  公开 ID；本期没有为浏览器查询新增 ODS 索引。

上述条件由集成测试自动断言。当前部署量级下不需要新增索引；后续以实际
`EXPLAIN (ANALYZE, BUFFERS)` 证据再调整 Serving 索引。
