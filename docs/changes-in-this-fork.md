# 本仓库相对 mosdns v5 的改动说明

本文件描述当前仓库（`github.com/IrineSistiana/mosdns/v5`，本地路径 `/Users/doumao/code/github/mosdns`）在原版 mosdns v5 基础上做的扩展和魔改。  
原版行为与官方文档请参考：`docs/original-v5.md`。

---

## 1. 改动总览

在保持原版插件模型和配置语义的前提下，本仓库主要做了以下几类增强：

- 内置 Web 仪表盘（Dashboard）与前端 UI。
- 审计日志系统重写 + 查询日志 UI。
- 内置在线更新（GitHub Release）与自重启机制。
- 全局 overrides（socks5 / ECS）运行时覆盖。
- 配置加载与路径处理增强（baseDir / MainConfigBaseDir）。
- 新增多种插件：sd_set / si_set / adguard_rule / domain_output / requery / rewrite / webinfo / switch 等。
- 提供完整 demo 配置、预置规则集、release 打包脚本与 CI。
- 更新依赖版本，适配新的构建与运行环境。

以下按功能模块展开。

---

## 2. 内置 Web 仪表盘与前端 UI

相关文件：

- `coremain/mosdns.go`
- `coremain/www/**/*`
- `coremain/www/assets/js/log.js`
- README 中“版本与下载”、“内置在线更新”一节

主要改动：

- 使用 `embed.FS` 将整个 `coremain/www` 嵌入二进制：
  - HTML：`index.html`、`log.html`、`log_plain.html`、`mosdns.html`、`mosdnsp.html`。
  - CSS：`assets/css/index_overrides.css`、`assets/css/log_refactored.css`、`assets/css/pico.min.css`。
  - JS：`assets/js/log.js`、`assets/js/chart.umd.min.js`、`assets/js/chartjs-plugin-datalabels.min.js`。
  - 字体与图标：`assets/fontawesome/...`。
- 在 `NewMosdns` 中新增路由：
  - `/`：主仪表盘（index 页面）。
  - `/index`：同上。
  - `/log`：图形化查询日志页。
  - `/plog`：纯文本/精简日志页。
  - `/rlog`：重定向到 `/log`。
  - `/assets/*`：静态资源。
- 前端脚本 `log.js` 实现：
  - Tab 化的 UI：概览、查询日志、规则管理、系统控制等。
  - 与审计 API（v1/v2）、更新 API、overrides API、Requery API、规则管理 API 交互。
  - 图表（Chart.js）展示指标和历史趋势。
  - 客户端别名管理、本地设置（主题/布局）、数据视图弹窗等。

原版 mosdns v5 只有插件 API + metrics + pprof，没有统一的 Web 控制台。本仓库在原有 API 基础上提供了完整仪表盘，所有额外 HTTP API 设计都围绕这个仪表盘服务。

---

## 3. 审计日志与调试日志捕获增强

相关文件：

- `coremain/audit.go`
- `coremain/api_audit.go`
- `coremain/api_audit_v2.go`
- `coremain/capture.go`
- `coremain/api.go`
- `plugin/server/*_server.go` 中 `EnableAudit` 字段
- demo 配置 `demo/mosdns1/config.yaml` 中的 `enable_audit: true`

### 3.1 审计日志系统重写

主要改动点：

- 新的 `AuditLog` 结构：
  - 包含：`ClientIP`、`QueryType`、`QueryName`、`QueryClass`、`QueryTime`、`DurationMs` 等；
  - 新增：`TraceID`、`ResponseCode`、`ResponseFlags`（AA/TC/RA）、`Answers`（记录类型、TTL、具体值）、`DomainSet`。
