# 变更提案: mosdns demo 请求流程图

## 需求背景
当前 `docs/mosdns1_request_flow.md` 与 `docs/mosdns流程.md` 以文字描述 demo/mosdns1 的查询决策，但缺少可视化图形，导致读者难以全局把握入口过滤、主分流、名单匹配和模式切换之间的关系。为使运维人员快速理解请求走向，需要提供一份完整的 SVG 流程图并纳入仓库。

## 变更内容
1. 汇总两份文档的节点与判定顺序，定义统一的流程层次（入口过滤 → 客户端策略 → 主分流 → 名单落点 → 模式分支 → 终态）。
2. 编写 Mermaid flowchart 描述，并通过 `npx @mermaid-js/mermaid-cli` 生成 SVG，保存于 `docs/mosdns_request_flow.svg`。
3. 在 `docs/` 目录补充使用说明，确保后续可再生成或更新。

## 影响范围
- **模块:** 文档 (`docs`)
- **文件:** `docs/mosdns1_request_flow.md`, `docs/mosdns流程.md`, 新增 `docs/mosdns_request_flow.mmd`、`docs/mosdns_request_flow.svg`
- **API:** 无
- **数据:** 无

## 核心场景

### 需求: demo/mosdns1 请求调度
**模块:** 文档
将 `sequence_6666` 入口过滤、开关控制、灰/白名单、国内外列表、自生成规则、兼容/安全模式到 fakeip/realip 输出的全流程转化为流程图。

#### 场景: 入口过滤
针对 AAAA、SOA/PTR/HTTPS、DDNS、黑名单、广告拦截、客户端 IP 白名单进行条件判断并给出终态。

#### 场景: 主分流与名单
展示 rewrite、灰名单 fakeip、白名单 realip+nov4/nov6、自生成/在线列表的国内/国外落点。

#### 场景: 模式切换
兼容/安全模式下，国内/国外 DNS 查询顺序、CNtoMihomo fallback 以及 fakeip 输出路径。

## 风险评估
- **风险:** Mermaid CLI 未预装，npx 首次下载较慢。
- **缓解:** 使用 `npx @mermaid-js/mermaid-cli` 临时获取，无需全局安装；如失败可退回纯手写 SVG。
