# 任务清单: mosdns 请求流程 SVG (3840x2160)

目录: `helloagents/plan/202511281520_flow_svg_uhd/`

---

## 1. 布局设计
- [√] 1.1 绘制新版布局草稿：入口/主分流/模式 3 列并整理节点文本长度。
- [√] 1.2 在 Mermaid 文件中加入 init 配置（字体、间距、classDef），确保 4K 友好。

## 2. 生成与验证
- [√] 2.1 使用 `npx @mermaid-js/mermaid-cli -w 3840 -H 2160` 生成 SVG。
- [√] 2.2 在 100% 缩放下预览 SVG，检查字体和留白（通过 SVG viewBox + transform 确保居中）。

## 3. 知识库同步
- [√] 3.1 更新 `helloagents/wiki/modules/demo_mosdns1.md` 中的说明。
- [√] 3.2 更新 `helloagents/CHANGELOG.md`，记录 4K 流程图优化。
- [√] 3.3 迁移方案包至 history 并更新索引。
