# 结构已提供，可以直接保存
| 数据 | 现有接口字段 | 处理方式 |
|---|---|---|
| 岗位ID、标题 | `id`、`title` | 直接保存 |
| 工作地点 | `addressDetailList[].addressDetail` | 保留数组，支持多城市 |
| 官方岗位大类 | `competencyType`、`competencyTypeId` | 如“程序&技术类” |
| 工作性质 | `jobNature`、`jobNatureId` | 保存名称和编码 |
| 招聘相关标签 | `projectName`、`hireTypeName`、`objectName` | 保留来源含义，不自行重命名解释 |
| 紧急标记 | `hurry` | 保留布尔值，缺失时不要自动当作否 |
| 岗位职责 | `description` | 保存原文及清洗后的文本 |
| 任职要求 | `jobRequire` | 保存原文，再解析内部要求 |
| 补充说明、投递说明、摘要 | `addition`、`deliveryInstructions`、`jobSummary` | 有值则保存 |

# 额外解析目标

## 需要AI
1. 工作年限 不考虑子项目
2. 技术熟练程度 （"key": 技术tag, "value": 熟练程度）

## 无需AI
1. 学历：使用版本化学历词典和固定解析规则，不调用模型。
2. 技术关键词（Go、Kubernetes、k8s、MySQL）注意多映射关系，比如 k8s == Kubernetes。
3. 项目维度：使用人工维护、版本化的项目词典，只匹配岗位标题；不从 `projectId`/`projectName`、JD 正文、技术栈或 AI 输出推断项目。
4. 各种图表构建的必要数据，如新增、移出、修改、图表数字、文本差异。

# 首期AI解析目标
1．年限：按经验领域记录，base_year 取已提取的必要经验下限中的最大值。

例如“后端工作三年以上，其中K8s使用一年以上”：
{
  "experience_years": {
    "后端": 3,
    "Kubernetes": 1,
    "base_year": 3
  }
}

建议约定：
- 技术领域名称复用技术字典，例如 K8s 统一为 Kubernetes；后端等非具体技术领域使用固定名称。如果AI给出了技术词典中没有的技术关键词，也可以记录下来，作为潜在的技术词典拓展项目
- “3年以上”取 3；“3—5年”取下限 3，然后再计算各项下限的最大值。
- 只统计候选人的必要经验，不把“5年经验者优先”或“项目已运营5年”计入。
- base_year 是首期计算口径，不保证等于原文明示的总工作年限。假如只写“5年C++经验”，它也会是 5。
没有明确数字年限，或明确不限经验时，可以统一返回：
{
  "experience_years": {
    "not_configured": 0
  }
}

此时不再同时输出 base_year: 0。页面展示为“未设置明确年限”，不要显示为“要求0年”。这会合并“未提及”和“明确不限”两种情况，是首期主动接受的简化。

### 学历：确定性正式结果

学历不再由 AI 作为正式来源。固定解析器把已批准的词典别名（例如专科/职校、本科、硕士/研究生、博士/PhD）映射到 `associate_or_lower`、`bachelor`、`master`、`doctor`，并保留规则版本、字段哈希与 Unicode 码点证据。

结果有 `configured`、`not_configured`、`uncertain` 三种状态，并分开保存下限与上限：

- `本科及以上，硕士优先`：下限为 `bachelor`；“硕士优先”仅作为加分证据，不能提升硬性要求。
- `本科及以下`：上限为 `bachelor`，不伪造“最低本科”。
- `硕士或博士均可`：明确备选的最低允许等级为 `master`。
- `学历不限` 或仅有“硕士优先”：没有正式上下限，状态为 `not_configured`。

产品展示使用 `专科及以下 / 本科 / 硕士 / 博士` 这四档；当 JD 表达的是上限时，展示层必须保留“及以下”的方向。AI 的历史学历投影和审计记录仍可读取，但不得作为正式学历字段的回退来源。完整的词典、增量处理和 CVM 验收规则见[确定性学历解析运行与验收指导](确定性学历解析运行与验收指导.md)。

### 项目维度：确定性标题归属

项目维度的正式来源是人工维护的项目词典与岗位 `title` 的确定性匹配，不调用 AI。词典条目分为 `named_game`（明确游戏）和 `source_direction`（来源方向/预研标签）；后者不得展示为已确认游戏。

