# 技术关键词归一化映射表 v0.1

日期：2026-10-22。状态：供项目负责人审核的建议词典，不表示已完成人工批准或已写入生产规则。

原则：仅识别明确登记的名称和别名，不做编辑距离、前缀、子串或向量相似度合并。人工批准后才能启用对应条目。保留原始写法、规范名称、规则ID、证据坐标和词典版本。

## 数据范围

本版结合2026-10-22公开接口当前批次扫描与此前已核验样本整理。扫描范围仅为description、jobRequire、addition，不含title、jobSummary、tagList。当前批次完整性结果见文末。

字符串候选盘点只用于发现拼写，不是最终技能统计；长词、复合路径、版本号和英文普通词需要再次判断。本文没有声称候选列表覆盖全部技术，也没有把未登录的来源标签直接计入统计。

## A. 建议批准的名称归一化组

下表列出样本中观察到的写法；逗号仅用于分隔不同词面，不是匹配表达式。“精确词面”仍需正确的边界与语境判断。

| 规则ID | 标准名称 | 可登记的已观察写法 | 条件或排除项 |
|---|---|---|---|
| alias.go | Go | Go、Golang | Go需处于编程语言语境；不匹配英文普通动词，不从go-zero等复合标识内部取词 |
| alias.kubernetes | Kubernetes | Kubernetes、K8s、K8S | 登记这些词面；不匹配KubeFed或Kubelet等其他组件 |
| alias.python | Python | Python、python | 不由PyTorch、PySpark等名称推导Python |
| alias.grpc | gRPC | gRPC、grpc | 与RPC分别保存 |
| alias.sglang | SGLang | SGLang、sglang、SGlang | 仅这些名称变体 |
| alias.vllm | vLLM | vLLM、vllm | 与vLLM bench等工具关系另处理，不凭子串命中 |
| alias.tensorrt_llm | TensorRT-LLM | TensorRT-LLM、TRT-LLM | 不包含TensorRT；连字符样式变化另显式登记 |
| alias.pytorch | PyTorch | PyTorch、Pytorch | Pytouch是疑似拼写错误，暂不收录 |
| alias.tensorflow | TensorFlow | Tensorflow | TF缩写待复核，不无条件映射 |
| alias.unreal | Unreal Engine | Unreal、unreal | UE单独作为有语境条件的候选；UE4/UE5保留版本，不作为无版本别名直接丢失版本 |
| alias.unity | Unity | Unity、unity | 需游戏引擎语境，不采用全文任意英语词替换 |
| alias.shell | Shell | Shell、SHELL | 表示脚本/命令解释环境，不等同Bash |
| alias.aiops | AIOps | AIOps、AIOPS | 运维概念，不与AI整体合并 |
| alias.aibrix | AIBrix | aibrix、AIBrix | 不推导其内部依赖技能 |
| alias.kubebuilder | Kubebuilder | kubebuilder、Kubebuilder | 与controller-runtime分别保存 |
| alias.verl | verl | verl、VeRL、VERL | 项目名明确的后训练语境；不是普通词或其他项目 |
| alias.etcd | etcd | etcd、ETCD | 独立名称 |
| alias.juicefs | JuiceFS | JuiceFS、Juicefs | 独立名称 |
| alias.blendshape | BlendShape | BlendShape、Blendshape | 方法概念；与模型或具体软件区分 |

Go/Golang、TRT-LLM/TensorRT-LLM的对应关系可在官方文档核实；其余条目仍应结合远端原文进行审核。大小写归一化必须是每条词典规则显式允许的行为，不应对整个词典统一case-fold后匹配。

## B. 建议按原名独立登记的首批关键词

以下通常不需要“合并”，保留规范名称本身即可；新词面与别名仍走候选审核。此表是可选种子集合，不要求首轮一次性启用全部条目。

