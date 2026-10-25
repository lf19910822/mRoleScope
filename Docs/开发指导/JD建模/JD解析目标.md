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
2. 学历
3. 技术熟练程度 （"key": 技术tag, "value": 熟练程度）
4. 项目名称 （原神，绝区零。。。）

## 无需AI
1. 技术关键词（Go、Kubernetes、k8s、MySQL）注意多映射关系，比如 k8s == Kubernetes
2. 各种图表构建的必要数据，如 新增、移出、修改, 图表数字、文本差异.

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

最低学历：value建议表示学历编码，不是年限。
key	value
大专及以下	1
本科	2
硕士	3
博士	4
not_configured	0


例如“本科及以上，硕士优先”：
{
  "minimum_education": {
    "本科": 2
  }
}

“硕士或博士学历”：
{
  "minimum_education": {
    "硕士": 3
  }
}

建议每份JD只输出一个最低学历标签：
- 取必要资格中的最低学历，忽略更高学历的加分条件。
- 没有明确最低要求，或者明确不限学历，返回 {"not_configured": 0}。
- 只有“硕士优先”、没有基础学历要求时，也应返回未配置，不能据此认定最低硕士。
- 大专及以下 是合并展示档，不代表高中、中专、大专是同一种学历。
这样首期保持两份简单映射即可。原文证据和解析版本继续由现有公共机制保存，不需要把复杂条件组塞进这两个结果中。 对“本科3年／硕士2年”等条件分支，可以按约定简化为最低学历本科、base_year 为3；但不能用这两个简化值声称它们适用于所有候选人。

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