- 内部实现：
  - 使用固定容量 ring buffer 保存最近 N 条日志，对应参数 `defaultAuditCapacity` / `maxAuditCapacity`。
  - 通过 `domainCounts` / `clientCounts` / `domainSetCounts` 保存聚合统计。
  - 使用 `slowestQueryHeap` 维护最慢查询列表。
  - 为字符串设计全局 LRU 缓存（`internString` + `globalStringLRU`），减少 GC 压力。
- 处理方式：
  - 引入 `auditContext`（携带 `*query_context.Context` 与处理耗时）。
  - 前端 worker goroutine 从 channel 读取 `auditContext`，异步处理，降低锁竞争。
  - 支持 `audit_settings.json` 持久化容量设置，通过 `InitializeAuditCollector(MainConfigBaseDir)` 在启动时加载。

### 3.2 审计 API v1：`/api/v1/audit`

- `coremain/api_audit.go`
- 路由：
  - `POST /api/v1/audit/start`：开启审计采集。
  - `POST /api/v1/audit/stop`：停止审计采集。
  - `GET /api/v1/audit/status`：返回是否在采集。
  - `GET /api/v1/audit/logs`：返回当前内存中的所有审计日志。
  - `POST /api/v1/audit/clear`：清空内存审计日志。
  - `GET /api/v1/audit/capacity`：获取当前 log 容量。
  - `POST /api/v1/audit/capacity`：设置新的容量（写入 `audit_settings.json` 并重建 Collector）。

这些 API 支持前端动态控制审计开关与容量。

### 3.3 审计 API v2：`/api/v2/audit`

- `coremain/api_audit_v2.go`
- 路由：
  - `GET /api/v2/audit/stats`：
    - 返回 `TotalQueries`（uint64）与 `AverageDurationMs`。
  - `GET /api/v2/audit/rank/domain`：
    - 返回最近日志中的域名排行（可指定 `limit`）。
  - `GET /api/v2/audit/rank/client`：
    - 返回客户端 IP 排行。
  - `GET /api/v2/audit/rank/domain_set`：
    - 按 `DomainSet` 统计（便于观察不同规则集命中情况）。
  - `GET /api/v2/audit/rank/slowest`：
    - 返回最慢查询列表。
  - `GET /api/v2/audit/logs`：
    - 支持分页与多条件过滤的日志查询：
      - 参数：`page`、`limit`、`domain`、`answer_ip`、`cname`、`client_ip`、`q`、`exact` 等。

UI 页面上的“历史总量/平均耗时折线图”、“TOP 域名/客户端/规则集”、“慢查询页面”、“日志搜索”都依赖这组 v2 API。

### 3.4 启用/关闭审计：server 层 `EnableAudit`

在以下插件的 `Args` 中新增字段：

- `plugin/server/udp_server/udp_server.go`
- `plugin/server/tcp_server/tcp_server.go`
- `plugin/server/quic_server/quic_server.go`
- `plugin/server/http_server/http_server.go`

字段：

```yaml
enable_audit: true|false
```

server 创建 handler 时把此标志传入 `server_utils.NewHandler`，从而决定是否向 `GlobalAuditCollector` 发送审计事件。

### 3.5 调试日志捕获：`/api/v1/capture`

相关文件：

- `coremain/capture.go`
- `coremain/api.go`
- `coremain/mosdns.go` 中对 logger 的初始化

行为：

- 使用 `InMemoryLogCollector` + 自定义 `TeeCore`（包装 zap.core）：
  - 所有日志仍照常输出到文件/console；
  - 额外将 DEBUG 级别日志复制到内存 collector。
- API：
  - `POST /api/v1/capture/start`：
    - 可选 JSON：`{"duration_seconds": 120}`；
    - 在一段时间内把日志等级提升为 DEBUG，并捕获日志到内存。
  - `GET /api/v1/capture/logs`：
    - 返回捕获期间的日志并清空缓冲。

前端可以用这个功能在 UI 上“临时抓一段 DEBUG 日志”，辅助排查问题，而无需改配置文件重启。

---

## 4. 内置在线更新与自重启

相关文件：

