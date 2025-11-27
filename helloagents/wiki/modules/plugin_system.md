# plugin_system 模块

## 目的
提供 mosdns 可组合插件机制，支持 sequence、fallback、forward、cache、rewrite、switch 等类型。

## 模块概述
- **职责:** 在 `plugin/` 目录注册 Go 插件类型，并在 YAML 中通过 `tag` 实例化。
- **状态:** ✅ 稳定
- **最后更新:** 2025-11-27

## 规范
### 需求: 插件注册
**模块:** plugin_system
- 每个 Go 文件调用 `coremain.RegisterPlugin`（在 `plugin` 包内部）绑定类型。
- YAML 中 `tag` 唯一，引用形式 `$tag`。

#### 场景: sequence + fallback
- `sequence` 顺序执行 `args`，支持 `matches` 条件字段。
- `fallback` 同时运行 primary/secondary，根据 threshold 与响应情况切换。

## API 接口
- `plugin.Register`（内部使用）: 绑定名字与构造器。

## 数据模型
- `Plugin` 结构包含 `Tag`, `Type`, `Args`。

## 依赖
- `pkg/expression` for matches, `pkg/dns` for message structs。

## 变更历史
- 202511270000_init_kb: 初始化知识库记录。
