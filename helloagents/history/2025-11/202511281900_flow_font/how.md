# 技术设计: mosdns 流程图字体放大

## 技术方案
### 核心技术
- Mermaid CLI (`@mermaid-js/mermaid-cli`) 渲染 SVG。
- 自定义 `themeVariables.fontSize`、`lineHeight` 与局部节点样式，保证视觉一致。

### 实现要点
- 将两份 `.mmd` 的 `themeVariables.fontSize` 从 38px 提升到 76px，并按需同步 `classDef` stroke 宽度/label 间距。
- 调整 `flowchart` 配置的 `rankSpacing`、`nodeSpacing` 以防止放大导致节点拥挤；若无必要保持现值。
- 重新运行：
  - `npx @mermaid-js/mermaid-cli -i docs/mosdns_request_flow.mmd -o docs/mosdns_request_flow.svg -w 3840 -H 2160`
  - `npx @mermaid-js/mermaid-cli -i docs/mosdns_request_flow.mmd -o docs/mosdns_request_flow_1080p.svg -w 1920 -H 1080`
  - `npx @mermaid-js/mermaid-cli -i docs/mosdns_request_flow_portrait.mmd -o docs/mosdns_request_flow_portrait.svg -w 2160 -H 3840`
- 渲染完成后检查 SVG 根 `<svg>` 与 `<g>` 的 `viewBox`、`transform`，必要时等比缩放以贴合画布边界。

## 架构设计
无新增架构。

## 架构决策 ADR
无。

## API设计
无。

## 数据模型
无。

## 安全与性能
- **安全:** 仅本地文档渲染无外部依赖。
- **性能:** 生成时间可忽略。

## 测试与部署
- 手动打开三份 SVG，检查字体、节点间距与可读性。
- 确保 Git diff 中仅包含 `.mmd` 与 `.svg` 变更（以及必要的知识库更新）。