- `coremain/update_manager.go`
- `coremain/api_update.go`
- `coremain/api_system.go`
- `coremain/versioninfo.go`
- `main.go`
- README 中“版本与下载”、“内置在线更新”、“重要变更：移除固定 tag 回退”

### 4.1 版本号规范

README 中重新定义了版本号规范：

- 自 2025-11-06 起，版本统一为：
  - `v5-ph-srs-YYYYMMDD-<shortsha>`，示例：`v5-ph-srs-20251104-4f2f1c9`。
- 可执行文件内 `./mosdns version` 输出与 Release 标签一致。

在 `main.go` 中：

- `var version = "v5-ph-srs"`（实际发布时会被构建系统替换为完整 tag）。
- `init()` 中调用 `coremain.SetBuildVersion(version)`：
  - 将构建版本写入全局 `buildVersion`。
  - 通知 `GlobalUpdateManager` 当前运行版本。

### 4.2 UpdateManager：检查与安装更新

`coremain/update_manager.go` 实现了完整的更新流程：

- 数据结构：
  - `UpdateStatus`：向前端暴露当前版本、最新版本、架构、下载 URL、是否有更新、是否已安装等待重启、CPU 特性（是否支持 amd64-v3）、当前构建是否为 v3 等。
  - `UpdateActionResponse`：执行更新后的结果（是否安装成功、是否需要重启、说明文字）。
  - `updateState`：记录已安装资产的签名和时间，保存在 `.mosdns-update-state.json`。
- 核心行为：
  - 使用 GitHub API 或 HTML 解析最新 Release：
    - `https://api.github.com/repos/yyysuo/mosdns/releases/latest`
    - 回退解析 `/releases/latest` 页面的 HTML，提取 tag、资产链接和 sha256 签名。
  - 根据当前平台选择合适的资产：
    - `mosdns-<os-arch>.zip` 或 `mosdns-<os-arch>-v3.zip`。
    - 对于 amd64/linux 与 amd64/windows，尝试选择 v3 版本（若 CPU 支持）。
  - 下载更新包：
    - 将 zip 下载到临时文件，计算 sha256 与签名比对。
  - 安装：
    - Unix：
      - 解压并覆盖当前可执行文件，保留可执行位。
    - Windows：
      - 写入 `.new` 文件，并告知用户改名覆盖，避免文件锁问题。
  - 状态管理：
    - 使用 `.mosdns-update-state.json` 记录当前已安装资产签名；
    - 更新 `currentAssetSignature` 与 `pendingSignature`，决定是否需要再次更新。
  - CPU 特性检测：
    - 使用 `golang.org/x/sys/cpu` 检测 AVX2/BMI1/BMI2/FMA 等；
    - 读取 GOAMD64 环境变量；
    - 通过 `runtime/debug.ReadBuildInfo` 检查当前二进制是否为 `v3` 构建。

### 4.3 更新 API：`/api/v1/update`

`coremain/api_update.go` 注册了更新相关的 API：

- `GET /api/v1/update/status`：
  - 在 3 秒超时时间内调用 `CheckForUpdate`；
  - 即使 GitHub 请求出错也会返回 200 + 降级消息（`Message` 字段），避免前端长时间卡在“检测中...”。
- `POST /api/v1/update/check`：
  - 强制刷新缓存（忽略上次检查时间），超时时间 8 秒。
- `POST /api/v1/update/apply`：
  - 接收 JSON：`{"force": bool, "prefer_v3": bool}`；
  - 调用 `PerformUpdate` 执行下载与安装；
  - 优先返回成功/失败信息；若 `ErrNoUpdateAvailable`，也返回 200。

前端的“版本与更新”模块依赖这组 API。

### 4.4 自重启 API：`/api/v1/system/restart`

`coremain/api_system.go` 提供系统级自重启操作：

- 路由：
  - `POST /api/v1/system/restart`。
