# 技术设计: mosdns 请求流程 SVG (3840x2160)

## 技术方案
### 核心技术
- Mermaid Flowchart with `%%{init: ... }%%` 自定义主题变量
- `npx @mermaid-js/mermaid-cli -w 3840 -H 2160` 输出固定尺寸 SVG

### 实现要点
1. **区域分布**：
   - 左列: Ingress（过滤 + 客户端策略）。
   - 中列: 主分流/名单/副作用。
   - 右列: 模式泳道与终态。
2. **字体与间距**：`themeVariables.fontSize=22px`、`flowchart.nodeSpacing=40`、`rankSpacing=55`；必要时为重要节点设置 `classDef bold`。
3. **节点优化**：缩短描述，使用标签说明（如 `label / action`），避免 `<br/>` 多行。
4. **CLI 参数**：`npx @mermaid-js/mermaid-cli -i docs/mosdns_request_flow.mmd -o docs/mosdns_request_flow.svg -w 3840 -H 2160`。

## 安全与性能
- 纯文档操作，Mermaid CLI 本地运行，无安全风险。

## 测试与部署
- 生成新 SVG 后人工查看是否在 4K/1080p 下清晰。
- 更新知识库引用，记录在 CHANGELOG。
