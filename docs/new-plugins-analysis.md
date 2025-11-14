# 本仓库新增插件深度分析与耦合性说明

本文专门分析本仓库在原版 mosdns v5 基础上**新增的插件**：它们各自的作用、配置方式、实现思路，以及与原版框架之间的耦合关系。

原版整体架构与插件体系见：`docs/original-v5.md`  
本仓库所有改动概览见：`docs/changes-in-this-fork.md`

---

## 1. 新增插件一览

相对原版 v5，本仓库新增的插件目录为：

- Data Provider（数据源）  
  - `sd_set`（SRS 域名规则集）
  - `si_set`（SRS IP 规则集）

- Executable（执行器）
  - `adguard`（AdGuard 风格规则管理器，代码中包名 `adguard_rule`）
  - `domain_output`（域名统计与规则生成输出）
  - `requery`（重查/刷新任务调度）
  - `rewrite`（按规则重写域名或 IP）
  - `webinfo`（前端配置/状态持久化）
  - `aliapi`（面向阿里 API 的集成）
  - `cname_remover`（清理/规整 CNAME）
  - `switcher1` ~ `switcher9`（一组专用“开关”插件）

- 顶层插件类别：
  - `switch`（新增的顶层插件目录，内含 `switch` 插件）

原版所有插件保持保留与兼容；上述插件是**新增**而不是替换。

之后按功能模块逐一分析。

---

## 2. sd_set：SRS 域名规则集数据源

**路径**：`plugin/data_provider/sd_set/sd_set.go`  
**类型名（type）**：`sd_set`  
**注册方式**：

```go
const PluginType = "sd_set"

func init() {
    coremain.RegNewPluginFunc(PluginType, newSdSet, func() any { return new(Args) })
}
```

### 2.1 作用与典型场景

- 将多个“域名规则源”（本地文件或远程 URL）统一收敛为一个 `DomainMatcherProvider`：
  - 支持 SRS/压缩（zlib）格式；
  - 支持 HTTP 下载，带可选 SOCKS5 代理；
  - 支持自动更新与统计。
- 对上层而言，它提供一个**域名集合**接口，可被 `qname $tag` 之类的 matcher 使用，用于分流和过滤。

典型场景：

- 管理 geosite/geolocation 等巨大规则集，按需按源拆分；
- UI 上提供“规则源列表”，可视化启用/禁用某条规则源、查看规则数量和最近更新时间；
- 周期性自动拉取规则更新，而无需重启 mosdns。

### 2.2 配置结构

配置结构体（简化）：

```go
type Args struct {
    Socks5      string `yaml:"socks5,omitempty"`  // 可选 SOCKS5 代理
    LocalConfig string `yaml:"local_config"`      // JSON 配置文件路径
}
```

`LocalConfig` 指向一个 JSON 文件，内部是若干 `RuleSource`：

```go
type RuleSource struct {
    Name                string    `json:"name"`
    Type                string    `json:"type"`
    Files               string    `json:"files"`
    URL                 string    `json:"url"`
    Enabled             bool      `json:"enabled"`
    AutoUpdate          bool      `json:"auto_update"`
    UpdateIntervalHours int       `json:"update_interval_hours"`
    RuleCount           int       `json:"rule_count"`
    LastUpdated         time.Time `json:"last_updated"`
}
```

在 YAML 中使用示例：

```yaml
- tag: sd_main
  type: sd_set
  args:
    local_config: /path/to/sd_sources.json
    socks5: 127.0.0.1:7890
```

然后在其它插件中可以使用 `$sd_main` 作为域名集合引用。

### 2.3 实现要点

- 内部通过 `atomic.Value` 存储当前的域名 matcher 实例：
  - 读路径无锁，写路径重建 matcher 后一次性替换，避免长时间加锁。
- 加载/更新流程：
  1. 启动时读取 JSON 配置，构建 `sources` map。
  2. 调用 `reloadAllRules`：
     - 遍历 `Enabled == true` 的源；
     - 从本地文件或 `URL` 下载规则；
     - 解压/解析 SRS，将域名写入 `domain.MixMatcher`；
     - 统计规则数、更新时间并回写到 `RuleSource` 中。
  3. 将新 matcher 通过 `atomic.Value.Store` 替换旧 matcher。
  4. 启动后台 goroutine `backgroundUpdater`：
     - 周期性根据 `UpdateIntervalHours` 刷新启用的源并保存 JSON。

### 2.4 对外 API