- 行为：
  - 支持可选 JSON：`{"delay_ms": 300}`。
  - 在非 Windows 平台：
    - 先返回一个 `{"status": "scheduled", "delay_ms": ...}`；
    - 异步等待指定毫秒后调用 `syscall.Exec` 以当前参数/环境重启进程。
  - 在 Windows 平台：
    - 返回 501（Not Implemented）与错误信息说明不支持自重启。

UpdateManager 在安装更新后会优先尝试调用此 API 进行本地重启（局部 HTTP 请求无代理），若失败则回退到直接 `Exec` 自重启。

---

## 5. 全局 overrides（socks5 / ECS 动态覆盖）

相关文件：

- `coremain/overrides.go`
- `coremain/api_overrides.go`
- `coremain/mosdns.go` 中 overrides 初始化逻辑
- `coremain/run.go` 设置 `MainConfigBaseDir`
- `testutils/mosdns_overrides_api.md`

设计目标：

- 在不修改 YAML 配置文件的情况下，运行中通过 API 动态设置：
  - 所有涉及上游访问的插件统一使用一个 socks5 代理；
  - 所有 `ecs ...` 字符串统一替换为新的 ECS 地址。

### 5.1 配置发现与缓存

`coremain/overrides.go` 中：

- `GlobalOverrides`：
  - 结构：`{ Socks5 string, ECS string }`，对应 `config_overrides.json` 的字段。
- `DiscoverAndCacheSettings(cfg *Config)`：
  - 遍历主配置及 include 链中的 `plugins[].args`，查找：
    - 第一次出现的 `socks5: <addr>`；
    - 第一次出现的以 `"ecs "` 开头的字符串。
  - 将结果缓存到 `discoveredSocks5` / `discoveredECS`，在未设置 overrides 或 overrides 文件损坏时作为回退值。

### 5.2 应用 overrides

`ApplyOverrides(pluginConf *PluginConfig, overrides *GlobalOverrides)`：

- 深度遍历 `pluginConf.Args`（支持 `map[string]any`、`[]any`、string 等）：
  - 若 `overrides.Socks5` 非空且当前 map 中有 `socks5` 字段，则替换为 overrides 中的值。
  - 若 `overrides.ECS` 非空且某字符串以 `"ecs "` 开头，则替换为 `"ecs <overrides.ECS>"`。
- 在 `coremain/mosdns.go` 的 `loadPluginsFromCfg` 中，在创建每个插件实例前调用：

```go
if m.globalOverrides != nil {
    ApplyOverrides(&pc, m.globalOverrides)
}
```

这样原配置保持不变，但运行时实际使用的 socks5 / ECS 会被 overrides 覆盖。

### 5.3 Overrides API 与持久化

`coremain/api_overrides.go` 中：

- 文件位置：
  - `config_overrides.json` 位于 `MainConfigBaseDir` 下，与主配置同目录。
- API：`/api/v1/overrides`
  - `GET /`：
    - 若存在 `config_overrides.json` 且能成功解析：
      - 返回文件中的 `socks5`/`ecs`。
    - 若文件不存在或解析失败：
      - 返回 `discoveredSocks5`/`discoveredECS`。
  - `POST /`：
    - 接收 JSON patch：

      ```json
      {
        "socks5": "10.0.0.1:1080",
        "ecs": "2001:db8::ffff"
      }
      ```

    - 任一字段若在 JSON 中出现，就更新；未出现则保持原值。
    - 将合并后的 `GlobalOverrides` 以美化的 JSON 写回 `config_overrides.json`。
    - 记录日志并返回一条提示消息：“已保存，全局覆盖生效需重启 mosdns”。

配套文档 `testutils/mosdns_overrides_api.md` 对 API 使用方法、典型流程（查询现值 → 设置新值 → 重启验证）做了说明。

---

## 6. 配置加载与路径处理增强

相关文件：