| 类型 | 独立标准名称 |
|---|---|
| 编程语言 | C++、C#、Java、JavaScript、TypeScript、Rust、Lua |
| 前端框架 | Vue、React |
| 图形API与着色语言 | Vulkan、Metal、DX11、DX12、HLSL、Shader |
| 数据库与存储 | MySQL、PostgreSQL、Redis、MongoDB、ClickHouse、StarRocks、TiDB、TiKV、Milvus、Alluxio |
| 数据计算与队列 | Kafka、Flink、Spark、Hive、Hadoop、PySpark |
| 容器与云原生组件 | Docker、containerd、Helm、Kustomize、Volcano、Kueue、Koordinator、YuniKorn、Karmada、Karpenter、KServe、Knative、Calico、Cilium |
| 网关与服务治理 | Envoy、Higress、Kong、APISIX、Traefik、Istio、Linkerd、Nginx、OpenResty、HAProxy |
| 可观测性 | Prometheus、Grafana、OpenTelemetry、Jaeger、Loki、Thanos、VictoriaMetrics、Zabbix、Nightingale |
| 推理、模型与训练工具 | TensorRT、ONNX、LMDeploy、TGI、DeepSpeed、Megatron-LM、TRL、OpenRLHF、LLaMA-Factory、ComfyUI |
| 评测工具与基准 | EvalScope、Inspect AI、Langfuse、Promptfoo、OpenCompass、lm-eval-harness、SWE-bench、Terminal-Bench、GAIA |
| GPU与并行计算 | CUDA、cuDNN、NCCL、ROCm、NVLink、RDMA、InfiniBand、RoCEv2 |
| 工程与交付工具 | Git、Perforce、SVN、Jenkins、Teamcity、GitLab CI、Airflow、ArgoCD、Terraform、Ansible |
| 安全工具 | CodeQL、Semgrep、IDA、x64dbg |
| AI方法与机制 | RAG、Embedding、SFT、RLHF、DPO、PPO、GRPO、DAPO、LoRA、MoE、PagedAttention、LLM-as-Judge |
| 协议与接口规范 | HTTP、HTTPS、TCP、UDP、WebSocket、Protobuf、OpenAPI、OAuth2、OIDC |

包含标准名称并不表示任何上下文都可自动命中。例如GAIA在此是评测基准，Shader是泛化技术概念，OpenAPI是接口规范。不同技术类型宜分开统计，避免与具体软件混为同一种度量。

## C. 需要上下文或单独建模的候选

| 观察词面/组合 | 建议处理 | 原因 |
|---|---|---|
| UE | 明确游戏引擎语境可审核映射Unreal Engine，否则候选 | 短缩写不能全局替换 |
| UE4、UE5、UE5.4等 | 引擎族=Unreal Engine；另保留版本 | 版本归属不是无损别名合并；UE4与UE5可在族级汇总但仍可区分 |
| C++17 | 语言=C++，标准版本=17 | 不抹掉具体版本要求 |
| JS | 明确语言语境后映射JavaScript | 不能把JavaScript子串识别为Java |
| TF | 暂候选 | 未确认语境前不能当TensorFlow |
| HF | 暂候选 | 可能指生态、组织或特定组件，不能直接当Transformers |
| Transformers、transformers | 需区分Hugging Face软件库与模型架构描述 | 同词不同义，精确词面仍可能误判 |
| Triton | 暂候选或语境规则 | NVIDIA推理服务器与Triton GPU编程语言不是同一技术 |
| Argo | 暂候选 | 不能无条件映射ArgoCD或Argo Workflows |
| Megatron | 暂候选 | 不自动补全为Megatron-LM |
| Inspect | 需结合AI评测语境审核为Inspect AI | 普通英文动词与项目名混杂 |
| KVCache、kvcache、KV Cache | 候选概念组，人工确认后登记明确词面 | 必须区别Prefix Cache与PagedAttention等不同机制 |
| RoCE、ROCE、RoCEv2 | 可登记基础协议与版本关系 | RoCEv2有具体版本，不应默认把所有RoCE都标成v2 |
| ReAct | AI推理/行动方法，独立于React前端框架 | 全局忽略大小写会导致错误合并 |
| MCP、Skills、Harness | 单独候选，需确定具体含义与类型 | 技术、通用概念、产品称谓可能混用 |
| Pytouch、SIGGRAPGH等疑似误拼 | 不自动纠错；逐项审核 | 疑似拼写错误不等于已确认别名；会议名也不是技术工具 |
| CUDA相关的Cublas、Cutlass等 | 逐项核验规范名称再登记 | 不因名称相似或同生态而互相合并 |

