# 变更提案: mosdns 请求流程图多视图

## 需求背景
目前仓库中已有 3840×2160 的横版流程图，但用户还需要：
1. 一份 1920×1080（横版）版本，方便在常规显示器或 PPT 中引用。
2. 一份竖版流程图（保持横版不变），以便移动端或纵向屏幕展示。

## 变更内容
1. 复用 `docs/mosdns_request_flow.mmd`，生成 1920×1080 的 SVG，并确保字体与布局适配新分辨率。
2. 新建 `docs/mosdns_request_flow_portrait.mmd`，以纵向结构重排入口→主分流→模式→副作用，输出 2160×3840 的竖版 SVG。
3. 更新 wiki 引用与 CHANGELOG，记录新增视图；归档方案包。

## 影响范围
- **模块:** 文档、知识库
- **文件:** `docs/*.mmd`、`docs/*.svg`、`helloagents/wiki/modules/demo_mosdns1.md`、`helloagents/CHANGELOG.md`、history 条目

## 核心场景

### 需求: 多尺寸可视化
**模块:** 文档
- 提供 1080p 横版流程图。
- 提供竖版流程图，方便移动端展示。
- 确保横版 4K 原图不受影响。

## 风险评估
- **风险:** 竖版布局可能导致节点重叠。
- **缓解:** 调整 `flowchart` 方向、rankSpacing、子图拆分；必要时增加注释节点。