- `coremain/run.go`
- `coremain/config.go`
- `coremain/mosdns.go` 中 include 解析逻辑

主要改动：

- `Config` 增加字段：

```go
type Config struct {
    Log     mlog.LogConfig `yaml:"log"`
    Include []string       `yaml:"include"`
    Plugins []PluginConfig `yaml:"plugins"`
    API     APIConfig      `yaml:"api"`
    baseDir string         `yaml:"-"`    // 新增，内部使用
}
```

- `loadConfig` 返回时设置 `cfg.baseDir`：
  - 使用 `resolveBaseDir(fileUsed)` 将配置文件路径转换为绝对路径，再取其目录。
  - 若找不到配置文件，则退回当前工作目录。
- 在 `NewServer` 中新增 `MainConfigBaseDir`：
  - 根据 `fileUsed` 或 `-d` 参数、当前工作目录计算；
  - 用于：
    - 记录日志：“main config base directory set”；
    - 初始化审计 Collector（读写 `audit_settings.json`）；
    - 决定 `config_overrides.json`、其它辅助文件的默认位置。
- 在 `coremain/mosdns.go` 的 `loadPluginsFromCfg` 中：
  - 处理 include 时：

    ```go
    for _, includePath := range cfg.Include {
        resolvedPath := includePath
        if len(cfg.baseDir) > 0 && !filepath.IsAbs(includePath) {
            resolvedPath = filepath.Join(cfg.baseDir, includePath)
        }
        subCfg, path, err := loadConfig(resolvedPath)
        ...
    }
    ```

  - 这样 include 路径统一以当前配置文件所在目录为基准，而不是进程工作目录，更直观、更稳健。

---

## 7. 新增与扩展的插件与数据源

本仓库在原版基础上新增了多个插件，主要围绕“规则集管理、在线更新、重查任务、UI 配置和开关”等场景。

### 7.1 新增 data_provider：sd_set / si_set

相关文件：

- `plugin/data_provider/sd_set/sd_set.go`
- `plugin/data_provider/si_set/si_set.go`
- `plugin/enabled_plugins.go` 中启用这两个插件

#### sd_set：SRS 域名规则集

- 类型名：`sd_set`。
- 功能：
  - 管理一个 JSON 配置文件（`Args.LocalConfig`），内部是多个 `RuleSource`：
    - `Name` / `Type` / `Files` / `URL` / `Enabled` / `AutoUpdate` / `UpdateIntervalHours` / `RuleCount` / `LastUpdated`。
  - 支持：
    - 从本地 `.srs` 文件加载域名规则；
    - 从远程 URL 下载压缩规则（zlib或其它形式），支持 SOCKS5 代理；
    - 自动更新：根据 `UpdateIntervalHours` 周期性拉取并重装；
    - 将规则构建为 `domain.MixMatcher`，提供 `DomainMatcherProvider` 接口给 matcher 使用。
- API：
  - 使用 `BP.RegAPI` 注册自己的 http 路由（基于 chi）；
  - 提供增删改查规则源、触发更新等接口；
  - 前端“规则管理”页面中的域名规则部分通过此插件管理。

#### si_set：SRS IP 规则集

- 类型名：`si_set`。
- 功能与 sd_set 类似，但处理对象为 IP 段：
  - 使用 `pkg/matcher/netlist` 构建 IP 列表匹配器；
  - 支持从 SRS / JSON / 文本格式加载 IP 规则；
  - 同样支持远程拉取、自动更新及 API 管理。

### 7.2 新增 executable：adguard_rule

相关文件：

- `plugin/executable/adguard/adguard.go`

功能：

- 解析与维护 AdGuard 风格的广告/过滤规则：
  - 工作目录由 `Args.Dir` 控制；
  - `config.json` 记录所有在线规则源：
    - ID / Name / URL / Enabled / AutoUpdate / UpdateIntervalHours / RuleCount / LastUpdated。
  - 每个源下载后转换为多种域名规则（allow/deny 等），分别放入 `allowMatcher`/`denyMatcher`。
