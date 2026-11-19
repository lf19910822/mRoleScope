# Spec Delta

## Purpose

让趋势看板把用户的自然语言问题转换为安全、可解释、可编辑的岗位趋势曲线草稿。AI 负责判断应该展示哪些曲线，正式趋势数据仍由 Serving 提供，用户确认后才改变看板配置。

## ADDED Requirements

### Requirement: Trend dashboard entry and supported scope
AI 作图 SHALL only be available from the trend dashboard and SHALL return an editable draft of at most eight ordinary daily active-job curves.一期 SHALL support technology, project, job-family, and location dimensions.

#### Scenario: User asks a natural question in the trend dashboard
- **WHEN** the user enters “哪些后台数据库” or “哪些项目正在增加岗位”
- **THEN** the system SHALL treat the input as a curve-draft request without requiring the words “画趋势” or “生成图表”
- **AND THEN** it SHALL not change already applied curves before explicit user application

#### Scenario: User asks for an unsupported natural-language dimension
- **WHEN** a request includes education or work-experience requirements
- **THEN** the system SHALL continue any supported sub-request
- **AND THEN** it SHALL clearly state that the unsupported sub-request is unavailable

### Requirement: AI remains a curve selector
The system SHALL use AI to understand intent, choose curve conditions, and explain selections. It MUST use Serving for formal trend facts and MUST NOT let AI generate images, daily points, SQL, database reads, JD reads, credentials, or direct database mutations.

#### Scenario: AI selects curves
- **WHEN** AI identifies database candidates
- **THEN** the draft SHALL contain validated curve conditions, labels, and short server-derived reasons
- **AND THEN** the browser SHALL obtain complete daily points from Serving only after the user applies the draft

#### Scenario: Complex metric selects ordinary curves
- **WHEN** the user asks for “增长最快的项目” or “进入岗位最多的技术关键词”
- **THEN** a deterministic service SHALL use the requested metric to rank candidates
- **AND THEN** the resulting chart curves SHALL still use ordinary daily active-job counts
- **AND THEN** the system SHALL not draw growth-rate, percentage, or event-count curves in this capability

### Requirement: Deterministic intent metrics and time ranges
The system SHALL express intent with fixed metric, dimension, ordering, limit, output, and time-range fields. Without a qualifier, “岗位最多/数量最高” SHALL mean the period average active-job count. “截至目前” SHALL mean period-end count, “峰值/最高一天” SHALL mean period peak, and “期间出现过的不同岗位” SHALL mean period unique jobs.

#### Scenario: Default ranking metric
- **WHEN** the user asks for the jobs with the highest count without another qualifier
- **THEN** the system SHALL rank by average daily active jobs over the selected period

#### Scenario: Missing observations and a real zero
- **WHEN** a selected day is unavailable
- **THEN** the metric SHALL omit that day according to the established trend rule and SHALL not turn it into zero
- **WHEN** a complete selected day has no matching jobs
- **THEN** the metric SHALL treat that day as a real zero

#### Scenario: Conflicting explicit ranges
- **WHEN** one request explicitly contains conflicting ranges such as “本月数据库、去年项目”
- **THEN** the system SHALL report the conflict and ask the user to rewrite the request with one range
- **AND THEN** it SHALL not silently choose the first range or open a range selector

#### Scenario: No explicit range
- **WHEN** the user does not specify a range
- **THEN** the system SHALL use the trend dashboard default week without asking a time clarification

### Requirement: Formal release and Serving data boundary
AI tools SHALL read only current published formal values and the complete published trend interpretation used by Serving. A draft SHALL be revalidated against the current active configuration when applied.

#### Scenario: A new formal release is incomplete
- **WHEN** a dictionary or deterministic interpretation release is not complete
- **THEN** AI candidates and Serving trends SHALL continue using the prior complete release
- **AND THEN** the system SHALL not mix release results

#### Scenario: A draft value was retired before application
- **WHEN** a generated draft contains a value no longer in the active configuration
- **THEN** application SHALL mark that value unavailable and require user adjustment
- **AND THEN** it SHALL not silently replace the value with a similar one

#### Scenario: Existing applied curve meets a newer release
- **WHEN** the trend dashboard refreshes after a newer complete formal release
- **THEN** affected applied curves SHALL use the newer formal interpretation together
- **AND THEN** the dashboard SHALL not show a mixed-release comparison

### Requirement: Safe autonomous tool orchestration
AI MAY choose and sequence registered tools autonomously, but the server SHALL enforce a fixed whitelist, validated parameters, bounded results, and deterministic tool outputs. A request SHALL use at most 20 tool calls, each tool SHALL return at most 100 candidate values, and each batch tool SHALL process at most eight candidates.

