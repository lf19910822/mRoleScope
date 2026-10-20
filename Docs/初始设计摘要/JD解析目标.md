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