- 支持 SOCKS5：
  - 参数 `Socks5` 可指定代理地址；
  - 通过 `x/net/proxy` 创建支持 context 的 SOCKS5 dialer。
- API：
  - 注册一组路由，用于：
    - 查看当前规则源和统计信息；
    - 添加/删除/修改规则源；
    - 手工触发某源的更新；
    - 查看解析后的规则数量等。

前端 UI 中“广告规则管理”等模块主要依赖这个插件。

### 7.3 新增 executable：domain_output

相关文件：

- `plugin/executable/domain_output/domain_output.go`

功能：

- 在请求流中统计域名访问，并周期性输出到文件／远程接口：
  - 关键参数：
    - `file_stat`：输出统计（域名 -> 计数）的文件。
    - `file_rule`：输出规则文件。
    - `gen_rule`：生成规则的模板直写字符串。
    - `pattern`：规则中用到的匹配模式片段。
    - `appended_string`：追加文本（可选）。
    - `max_entries`：累计访问次数达到该值时触发一次写出。
    - `dump_interval`：定时写出的间隔秒数。
    - `domain_set_url`：可选，将规则推送到某 HTTP 接口。
- 实现：
  - 在 `Exec` 中累积统计（基于 `map[string]int`），定期或被触发时写入文件；
  - worker goroutine 负责实际 I/O 与推送。

适合用来自动导出“最常访问的域名”生成规则集，与 `sd_set` 等组合使用。

### 7.4 新增 executable：requery

相关文件：

- `plugin/executable/requery/requery.go`
- 前端 index 页面中与“分流与缓存刷新”相关的 UI

功能：

- 管理定时“域名规则重查 / 刷新”任务：
  - 使用 `robfig/cron/v3` 作为调度器。
  - 配置文件 `requeryconfig.json` 中包含：
    - `DomainProcessing`：
      - 多个域名来源文件（`SourceFiles`），每个包含别名与路径；
      - 输出文件（`OutputFile`）。
    - `URLActions`：
      - `SaveRules`：保存规则时调用的 URL 列表；
      - `FlushRules`：刷新规则时调用的 URL 列表。
    - `Scheduler`：
      - cron 表达式或固定间隔。
    - `ExecutionSettings`：
      - 并发度、超时、重试等执行参数。
    - `Status`：
      - 当前任务状态（idle/running/failed/finished）、开始/结束时间等。
- 在初始化时：
  - 若发现上次运行时状态为 `running`，会标记为 `failed` 并补上结束时间，避免“卡死状态”。
- API：
  - 通过 chi 路由提供配置读取/修改、手动触发执行、查看进度等接口。

前端中 Requery 面板（带进度条和执行日志）的所有行为都依赖该插件。

### 7.5 新增 executable：rewrite

相关文件：

- `plugin/executable/rewrite/rewrite.go`

功能：

- 基于规则文件对域名进行重写：
  - 规则格式：每行 `pattern target`。
  - `pattern` 使用 mosdns 域名表达式；
  - `target` 可以是 IP 或域名：
    - IP：构造一个包含该 IP 的 A/AAAA 应答；
    - 域名：把查询重写为该域名，并通过内部 DNS client 查询。
- 核心逻辑：
  - 初始化时加载多个规则文件，并构建 `domain.MixMatcher[*rewriteTarget]`；
  - 在 `Exec` 中：
    - 判断是否为单问题的普通查询；
    - 查找匹配规则：
      - 未命中：调用下一个 executable；
      - 命中 IP：直接构造应答；
      - 命中域名：转向上游查询并返回结果。
- 提供 API：
  - 允许通过 HTTP 动态添加/删除规则、重新加载规则文件、查看现有规则等。

### 7.6 新增 executable：webinfo

相关文件：

