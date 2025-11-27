# API 手册

## 管理接口
| Method | Path | 描述 | 认证 |
|--------|------|------|------|
| GET | /api/v1/update/status | 查询在线更新状态 | 同主控 token（如启用） |
| POST | /api/v1/update/check | 强制刷新更新状态 | 同上 |
| POST | /api/v1/update/apply | 执行更新，`{"force": bool}` | 同上 |
| GET | /api/v1/system/info | 系统信息，端口 9099 | 同上 |

- 默认 `api.http` 监听 `0.0.0.0:9099`，由 `coremain/api_*.go` 提供。
- Web UI 通过 `plugins/webinfo` 与 `switch*` 插件读写配置，POST JSON 即可在线调整。

## 插件运行态 API
| Method | Path | 描述 |
|--------|------|------|
| POST | /plugins/<tag>/post | 将 JSON 配置写入目标插件（如 switch、adguard） |
| GET | /plugins/<tag>/get | 获取插件当前状态或统计 |

## 远程统计
- `/metrics` 暴露 Prometheus 指标，需要在配置中启用 `prometheus` 插件。
- `api_audit_v2.go` 支持分页审计查询，需在配置打开 `enable_audit`。
