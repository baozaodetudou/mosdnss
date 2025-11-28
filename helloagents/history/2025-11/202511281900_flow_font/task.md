# 任务清单: mosdns 流程图字体放大

目录: `helloagents/plan/202511281900_flow_font/`

---

## 1. Mermaid 源文件调整
- [√] 1.1 将 `docs/mosdns_request_flow.mmd` 的 `themeVariables.fontSize` 等相关样式翻倍，并验证节点间距 (why.md#需求-放大-mosdns-流程图字体-场景-uhd-横版)。
- [√] 1.2 同步更新 `docs/mosdns_request_flow_portrait.mmd` 的字体/样式参数 (why.md#需求-放大-mosdns-流程图字体-场景-竖版)。

## 2. SVG 重新生成
- [√] 2.1 使用更新后的 `.mmd` 生成 3840×2160 与 1920×1080 SVG，必要时调整 `viewBox`/`transform` (why.md#需求-放大-mosdns-流程图字体-场景-1080p-横版)。
- [√] 2.2 使用 portrait `.mmd` 生成 2160×3840 SVG，并校正布局。

## 3. 知识库同步与验证
- [√] 3.1 若 wiki/CHANGELOG 已描述字体或生成参数，视情况更新说明。
- [√] 3.2 手动打开三份 SVG 确认字体、留白与泳道未重叠，完成后将方案包迁移至 history。