- `plugin/executable/webinfo/webinfo.go`
- `demo/mosdns1/webinfo/*.json`

功能：

- 为 Web UI 提供持久化存储：
  - 如客户端别名、部分 UI 配置、Requery 配置等；
  - 将 JSON 文件存储在指定目录（demo 中即 `demo/mosdns1/webinfo`）。
- 提供 API：
  - 读取/写入这些 JSON，前端通过此 API 管理状态，而不是直接读写 YAML。

### 7.7 新增 executable：aliapi、cname_remover、switcher1~9

相关文件：

- `plugin/executable/aliapi/aliapi.go`
- `plugin/executable/cname_remover/cname_remover.go`
- `plugin/executable/switcher1` ~ `switcher9`

功能简述：

- `aliapi`：
  - 面向阿里系 API 的集成（例如某些 DNS 动态更新或其它云端配置），具体逻辑见源码。
- `cname_remover`：
  - 在应答中清理/规整特定 CNAME 结构，避免“多层 CNAME 导致的解析问题或泄露”。
- `switcher1` ~ `switcher9`：
  - 一组预定义的“场景开关”执行器，与 `switch` matcher 和 demo 中的模式相关：
    - 例如泄露/不泄露模式、是否屏蔽 AAAA、是否开启广告拦截等。
  - 在 `demo/mosdns1/config.yaml` 中大量使用 `switchN 'A'/'B'` 形式控制逻辑。

### 7.8 新增 matcher+exec：switch 插件

相关文件：

- `plugin/switch/switch/switch.go`

功能：

- 类型名：`switch`，既有 exec 部分也有 matcher 部分。
- 参数：

```yaml
args:
  name: some_switch_name
  state_file_path: /path/to/state.txt
```

- 行为：
  - 插件 init 时：
    - 从 `state_file_path` 读取当前值（若不存在则写入空值）；
    - 将自身注册到全局 registry 中（按 `name` 索引）。
  - Exec：
    - 本身对请求不做任何修改，完全作为“状态容器”使用。
  - Matcher：
    - 快捷语法：`switch "name:value"`；
    - 通过全局 registry 查找对应 `name` 的实例，比较当前值与期望值。
  - API：
    - `GET /api/plugins/<tag>/`：返回当前值。
    - `PUT/POST /api/plugins/<tag>/`：更新当前值。

与 demo + UI 结合：

- UI 中提供切换按钮，通过调用 `switch` API 写入状态文件；
- YAML 中使用 `switchN 'A'/'B'` 等表达式控制 sequence 行为；
- 实现“模式一键切换、重启后仍保持”的效果。

---

## 8. Demo 配置、预置规则与 release 目录

相关路径：

- `demo/mosdns1/**/*`
- `release/*`

### 8.1 demo/mosdns1

这是一个集成所有魔改功能的完整 demo 配置：

- `demo/mosdns1/config.yaml`：
  - 顶层配置：
    - 日志输出到 `/tmp/mosdns.log`；
    - `api.http: "0.0.0.0:9099"` 与 Web UI 绑定（README 中也强调此端口不可随意改）。
  - `include` 指向多个 `sub_config/*.yaml`：
    - `adguard.yaml`：AdGuard 规则插件。
    - `domain_output.yaml`：规则输出插件。
    - `rule_set.yaml`：各种 data_provider（sd_set / si_set / ip_set 等）。
    - `cache.yaml`：缓存插件定义。
    - `forward_local.yaml` / `forward_nocn.yaml` / `forward_nocn_ecs.yaml`：本地/国外/带 ECS 的上游定义。
    - `con_match.yaml`：组合匹配生成黑洞 IP。
    - `switch.yaml`：开关定义。
    - `not_in_list_*` / `main.yaml` / `for_singbox.yaml` / `forward_2.yaml`：主分流逻辑。
    - `webinfo.yaml`：前端配置存储。
    - `requery.yaml`：分流与缓存刷新逻辑。
  - server 部分：
    - `udp_all` / `tcp_all` 监听 `:53`，entry 指向 `sequence_6666`，并 `enable_audit: true`。
    - `udp_requery` / `tcp_requery` 监听 `:66` 用于刷新任务。
