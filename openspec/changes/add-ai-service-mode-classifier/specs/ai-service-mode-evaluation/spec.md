# Spec Delta

## Purpose

让 Trend 与 Agent 评测明确检查前置服务类别分类，并保证题目、固定快照、scorer 和实际工作流使用同一套可实现的合同。

## ADDED Requirements

### Requirement: Benchmark cases declare expected service tags

每道 Trend 和 Agent 题卡 SHALL 记录 `expected_service_tags`；评测执行 SHALL 将分类节点输出与题卡期望进行确定性比较。

#### Scenario: Trend case expects trend

- **WHEN** 题目是“哪些项目的岗位最多？”
- **THEN** 题卡声明 `expected_service_tags: ["trend"]`，分类结果不包含 guidance 时通过分类分项

#### Scenario: Mixed case expects two tags

- **WHEN** 题目同时要求生成曲线和解释如何修改
- **THEN** 题卡声明 `expected_service_tags: ["trend","guidance"]`，缺少任一 TAG 时分类分项失败

### Requirement: Classification is a hard evaluation score

评测 SHALL 将分类 TAG、工具边界、Draft 存在性和最终模式作为硬判定；自然语言回复的具体措辞继续作为质量记录，不得替代分类判定。

#### Scenario: Wrong mode is recorded as semantic failure

- **WHEN** 混合题只返回 `trend`
- **THEN** 评测记录分类语义失败，即使趋势 Draft 成功生成

#### Scenario: Guidance-only tool call fails boundary score

- **WHEN** 纯指导题调用趋势工具
- **THEN** 分类/工具边界分项失败，同时保留完整轨迹供诊断

### Requirement: Benchmark data must match snapshot capacity

评测题目的数量、类别范围、候选数量和 Draft 容量要求 SHALL 能由当前固定快照和 scorer 实际判分；候选不足时不得要求不可能的曲线数量，快照更新时必须创建新的版本标识。

#### Scenario: Candidate shortage

- **WHEN** 类别只有两个正式候选但题目请求最多八条
- **THEN** 评测允许按实际候选数量返回，不将合理结果判为失败

#### Scenario: Capacity case

- **WHEN** 题目要求验证八条曲线容量
- **THEN** 固定快照至少提供八个该维度正式候选，题卡和 snapshot version 一致

### Requirement: Evaluation records classifier operations and cost

评测轨迹 SHALL 记录分类节点操作状态、分类 TAG、分类模型 token/耗时和后续工具 token/耗时，使分类节点增加的成本与延迟可单独统计。

#### Scenario: Classifier usage accounting

- **WHEN** 一次评测请求先完成分类再进入趋势工具
- **THEN** trace 同时包含分类节点状态和分类模型 usage，并将趋势工具次数与分类节点分开统计
