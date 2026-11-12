# Proposal

## Why

mRoleScope 需要通过现有域名向 HR、面试官和亲友提供持续的真实体验验证，但当前 Web Edge 只有浏览器原生 Basic Auth，无法提供“填写一次邀请码后进入全部功能”的产品流程。现在域名尚未配置解析，正适合在首次公网开放前完成访问门禁、HTTPS 证书续期和发布验收链路的统一设计，避免先暴露一个不完整或可绕过的入口。

## What Changes

- 新增独立的邀请码输入入口；验证成功后签发有效期为 3 天的安全登录 Cookie。
- 使用一个由维护者自主设置的共享邀请码；服务端只保存不可逆哈希，邀请码不设过期，泄露时通过整体替换哈希撤销旧邀请码。
- 对业务 SPA、业务静态资源和全部业务 `/api/` 请求执行 Web Edge 与 API 双层访问门禁；邀请码入口页和校验接口保持匿名可用。
- 保留现有同源 `/api/` 路径、CVM 私网 API、数据库私网和裸 IP 拒绝边界；不增加用户账号、分级权限或访问者身份审计。
- **BREAKING** 移除面向最终用户的浏览器原生 Basic Auth，更新 Web Edge、部署助手、容器测试和公网 smoke check 的认证验收方式。
- 以 Let's Encrypt 单域名 DV 证书替代购买腾讯云付费证书的前提，增加 root 受控的首次签发、自动续期、证书替换/reload 和到期失败发现方案。
- 固化 `mrolescope.com` 的唯一入口；不开放 `www`、裸 IP、`8080` 或 `5432`，不自动修改真实 DNS、CVM、安全组或运行时邀请码。
- 补充桌面端、390px 移动端、未认证/认证 API、Cookie 到期、邀请码轮换、证书流程和回滚验收文档与测试。

## Capabilities

### New Capabilities

- `invite-gated-domain-access`: 通过共享邀请码和 3 天登录 Cookie 保护同源业务页面与 API，同时保持一次验证解锁全部功能。
- `public-web-edge-operations`: 为批准域名提供 HTTPS Web Edge、Let's Encrypt HTTP-01 首次签发与自动续期、最小运维日志、发布前置条件和可回滚验收流程。

### Modified Capabilities

- 无。当前项目没有已同步到 `openspec/specs/` 的持久化主规格；历史变更中的 `secured-domain-access` 仅作为相关背景，不直接修改其历史 delta 文件。

## Impact

- 前端：新增不加载业务数据的访问入口，调整应用启动/路由门禁和登录失败、到期、轮换状态处理。
- API：新增邀请码验证和会话校验边界；现有岗位、历史、趋势 API 的路径与响应契约保持不变。
- Web Edge：替换 Basic Auth 方案，增加匿名入口例外、Cookie 校验、业务静态资源与 `/api/` 保护、限流和安全响应头。
- 部署：扩展 root-owned runtime 配置、Compose/部署助手、bootstrap 或 systemd 运维资产，增加证书目录、ACME challenge、续期 reload 和受控回滚。
- 测试与文档：更新 Web Edge 容器测试、部署助手测试、HTTPS smoke check、发布运行手册和开放决策记录。
- 外部系统：首次发布时需要维护者在 DNSPod 手工添加唯一 A 记录，在腾讯云安全组/防火墙仅开放 80/443，并在验收通过后手工发放邀请码；这些动作不由代码或 CI 自动执行。
