# 技术设计: mosdns1 Google 请求流程

## 技术方案
### 核心技术
- 复用现有 demo 配置，静态解析 `sequence_6666`、`sequence_main`、`sequence_fakeip`、`sequence_not_in_list_*`。
- 对照 `greylist.txt`、`switch*.txt`、`client_ip.txt`，总结默认与放行模式差异。
- 更新 `docs/mosdns1_request_flow.md`，新增章节“google.com 场景”。
- 在 `helloagents/wiki/modules/demo_mosdns1.md` 增补该章节的引用。

### 实现要点
1. **入口短路说明**：结合 `switch2=A` 和空 `client_ip.txt`，解释 `mark3` 如何将请求送往 `$sequence_local`。
2. **灰名单路径**：补充 `mark22 → sequence_fakeip → fallback` 的步骤，包括 `cache_google`, `forward_google`, `forward_fakeip`。
3. **泄露 vs 不泄露**：以表格比较 `sequence_not_in_list_leak` / `_noleak` 对 google 的影响（ECS、fallback、fakeip 标记）。
4. **操作提示**：记录如何修改 `client_ip.txt` 或 `switch2` 以观察完整流程。

## 安全与性能
- 文档中提醒修改 `client_ip.txt` 意味着开放外部访问，防止误操作。
- 无代码改动，不影响性能。

## 测试与验证
- 通过阅读配置 + 实际执行 `nslookup`（可选）验证描述；若实际运行需 root，说明是可选步骤。