#### Scenario: AI chooses a tool sequence
- **WHEN** the user asks for a technology category
- **THEN** AI MAY choose category listing, category-value listing, ranking, summary, event, batch, and draft-validation tools in its own order
- **AND THEN** every call SHALL pass server-side schema, range, current-value, timeout, and result-size checks

#### Scenario: Tool budget is reached
- **WHEN** the request reaches 20 calls or the server time limit
- **THEN** the system SHALL stop further tool calls
- **AND THEN** it SHALL retain valid results already obtained and clearly state that the analysis was limited

#### Scenario: Batch and single tools
- **WHEN** multiple candidates need the same bounded analysis
- **THEN** AI MAY use a batch tool
- **AND THEN** a failure for one batch element SHALL not discard successful elements

### Requirement: Tool input and AI summary boundary
Analysis tools SHALL return finite safe summaries to AI, including formal curve names, ranking summaries, peak information, event summaries, and valid or missing observation-day counts. They MUST NOT return complete daily TrendSeries points, JD content, credentials, raw SQL, internal table names, or internal release identifiers to AI.

#### Scenario: AI chooses from summaries
- **WHEN** AI compares candidates
- **THEN** it SHALL use server-computed summaries and SHALL not recalculate daily values itself
- **AND THEN** the browser SHALL retrieve complete daily points only after draft application

### Requirement: Natural-language matching and category mapping
The system SHALL resolve user expressions against current formal values using normalization, formal aliases, and bounded similarity matching. One expression MAY expand to at most three high-similarity candidates. Non-exact candidates SHALL be labeled “可能匹配”. AI SHALL be able to discover current technology categories and values through registered tools.

#### Scenario: Alias and fuzzy matching
- **WHEN** the user writes a casing, punctuation, alias, prefix, containment, or similar form of a formal value
- **THEN** the system SHALL resolve it against current formal values
- **AND THEN** it SHALL show the actual formal value in the draft
- **AND THEN** non-exact results SHALL carry the possible-match label

#### Scenario: Category question
- **WHEN** the user asks “哪些后台数据库” or “哪些模型训练工具”
- **THEN** AI SHALL map the phrase to a current formal category
- **AND THEN** candidates SHALL be ranked by period-average active jobs and limited to eight curves
- **AND THEN** an omitted-candidate count SHALL be stated when more values exist than the limit

#### Scenario: Exclusion phrase
- **WHEN** the user says “除了 MySQL” or “不包括 MySQL”
- **THEN** the system SHALL exclude that formal value from the candidate set
- **AND THEN** the draft explanation SHALL state that MySQL was excluded

#### Scenario: No exact value but a wider category is known
- **WHEN** “量子数据库” has no formal exact value but “数据库与存储” is identifiable
- **THEN** the system SHALL include the wider category’s candidates as default-selected expansion recommendations
- **AND THEN** each candidate SHALL be labeled “扩展推荐，不是精确匹配” in the draft explanation

#### Scenario: No category is identifiable
- **WHEN** the system cannot map a request to any supported formal category or value
- **THEN** it SHALL preserve the current dashboard and produce no unrelated curves
- **AND THEN** it SHALL show actionable example prompts that can be clicked to fill the input without submitting

### Requirement: Candidate ordering, capping, zero values, and identity
All candidate sets SHALL obey one universal eight-curve hard limit. Exact and formal-alias matches take priority over possible matches, earlier user conditions take priority over later ones, and candidates within one expression are ordered by similarity. Equal ranking metrics SHALL use stable formal-name ordering. A curve identity SHALL include dimension, formal value, comparison condition, and the job-family secondary-family option.

#### Scenario: More than eight candidates
- **WHEN** any candidate set contains more than eight curves, including all-zero candidates
- **THEN** the system SHALL retain the first eight under the fixed ordering
- **AND THEN** it SHALL state the total and omitted counts

#### Scenario: Equal ranking values
- **WHEN** two candidates have the same ranking metric
- **THEN** the system SHALL order them by stable formal name

#### Scenario: Duplicate across sub-questions
- **WHEN** different sub-questions resolve to the same full curve identity
- **THEN** the system SHALL merge them into one curve
- **AND THEN** the draft SHALL state which sub-questions it answers

#### Scenario: Job-family scope differs
- **WHEN** one curve is “仅主岗位族” and another is “含副岗位族”
- **THEN** they SHALL remain separate curves

