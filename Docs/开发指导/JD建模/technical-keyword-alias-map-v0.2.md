# 技术关键词归一化映射表 v0.2

状态：**待受控导入、逐条批准和候选分析**。本文件是 v0.2 的不可变词典来源说明；不含真实 JD、岗位信息、命中片段、原始响应或凭据。

## 版本边界

- v0.2 以 [v0.1 映射表](technical-keyword-alias-map-v0.1.md) 为基线，以 [v0.1 人工审阅表](技术词典v0.1人工审阅表.md) 的已完成决定为准。
- 已部署的 v0.1 及其候选分析记录不修改、不覆盖，也不发布为 v0.2 结果。
- 解析器仍为 `technology-mentions-v1`；仅扫描 `description`、`jobRequire`、`addition`。
- 每个规则保持独立 `rule_key`，每条证据仍保存实际字段、规范化文本版本与 Unicode 码点区间。归一化只统一技术身份，不改写原文或坐标。

## v0.2 匹配政策

### A. 已审核的别名组

以下词面作为**候选精确规则**导入；只有经受控逐条批准后才可产生技术提及。除另有说明外，A 组使用大小写敏感匹配：只接受列出的词面，不因相似拼写或任意大小写组合扩展范围。

| 技术身份 | 可批准的精确词面 | 保留约束 |
| --- | --- | --- |
| Kubernetes | `Kubernetes`、`K8s`、`K8S` | 不从 Kubelet、KubeFed 等组件名称推导。 |
| Python | `Python`、`python` | 不由 PyTorch、PySpark 等名称推导。 |
| gRPC | `gRPC`、`grpc` | 与 RPC 保持不同身份。 |
| SGLang | `SGLang`、`sglang`、`SGlang` | 只接受列出的词面。 |
| vLLM | `vLLM`、`vllm` | 不扩大到相关工具或子串。 |
| TensorRT-LLM | `TensorRT-LLM`、`TRT-LLM` | 不把 TensorRT 归为 TensorRT-LLM；未登记连字符变体不自动接受。 |
| PyTorch | `PyTorch`、`Pytorch`、`Pytouch` | `Pytouch` 是项目负责人逐项确认的精确例外，不建立通用拼写纠错。 |
| TensorFlow | `TensorFlow`、`Tensorflow` | `TF` 仍为候选。 |
| AIOps | `AIOps`、`AIOPS` | 不与泛称 AI 合并。 |
| AIBrix | `aibrix`、`AIBrix` | 不推导内部依赖技术。 |
| Kubebuilder | `kubebuilder`、`Kubebuilder` | 与 controller-runtime 分开。 |
| etcd | `etcd`、`ETCD` | 独立技术身份。 |
| JuiceFS | `JuiceFS`、`Juicefs` | 独立技术身份。 |
| BlendShape | `BlendShape`、`Blendshape` | 不与模型或具体软件混同。 |

`Go`/`Golang`、`Unreal`/`unreal`、`Unity`/`unity`、`Shell`/`SHELL`、`verl`/`VeRL`/`VERL` 均保持条件规则；本版本不将它们列为可批准的首期精确规则。

### B. 已审核的完整技术名称

人工审阅表 B01–B15 中列出的全部名称均可批准。它们使用**ASCII 大小写不敏感 + 完整名称 + ASCII 标识符边界**的匹配方式：例如 `Docker`、`docker`、`DOCKER` 归为同一 Docker 身份，而 `Dockerfile` 不命中。该规则不接受未登记缩写、拼写相近词或子串推导。

技术间的分离约束沿用 v0.1：例如 Java/JavaScript/TypeScript、C/C++/C#、React/ReAct、TensorRT/TensorRT-LLM、RoCE/RoCEv2、Git/GitLab/GitHub/GitOps、HTTP/HTTPS、RPC/gRPC、OpenAI/OpenAPI 均不能合并或相互推导。

### C. 保持条件或候选的项

v0.1 的候选/条件项默认继续保留且不能由 v0.2 批准入口启用，包括 `UE`、UE4/UE5/UE5.4、C++17、JS、TF、HF、Transformers、Triton、Argo、Megatron、Inspect、KVCache、RoCE、ReAct、MCP、Skills、Harness、SIGGRAPGH、Cublas、Cutlass 等。它们需要新的、独立的语义或版本建模设计。

## 受控生命周期

1. 使用固定的 `seed-v0.2-dictionary` 导入全部候选/条件规则。
2. 使用固定的 `approve-v0.2-rule` 逐条批准；仓储层仅接受本文件 A、B 组对应的固定白名单规则键。
3. 使用 `prepare-v0.2-analysis` 冻结已启用规则集并创建新的候选配置。
4. 使用隔离的 `run-v0.2-candidate-analysis` 执行候选分析。该操作只访问远端 PostgreSQL 的规范化正文，不访问来源网站、COS 或本地真实 JD。

候选结果仍未发布；发布、回退及真正的远端人工文本复核门禁尚未实现，不能绕过。
