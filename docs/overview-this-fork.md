# 本 fork 概览：与原版 mosdns v5 有什么本质不同？

本仓库基于 upstream mosdns v5（`github.com/IrineSistiana/mosdns/v5`）做了大量魔改。  
核心思路不是改内核算法，而是在相同的插件框架上叠了一整层“控制面 + 规则中心 + 预制方案”。

如果一句话总结：

> 原版 mosdns v5 更像“可编程 DNS 引擎”，本 fork 更像“一台带 Web 控制台的成品 DNS 设备”。

详细对比参考：

- 原版说明：`docs/original-v5.md`
- 全部改动清单：`docs/changes-in-this-fork.md`
- 新增插件分析：`docs/new-plugins-analysis.md`

下面只讲“为什么用这个 fork”以及“核心魔改在哪几层”。

---

## 1. 控制面：从 API → Web 仪表盘

原版 v5：

- HTTP 端口只提供：
  - `/metrics`（Prometheus）
  - `/debug/pprof`（pprof）
  - 各插件各自的 API（无统一 UI）。

本 fork：

- 在 `coremain/www` 下内置完整 Web 前端，编译进二进制：
  - 仪表盘主页（指标/状态）；
  - “查询日志”页（可过滤/分页/查看详情）；
  - “规则管理”页（sd_set/si_set/adguard_rule 等）；
  - “系统控制”页（更新、重启、开关、全局 overrides）。
- `coremain/mosdns.go` 注册了：
  - `/`、`/index`、`/log`、`/plog` 等 HTML 页面；
  - `/assets/*` 的静态资源（JS/CSS/字体）。

**意义**：  
不再是“只给运维/脚本玩”的 API，而是一个真正“可点可看”的 Web 控制台。

---

## 2. 审计系统：从“无审计”→“结构化日志 + 统计 + 慢查询分析”

原版 v5：

- 没有统一的审计系统；只能靠插件零散打 log 或导出 metrics。

本 fork：

- 新增 `coremain/audit.go` + `api_audit.go` + `api_audit_v2.go`：
  - 按请求颗粒度记录：
    - 客户端 IP、QNAME/QTYPE/QCLASS、起始时间；
    - 耗时（ms）、RCODE、应答 flags（AA/TC/RA）；
    - 每条答案的类型/TTL/内容；
    - 命中的 DomainSet 名（无则归为 `"unmatched_rule"`）。
  - 使用固定容量 ring buffer 存最近 N 条记录（容量可持久化配置）。
  - 维护：
    - 按域名 / 客户端 / DomainSet 的命中计数；
    - 慢查询堆（top N 最慢）。
- API：
  - v1：`/api/v1/audit`：
    - 开/关采集、查看状态、导出/清空日志、设置容量。
  - v2：`/api/v2/audit`：
    - 获取统计（总请求数/平均耗时）；
    - 域名/客户端/DomainSet 排行；
    - 慢查询列表；
    - 支持过滤 + 分页的日志查询。
- server 插件支持：
  - `udp_server` / `tcp_server` / `quic_server` / `http_server` 增加 `enable_audit`；
  - 可按入口粒度控制审计。
- 调试日志捕获：
  - 自定义 TeeCore + `coremain/capture.go`：
    - `POST /api/v1/capture/start` 临时提升日志等级到 DEBUG 并写入内存；
    - `GET /api/v1/capture/logs` 一次性取回并清空捕获日志；
    - UI 上可以“一键抓一段 DEBUG 日志”。

**意义**：  
从“看不到请求细节”变成“有完整 DNS 审计 + 慢查询分析 + 一键 debug 抓包”，对排故/优化非常关键。

---

## 3. 规则与分流：从“手写 domain_set/ip_set”→“规则生态 + 任务系统”

原版 v5：

- 提供基础 data_provider（`domain_set` / `ip_set`）和 matcher；
- 典型配置是“手写 domain_set/ip_set + sequence 拼逻辑”。

本 fork 新增了一整套“规则生态”插件（详见 `docs/new-plugins-analysis.md`）：

### 3.1 规则源管理（Rules Source）

- `sd_set`：
  - SRS 域名规则集数据源；
  - 通过 JSON 管理多个源（本地/远程 URL），支持自动更新和 SOCKS5；
  - 对上层暴露为 `DomainMatcherProvider`，供 matcher 使用。
- `si_set`：
  - SRS IP 集数据源；
  - 同样支持本地/远程/自动更新，暴露为 `IPMatcherProvider`。
- `adguard_rule`：
  - AdGuard 风格规则管理器；
  - 支持多个在线规则源，自动更新；
  - 将规则解析成 `domain.MixMatcher`，便于 qname 匹配；
  - 前端提供专门的“广告规则管理”界面。

### 3.2 规则生成与统计（Rules Generation）

- `domain_output`：
  - 在请求流中统计域名访问；
  - 周期性或按需：
    - 把统计写入文件；
    - 根据模板生成规则文件；
    - 可选推送到某个 HTTP 接口（配合 sd_set/si_set）。

### 3.3 规则刷新与重查（Requery）

- `requery`：
  - 使用 cron 定时执行“重查任务”：
    - 读取多个源文件中的域名；
    - 生成规则或刷新缓存；
    - 调用配置好的 HTTP 接口（如刷新上游/规则服务器）。
  - 自带任务状态机（idle/running/failed/finished），可从 UI 查看进度、手动触发。

### 3.4 状态与配置存储（Webinfo + Switch）

