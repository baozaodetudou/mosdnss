# 变更提案: mosdns1 查询流程文档

## 需求背景
`demo/mosdns1` 提供复杂的多序列 DNS 分流逻辑。当前仓库缺少按启动命令与查询视角梳理的文档，导致管理员需要阅读大量 YAML 才能理解流程。需要总结 `start --dir demo/mosdns1` 后，`nslookup baidu.com 127.0.0.1` 触发时的内部细节。

## 变更内容
1. 分析 CLI 启动、配置 include 顺序以及 UDP/TCP 监听如何建立。
2. 追踪 `nslookup baidu.com` 从入口缓存、`sequence_6666`、`sequence_main`、`sequence_not_in_list_*` 的分支条件。
3. 汇总缓存/规则/上游的关键状态，形成结构化文档供工程师查阅。
4. 在知识库中登记该流程，保持 SSOT。

## 影响范围
- **模块:** demo_mosdns1、coremain、plugin_system
- **文件:** docs（新增文档）、helloagents/wiki/modules/demo_mosdns1.md（补充链接）
- **API:** 无直接改动
- **数据:** 引用 `sub_config/*.yaml` 与 `rule/` 数据

## 核心场景

### 需求: R1 启动阶段说明
**模块:** coremain
- 场景: S1 通过 `start --dir demo/mosdns1` 启动
  - 条件: CLI 在项目根执行
  - 预期: 解释工作目录切换、配置加载、插件初始化

### 需求: R2 查询路径溯源
**模块:** demo_mosdns1
- 场景: S2 使用 `nslookup baidu.com 127.0.0.1`
  - 条件: AdGuard 默认关闭、switch3 为泄露模式、baidu 属于 geosite_cn
  - 预期: 描述请求在 sequence/cached/forward 中的判定与调用顺序

### 需求: R3 配置依赖速查
**模块:** plugin_system
- 场景: S3 工程师想快速定位相关子配置
  - 条件: 阅读文档
  - 预期: 附录列出关键子配置、标记/开关意义

## 风险评估
- **风险:** 文档与配置更新不同步。
- **缓解:** 每次调整 demo 配置时同步更新知识库；在文档中标注依赖的子配置文件。
