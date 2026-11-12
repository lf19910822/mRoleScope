# JD 浏览器安全发布运行指导

> 状态：邀请码门禁代码与部署资产已实现；**未完成 DNS、TLS、CVM 安全组和最终发布批准前，Web Edge 必须保持关闭。** 本文不记录域名账户、付款、实名材料、TLS 私钥或真实邀请码。

## 交付边界

发布链现在固定构建并以不可变 digest 校验四个镜像：`mrolescope-api`、`mrolescope-migrate`、`mrolescope-worker` 和 `mrolescope-web-edge`。Web Edge 编译 Vue 产物，终止 TLS，并将同源 `/api/` 转发到 Docker `edge` 网络中的 Go API。

PostgreSQL 仅在 `backend` 内部网络；API 不再发布 `8080` 到宿主机。Web Edge 是唯一配置了宿主机端口的服务，且其 `web` Compose profile 默认不启动。因此，代码合入后的日常 API、同步、确定性解析和历史数据不会因为尚未具备域名而中断，也不会意外占用公网 `80/443`。

## 人工发布门槛

在把 `MROLESCOPE_WEB_EDGE_ENABLED` 改为 `true` 之前，维护者必须完成并留存以下**非敏感完成事实**：

1. 已取得批准的域名，完成注册商的所有权／实名步骤，且已开启账户 MFA、自动续费和转移锁。
2. 已确认并完成该中国大陆 CVM 的腾讯云、接入商和监管要求；适用时包括 ICP 备案。只记录“已确认／批准编号或引用”，不要将身份证明、支付信息或注册商密码放进仓库、CI、服务器配置或聊天记录。
3. 已准备通过 Let's Encrypt HTTP-01 获取的受信任 TLS 证书链和私钥；首次签发使用 root helper `/usr/local/sbin/mrolescope-cert-issue`，续期使用 `/usr/local/sbin/mrolescope-cert-renew`。最终文件由 root 分别保存在 `/etc/mrolescope/tls/fullchain.pem` 与 `/etc/mrolescope/tls/privkey.pem`；目录和私钥只能由 root 读取。
4. 已取得本次四镜像发布及批准域名的明确最终批准。

在上述事项全部完成前：不建 DNS 记录，不开放安全组 `80/443`，不启用 `web` profile。

## 根所有运行配置

运行配置文件固定为 `/etc/mrolescope/secrets/remote-development.env`，必须为 `root:root` 与 `0600`。它已有数据库和对象存储配置；API 额外读取邀请码哈希和会话签名，Web Edge 只读取批准主机名：

```dotenv
# 人工门槛未完成时保持 false
MROLESCOPE_WEB_EDGE_ENABLED=false

# 仅在门槛完成、准备发布时填写实际值；不要提交示例中的值。
MROLESCOPE_WEB_HOSTNAME=approved.example
MROLESCOPE_INVITE_BCRYPT_HASH='$2b$...bcrypt-hash-only...'
MROLESCOPE_SESSION_SIGNING_SECRET='...at-least-32-byte-hex...'
```

`MROLESCOPE_INVITE_BCRYPT_HASH` 只能是邀请码的 bcrypt 哈希，`MROLESCOPE_SESSION_SIGNING_SECRET` 只能是 root 运行配置中的随机十六进制材料，二者不能写入 Web Edge 环境。镜像层、Vue 静态资源、日志和发布记录都不保存邀请码明文。未知 Host 的 TLS 握手被拒绝，HTTP 只允许 ACME challenge 或跳转，未验证 HTTPS 根路径只重定向到 `/access`，业务 `/api/` 返回非数据 `401`。验证成功后由 Go API 设置 3 天安全 Cookie。

## 发布、验证与回滚

1. 从受保护 `main` 使用自主镜像发布链。根部署助手只接受已配置 Docker Hub 命名空间内的四个 digest，拒绝可变 tag、外部命名空间或少传 Web Edge digest；四者必须具有相同的源码 revision 标签。
2. 第一次使用四镜像链前，root 必须先以同一已审核 commit 重新运行 bootstrap，更新 CVM 的 root 部署助手和 Compose 文件；GitHub runner 无权自行替换这些 root 资产。过渡期间旧的三镜像发布记录仍可驱动既有同步／解析任务：Web Edge 解析为不可运行的占位镜像，且 `web` profile 从不被这些任务选择。
3. 若 `MROLESCOPE_WEB_EDGE_ENABLED=false`，部署继续发布 API、迁移与 worker，但会保持 Web Edge 停止。这是当前正常状态。
4. 人工门槛完成后，先使用 `/usr/local/sbin/mrolescope-cert-issue` 完成首次 HTTP-01 签发；证书成功写入并通过权限检查后，再开启 Web Edge。助手先验证数据库、迁移和 API 健康，再启动 Web Edge；其本机未验证 HTTPS 根路径必须得到 `302`，业务 API 必须得到 `401`。新 Edge 未通过时，助手只回滚 Edge 到上一已验证 digest，不回滚 API、worker、同步、不可变历史或确定性解析。
5. DNS 已指向 CVM 且证书就绪后，维护者在受控终端执行 `bash scripts/check-web-edge-smoke.sh <批准域名>`。该脚本交互读取邀请码但不接受命令行明文参数，验证可信证书、HTTP 跳转、匿名 `/access`、错误/正确邀请码、Cookie、业务列表与详情、同源 API 和裸 IP 拒绝；不打印 JD 正文或邀请码。
6. 外部验证通过后，才由维护者将批准域名 DNS 指向 CVM，并仅开放 Web Edge 的 `80/443`。任何失败都先撤 DNS 或停 Edge；不要回滚数据库迁移。

Web Edge 自动失败回滚独立于后端。若需要人工撤回访问面，先停止/禁用 Edge 或撤 DNS；API、worker、每日同步和数据处理仍应持续运行。恢复到先前 Edge digest 必须通过受保护发布链和发布记录完成，不能在 CVM 手工拉取 `latest` 或任意镜像。

## Let's Encrypt 运维

- Let's Encrypt 证书本身免费，但域名解析、CVM 带宽、安全组和适用备案要求仍由维护者负责；HTTP-01 需要公网 TCP 80。
- bootstrap 不会自行执行系统包安装；在 CVM 上先按 Ubuntu 官方/腾讯云软件源安装 `certbot`，再运行受审核 commit 的 bootstrap。
- 首次签发前不得让 Web Edge 监听业务端口；`mrolescope-cert-issue` 会停止 Web Edge，并通过 Certbot standalone 临时占用 80。签发成功后才将证书安装到 `/etc/mrolescope/tls`。
- 后续由 `mrolescope-cert-renew.timer` 每日触发 webroot 续期检查。成功续期后先验证证书，再原子替换并 reload Web Edge；验证失败恢复上一套证书。
- 通过 `systemctl status mrolescope-cert-renew.timer`、`journalctl -u mrolescope-cert-renew.service` 和 `/usr/local/sbin/mrolescope-cert-renew check` 检查续期状态；输出不得包含邀请码、会话签名、Cookie 或 JD 正文。
- 紧急撤回时执行 `docker compose --profile web stop web-edge` 或移除唯一 DNS `A` 记录；不要删除 TLS 目录、数据库卷或手工修改迁移状态。
