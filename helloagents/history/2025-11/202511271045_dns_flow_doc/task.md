# 任务清单: mosdns1 查询流程文档

目录: `helloagents/plan/202511271045_dns_flow_doc/`

---

## 1. 启动链路
- [√] 1.1 解析 `coremain` 与 CLI 参数，整理 `start --dir` 的行为，验证 why.md#需求-r1-启动阶段说明。

## 2. 查询路径
- [√] 2.1 逐步梳理 `sequence_6666` 与 `sequence_all` 逻辑，记录关键 `matches`/`mark`，验证 why.md#需求-r2-查询路径溯源。
- [√] 2.2 分析 `sequence_main` 及 `sequence_not_in_list_*`，说明 `baidu.com` 流程，验证 why.md#需求-r2-查询路径溯源。

## 3. 配置依赖
- [√] 3.1 对 `cache.yaml`、`forward_*.yaml`、`switch.yaml` 做速查表，验证 why.md#需求-r3-配置依赖速查。

## 4. 文档输出
- [√] 4.1 编写 `docs/mosdns1_request_flow.md`，涵盖启动、请求、附录三个部分。
- [√] 4.2 更新 `helloagents/wiki/modules/demo_mosdns1.md`，添加文档链接并同步摘要。

## 5. 质量与安全
- [√] 5.1 自检文档与配置是否一致，确保无敏感信息泄露。
- [√] 5.2 复核 `task.md` 勾选状态并准备迁移。
