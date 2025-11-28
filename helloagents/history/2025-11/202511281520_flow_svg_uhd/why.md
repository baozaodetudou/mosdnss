# 变更提案: mosdns 请求流程 SVG (3840x2160)

## 需求背景
现有 `docs/mosdns_request_flow.svg` 尺寸超 5700px，字体默认 16px，用户在 4K/1080p 屏幕上需要不断缩放才能看清节点，且左右留白较多。需要重新设计一份 3840×2160 的高分辨率 SVG，将入口、主分流、模式泳道等内容合理分区并调大字体，确保读者无需额外缩放即可阅读。

## 变更内容
1. 重新规划 Mermaid 布局：入口位于左侧、主分流居中、模式/终态在右侧，上下划分为“过滤→分流→模式”。
2. 调整节点 granularity：合并次要说明、拆分长段文本，并利用注释节点减少换行。
3. 设置自定义主题变量（字体 ≥ 22px、节点间距更紧凑）并通过 `mmdc` 设置输出宽高 3840×2160。
4. 更新 `docs/mosdns_request_flow.mmd`、导出新的 SVG，并同步知识库引用与 CHANGELOG。

## 影响范围
- **模块:** 文档、知识库
- **文件:** `docs/mosdns_request_flow.mmd`, `docs/mosdns_request_flow.svg`, `helloagents/wiki/modules/demo_mosdns1.md`, `helloagents/CHANGELOG.md`

## 核心场景

### 需求: 4K 友好流程图
**模块:** 文档
- 节点可读性（字体≥22px）。
- 横向 3 栏布局，减少留白。
- 输出固定 3840×2160（适配展示屏幕/汇报幻灯）。

## 风险评估
- **风险:** 画布压缩可能导致长链路重叠。
- **缓解:** 使用 rank/spacing 调整，必要时拆分为多子图；生成前在 mmd 设定 `flowchart` 参数。