- 在 `newSdSet` 里调用 `bp.RegAPI(p.api())`：
  - 使用 chi router 实现一组 HTTP API，包括：
    - 列出所有规则源；
    - 查看单个源详情；
    - 新增/删除/修改规则源；
    - 手动触发某源更新；
    - 触发全量重载等。
- 这些 API 主要由 Web UI 调用，不改变原有 mosdns 插件语义。

### 2.5 与原版的耦合性

耦合点主要体现在三处：

1. **插件注册与生命周期**  
   - 完全按原版的 `RegNewPluginFunc` 机制实现，构造函数 `newSdSet` 只拿到 `BP` 和 `Args`，不依赖额外的 core hack。
2. **数据接口**  
   - 实现接口 `data_provider.DomainMatcherProvider`，供原有 matcher 使用。
   - matcher 侧调用方式与原版 `domain_set` 插件完全一致，兼容原先语义。
3. **网络栈与 proxy**  
   - HTTP 访问使用标准库 `net/http` + `x/net/proxy`，与 mosdns 内部 upstream 逻辑无直接耦合；
   - 仅在配置上可能被全局 overrides（socks5）覆盖，这属于“可选增强”，不破坏原版逻辑。

总体来看，`sd_set` 是**纯新增的数据源插件**，与原版 v5 的核心逻辑是“低耦合、单向依赖”的：  
它依赖 core 插件框架与 matcher 接口，但原有代码不会因为它的存在而改变行为。

---

## 3. si_set：SRS IP 规则集数据源

**路径**：`plugin/data_provider/si_set/si_set.go`  
**类型名**：`si_set`

### 3.1 作用

- 与 `sd_set` 类似，但作用对象是 **IP / 网段**：
  - 从本地/远程规则源加载 IP 段；
  - 生成 `netlist.Matcher` 实例；
  - 对上层暴露为 `IPMatcherProvider`，供 `resp_ip`、`client_ip` 等 matcher 间接使用。

典型用例：

- 管理 geoip-cn、某运营商节点 IP 段、回源节点列表等；
- 与分流策略结合（比如 resp_ip 落在特定 IP 集时标记/走不同链路）。

### 3.2 实现与 `sd_set` 的对比

整体架构与 `sd_set` 基本一致：

- 使用 `atomic.Value` 存储当前活动的 `netlist.Matcher`；
- 使用 JSON 配置文件记录多个 `RuleSource`；
- 支持 HTTP 下载（可选 SOCKS5）和自动更新；
- 内部使用读写锁保护配置 map 与写文件操作。

差异点在于：

- 匹配器类型为 `netlist.Matcher` 而非 `domain.MixMatcher`；
- 规则解析逻辑不同，针对 IP/SRS 格式做了相应处理。

### 3.3 耦合性分析

与 `sd_set` 一样，`si_set` 对原版的耦合性也非常有限：

- 仅依赖 data_provider 接口与 matcher 子系统；
- 不修改 coremain、不影响原有插件；
- 上层使用方式与原有 `ip_set` 插件相似，配置迁移成本低。

---

## 4. adguard_rule：AdGuard 规则管理执行器

**路径**：`plugin/executable/adguard/adguard.go`  
**类型名**：`adguard_rule`

### 4.1 作用与场景

- 把 AdGuard 风格的“广告/拦截规则”整合到 mosdns 的域名匹配体系中：
  - 支持多个在线规则源，每个源对应一个 URL；
  - 支持自动更新（按小时），记录每条规则源的规则数量和最近更新时间；
  - 把 AdGuard 规则分类转换成可由 `domain.MixMatcher` 处理的模式；
  - 通过 API 与 Web UI 一起，实现可视化的广告规则管理。

典型场景：

- 从常见的 AdGuard 规则订阅源下载规则；
- 定期自动刷新；
- 在分流链路中，使用 `$adguard` 之类的标签配合 `qname`/`black_hole` 实现广告拦截。

### 4.2 配置结构

```yaml
- tag: adguard
  type: adguard_rule
  args:
    dir: /path/to/adguard
    socks5: 127.0.0.1:7890    # 可选
```

在 `dir` 下会维护：

- `config.json`：记录所有在线规则源及其状态；
- 若干下载后的规则文件，供解析使用。

### 4.3 实现要点

- 启动流程：
  - 创建工作目录；
  - 初始化 HTTP client（支持 SOCKS5）；
  - 读取 `config.json`，载入所有在线规则源；
  - 下载/解析规则，构建 `allowMatcher` 和 `denyMatcher`。
