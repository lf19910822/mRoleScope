# 技术关键词别名映射 v0.4

## 发布身份

- 版本：`technology-keyword-alias-map-v0.4`
- 匹配策略：`technology-mentions-v4`
- 来源：技术关键词候选审计文档（2026-11-13）
- 规则默认采用精确词面、显式边界和逐别名大小写策略。
- 已发布版本不可原地修改；v0.3 只保留为历史审计事实，不作为 v0.4 的 Serving 回退。

## Unreal 与 Unity

| 词面 | 正式身份 | 说明 |
| --- | --- | --- |
| `Unreal`、`unreal`、`Unreal Engine`、`UE` | `Unreal Engine` | 直接命中；`UE` 使用标识符边界 |
| `UE4` | `Unreal Engine`、`Unreal Engine 4` | 两个身份共享同一正文区间 |
| `UE5`、`UE5.4` | `Unreal Engine`、`Unreal Engine 5` | 不创建 `Unreal Engine 5.4` |
| `Unity`、`unity`、`Unity 3D`、`Unity3D` | `Unity` | 所有词面归一为一个身份 |

## React 与 ReAct

| 词面 | 正式身份 | 大小写策略 |
| --- | --- | --- |
| `React` | `React` | 精确大小写 |
| `react` | `React` | 精确小写 |
| `ReAct` | `ReAct` | 精确大小写，独立于前端 React |
| `REACT` | 无自动命中 | 未登记变体不推断 |

## Unreal 子系统

以下身份分别统计，不自动合并为 `Unreal Engine`：

`UObject`、`World Partition`、`WorldPartition`、`StateTree`、`MassEntity`、`GameThread`、`BuildGraph`

其中 `World Partition` 与 `WorldPartition` 是同一身份的两个登记词面。

## 游戏工具

| 词面 | 正式身份 |
| --- | --- |
| `Wwise` | `Wwise` |
| `FMOD`、`FMOD Studio` | `FMOD` |
| `CRIWARE`、`Criware` | `CRIWARE` |
| `Maya` | `Maya` |

## GPU 与性能

正式身份：`DCGM`、`Nsight Systems`、`Nsight Compute`、`NVSwitch`、`CANN`、`NUMA`、`PCIe`、`RoCE`。

`ROCE` 是 `RoCE` 的登记别名。`RoCEv2`、`NVIDIA`、`GPU`、`CUDA`、`NVLink` 和裸 `Nsight` 不由这些词面自动推导。

## Kubernetes 组件

正式身份：`kubelet`、`kube-scheduler`、`controller-manager`、`KubeFed`、`controller-runtime`、`client-go`、`KEDA`。

组件不会自动推导 `Kubernetes`、`Kubebuilder` 或 `Go`；只出现 `Kubernetes` 也不会自动增加任何组件身份。

## 基础设施

正式身份：`Elasticsearch`、`Ceph`、`OpenStack`、`QEMU`、`libvirt`、`cloud-init`。

这些词面不会自动推导 `Elastic`、`ELK`、`Kibana`、`KVM` 或 `Kubernetes`。

## 直接命中与排除

已登记且批准的技术词默认直接命中，不要求附近再出现“游戏”“引擎”“前端”等陪跑词。排除仅使用版本化、固定字符串和明确来源的局部模式：

| 排除模式 | 作用 | 局部策略 |
| --- | --- | --- |
| `react to` | 抑制小写 `react` 的普通动词短语 | 同一正文段，最多 96 个 Unicode code point |
| `user experience` | 抑制裸 `UE` 的明确普通语境 | 同一正文段，最多 96 个 Unicode code point |
| `用户体验` | 抑制裸 `UE` 的明确普通语境 | 同一正文段，最多 96 个 Unicode code point |

强分隔符为 `。`、`！`、`？`、`；`、`.`、`!`、`?`、`;`。没有登记排除模式时，不因缺少正向陪跑词而抑制命中，也不使用模糊匹配或整份 JD 的远处内容推断。

## 正文范围与证据

分析只读取规范化后的 `description`、`jobRequire` 和 `addition`。每个正式身份均保留真实正文区间、来源规则和规范化字段指纹；父子身份展开共享原始区间，不生成虚假正文证据。

`title`、摘要、来源标签、未登记缩写、普通英文词和候选词不会成为正式技术结果。