- 子目录：
  - `rule/`：各种规则列表（blocklist、geoip、pcdn 等）。
  - `srs/`：SRS 格式规则与 JSON 支持文件。
  - `gen/`：各种由 domain_output 或 requery 生成的中间文件。
  - `adguard/`：adguard_rule 使用的配置与下载结果。
  - `sub_config/`：前述拆分配置文件。
  - `webinfo/`：前端用的 JSON 状态文件。

这个 demo 基本就是一套“开箱即用”的综合玩法示例。

### 8.2 release 目录与 CI

- `release/config.yaml`：一个简化版打包配置示例。
- `release/mosdns`：示例二进制（测试用）。
- `release/mosdns-linux-amd64-v3.zip` 等：示例打包产物。
- `.github/workflows/release-main.yml`：
  - 新增 CI 工作流，用于 build 多平台二进制并创建 GitHub Release；
  - 与 README 中的版本命名规范、在线更新逻辑配合使用。

---

## 9. 依赖与构建环境变更

相关文件：

- `go.mod`
- `go.sum`

主要变化（与原版 v5 相比）：

- Go 版本：
  - 从 `go 1.22.0` 改为 `go 1.25.4`（并更新 toolchain 声明以适配新环境）。
- 第三方库升级：
  - `github.com/go-chi/chi/v5`、`github.com/spf13/viper`、`github.com/miekg/dns`、`github.com/prometheus/client_golang` 等均升级到较新的版本。
  - `github.com/mitchellh/mapstructure` 被替换为 `github.com/go-viper/mapstructure/v2`，并在 `coremain/run.go` 中调整了 decoder 配置。
  - 新增：
    - `github.com/robfig/cron/v3`（requery 调度）。
    - `github.com/sagernet/sing`（sd_set/si_set 读取 SRS/domain/varbin）。
    - 其它辅助库（如 `github.com/sourcegraph/conc` 等）。
- 整体构建逻辑依然遵循原版：
  - 模块路径不变：`module github.com/IrineSistiana/mosdns/v5`；
  - 保留原有 `replace github.com/nadoo/ipset => github.com/IrineSistiana/ipset`；
  - main 包仍在根目录，并通过 `main.go` 注册 `plugin` 和 `tools`。

---

## 10. 与原版兼容性与迁移建议

兼容性方面：

- 配置模型保持向后兼容：
  - 原版 v5 的 log/api/include/plugins 结构不变；
  - 原有插件类型和 sequence 语法仍然有效。
- 新增功能默认是“增量可选”的：
  - 审计、override、在线更新、UI 等只有在显式配置（例如 `api.http`、`enable_audit`、sd_set/si_set/adguard 等插件）时才会生效。
- 配置路径与 include 行为更稳健：
  - 使用 `baseDir` 和 `MainConfigBaseDir` 统一处理相对路径；
  - 一般情况下，只要 demo 能跑，你的旧配置改成类似结构也能正常迁移。

迁移时建议：

1. 先按原版 v5 文档确认现有配置在原版上工作正常。
2. 将配置复制到本仓库，在 `api.http` 中启用 UI 端口（推荐与 demo 相同的 9099）。
3. 如需使用覆盖配置（统一 socks5/ECS），参考 `testutils/mosdns_overrides_api.md` 和前端的“全局覆盖”模块。
4. 如需使用在线更新，确保二进制是从本仓库 Release 获取，并按照 README 的版本号规范命名。

---

本文件只描述“相对 upstream v5 的改动”。如需了解原版行为，请参阅 `docs/original-v5.md` 与官方 wiki；如需了解具体实现细节，请直接查阅对应源码文件。 