## D. 明确禁止的合并或子串推导

| 不应合并的词 | 理由 |
|---|---|
| Java / JavaScript / TypeScript | 三种不同语言；JS最多是JavaScript的条件别名 |
| C / C++ / C# | 不同语言；C#和C++中的标点属于名称本体 |
| React / ReAct | 前端框架与AI方法不同 |
| TensorRT / TensorRT-LLM | 相关但不同项目 |
| Triton Inference Server / Triton编程语言 | 同名不同技术 |
| PyTorch / PyTorchJob | 框架与Kubernetes训练工作负载不同 |
| Ray / RayJob | 平台与工作负载不同 |
| Kubernetes / Kubelet / kube-scheduler / KubeFed | 平台及不同组件；不是别名 |
| Go / go-zero / go-micro | 语言与框架分开，不从复合项目名自动补语言标签 |
| SQL / MySQL / PostgreSQL | 查询语言与数据库产品不同 |
| Git / GitHub / GitLab / GitOps / GitLab CI | 工具、平台、方法与交付功能不同 |
| OpenAI / OpenAPI / OpenAI-compatible | 组织/平台、规范与兼容性描述不同 |
| Unity / Unreal Engine | 不同引擎；可同属游戏引擎族 |
| GPU / CUDA / NVIDIA | 硬件类型、技术栈、厂商不同 |
| AI / LLM / AIGC / 游戏行为AI | 范围不同，不相互替换 |
| RPC / gRPC；HTTP / HTTPS | 相关但不是同一标签 |
| RAG / Embedding / 向量数据库 | 应用方法、表示技术和基础设施不同 |

同属一族可在聚合层汇总，但聚合关系不应伪装成别名关系。

## E. 建议匹配约束

1. 精确匹配不是任意位置的字符串contains。英文标识字符和标点要参与边界判定，中文相邻并不自动阻止技术词命中。
2. C++、C#、TensorRT-LLM等完整名称需完整识别，不能依赖通用单词边界。不要在TensorRT-LLM内部再产生TensorRT命中。
3. 最长合法名称优先处理重叠区间，但保留同句中独立出现的其他技术。C/C++、Go/Python等斜线组合可拆为候选词，关系另记录；这不等于自动解析“任选”要求。
4. 版本可单独登记；主次关系不代表自动添加隐含技能。例如只出现PyTorch，不补Python标签。
5. 本轮只记录提及，不能把熟悉程度、硬性要求、任选关系全部默认为已解析。
6. 证据使用来源字段+规范化文本版本+Unicode码点半开区间。别名归一化只改变标签值，不能修改证据文本或坐标。
7. 标准技能以稳定内部ID识别，展示名可变化；审核状态、审核人/时间、规则版本及停用记录有据可查。
8. 新技术词、未知来源标签进入候选；不自动影响已发布统计。source_tags与正文命中分开保存。

## F. 官方核验参考

- Go命名：https://go.dev/doc/faq#go_or_golang
- NVIDIA明确TensorRT-LLM (TRT-LLM)：https://docs.nvidia.com/deeplearning/triton-inference-server/user-guide/docs/getting_started/trtllm_user_guide.html
- Unreal Engine 5 (UE5)：https://www.unrealengine.com/unreal-engine-5?lang=en-US

## 当前批次扫描结果

2026-10-22本次范围内列表164个唯一岗位，详情成功164份、失败0。对三个正文区块完成词面候选盘点；候选包含普通英语词和复合名称，不作为技能统计结果。没有保存此次JD正文或命中片段。映射表还应由项目负责人在CVM中结合远端原文复核，审核通过后发布词典版本。

本表B组中的规范名称用于定义标签，例如JavaScript对应样本JS、TensorFlow对应样本Tensorflow，不表示规范名称的每种拼写都在本次样本中直接出现。候选盘点不覆盖全部中文技术概念，后续按需扩展。
