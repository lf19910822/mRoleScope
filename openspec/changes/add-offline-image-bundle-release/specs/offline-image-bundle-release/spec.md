# Spec Delta

## Purpose

为 Docker Hub 不可达时提供一个不依赖镜像仓库的受控发布传输能力，同时保持镜像不可变、部署可审计、失败不切换和既有回滚语义。

## ADDED Requirements

### Requirement: Bundle contains one reviewed release
离线发布包 SHALL 包含 API、migration、worker、Web Edge 和 pinned PostgreSQL 五个 `linux/amd64` 镜像、同一 Git revision、版本化清单和每个文件的 SHA-256 校验值。

#### Scenario: Complete bundle is accepted
- **WHEN** the bundle contains all five required images and a manifest whose revision and checksums match the archive files
- **THEN** the deployment boundary accepts the bundle for validation and does not contact Docker Hub

#### Scenario: Missing image is rejected
- **WHEN** any required image is absent from the manifest or archive
- **THEN** the deployment boundary rejects the bundle before loading or changing running services

### Requirement: Bundle integrity is verified before activation
部署边界 MUST 在加载镜像和执行迁移前验证包文件、清单、镜像架构及 OCI source revision；任一验证失败都不得启动新版本。

#### Scenario: Tampered archive is rejected
- **WHEN** an archive checksum differs from the manifest
- **THEN** the deployment records a safe failure and leaves the current release active

#### Scenario: Mixed revision is rejected
- **WHEN** one image has a different OCI source revision from the manifest revision
- **THEN** the deployment rejects the bundle without partially activating any image

### Requirement: Offline activation preserves controlled lifecycle
离线包发布 SHALL 复用现有迁移、健康检查、发布记录、失败回滚和两代镜像保留规则，并以不可变 bundle revision 作为发布身份。

#### Scenario: Valid bundle is deployed
- **WHEN** all bundle checks pass and the controlled deployment operation runs
- **THEN** it loads the five images, runs the migration and health checks, records one successful release, and retains rollback images

#### Scenario: Health check fails
- **WHEN** a loaded bundle fails migration or health checks
- **THEN** the deployment rolls back according to the existing policy and records a failed release without advancing the active service

### Requirement: Docker Hub release remains compatible
新增离线传输模式 MUST 是显式选择的替代路径；默认 Docker Hub 发布行为、镜像命名和现有 `--dry-run` 语义不得改变。

#### Scenario: Default release uses Docker Hub
- **WHEN** the operator runs the existing default deployment command without offline-bundle mode
- **THEN** the release follows the current local build, Docker Hub push and digest deployment path

#### Scenario: Offline mode bypasses Docker Hub
- **WHEN** the operator explicitly selects offline-bundle mode with a valid bundle
- **THEN** the release transfers and loads the bundle over the controlled SSH channel and does not push or pull application images through Docker Hub

### Requirement: Bundle staging is bounded and auditable
离线包及其临时解压内容 SHALL 仅存放在受控 staging 目录，部署结束后按成功/失败策略清理；日志只能输出 revision、校验结果和安全计数，不得输出凭据或运行时连接串。

#### Scenario: Successful staging is cleaned
- **WHEN** an offline release completes successfully
- **THEN** temporary bundle files are removed while the immutable release record remains

#### Scenario: Failed staging is retained safely
- **WHEN** validation or activation fails before cleanup
- **THEN** only the bounded failed-release diagnostic record may remain and no secret-bearing file is exposed