- `webinfo`：
  - 通用 JSON 存储插件；
  - 前端通过它读写一些 UI 状态/配置（如客户端别名、Requery 配置等）。
- `switch` + `switcher1`~`switcher9`：
  - 一套“命名开关 + 持久化”的插件：
    - 支持 `switch "name:value"` 这种 matcher 写法；
    - 值持久化在文件中；
    - 通过 HTTP API 或 UI 控制；
  - 用来实现“泄露/不泄露模式”、“是否启用广告过滤”、“是否启用缓存”等高层策略切换。

**意义**：  
从“单纯的规则集合加载器”升级为一个带“规则源管理 + 生成 + 刷新 + 开关 + UI”的**规则平台**，适合真实长期运行环境下维护复杂策略。

---

## 4. 运维闭环：在线更新 + 自重启 + 全局 overrides

原版 v5：

- 版本号只是打印；
- 没有在线更新；
- 没有运行时的 socks5/ECS 覆盖；
- 重启完全依赖系统服务工具或手工。

本 fork 添加了一整套运维闭环：

### 4.1 在线更新 + 自重启

- `coremain/update_manager.go`：
  - 从 GitHub Releases（`yyysuo/mosdns`）拉取最新版本信息；
  - 根据当前平台选择合适资产（含 amd64-v3 逻辑）；
  - 下载、校验 sha256、安装（Unix 覆盖自身；Windows 写 `.new`）。
  - 使用 `.mosdns-update-state.json` 记录已安装资产签名，避免重复。
- `coremain/api_update.go`：
  - `/api/v1/update/status`：查询更新状态（带 CPU 特性、当前/最新版本、是否需要更新等）。
  - `/api/v1/update/check`：强制检查。
  - `/api/v1/update/apply`：执行更新（可指定 `force` / `prefer_v3`）。
- `coremain/api_system.go`：
  - `/api/v1/system/restart`：在非 Windows 平台通过 `syscall.Exec` 实现自重启（带可选延时）。

前端 UI 直接提供“检查更新 / 安装更新 / 自动重启”的操作入口，日志里会打印 CPU 能力、GOAMD64 等信息，便于判断是否使用 v3 构建。

### 4.2 全局 overrides（socks5 / ECS）

- `coremain/overrides.go + api_overrides.go`：
  - 启动时扫描所有配置/子配置：
    - 找到第一次出现的 `socks5`；
    - 找到第一次出现的 `ecs ...` 字符串；
    - 缓存在 `discoveredSocks5` / `discoveredECS`，作为“默认原始值”。
  - 使用 `config_overrides.json` 持久化用户覆盖的值：
    - `GET /api/v1/overrides`：返回当前生效值（优先文件，其次自动发现）。
    - `POST /api/v1/overrides`：以 JSON patch 形式更新部分字段。
  - 在真正加载插件前，通过 `ApplyOverrides` 统一把配置里的 socks5/ECS 替换成覆盖值。

结合 `MainConfigBaseDir`：

- 所有这些文件都自然地放到“主配置所在目录”，路径稳定，行为可预期。

**意义**：  
从“静态配置 + 手工升级”变成“有 OTA 能力的应用”，并且可以在不改 YAML 的情况下统一调整 socks5/ECS 这类横切配置。

---

## 5. demo 方案：从“示例配置”→“开箱即用的完整场景”

原版 v5：

- 文档里有若干示例配置，但多为示意性质；
- 官方推荐更多是“按需自己拼 sequence + 插件”。

本 fork：

- `demo/mosdns1/` 提供一整套可直接运行的配置：
  - 完整拆分的 `sub_config/*.yaml`：
    - AdGuard 集成、国内/国外上游、带 ECS 的上游；
    - 缓存策略、主分流链路、sing-box 专用链路；
    - webinfo、requery、规则集合等子配置。
  - 成体系的规则文件（`rule/`、`srs/`、`gen/` 等）。
  - UI 所需的 webinfo JSON 文件。
- 这一套 demo 实际上就是：
  - “本机直面客户端 53 端口 + 国内/国外分流 + 广告过滤 + 缓存 + 审计 + 规则更新 + 在线更新”的**一条龙配置**。

**意义**：  
对大部分用户来说，不再需要从头理解所有插件，只需在 demo 的基础上微调上游/规则/开关即可部署生产环境。

---

## 6. 适合什么人用这个 fork？

如果你符合以下情况，本 fork 比 upstream v5 更合适：

- 想要：
  - 完整 Web UI（审计、规则、更新、系统控制）；
  - 在线更新、自重启能力；
  - 复杂的国内/国外分流和广告/规则集生态；
  - Demo 级别可以直接部署的成品配置。
- 并且你能接受：
  - 相比原版，多了一些逻辑层的复杂度；
  - 部分行为会优先以“产品体验”为目标（例如自动更新、全局 overrides）；
  - 合并 upstream 更新的难度略高一些（coremain/API 层改动较多）。

如果你只想要一个：

- 非常干净、只提供基础插件与 API 的 DNS 引擎，
- 自己完全控制所有逻辑与集成，

那么 upstream 的纯 v5 可能更简单、更轻量。

---

## 7. 进一步阅读

如果你想继续深入：

- 全部插件与原版对比：`docs/new-plugins-analysis.md`
- 原版官方文档整理：`docs/original-v5.md`
- 详细改动列表：`docs/changes-in-this-fork.md`

这三份文档 + demo/mosdns1/ 下的配置文件，基本就构成了你这套“魔改 mosdns”的完整说明书。 

