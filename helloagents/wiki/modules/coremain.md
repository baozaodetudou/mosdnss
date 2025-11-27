# coremain 模块

## 目的
提供 CLI、配置加载、服务生命周期管理与系统 API。

## 模块概述
- **职责:** 解析命令行、读取配置、初始化插件、管理服务与系统信号。
- **状态:** ✅ 稳定
- **最后更新:** 2025-11-27

## 规范
### 需求: CLI 启动
**模块:** coremain
- `mosdns start [-c|-d]` 负责设置工作目录、加载配置、设定 GOMAXPROCS。
- `NewServer` 在解析配置后设置 `MainConfigBaseDir` 并初始化审计。

#### 场景: start --dir 指定 demo 目录
- 当 `--dir` 提供路径时，`os.Chdir` 切换工作目录再加载 `config.yaml`。
- `cfg.baseDir` 和全局 `MainConfigBaseDir` 会成为 include、dump 的相对根。

### 需求: HTTP API 服务
**模块:** coremain
- `api.go` 中注册 `/api/v1/*`，端口通过 `api.http` 配置。
- 审计 (`api_audit_v2.go`) 依赖 `enable_audit`。

## API 接口
- `AddSubCmd`：允许其他包（如 tools）注册 CLI 子命令。
- `coremain.Run`: 执行 rootCmd。

## 数据模型
- `serverFlags` 包含 `config`, `dir`, `cpu`, `asService`。

## 依赖
- `cobra`, `viper`, `zap`, `kardianos/service`。

## 变更历史
- 202511270000_init_kb: 初始化知识库记录。
