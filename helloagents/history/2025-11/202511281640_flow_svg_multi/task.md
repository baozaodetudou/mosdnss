# 任务清单: mosdns 请求流程图多视图

目录: `helloagents/plan/202511281640_flow_svg_multi/`

---

## 1. 1080p 横版
- [√] 1.1 复用现有 `.mmd` 调整输出参数，生成 `docs/mosdns_request_flow_1080p.svg`。
- [√] 1.2 检查字体与留白，必要时通过 viewBox/transform 调整。

## 2. 竖版流程图
- [√] 2.1 创建 `docs/mosdns_request_flow_portrait.mmd`，重排为纵向结构。
- [√] 2.2 生成 `docs/mosdns_request_flow_portrait.svg`（-w 2160 -H 3840），并验证节点不重叠。

## 3. 知识库同步
- [√] 3.1 更新 wiki 模块文档，列出三份流程图与再生成命令。
- [√] 3.2 更新 CHANGELOG，记录新增 1080p 与竖版视图。
- [√] 3.3 迁移方案包至 history，并更新索引。
