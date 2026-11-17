# Spec Delta

## Purpose

为每次技术词典升级提供完整历史重算、证据验证和原子发布能力，确保候选分析不会提前影响公开产品，发布后所有 Serving 数据只来自同一套最新词典结果。

## ADDED Requirements

### Requirement: Candidate analysis freezes every matching input
候选分析配置 SHALL 冻结解析器版本、词典版本、已批准词面、排除规则、身份展开关系及其稳定指纹。配置创建后，任何输入变化 MUST 创建新的配置，不得修改原配置以改变其可复演语义。

#### Scenario: Matching policy change creates a new configuration
- **WHEN** an exclusion pattern or identity expansion relation changes after a candidate configuration is prepared
- **THEN** the existing configuration remains unchanged and a new analysis configuration is required

#### Scenario: Repeated preparation is idempotent
- **WHEN** the same parser, dictionary, rule set and matching-policy fingerprint are prepared more than once
- **THEN** the operation reuses the same candidate configuration instead of creating competing configurations

### Requirement: Every historical body version is reanalyzed before release
新词典候选配置 SHALL 为全部历史 JD 正文版本建立幂等分析工作项，并在发布前完成每项结果、证据坐标和规则快照验证。单条正文失败 MUST 被安全记录，不得伪造成功结果或沿用旧词典结果冒充新结果。

#### Scenario: Historical bodies are queued once
- **WHEN** v0.4 analysis preparation runs against existing body history
- **THEN** every historical body version has at most one v0.4 work item, and repeated preparation does not duplicate work

#### Scenario: Incomplete analysis blocks publication
- **WHEN** any required v0.4 work item is pending, missing, or has invalid evidence
- **THEN** v0.4 is not published and the current active configuration remains unchanged

#### Scenario: One failed body does not rewrite source history
- **WHEN** one body analysis fails after source synchronization has succeeded
- **THEN** the source run and JD history remain published, the failure is auditable, and no v0.3 result is relabeled as v0.4

### Requirement: The current publication remains active during candidate analysis
候选词典和候选分析结果 MUST remain invisible to Serving until the release gate succeeds. Incomplete v0.4 analysis MUST NOT cause the product to show empty or mixed-version technology results while a prior published configuration exists.

#### Scenario: v0.3 remains visible during v0.4 preparation
- **WHEN** v0.4 has been seeded or is still being analyzed while v0.3 is active
- **THEN** current filter options, job details and trend queries continue reading v0.3 results only

#### Scenario: Unpublished v0.4 result is excluded
- **WHEN** a complete v0.4 result exists but the v0.4 configuration has not been published
- **THEN** Serving does not expose that result or its technology option

### Requirement: Publication switches atomically after validation
发布操作 SHALL 在同一受控事务中确认完整分析、规则集合、证据坐标和发布前置状态，然后切换 active publication 到 v0.4，并将 v0.3 配置标记为 `superseded`。发布失败 MUST 保持原 active publication 不变。

#### Scenario: Successful v0.4 release supersedes v0.3
- **WHEN** every v0.4 work item and release check passes while v0.3 is the active configuration
- **THEN** the active pointer references v0.4, v0.4 is published, v0.3 is marked superseded, and one immutable release attestation is recorded

#### Scenario: Failed release preserves v0.3
- **WHEN** a release check fails or the publication transaction cannot commit
- **THEN** v0.3 remains active, v0.4 remains unavailable to Serving, and no partial pointer switch is visible

#### Scenario: Repeated successful release is safe
- **WHEN** the fixed v0.4 release operation is invoked again after v0.4 is already active
- **THEN** it returns a reused successful outcome without creating duplicate release facts or changing the active pointer to an older configuration

### Requirement: Serving reads only the active technology configuration
岗位筛选、技术选项、岗位详情中的技术结果和趋势统计 SHALL 约束到 `technology_analysis_publication_state` 指向的唯一 active configuration。完成时间、创建时间或结果新旧不得绕过该约束。

#### Scenario: Superseded result is not served
- **WHEN** both v0.3 and v0.4 results exist for the same body version after v0.4 publication
- **THEN** Serving returns only the v0.4 result

#### Scenario: No fallback to old dictionary
- **WHEN** a v0.4 result is unavailable for a body version after v0.4 becomes active
- **THEN** Serving does not substitute the v0.3 result; it returns the product-defined no-result state for that body version

### Requirement: Previous dictionary facts remain auditable but non-serving
被 supersede 的词典、分析配置、结果、证据、审核和发布证明 SHALL 保留为不可变历史事实，但 MUST NOT 出现在当前技术筛选选项、当前岗位技术标签或当前趋势统计中。

#### Scenario: Historical audit can distinguish versions
- **WHEN** an authorized remote audit queries a job analyzed by both versions
- **THEN** it can distinguish v0.3 and v0.4 by dictionary/configuration identity without either result being overwritten

#### Scenario: Public projection excludes superseded version
- **WHEN** a public query requests technology options after v0.4 publication
- **THEN** only identities with complete v0.4 results in the active projection are returned

### Requirement: Release operations do not expose source text or secrets
候选准备、分析、发布和投影操作 SHALL only emit safe identifiers, counts, statuses and fixed error categories. 日志、审计记录、命令参数和公开响应 MUST NOT contain JD 正文、命中片段、来源凭据、数据库连接串或秘密材料。

#### Scenario: Safe audit records contain no source snippet
- **WHEN** a v0.4 analysis run records its completion or failure
- **THEN** the audit record contains safe counts and identifiers only, with no normalized JD text or matched snippet

#### Scenario: Release rejects arbitrary operator input
- **WHEN** an operator supplies an arbitrary dictionary label, SQL statement, database URL, source body or image reference to the release operation
- **THEN** the fixed operation rejects the input and does not execute it