#### Scenario: Category candidates are all zero
- **WHEN** every candidate in a requested category has zero jobs in the selected period
- **THEN** the system SHALL still create the bounded all-zero draft
- **AND THEN** it SHALL warn that the category has no jobs in the current period
- **WHEN** the user explicitly names a formal zero-value curve
- **THEN** the system SHALL retain that curve

#### Scenario: Category has mixed zero and positive values
- **WHEN** a category contains both positive-job and zero-job formal values
- **THEN** a category recommendation SHALL include the positive-job candidates and omit zero-job candidates
- **AND THEN** an explicitly named zero-job formal value SHALL still be retained

### Requirement: Multi-sub-question draft allocation and display
Multiple sub-questions SHALL share one eight-curve draft and one trend chart. The system SHALL reserve at least one curve for each valid sub-question before filling remaining slots by ranking, match confidence, and user order.

#### Scenario: Multiple categories in one request
- **WHEN** the user asks for databases, training tools, and projects together
- **THEN** valid sub-questions SHALL each receive at least one curve when candidates exist
- **AND THEN** the draft list SHALL be grouped by sub-question while the applied chart remains one chart

#### Scenario: Ambiguous dimension
- **WHEN** a term such as “平台” may be a job family, project, or technology
- **THEN** the system SHALL show possible candidates from multiple dimensions
- **AND THEN** each candidate SHALL display its dimension and possible-match status

#### Scenario: Project and technology combination
- **WHEN** the user asks “绝区零项目使用了哪些数据库”
- **THEN** the system SHALL provide the project curve and global technology curves as maximum-effort results
- **AND THEN** the AI explanation SHALL say that the technology curves are global and not project-filtered
- **AND THEN** global technology curves SHALL be default-selected in the draft

### Requirement: Unsupported requests preserve useful results
Unsupported chart styles, unsupported vertical metrics, and unsupported sub-questions SHALL not discard valid curve work when an ordinary岗位数量曲线 can still answer part of the request.

#### Scenario: Unsupported chart type
- **WHEN** the user requests a bar or pie chart
- **THEN** the system SHALL generate ordinary line-curve drafts and state that the dashboard uses line charts

#### Scenario: Unsupported vertical metric
- **WHEN** the user requests percentages or another unsupported vertical metric
- **THEN** the system SHALL generate ordinary岗位数量曲线 and state that the requested metric is not directly drawable
- **AND THEN** it SHALL not present counts as percentages

#### Scenario: Supported and unsupported sub-requests are mixed
- **WHEN** one prompt asks for projects and unsupported education requirements
- **THEN** the project curves SHALL remain available
- **AND THEN** the unsupported education portion SHALL be explicitly reported

### Requirement: Draft presentation and application
Each draft curve SHALL show a concise server-derived reason, such as ranking summary, possible-match status, expansion status, or global scope. Candidates are selected by default unless the user removes them. The user MUST explicitly choose application behavior when existing applied curves exist.

#### Scenario: Draft reasons
- **WHEN** a draft contains ranked or fuzzy candidates
- **THEN** each row SHALL show a concise reason and relevant “可能匹配” or “扩展推荐” label
- **AND THEN** the UI SHALL not show complete daily points or model reasoning

#### Scenario: No existing applied curves
- **WHEN** the user confirms a draft with no current curves
- **THEN** the UI SHALL offer “应用草稿” and apply only after the user clicks it

#### Scenario: Existing applied curves
- **WHEN** the user has existing applied curves
- **THEN** the UI SHALL offer explicit replace and append actions
- **AND THEN** append SHALL report a capacity conflict instead of silently dropping or replacing curves

#### Scenario: Scope labels after application
- **WHEN** an expansion recommendation is applied
- **THEN** the formal chart legend SHALL use the formal curve name without the expansion label
- **WHEN** a global-scope fallback is applied
- **THEN** the formal chart legend SHALL use the formal curve name, while the AI answer retains the global-scope explanation

### Requirement: Temporary draft and independent request lifecycle
Unapplied drafts, operation records, and stream state SHALL be temporary to the current trend page. Each user submission SHALL be an independent request without conversational context.

#### Scenario: Page lifecycle
- **WHEN** the user refreshes or leaves the trend page
- **THEN** the unapplied draft and expandable operation record SHALL be cleared

#### Scenario: Independent short follow-up
- **WHEN** the user submits “再加 Redis” after a previous request
- **THEN** the system SHALL treat it as a new request without inheriting the previous AI draft

#### Scenario: New request replacement
- **WHEN** a new request produces a successful draft
- **THEN** it SHALL replace the old unapplied draft
- **WHEN** a new request fails, is stopped, or produces no usable result
- **THEN** the old unapplied draft SHALL remain

