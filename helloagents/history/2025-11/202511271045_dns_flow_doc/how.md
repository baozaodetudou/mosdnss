# 技术设计: mosdns1 查询流程文档

## 技术方案
### 核心技术
- 静态分析 `demo/mosdns1/config.yaml` 及 `sub_config/*.yaml`
- 利用 CLI 与源码 (`coremain/run.go`, `plugin/*`) 理解启动过程
- 以 Markdown 文档呈现流程、并在知识库中加索引

### 实现要点
1. **启动链路**: 解释 `start --dir` 如何设定工作目录、查找 `config.yaml`、include 子配置、初始化 `udp_all`/`tcp_all`。
2. **查询链路**: 以 `nslookup baidu.com` 为例，拆解自入口缓存 → 主分流 → 列表判断 → 上游调用 → 缓存写回的步骤。
3. **标记/缓存表格**: 汇总 `mark` 与 `switch` 的含义，帮助快速定位行为。
4. **输出位置**: 新建 `docs/mosdns1_request_flow.md`，并在 `wiki/modules/demo_mosdns1.md` 中追加链接。

## 架构设计
- 文档结构分为「启动阶段」「请求处理」「附录」，与现有 demo 功能保持解耦。

## 架构决策 ADR
*(本变更仅补充文档，暂不创建 ADR)*

## 安全与性能
- **安全:** 不引入代码更改；强调 CLI 使用及 API 端口，以防误配置。
- **性能:** 文档中提示缓存和 fallback 的阈值，帮助后续调优。

## 测试与部署
- 通过自查确保文档中的路径可在现有配置下复现。
- 无需额外部署步骤。
