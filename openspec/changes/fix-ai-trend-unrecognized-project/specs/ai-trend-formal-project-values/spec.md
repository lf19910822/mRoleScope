# Spec Delta

## Purpose

确保 AI 趋势草稿只使用 Serving 趋势接口能够读取和应用的正式项目值，同时保留“未识别项目”作为岗位筛选专用选项。

## ADDED Requirements

### Requirement: Trend AI excludes filter-only project values

趋势 AI 的项目候选和可用值工具 SHALL 排除 `unrecognized` 这类没有正式趋势曲线的筛选伪值；岗位筛选接口仍 SHALL 保留该值。

#### Scenario: Broad project question

- **WHEN** 用户询问“哪些项目的岗位最多？”
- **THEN** 批量趋势工具只收到正式项目键，不收到 `unrecognized`

#### Scenario: Filter still supports unrecognized

- **WHEN** 用户打开岗位筛选的项目条件
- **THEN** 仍可看到并使用“未识别项目”筛选值

### Requirement: Valid project failures do not originate from the pseudo-value

趋势 AI 的正式项目批量摘要 SHALL 不因 `unrecognized` 触发 `invalid_query`；其他真实 Serving 错误继续按最大努力和部分结果规则处理。

#### Scenario: Project draft can be applied

- **WHEN** 正式项目批量摘要成功
- **THEN** 草稿候选均为可读取的项目曲线，可继续应用到看板

### Requirement: Batch trend summaries preserve successful items

批量趋势摘要 SHALL 对每个请求独立执行；单项失败时返回该项的安全错误和维度/值，同时保留同批成功摘要，并将整次草稿标记为部分完成。

#### Scenario: One project summary fails

- **WHEN** 批量请求中一个项目趋势返回 `invalid_query` 或后台错误
- **THEN** 其他成功项目摘要仍进入排序和草稿，失败项被审计并不会使整批数据丢失