- 接口的 `projectId`/`projectName` 是来源元数据，不能替代项目归属。当前“社会招聘”等来源值不产生项目标签。
- 只有标题命中启用关键词才会产生归属；正文提及仅可作为人工维护线索，技术栈、相似职位和传闻代号均不能推断归属。
- 标题无命中时保存并展示为“未识别”，不默认映射为公共平台、无项目或其他已有项目。
- 同一标题可保留多个独立匹配；明确游戏与来源方向必须分组展示，统计时区分唯一岗位数和岗位—项目关系数。
- 同一标题在后续同步中复用结果；仅标题或词典/解析器版本变化才会创建新的历史结果。来源地点等其他元数据变化不重跑项目匹配，也不触发技术或 AI 解析。

完整的初始词典边界见[项目维度开发](项目维度开发.md)，运行和验收规则见[确定性项目维度解析运行与验收指导](确定性项目维度解析运行与验收指导.md)。

3. 技术要求关系
参考结构：
{
  "jd_version_id": "jd-version-001",
  "requirement_groups": [
    {
      "id": "g1",
      "role": "qualification",
      "necessity": "required",
      "operator": "at_least",
      "min_satisfied": 1,
      "options": [
        {
          "skill_id": "skill.python",
          "proficiency": "P3",
          "evidence_refs": ["e1"]
        }
      ],
      "evidence_refs": ["e1"]
    },
    {
      "id": "g2",
      "role": "qualification",
      "necessity": "required",
      "operator": "at_least",
      "min_satisfied": 1,
      "options": [
        {
          "skill_id": "skill.go",
          "proficiency": "P4",
          "evidence_refs": ["e2"]
        },
        {
          "skill_id": "skill.cpp",
          "proficiency": "P4",
          "evidence_refs": ["e2"]
        },
        {
          "skill_id": "skill.java",
          "proficiency": "P4",
          "evidence_refs": ["e2"]
        }
      ],
      "evidence_refs": ["e2"]
    }
  ],
  "candidate_mentions": []
}
含义为：
Python必须达到熟练；
Go、C++、Java中至少一种达到精通。

| 字段 | 含义与取值 |
|---|---|
| `requirement_groups` | 一份JD中的技术要求组 |
| `id` | 当前解析结果内唯一的组标识 |
| `role` | `qualification` 任职资格、`responsibility` 工作职责、`mention` 普通提及、`unspecified` 待判断 |
| `necessity` | `required` 必要、`preferred` 加分、`unspecified` 未明确 |
| `operator` | 首版统一为 `at_least` |
| `min_satisfied` | 至少满足多少个技术选项 |
| `options[].skill_id` | 已确认技术字典中的稳定ID |
| `options[].proficiency` | `P1`了解、`P2`熟悉、`P3`熟练、`P4`精通；未明示或无法判断用 `null` |
| `evidence_refs` | 引用远端证据记录 |

组级证据支持“必须、优先、任选”等整体关系；选项级证据支持具体技术及其熟练程度。两者可以引用同一段证据，也可以不同。

单项、任选、全部满足，都使用相同结构：
| 表达 | 选项数量 | `min_satisfied` |
|---|---:|---:|
| 必须熟练Python | 1 | 1 |
| 三种语言至少会一种 | 3 | 1 |
| 三种语言至少会两种 | 3 | 2 |
| 三种语言全部需要 | 3 | 3 |

还建议统一以下约定：
- 多个 required 组之间需要同时满足；preferred 组单独表达加分条件。
- 同一技术可以出现在不同组中，不能按 skill_id 覆盖旧记录。例如“熟悉Python；有Python性能优化经验优先”是两条不同要求。
- 字典外名称放入 candidate_mentions，保存候选词与证据，不自动生成正式技术ID。
- 无法确定关系时，记录待复核状态，不强填 min_satisfied。
- 证据记录沿用既定的“来源字段＋规范化文本版本＋Unicode码点区间”，真实文本留在远端。
以要求组作为原始分析结构；技术ID → 要求列表可以作为查询时生成的索引视图。 这样既能按技术统计，也能完整保留任选关系。