- 规则解析：
  - 支持 AdGuard 常见规则语法（域名类规则部分）；
  - 解析后转成 mosdns 的域名表达式，写入 `domain.MixMatcher`。
- 运行时行为：
  - 在流水线中，可被 sequence 某条 rule 调用，检查当前域名是否命中某类规则；
  - 通过 API 可以动态开启/关闭某个源、立即刷新等。

### 4.4 API 与 UI

通过 `bp.RegAPI` 注册 API（基于 chi）：

- 列出所有规则源；
- 添加/删除/修改源；
- 手工触发更新；
- 查看每个源的 RuleCount / LastUpdated；
- 部分操作会触发重新构建 matcher。

Web UI 的“广告规则管理”部分依赖这些 API。

### 4.5 耦合性分析

与原版的耦合主要有：

- 插件注册：标准 `RegNewPluginFunc`，生命周期受 coremain 管理。
- 域名匹配：依赖原版 `pkg/matcher/domain` 作为实现基础。
- Socks5：其 `socks5` 字段可被全局 overrides 覆盖（如果开启），属于增强点。

没有修改原有 forward/cache/matcher 的内部逻辑，只是在“域名规则来源”这一块新增了一个更高级的源管理器，属于**高层扩展、低层重用**的模式。

---

## 5. domain_output：域名统计与规则输出

**路径**：`plugin/executable/domain_output/domain_output.go`  
**类型名**：`domain_output`

### 5.1 作用

- 在流水线中统计经过的域名，周期性/按需：
  - 把访问统计写入统计文件；
  - 把生成的规则写入规则文件；
  - 可选将规则推送到某个 HTTP 接口，从而与 `sd_set`/`si_set` 等组合形成闭环。

典型用途：

- 挖掘“常访问但未在规则集中出现的域名”，生成自定义白/黑名单；
- 将这些域名推送到 sd_set，使规则集可以根据真实流量自动更新。

### 5.2 配置结构

```yaml
- tag: do_stat
  type: domain_output
  args:
    file_stat: /path/to/stat.txt
    file_rule: /path/to/rules.txt
    gen_rule: "suffix $RULE"
    pattern: "*"
    appended_string: ""
    max_entries: 1000
    dump_interval: 60
    domain_set_url: "http://127.0.0.1:9099/..."
```

核心字段：

- `file_stat`：统计文件；记录域名与访问频次。
- `file_rule`：规则文件（可直接被 domain_set/其他工具使用）。
- `gen_rule` / `pattern` / `appended_string`：生成规则文本的模板。
- `max_entries`：累积条目数达到该值会触发一次写出。
- `dump_interval`：后台 worker 的周期性写出间隔（秒）。
- `domain_set_url`：可选，向远程接口推送规则。

### 5.3 实现与行为

- `Exec` 中：
  - 对每个问题的 `q.Question` 提取域名（去尾点），累加计数；
  - 当 `entryCounter >= max_entries` 时向 `writeSignalChan` 发送信号。
- worker：
  - 使用 ticker + channel：
    - 到期或收到 signal 时，调用 `performWrite`；
    - `performWrite`：
      - 根据模式（周期 flush/save 等）构建一个快照 map；
      - 写入文件；
      - 如配置了 `domain_set_url`，则构造 HTTP 请求推送。

### 5.4 耦合性分析

- 与原版的耦合非常弱：
  - 完全通过 `query_context` 读取域名与请求信息；
  - 不修改应答、不影响其它插件，只做“旁路统计 + 文件/HTTP 输出”。
- 依赖：
  - 依赖 core 的 `BP` 与 `RegAPI` 用于暴露 API（如手动刷新）；
  - 不依赖 mosdns 内部 upstream 或 server 实现细节。

整体属于**旁路型增强插件**，对原逻辑几乎是零侵入。

---

## 6. requery：重查与刷新任务调度器

**路径**：`plugin/executable/requery/requery.go`  
**类型名**：`requery`

### 6.1 作用

- 周期性或按需执行“域名列表 → 规则 → 刷新上游/缓存”的批处理任务：
  - 支持多源（多个输入文件）；
  - 支持调度（cron 表达式或固定间隔）；
  - 支持通过 HTTP 调用外部接口（如刷新规则、通知上游）。

与 `domain_output` 和 `sd_set` / `adguard_rule` 配合时，可形成：

> 流量统计 → 生成/调整规则 → 周期性重查/刷新 → 外部系统（如上游或前端）同步

### 6.2 配置结构（概念级）

requery 使用一个 JSON 文件（由 `Args.File` 指定）作为主配置，核心结构包括：

