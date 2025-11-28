# demo_mosdns1 模块

## 目的
演示一个内置的“国内优先 + 列表驱动 + 多缓存层” DNS 服务，位于 `demo/mosdns1`。

## 模块概述
- **职责:** 通过 `config.yaml` + `sub_config/` 组织大量插件，提供 53 端口服务与 Web UI。
- **状态:** 🚧 开发中（需要依据部署环境调整上游、SOCKS5、开关文件）
- **最后更新:** 2025-11-28

## 规范
### 需求: 启动 Demo 服务
**模块:** demo_mosdns1
- 使用 `mosdns start --dir demo/mosdns1`，CLI 会在该目录查找 `config.yaml`。
- `config.yaml` 先 include 数据与基础能力，再 include 主逻辑与导出。
- `plugins` 部分创建 `sequence_all`, `sequence_6666`, `udp_all`, `tcp_all` 等入口。

#### 场景: 客户端查询流程
1. UDP/TCP 查询进入 `sequence_6666`：包含类型过滤、DDNS 检查、AdGuard、client_ip 白名单、缓存选择等逻辑。
2. 命中缓存直接返回，否则进入 `sequence_all -> sequence_main`。
3. `sequence_main` 根据列表和动态标记（`mark 11/22/33/66/68/777/888/999`）在国内、fakeip、灰名单、realip 等序列间跳转。
4. 末尾根据 `switch3` 决定走 `sequence_not_in_list_leak`（先国内后国外）或 `_noleak`（ECS+国外再兜底国内）。
5. `switch4` 控制 Lazy Cache；`switch1` 控制黑名单；`switch7` 控制 AdGuard。
- `docs/mosdns1_request_flow.md` 提供 `baidu.com` 与 `google.com` 两个完整示例，其中 `google` 场景详述了 `switch2` 白名单、`greylist`、`sequence_fakeip` 的联动。

### 需求: Web UI / API 交互
- `api.http` 绑定 9099，`sub_config/webinfo.yaml` 为前端持久化空间。
- `switch*.txt` 可被 API POST 替换，实现动态策略。

## API 接口
- 依赖 coremain 提供的 `/plugins/<tag>` 接口写入 `switch`、`webinfo`。

## 数据模型
- 所有运行期生成的 domain set 存储在 `gen/*.txt`，cache dump 存储在根目录。

## 依赖
- `sub_config` 中的 forward/cache/domain_set/switch 插件。

## 参考资料
- [docs/mosdns1_request_flow.md](../../docs/mosdns1_request_flow.md): 详细描述 `start --dir demo/mosdns1` 后一次查询的分层流程、标记与开关含义，是排查实际请求路径的首选文档。
- [docs/mosdns_request_flow.svg](../../docs/mosdns_request_flow.svg): demo 请求流程图，覆盖入口过滤、名单落点、兼容/安全模式分支，可配合 `.mmd` 源文件重建图形。
- [docs/mosdns_request_flow.mmd](../../docs/mosdns_request_flow.mmd): Mermaid 源文件，运行 `npx @mermaid-js/mermaid-cli -i docs/mosdns_request_flow.mmd -o docs/mosdns_request_flow.svg` 可再生成 SVG。

## 变更历史
- 202511270000_init_kb: 初始化知识库记录。
