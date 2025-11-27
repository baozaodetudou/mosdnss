# 项目技术约定

## 技术栈
- **核心语言:** Go 1.25.4
- **主要框架:** Cobra CLI、Viper 配置、zap 日志、miekg/dns、quic-go
- **可选组件:** Prometheus 指标、kardianos/service 服务管理

## 开发约定
- **模块划分:** `coremain` 提供 CLI 与生命周期管理；`plugin` 暴露可插拔组件；`pkg` 存放工具与共享逻辑；`demo` 目录提供典型配置。
- **命名规范:** Go 模块使用驼峰命名，YAML 插件 tag 以 `sequence_`/`forward_`/`switch` 等前缀区分类别。
- **配置组织:** 主配置 `config.yaml` 仅负责 include 顺序，其余逻辑拆分在 `sub_config/` 中便于复用。

## 错误与日志
- **日志级别:** 默认 warn，可通过 `log.level` 调整；文件输出由 `log.file` 指定。
- **错误处理:** CLI 入口捕获 fatal 并写入 zap；插件序列通过 `mark` 与 `fallback` 控制分支以避免 panic。

## 测试与流程
- **测试:** 推荐使用 `go test ./...`，并为自定义插件或序列编写集成测试。
- **提交规范:** 采用 Conventional Commits，例如 `docs: add mosdns1 flow doc`、`feat: support ecs upstream`。