- `DomainProcessing`：
  - `SourceFiles []SourceFile`：多个源文件（`Alias` + `Path`）。
  - `OutputFile string`：输出文件。
- `URLActions`：
  - `SaveRules []string`：写规则后调用的 URL 列表。
  - `FlushRules []string`：刷新规则时调用的 URL 列表。
- `Scheduler`：
  - 调度方式（cron/固定间隔）、起止时间等。
- `ExecutionSettings`：
  - 并发数、超时、重试策略等。
- `Status`：
  - 当前任务状态（idle/running/failed/finished）；
  - 上次开始/结束时间、错误信息等。

### 6.3 实现与行为

- 启动：
  - 加载 JSON 配置；
  - 若发现 `Status.TaskState == "running"`，认为上次运行中断，标记为 `failed` 并写回；
  - 初始化 `cron.Cron` 调度器；
  - 根据当前配置注册 job 并 `Start()`。
- 执行流程（简化）：
  1. 从 `SourceFiles` 读取域名列表；
  2. 按策略生成新规则并写入 `OutputFile`；
  3. 并发调用 `SaveRules` 与 `FlushRules` 指定的 URL；
  4. 更新 `Status`，记录运行时间与结果。
- API：
  - 提供获取/修改配置、手动触发任务、查询当前任务进度等接口，与 UI 集成。

### 6.4 耦合性分析

- 与 core 的耦合：
  - 通过标准插件接口和 `BP.RegAPI` 集成；
  - 不影响原有查询路径，对 DNS 请求路径的耦合接近于零。
- 与其它新插件的耦合：
  - 与 `domain_output`、`sd_set`、`adguard_rule` 在配置层组合使用；
  - 但从代码角度看，这些耦合是“松散的”：通过文件和 HTTP API 交互，而非直接调用其它插件内部函数。

综上，`requery` 是一个“独立的调度器插件”，严守 mosdns 原有插件边界，仅通过文件与 HTTP 与外部世界协作。

---

## 7. rewrite：按规则重写域名/IP

**路径**：`plugin/executable/rewrite/rewrite.go`  
**类型名**：`rewrite`

### 7.1 作用

- 对 DNS 请求的域名执行重写：
  - 将 `example.com` 重写为指定 IP；
  - 或将其重写为另一个域名（再通过上游查询），类似“自定义 CNAME”。

适合场景：

- 精细化控制特定域名走特定 IP；
- 在不改上游的前提下实现“域名转发/替换”。

### 7.2 规则格式

规则文件每行：

```text
pattern target
```

- `pattern`：mosdns 的域名表达式（如 `suffix example.com`、`full foo.bar` 等）。
- `target`：
  - 若为 IP，则返回构造好的 A/AAAA 应答；
  - 若为域名，则：
    - 构造一个新请求 `qname = target`；
    - 使用内部 DNS client（指向配置的上游）进行查询；
    - 把结果作为当前请求的应答。

### 7.3 配置结构

```yaml
- tag: rewrite_main
  type: rewrite
  args:
    files:
      - /path/to/rewrite_rules.txt
    dns: "8.8.8.8:53"  # 上游 DNS
```

### 7.4 实现与行为

- 初始化时：
  - 解析 `files` 中的所有规则文件；
  - 使用 `domain.MixMatcher[*rewriteTarget]` 构建模式 → 目标映射；
  - 支持多文件拼接；不存在的文件在启动时可以忽略。
- Exec 逻辑：
  - 仅处理“单问题”请求（只含一个 Question），否则直接走 `next`；
  - 按 `q.Question[0].Name` 查询 matcher；
    - 未命中：`next.ExecNext`，不做改变；
    - 命中 IP：构造响应（按上游 TTL 或固定 TTL）并返回；
    - 命中域名：调用内部 `dns.Client` 向指定上游查询，再写回当前上下文。

### 7.5 耦合性分析

- 对原版的耦合：
  - 使用 `pkg/matcher/domain` + `domain.Load`，完全复用原始域名表达式解析逻辑；
  - 使用 `pkg/query_context` 操作请求/响应；
  - 使用 `miekg/dns` 库构造响应。
- 不改变其它插件行为，也不依赖新加的 core 逻辑，是一个**典型的“插件内闭环”功能**。

---

## 8. webinfo：前端配置与状态持久化

**路径**：`plugin/executable/webinfo/webinfo.go`  
**类型名**：`webinfo`

### 8.1 作用