#### Scenario: Apply during a new request
- **WHEN** a new request is streaming while an old draft exists
- **THEN** the user SHALL still be able to apply the old draft
- **AND THEN** a later new result SHALL only replace the pending draft, never auto-change applied curves

### Requirement: Streaming operation events
The AI request SHALL use one server-to-browser HTTP stream and SHALL emit structured JSON events for operations and final state. The stream MAY show registered tool names and filtered parameters, but SHALL not show result summaries, raw tool output, SQL, secrets, JD content, internal error stacks, or model chain-of-thought.

#### Scenario: Operation stream
- **WHEN** the user submits a request
- **THEN** the stream SHALL emit operation names or tool calls with running, completed, or failed status
- **AND THEN** it SHALL end with a separate success, partial, failure, or user-stopped event

#### Scenario: Duplicate submission
- **WHEN** a request is streaming
- **THEN** the submit action SHALL be disabled
- **AND THEN** the UI SHALL provide a stop action

#### Scenario: Operation record after completion
- **WHEN** a request completes or stops
- **THEN** the operation record SHALL be collapsed by default
- **AND THEN** the user SHALL be able to expand it in the current page

### Requirement: Error responsibility and bounded AI correction
The system SHALL distinguish invalid AI parameters from valid parameters followed by backend failures. Invalid structured requests MAY be returned to AI for correction, with at most three total AI attempts. Backend, database, trend-service, release, and unavailable-value failures SHALL not trigger AI retries.

#### Scenario: Invalid AI parameter
- **WHEN** schema, enum, dimension combination, time range, ordering, or curve-limit validation fails
- **THEN** the server SHALL return a safe correction reason to AI
- **AND THEN** the stream SHALL show validation failure, AI correction, and retry

#### Scenario: Valid parameter with backend failure
- **WHEN** all AI parameters pass validation but a tool or backend fails
- **THEN** the stream SHALL mark that tool failed and continue independent steps
- **AND THEN** it SHALL not ask AI to retry the backend failure

#### Scenario: All backend work fails
- **WHEN** no usable curve can be produced because backend tools fail
- **THEN** the UI SHALL show a failure message without a retry button
- **AND THEN** the user SHALL need to edit or submit a new request manually

#### Scenario: Partial backend success
- **WHEN** some independent tools or candidates fail and others succeed
- **THEN** the system SHALL retain valid candidates and identify incomplete analysis
- **AND THEN** it SHALL not overwrite applied curves

### Requirement: User stop and partial draft
The user SHALL be able to stop an active request. Stopping SHALL cancel the stream, preserve applied curves, and keep any already found candidates as an explicitly incomplete temporary draft.

#### Scenario: User stops during analysis
- **WHEN** the user clicks “停止分析” after some candidates have been found
- **THEN** the system SHALL end the request and mark the stream as user-stopped
- **AND THEN** it SHALL keep found candidates in a draft marked “分析已停止，结果可能不完整”
- **AND THEN** the user SHALL be able to edit and apply that draft

### Requirement: Safe bounded audit summary
The system SHALL retain bounded AI trend audit summaries for quality and failure analysis. It SHALL retain at most 30 days and 10,000 records, deleting oldest records when either limit is reached.

#### Scenario: Audit content
- **WHEN** an AI trend request runs
- **THEN** the audit MAY retain irreversible request identity, structured intent validation, registered tool statuses, safe error codes, token/call/time aggregates, and draft summaries
- **AND THEN** it SHALL not retain full user text, full tool output, full database output, JD content, secrets, connection strings, raw model response, or internal diagnostic stacks

#### Scenario: Audit cleanup failure
- **WHEN** audit cleanup or audit persistence fails
- **THEN** the system SHALL emit an operational warning
- **AND THEN** it SHALL not turn the trend request or an unrelated batch into a failure

### Requirement: User-facing safety and public data boundaries
The AI trend surface SHALL expose only validated display-safe conditions, labels, explanations, operation names, filtered parameters, and formal trend results. It MUST NOT expose parser or dictionary versions, coverage or audit state, source versions, internal release identifiers, credentials, raw JD content, or arbitrary database details.

#### Scenario: Safe public response
- **WHEN** a valid draft or trend request completes
- **THEN** the response SHALL contain only bounded curve configuration and safe explanations
- **AND THEN** formal Serving endpoints SHALL remain read-only and private

#### Scenario: Invalid or unknown value
- **WHEN** a user value does not exist in the current valid set
- **THEN** the system SHALL report it as unavailable or possible-match failure
- **AND THEN** it SHALL not invent a new formal value or expose database diagnostics
