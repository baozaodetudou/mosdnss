# 任务清单: mosdns demo 请求流程图

目录: `helloagents/plan/202511281215_flow_svg/`

---

## 1. 信息整理
- [√] 1.1 汇总 `docs/mosdns1_request_flow.md` 与 `docs/mosdns流程.md` 的步骤，形成分层节点映射，验证 why.md#需求-入口过滤 与 why.md#需求-主分流与名单。

## 2. 流程图实现
- [√] 2.1 编写 `docs/mosdns_request_flow.mmd`，按照入口→主分流→名单→模式→终态结构绘制流程，验证 why.md#需求-模 式切换。
- [√] 2.2 运行 `npx @mermaid-js/mermaid-cli -i docs/mosdns_request_flow.mmd -o docs/mosdns_request_flow.svg` 生成 SVG，确认无报错。

## 3. 验证与文档
- [√] 3.1 使用浏览器/预览工具查看 SVG，确保节点与连接完整，必要时调整布局重新生成。
- [√] 3.2 在相关文档中补充“流程图生成说明”(若需要)或在提交描述中注明引用方式。

## 4. 安全检查
- [√] 4.1 确认流程中未泄露敏感 IP/域名之外的额外数据，符合G10要求。

## 5. 测试
- [√] 5.1 复现一次 `npx @mermaid-js/mermaid-cli` 命令，保证其他人可直接再生成。
