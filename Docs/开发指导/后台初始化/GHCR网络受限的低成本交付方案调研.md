# GHCR 网络受限的低成本交付方案调研

> 调研日期：2026-10-21。目标是让现有上海 CVM 可以稳定运行开发演示环境；不购买 TCR、不新增 CVM，也不把数据库或 API 暴露到公网。本文件是选型记录，不授权修改 CVM、云资源或部署代码。

## 已确认的事实

- CVM 已通过腾讯云官方 Docker Hub 镜像加速器成功取得 PostgreSQL；但两次部署在 GHCR 内容分发端 `pkg-containers.githubusercontent.com` 的 TLS/读取阶段超时。将 Docker 下载并发数降为 `1` 后仍失败。因此问题不是数据库密码、Compose、镜像 digest 或 Docker Hub。
- Docker 官方明确指出 registry mirror 是 **Docker Hub** 的 pull-through cache，不能镜像另一私有 registry；它不会自动代理 `ghcr.io`。所以腾讯镜像加速器应保留，但不能修好 GHCR。[Docker 官方说明](https://docs.docker.com/docker-hub/image-library/mirror/)、[腾讯云配置说明](https://cloud.tencent.com/document/product/457/9113)。
- 本项目 GitHub 仓库已经是公开仓库：[lf19910822/mRoleScope](https://github.com/lf19910822/mRoleScope)。当前 API 最终镜像仅放入编译后的二进制，迁移镜像仅放迁移文件；数据库密码仍只存在 CVM 的 root-only env 文件，绝不能进入镜像。

## 推荐顺序

| 优先级 | 路径 | 成本与可靠性 | 结论 |
|---|---|---|---|
| 1 | **公开 Docker Hub 运行镜像 + 已配置的腾讯 Docker Hub 加速器** | 不买 TCR、不换 CVM；需要一个 Docker Hub 账号和可撤销写入 Token。CVM 路径已由 PostgreSQL 验证。 | **建议采用**，保留自动部署和 digest 固定。 |
| 2 | GHCR 拉取作有限次数、退避重试 | 零额外成本，但只是赌间歇性网络恢复。 | 可作为短期诊断，不能作为稳定交付承诺。 |
| 3 | GitHub Actions Artifact 传递镜像归档 | 无 registry 实例费，但 CVM 仍需下载 GitHub/Azure 的 artifact；需要先实测网络。 | Docker Hub 公共镜像不能接受时的自动化备选。 |
| 4 | 个人电脑中转镜像归档 | 当前个人电脑 → CVM 的 SCP 已成功传过 216 MiB runner 包，成功概率最高；但每次发布需人工操作且暂时保留受限 SSH。 | 可作为紧急演示兜底，不是长期流程。 |
| 5 | 受控 HTTPS 出站代理 | 只在维护者**已有、信任且自管**的代理时可评估；否则会新增成本、认证信息和运维面。 | 不为此购买未知代理。 |

## 首选方案：运行时改用公开 Docker Hub

把 GitHub Actions 的构建产物推到 `docker.io/<维护者账号>/mrolescope-api` 与 `docker.io/<维护者账号>/mrolescope-migrate` 两个**公开**仓库；CVM 仍按形如 `docker.io/<账号>/mrolescope-api@sha256:<digest>` 的不可变 digest 拉取。腾讯镜像加速器会处理 Docker Hub 路径，API 和迁移镜像不再从 GHCR blob CDN 取得。

这不是购买另一台服务器或 TCR。需要的仅是 Docker Hub 账号；实际是否有免费配额、拉取限制或价格变动，必须由维护者在创建前以 Docker Hub 当前页面和条款为准，本文不作“永久免费”承诺。

实施时必须保留以下安全边界：

1. Docker Hub Token 仅授予推送所需权限，作为 GitHub Actions Secret 保存；不得写入 Git、CVM 或日志。Docker 对 access token 的官方说明见[此处](https://docs.docker.com/security/for-developers/access-tokens/)，登录 Action 见[此处](https://github.com/docker/login-action#docker-hub)。
2. root 部署入口的白名单从 `ghcr.io/lf19910822/...` 改为**精确的 Docker Hub 命名空间 + 两个仓库 + `sha256:<64 位>`**；不得放宽为任意 Docker Hub 镜像或 tag。
3. 工作流仍从受保护 `main` 构建，仍校验 API 和迁移镜像的同一 Git revision label，仍执行本机健康检查、发布记录和 API 回滚。公开仓库不使用 `latest`。
4. 数据库密码、COS/CAM 凭据、Token、内部地址均不得进入 Docker build context、Dockerfile `ARG`/`ENV` 或镜像层。若未来仓库、代码或镜像需保密，此方案必须重新评审。
5. 先发布一个无业务数据的小测试镜像，**由现有 CVM 经腾讯 mirror 拉取**，确认自定义公开 Docker Hub 命名空间也通畅，再迁移正式 workflow；Docker 官方对 Hub mirror 的范围说明并不等价于腾讯对每个账号仓库的 SLA。

Docker 按 digest 拉取的含义和 Container registry 参考见 [Docker pull 文档](https://docs.docker.com/reference/cli/docker/image/pull/) 与 [GitHub Container Registry 文档](https://docs.github.com/en/packages/working-with-a-github-packages-registry/working-with-the-container-registry)。换仓库不应改变“不可变版本”要求。

## 其它可行路径及边界

### 有限重试：可试，不可依赖

一次 `compose pull` 失败可在 root 受控入口内做 3–5 次有上限的指数退避重试，保留已成功的层，且不要在失败时执行 `docker system prune`、删除卷或重启 Docker。社区中有与本次报错相同的腾讯云 `pkg-containers.githubusercontent.com` TLS timeout 记录；维护者将其作为间歇网络问题并使用多次重试：[pintuotuo PR #448](https://github.com/sev7n4/pintuotuo/pull/448)。这是单个项目维护者的经验，不是 GitHub/Tencent 的服务保证，故只能作为低成本尝试。

### Actions Artifact：先测通，再决定是否实现

GitHub Actions 可将构建 job 的文件上传为 artifact，后续 self-hosted deploy job 下载。Docker 官方支持 `docker image save` 导出 tar、`docker image load` 从 tar 恢复镜像和 tag：[save](https://docs.docker.com/reference/cli/docker/image/save/)、[load](https://docs.docker.com/reference/cli/docker/image/load/)。GitHub 官方也确认 artifact 用于 workflow job 间传递文件：[文档](https://docs.github.com/en/actions/tutorials/store-and-share-data)。

合理流程是：GitHub 托管构建机构建两个镜像 → 导出 x86_64 归档与 SHA-256 manifest → 上传 artifact → CVM runner 先下载一个 5–10 MiB 的测试 artifact → 确认连续下载可用后，再设计 root helper 校验并 `docker load`，Compose 使用 `--pull never`。

它**不保证**绕开网络问题：artifact 仍从 GitHub/Azure 的下载端点进入 CVM，只是与 GHCR blob CDN 不是同一条已失败路径。更重要的是，不能为了图省事让 self-hosted runner 把任意 tar 交给 root `docker load`；这会把现有“只允许两个 registry digest”的边界扩大成“runner 可提供任意可执行镜像”。若实施自动化导入，应单独设计由受保护构建 job 产生、root 可验证的签名/哈希绑定；在此之前，Artifact 只做连通性试验。

### 个人电脑中转：已证实可用的临时兜底

构建机导出归档和 SHA-256 文件后，由维护者在个人电脑下载，再经来源限定的临时 SSH/SCP 上传 CVM；维护者以 root 比对 SHA-256 后 `docker load`。这避开 CVM 到 GHCR 的失败路径，也不需要 TCR。代价是每次发布人工参与，并且在关闭 22 入站规则后须使用腾讯云受控运维通道或重新临时限定开放。它只适合尽快演示，不能假装成无人值守发布。

### 不建议的做法

- **随机 GHCR 镜像站/加速 URL、免费 HTTP 代理或 VPN**：无法审计运营方、可用性与凭据处理；若把 GHCR Token 发给它们，风险更高。不要把它们写入 Docker daemon 或 workflow。
- **把 `registry-mirrors` 当 GHCR 加速器**：Docker 官方定义的范围仅为 Docker Hub，已被本次现象验证无效。
- **让 CVM runner checkout 源码并以 root 临时构建**：会扩大 root 信任边界，且还会暴露 Go modules、构建基础镜像和 `gcr.io` 的跨境网络依赖；2C2G 主机也不适合作为常规构建机。
- **GitHub Release 作为 CVM 直接下载源**：此前 CVM 下载 GitHub Releases 已出现 EOF；它不能证明比 GHCR 更可靠。

## 建议的决策

选择“公开 Docker Hub + 腾讯镜像加速器”作为正式开发演示路径。它以一个公开、可撤销账号 Token 换取自动化可用性；本项目代码本已公开，且运行时密钥不在镜像中，风险增量可接受。若维护者不同意公开镜像，则先做小 Artifact 下载测试；测试失败时以个人电脑中转作为临时发布方案，而不是购买昂贵 TCR 或接入未知镜像站。

## 资料与案例

- Docker，[*Mirror the Docker Hub library*](https://docs.docker.com/docker-hub/image-library/mirror/)：mirror 是 Docker Hub pull-through cache，不能镜像另一 private registry。
- 腾讯云，[配置镜像加速器](https://cloud.tencent.com/document/product/457/9113)：腾讯官方 Docker Hub 加速器配置。
- Docker，[`docker image save`](https://docs.docker.com/reference/cli/docker/image/save/) 与 [`docker image load`](https://docs.docker.com/reference/cli/docker/image/load/)：归档传递的官方命令语义。
- GitHub Docs，[Store and share data with workflow artifacts](https://docs.github.com/en/actions/tutorials/store-and-share-data)：artifact 跨 job 数据传递。
- Docker，[Daemon proxy configuration](https://docs.docker.com/engine/daemon/proxy/)：只有已有可信代理时，Docker daemon 可配置 HTTPS proxy；这不构成推荐第三方代理的理由。
- 社区案例：[sev7n4/pintuotuo#448](https://github.com/sev7n4/pintuotuo/pull/448)：项目维护者记录腾讯云到 `pkg-containers.githubusercontent.com` 的 TLS timeout，并采用限次重试；仅作为经验参照。
