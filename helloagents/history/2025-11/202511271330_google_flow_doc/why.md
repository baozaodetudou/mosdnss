# 变更提案: mosdns1 Google 请求流程

## 需求背景
已完成的《mosdns1 请求流程解析》聚焦 `nslookup baidu.com`。现在希望补充另一个典型场景：`nslookup google.com 127.0.0.1`，以说明灰名单命中、fakeip/国外上游的交互以及客户端白名单对入口控制的影响。

## 变更内容
1. 分析默认配置下（`switch2=A` 且 `client_ip.txt` 为空）google 请求为何在入口被重定向到国内链路。
2. 描述当发起 IP 加入 `client_ip.txt` 后，google 请求触发 `greylist` → `mark22` → `sequence_fakeip` → 国外上游/缓存/黑洞匹配的完整过程。
3. 补充 `switch3` 泄露/非泄露两种 fallback 策略在该域名上的差异。
4. 更新《mosdns1 请求流程解析》及模块文档，以维持 SSOT。

## 影响范围
- **模块:** demo_mosdns1、knowledge base 文档
- **文件:** `docs/mosdns1_request_flow.md`, `helloagents/wiki/modules/demo_mosdns1.md`
- **API:** 无代码变更，仅描述 API 交互

## 核心场景

### 需求: R1 默认入口短路
**模块:** demo_mosdns1
- 场景: S1 `nslookup google.com 127.0.0.1`，保持现有 `switch2=A`、`client_ip.txt` 为空。
- 预期: 解释 `sequence_6666` 如何通过 `switch2` 将请求直接交给 `$sequence_local`，以及为何结果与 `baidu.com` 类似。

### 需求: R2 灰名单 + fakeip 流程
**模块:** demo_mosdns1
- 场景: S2 将客户端地址加入 `client_ip.txt` 或把 `switch2` 置 `B`，再次查询。
- 预期: 追踪 `greylist` 命中 → `mark22/777/999` → `$sequence_fakeip` → `$sequence_google` 路径，包含缓存与 `domain_output` 的联动。

### 需求: R3 泄露/非泄露策略差异
**模块:** demo_mosdns1
- 场景: S3 切换 `switch3`，观察 `_leak` 与 `_noleak` 对 google 的 fallback 和 fakeip/realip 标记差别。
- 预期: 用文字+表格总结策略差异，并补充操作提示。

## 风险评估
- **风险:** 文档遗漏某些开关/标记，导致 SSOT 不准确。
- **缓解:** 逐一对照 `sub_config/*.yaml` 与现有文档；必要时引用行号，并在附录列出灰名单与 fakeip 标记说明。