- 提供一个“任意 JSON 数据的持久化存储”：
  - 后端只负责从 `Args.File` 指定的 JSON 读/写；
  - 结构完全透明，前端可以随意约定；
  - 适合存储 Web UI 的一些配置与状态，如：
    - 客户端别名表；
    - Requery 任务 UI 设置；
    - 其它与 DNS 核心逻辑无关的 UI 元数据。

### 8.2 配置结构

```yaml
- tag: webinfo
  type: webinfo
  args:
    file: /path/to/webinfo.json
```

`webinfo.json` 可能是任何 JSON 结构，demo 中给了示例。

### 8.3 API 与行为

- 启动时：
  - 若文件不存在，初始化为一个空 map；
  - 若存在，则读入 `interface{}` 存在内存。
- API：
  - `GET /api/plugins/<tag>/`：
    - 返回当前 JSON 数据。
  - `PUT/POST /api/plugins/<tag>/`：
    - 以 JSON 形式传入新数据；
    - 原子写回文件（`.tmp` + `Rename`）。

锁保护：

- 读写均使用 `sync.RWMutex`，避免并发修改/读写文件时出错。

### 8.4 耦合性分析

- 与原版核心完全解耦：
  - 不访问 DNS 请求/响应；
  - 不影响 upstream/sequence/server 行为；
  - 仅靠 API 为 UI 提供一个“简单的 KV/JSON 存储”。
- 对 core 的唯一依赖是 `RegNewPluginFunc` 和 `BP.RegAPI`。

---

## 9. aliapi / cname_remover：附加功能插件

这两个插件功能相对独立，耦合性较低。

### 9.1 aliapi：面向阿里 API

**路径**：`plugin/executable/aliapi/aliapi.go`  
**类型名**：`aliapi`

简要功能（从代码与命名推断）：

- 提供与阿里相关 Web API 的集成（例如阿里云 DNS 解析的更新/查询等）；
- 在 mosdns 中作为一个“控制型插件”存在：
  - 通过 HTTP API 与前端交互；
  - 在 Exec 中根据配置调用外部 API，可能用于动态构建某些响应或者管理外部资源。

耦合性：

- 与原版基本无耦合，仅依赖插件框架和 HTTP client；
- 与当前仓库的 UI 有较强耦合（UI 暴露相应按钮/表单触发该插件 API）。

### 9.2 cname_remover：CNAME 规整

**路径**：`plugin/executable/cname_remover/cname_remover.go`  
**类型名**：`cname_remover`

作用（从命名与典型需求推断）：

- 对返回的应答进行 CNAME 清理或规整：
  - 如折叠多层 CNAME；
  - 或移除某些特定的 CNAME 链以减少泄露/缩短解析链路；
  - 也可能用于处理某些特殊上游的 CNAME 行为。

耦合性：

- 使用 `query_context` 和 `miekg/dns` 操作应答；
- 对原版插件没有侵入，仅作为后处理步骤被插入 sequence 中；
- 可在 UI 或 YAML 配置中按需启用/关闭。

---

## 10. switcher1 ~ switcher9：特定场景开关

**路径**：`plugin/executable/switcher*/switcher*.go`  
**类型名**：`switcher1` ~ `switcher9`

### 10.1 作用

- 为特定的配置场景提供独立的“状态开关”，与 demo 和 UI 深度绑定；
- 每个 switcher 实际上：
  - 在 Exec 层不做实际处理，只用作“存储某个当前值”；
  - 在 matcher 层提供快捷语法：

    ```yaml
    - matches: switcher1 'A'
      exec: ...
    ```

  - 在 API 层提供简单接口，用于前端修改值并写回到对应文件。

与新加入的 `switch` 插件相比：

- `switch` 是一个通用、多实例的 switch 框架；
- `switcherN` 是为兼容/迁移现有配置和 UI 而保留的“专用版本”，每个有自己固定的文件/接口约定。

### 10.2 实现特点

以 `switcher9` 为例：

- 全局指针 `globalSwitcher9 *Switcher9` 保存单例；
- `Args` 通常只有一个字段（例如 `InitialValue` 或文件路径）；  
  有的 switcher 直接用该字段作为 state file 路径，有的可能稍有不同。
- 初始化：
  - 保证目录存在；
  - 读取 state file 不存在则写入空值；
  - 注册 API：
    - `GET /show`：返回当前值。
    - `POST /post`：更新值（JSON 或表单），并写回文件。
