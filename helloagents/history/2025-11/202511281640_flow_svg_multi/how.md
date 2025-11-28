# 技术设计: mosdns 请求流程图多视图

## 技术方案
### 核心技术
- Mermaid flowchart + 自定义主题
- `npx @mermaid-js/mermaid-cli` 输出指定宽高，必要时 post-process viewBox/transform

### 实现要点
1. **1080p 横版**：复用 `docs/mosdns_request_flow.mmd`，设置 `-w 1920 -H 1080`，如需保持可读性则按比例平移+缩放与 4K 版本一致。
2. **竖版**：复制横版结构为 `docs/mosdns_request_flow_portrait.mmd`，使用 `flowchart TB`、分层子图（入口→主分流→模式→副作用），生成 `-w 2160 -H 3840` SVG 并统一字体。
3. **文件命名**：新增 `docs/mosdns_request_flow_1080p.svg` 与 `docs/mosdns_request_flow_portrait.svg`，横版原文件保持 `mosdns_request_flow.svg`。
4. **引用**：wiki 模块文档列出三种视图及再生成命令；CHANGELOG 记录新增视图。

## 安全与性能
- 仅文档操作，无安全风险。

## 测试与部署
- 打开两份新 SVG 目视检查（节点分布、字体）。
- 更新历史记录，确保他人可复现 `npx mmdc` 命令。
