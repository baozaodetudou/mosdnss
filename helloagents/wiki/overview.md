# mosdns 总览

> mosdns 是一个通过插件序列实现多场景 DNS 流量治理的 Go 应用，支持 CLI 启动、HTTP API 与丰富的 demo 配置。

## 1. 项目概述

### 目标与背景
- 提供可高度定制的 DNS server，可在本地/网关中进行解析分流、缓存、规则匹配与伪装。
- 通过 `demo/` 配置向用户展示典型的国内/国际、泄露/非泄露、AdGuard 联动等场景。

### 范围
- **范围内:** CLI 管理、插件执行链、缓存/转发/规则集、HTTP API、demo 配置。
- **范围外:** 系统级安装脚本、第三方前端的实现细节。

### 干系人
- **维护人:** IrineSistiana（upstream 项目）与本地部署管理员。

## 2. 模块索引

| 模块名称 | 职责 | 状态 | 文档 |
|---------|------|------|------|
| coremain | CLI、配置加载、主循环 | ✅ 稳定 | [链接](modules/coremain.md) |
| plugin_system | 插件注册、通用组件 | ✅ 稳定 | [链接](modules/plugin_system.md) |
| demo_mosdns1 | 高级分流 demo 配置 | 🚧 开发中 | [链接](modules/demo_mosdns1.md) |

## 3. 快速链接
- [技术约定](../project.md)
- [架构设计](arch.md)
- [API 手册](api.md)
- [数据模型](data.md)
- [变更历史](../history/index.md)
