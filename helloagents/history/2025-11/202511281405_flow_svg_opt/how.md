# 技术设计: mosdns 请求流程图优化

## 技术方案
### 核心技术
- Mermaid flowchart：使用多子图 + 子泳道
- 自定义边样式：采用 `-.->` 表示“写入列表”类副作用

### 实现要点
- 将入口广告逻辑拆成“switch7 开关”与“在线列表”节点，并使用注释文本说明差异。
- 为白名单路径增加“国内查询”“记录缺失”“CNtoMihomo”连续节点。
- Mode 区域采用并列子图(`ModeCompat`、`ModeSecure`)，减少重复连线。
- 补查节点添加条件标签，例如“仍无 V4”“补查成功”。

## 架构设计
- 无代码结构变更。

## 安全与性能
- 纯文档更新，无运行时风险。

## 测试与部署
- 运行 `npx @mermaid-js/mermaid-cli -i docs/mosdns_request_flow.mmd -o docs/mosdns_request_flow.svg` 验证生成成功。
- 打开 SVG 目视检查布局与说明。
