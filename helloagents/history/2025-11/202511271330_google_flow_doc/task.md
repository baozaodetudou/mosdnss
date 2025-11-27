# 任务清单: mosdns1 Google 请求流程

目录: `helloagents/plan/202511271330_google_flow_doc/`

---

## 1. 入口分析
- [√] 1.1 说明 `switch2` 与 `client_ip` 对 google 请求的默认影响，验证 why.md#需求-r1-默认入口短路。

## 2. 灰名单路径
- [√] 2.1 梳理 `greylist` 命中后的 `sequence_fakeip` 流程，验证 why.md#需求-r2-灰名单--fakeip-流程。
- [√] 2.2 解释 `mark22/777/999` 与 cache、domain_output 的联动，验证 why.md#需求-r2-灰名单--fakeip-流程。

## 3. 泄露策略
- [√] 3.1 对比 `sequence_not_in_list_leak` 与 `_noleak` 对 google 的不同 fallback，验证 why.md#需求-r3-泄露-非泄露策略差异。

## 4. 文档输出
- [√] 4.1 在 `docs/mosdns1_request_flow.md` 中新增“google.com 场景”章节。
- [√] 4.2 更新 `helloagents/wiki/modules/demo_mosdns1.md`，记录新文档段落。

## 5. 质量
- [√] 5.1 自检内容与配置一致，准备迁移方案包。