- 匹配器：
  - 通过 `sequence.MustRegMatchQuickSetup` 注册；
  - `QuickSetup` 把 `switcherN 'X'` 形式解析为期望值 `X`；
  - `Match` 时：拿全局 switcher 的当前值与期望值比较。

### 10.3 耦合性分析

- 与 core 插件模型的耦合：
  - 用的是标准插件注册机制与 `sequence` 的 quick setup 接口；
  - 不修改 sequence 的语义，只是新增了一类 matcher。
- 与原有配置/生态的耦合：
  - demo 与 UI 中大量使用这几个 switcher；
  - 文件路径通常写死或可配置，便于在宿主系统中持久化“开关状态”。

从设计角度看：

- `switcherN` 更像是“为现有生产配置量身定制的开关插件”，是**高层场景耦合、高度定制**；
- `switch` 插件则是通用型开关建议今后优先使用。

---

## 11. switch：通用开关插件

**路径**：`plugin/switch/switch/switch.go`  
**类型名**：`switch`

### 11.1 作用

- 提供通用的“命名开关 + 持久化”能力：
  - 每个开关实例有一个 name 和 state file；
  - 在 matcher 中可通过 `switch "name:value"` 判断开关值；
  - 通过 API 修改值，并写回文件。

### 11.2 配置结构

```yaml
- tag: switch_my_feature
  type: switch
  args:
    name: my_feature
    state_file_path: /path/to/my_feature.state
```

### 11.3 实现要点

- 全局 registry：

```go
var globalRegistry = struct {
    sync.RWMutex
    instances map[string]*Switch
}{ instances: make(map[string]*Switch) }
```

- 初始化：
  - 创建 state file 目录；
  - 若文件存在，读取初始值；否则写入空值；
  - 将实例按 name 注册进 registry；
  - 注册 API：
    - `GET /`：返回当前值（text/plain）。
    - `PUT/POST /`：更新值（支持 JSON/body/form），写回文件。
- 匹配：
  - `QuickSetup` 把 `"my_switch:on"` 解析为 `name=my_switch, expected=on`；
  - `Match` 时从 registry 查找实例并比较值。

### 11.4 耦合性分析

- 与 core 的耦合仅限：
  - 插件注册；
  - `sequence` 的 matcher QuickSetup；
  - API 路由。
- 与现有配置的耦合：
  - 可替代一部分 `switcherN` 用法，未来有利于归一；
  - 不与原版 v5 配置冲突，属于增量能力。

从设计上看，`switch` 是一个比 `switcherN` 更推荐的通用方案，适合用于新的场景开关实现。

---

## 12. 综合耦合性总结

从整体上看，这些新增插件与原版 mosdns v5 的关系可以归纳为：

1. **插件框架层面**  
   - 所有插件都通过标准的 `RegNewPluginFunc`/`sequence.MustRegExecQuickSetup`/`MustRegMatchQuickSetup` 接入；
   - 没有修改 coremain 的公共 API 或原有插件的接口；
   - 原版插件可以与这些新插件共存，并按需互相引用。

2. **数据与执行路径层面**  
   - data_provider 类（sd_set/si_set）只作为 matcher 的数据依赖，与请求处理路径是“只读、单向”的关系；
   - 大部分 executable（adguard/domain_output/requery/rewrite/webinfo 等）在 Exec 中要么只读请求，要么对应答做局部改写，不影响其它插件的语义；
   - true 侵入性的改动基本没有，更多是“新增能力点”。

3. **外部耦合（UI/HTTP/文件）**  
   - 很多插件通过 `BP.RegAPI` 暴露 API，服务于 Web UI；
   - 插件间的复杂协作多数通过“文件 + HTTP”实现，而不是直接调用彼此的 Go 函数，降低了代码级耦合：
     - 例如：domain_output → 文件 → sd_set；  
       requery → 文件 + HTTP → 上游或 UI。

4. **向后兼容性**  
   - 原版 mosdns v5 的配置与插件模型保持不变；
   - 如果不启用这些新增插件，行为应与原版 v5 基本一致；
   - 新增插件主要提供：
     - 更丰富的规则管理能力（sd_set/si_set/adguard/domain_output）；
     - 更强的运维/调度能力（requery/webinfo/switch）；
     - 更适合 UI 的开关接口与专用逻辑（switcher1~9/aliapi/cname_remover）。

从工程设计角度看，这套改动遵循的是“在原有框架之上新增插件层功能，而不是改变框架本身”的策略：  
耦合集中在插件接口与少量路径扩展上，核心 DNS 处理引擎和原有插件的工作方式保持稳定，这对维护和回滚都非常有利。

