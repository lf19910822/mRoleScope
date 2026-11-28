# Spec Delta

## Purpose

为使用正式数据验证候选版本提供受控的 CVM 测试 API，同时保持正式容器和公网入口不受影响。

## ADDED Requirements

### Requirement: A test API runs independently of the formal stack
系统 SHALL 支持以一个经过 revision 校验的不可变 API 镜像启动或替换独立测试 API 容器。该操作只能管理测试 Compose 项目，不得启动、停止、替换或重建正式 PostgreSQL、迁移、worker、Web Edge 或正式 API 容器。

#### Scenario: Deploy a verified test API revision
- **WHEN** 维护者发布来自当前受保护主线 revision 的测试 API 镜像
- **THEN** CVM SHALL 仅创建或替换测试 API 容器，正式容器及其发布记录保持不变

#### Scenario: Image provenance is invalid
- **WHEN** 请求的镜像不是允许命名空间中的不可变 API digest，或其 revision 标签不匹配本次发布
- **THEN** 系统 SHALL 在启动测试容器前拒绝该请求

### Requirement: The test API shares data configuration without operating data services
测试 API SHALL 使用既有的受控数据库运行配置连接正式数据，但不得执行数据库迁移、启动数据库服务、启动同步或 worker 任务，也不得把数据库端口发布到宿主机。

#### Scenario: Start the test API
- **WHEN** 测试 API 被启动或替换
- **THEN** 系统 SHALL 只运行 API 服务，并复用既有的数据库连接配置

### Requirement: Test access remains loopback-only
测试 API SHALL 仅发布到 CVM 的 `127.0.0.1:18080`，不得监听公网接口或占用正式 API 与 Web Edge 的端口。运行手册 SHALL 说明通过 SSH 隧道和本地前端访问该端口的方法。

#### Scenario: Access through an SSH tunnel
- **WHEN** 维护者将本地端口转发到 CVM 的 `127.0.0.1:18080`
- **THEN** 测试 API SHALL 可由本地验证工具或本地前端访问

#### Scenario: Inspect host listeners
- **WHEN** 测试 API 正常运行
- **THEN** 它 SHALL 不在 CVM 公网接口上监听测试端口

### Requirement: Test API deployment has independent health and rollback behavior
部署 SHALL 在测试 API 的健康和就绪检查通过后记录当前测试 revision。若新测试容器未通过检查，系统 SHALL 恢复先前已验证的测试 API；不得回滚正式容器或数据库。

#### Scenario: New test API is unhealthy
- **WHEN** 新测试 API 未通过健康或就绪检查
- **THEN** 系统 SHALL 恢复先前已验证的测试 API（如存在）并报告部署失败
