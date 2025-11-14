# mosdns v5 原版说明（基于 upstream 文档与源码）

本文档概述 mosdns v5 原版的架构与功能，参考：

- 源码仓库：`github.com/IrineSistiana/mosdns/v5`
- 官方文档：<https://irine-sistiana.gitbook.io/mosdns-wiki/mosdns-v5>

本仓库在此基础上做了大量扩展，改动说明见 `docs/changes-in-this-fork.md`。

---

## 1. 项目定位与总体架构

mosdns v5 的官方定位是“插件化的 DNS 转发器”（modular DNS forwarder）：

- 既可以作为本地 DNS 服务器使用，也可以作为多协议的 DNS 中继/转发器。
- 所有行为都围绕“插件流水线”构建，绝大部分功能通过组合插件完成。

源码上大致分为几个层次：

- `coremain`：主程序、配置加载、插件初始化、HTTP API、metrics、pprof。
- `plugin`：所有插件实现，分为 data_provider / matcher / executable / server 等子类别。
- `pkg`：公共基础库（缓存、上游访问、匹配器、速率限制、工具函数等）。
- `mlog`：日志系统封装，基于 zap。
- `tools`：命令行工具（配置生成/转换、上游探测等）。

mosdns 的典型使用方式是：

1. 用 `sequence` 定义一次 DNS 请求的处理流程（分流、缓存、过滤等）。
2. 用 `udp_server` / `tcp_server` / `http_server` / `quic_server` 等 server 插件接收来自客户端的查询，并将请求交给某个 sequence。

---

## 2. 启动方式与命令行

主入口为 `main.go`，最终生成的可执行文件通常命名为 `mosdns`。核心命令：

- `mosdns start [-c config_file] [-d working_dir] [--cpu N]`
  - `-c/--config`：指定配置文件；为空时自动在当前目录查找以 `config` 开头的配置文件。
  - `-d/--dir`：启动前切换工作目录，便于相对路径引用规则文件。
  - `--cpu`：设置 `runtime.GOMAXPROCS`。
- `mosdns service ...`
  - 使用 `kardianos/service` 将 mosdns 注册为系统服务，提供 install/uninstall/start/stop/restart/status 等子命令。
- `mosdns config ...`
  - `mosdns config gen <config_file>`：生成一份最小模板配置。
  - `mosdns config conv -i in -o out`：在 YAML / TOML / JSON 之间转换配置。
- `mosdns probe ...`
  - 各种上游探测工具，例如空闲超时、是否支持连接复用、是否支持 pipeline。
- `mosdns version`
  - 输出当前二进制内嵌的版本号（默认为 `dev/unknown`，可通过构建参数覆盖）。

---

## 3. 配置文件结构与格式

官方文档默认使用 YAML，但 mosdns 基于 viper，也支持 JSON / TOML，扩展名不同会自动识别格式。

顶级配置结构（`coremain.Config`）：

```yaml
log:
  level: info
  file: ""        # 可选，日志文件；为空输出到 stderr
  production: false

api:
  http: ""        # 可选，HTTP API 监听地址，例如 "127.0.0.1:8080"

include:
  - other_config.yaml

plugins:
  - tag: ...
    type: ...
    args: ...
```

字段说明：

- `log`：
  - `level`：日志等级，传给 `zapcore.ParseLevel`（如 `debug|info|warn|error`），默认 `info`。
  - `file`：日志输出文件路径。官方说明中把写文件视为调试用；panic 仍会输出到 stderr。
  - `production`：为 `true` 时使用 JSON 编码（生产模式），否则使用开发模式 console 编码。
- `api`（实验性）：
  - 设置 `http` 后才会启动 HTTP API 服务器；未设置则不监听 HTTP。
- `include`：
  - 列出其它配置文件路径。加载时会先递归处理 include 中的配置，再处理当前文件。
  - 官方说明中强调：“include 的插件会比本配置文件中的插件先初始化”。
- `plugins`：
  - `tag`：插件实例的标签，用于被其它插件引用（例如 server 的 `entry` 指向某个 sequence 的 tag）。
  - `type`：插件类型字符串，对应 `plugin/enabled_plugins.go` 中注册的插件。
  - `args`：插件的具体配置，结构由各插件定义。

官方示例模板配置（等价逻辑）：

- 一个 `forward` 插件转发到 `https://8.8.8.8/dns-query`；
- 一个 UDP server 和一个 TCP server 监听 53 端口，entry 指向上面的 forward 插件。

---

## 4. 插件体系概览

插件按职责大致分为四类：

1. Data Provider（数据源）
   - 例如：
     - `data_provider/domain_set`：提供域名集合。
     - `data_provider/ip_set`：提供 IP 集合。
   - 这些插件本身不直接处理请求，而是被 matcher / executable 查询使用。

2. Matcher（匹配器）
   - plugin/matcher 目录下：
     - `client_ip` / `resp_ip` / `ptr_ip`：按客户端 IP、应答 IP、PTR 查询 IP 匹配。
     - `qname` / `qtype` / `qclass` / `rcode`：按域名、记录类型、class、返回码匹配。
     - `cname`：按 CNAME 匹配。
     - `has_resp` / `has_wanted_ans`：判断是否已有应答，是否有“目标类型”记录。
     - `env`：按环境变量匹配。
     - `random`：随机概率匹配（用于 A/B 流量）。
     - `string_exp`：实验性通用字符串表达式匹配。
   - 匹配器通常在 sequence 的 `matches` 字段内使用。

3. Executable（执行器）
   - plugin/executable 目录下，负责“做事”：
     - 上游和网络相关：
       - `forward` / `forward_edns0opt`：访问上游 DNS（UDP/TCP/DoT/DoH/DoQ 等）。
       - `ecs_handler`：为请求附加 ECS 选项。
     - 缓存与统计：
       - `cache`：应答缓存。
       - `metrics_collector` / `query_summary`：统计并导出 metrics。
       - `rate_limiter`：速率限制。
     - 过滤与改写：
       - `hosts`：类似系统 hosts 文件，为域名指定固定 IP。
       - `black_hole`：黑洞应答，返回指定 IP。
       - `ttl`：修改 TTL。
       - `redirect`：将 A 域名的查询重写为 B 域名。
       - `reverse_lookup`：支持 PTR 或 HTTP 查询“某 IP 对应哪些由 mosdns 处理过的域名”。
       - `drop_resp`：丢弃当前应答，为后续重查/分流腾地方。
     - 路由控制：
       - `ipset` / `nftset`：把应答 IP 写入 ipset/nftables，以便系统级路由策略使用。
     - 控制流程：
       - `sequence`：执行一串 rule 的“流水线”（见下节）。
       - `sequence/fallback`：当 primary 失败或超时后切到 secondary。
       - `sleep`：插入延迟。
       - `debug_print` / `arbitrary`：调试与手工构造应答。

4. Server（服务端）
   - plugin/server 目录下：
     - `udp_server`：UDP DNS 服务器。
     - `tcp_server`：TCP DNS 服务器；可选 TLS。
     - `http_server`：HTTP DNS 服务器。
     - `quic_server`：QUIC 协议 DNS 服务器。
   - 每个 server 通过 `entry` 字段指向一个可执行插件（通常是 `sequence`），由该 entry 负责完成所有处理逻辑。

还有一个特殊的 `mark` 插件：

- 既是 executable 也是 matcher，用于给请求打“标记”或者根据标签匹配，用来实现复杂的多阶段分流。

---

## 5. sequence 插件（官方核心概念）

sequence 是官方文档重点说明的插件，它把插件组合成类似 iptables/nftables chain 的“规则链”。

### 5.1 基础结构

一个 sequence 插件配置示例：

```yaml
- tag: my_seq
  type: sequence
  args:
    - matches:  # rule 1
        - qname &./adblock.txt
      exec: reject 3

    - matches:  # rule 2
        - has_resp
      exec: accept

    - exec: forward https://8.8.8.8/dns-query  # rule 3，无 matches
```

含义：

- `args` 是一个 rule 列表，顺序执行。
- 每个 rule：
  - `matches`：匹配器列表（与关系）；全部为 true 时才命中。
  - `exec`：命中后执行的操作（某个 executable 插件的“快捷语法”）。
- 执行流程：
  - 从第一个 rule 开始按顺序评估；
  - 若命中 rule，就执行对应的 exec；
  - 执行中如果产生了 accept/reject/return/错误，则 sequence 立即返回；
  - 否则继续下一条 rule；全部执行完仍未产生终止条件，则“正常返回”给上层调用者。

### 5.2 官方说明中的常用匹配器

官方文档中对 sequence 支持的 matcher 做了详细说明，这里只列出常用部分：

- `qname { domain_rule | $domain_set | &file } ...`
  - 按域名匹配，可以直接写域名表达式、引用 domain_set 插件 tag 或列表文件。
- `resp_ip { ip | $ip_set | &file } ...`
  - 按应答 IP 匹配（A/AAAA 记录）。
- `client_ip { ip | $ip_set | &file } ...`
  - 按客户端 IP 匹配。
- `qtype` / `qclass` / `rcode`：
  - 按类型、class、返回码匹配。
- `cname` / `ptr_ip`：
  - 按 CNAME 或 PTR 请求的 IP 匹配。
- `has_resp`：
  - 当前上下文中是否已有应答。
- `has_wanted_ans`：
  - 是否有“与请求类型一致”的 ANSWER 记录（例如请求 A 且 ANSECTION 中存在 A 记录）。
- `mark`：
  - 使用 `mark` 插件设置的标记。
- `env KEY [VALUE]`：
  - 按环境变量匹配。
- `random p`：
  - 按概率 p 返回 true，用于做随机分流。
- `_true` / `_false`：
  - 永远为真/假，用于占位或调试。

### 5.3 官方说明中的常用操作

只简要列出最常见的几类：

- 查询与缓存：
  - `forward upstream...`：转发到一个或多个上游；多个上游并发查询，取最快应答。
  - `cache [size]`：查询/写入缓存；不支持带 ECS 的缓存。
  - `drop_resp`：丢弃已有应答，方便重查。
- 过滤与改写：
  - `black_hole [ip]...`：构造包含指定 IP 的假应答。
  - `hosts`：本地 hosts 映射。
  - `ttl`：改写 TTL。
  - `redirect`：把请求域名 A 的查询重写为 B 域名。
  - `reverse_lookup`：根据 IP 查找近期由 mosdns 处理过的域名。
- 静态路由控制：
  - `ipset` / `nftset`：写入 ipset / nftables，交由系统路由处理。
- 控制流：
  - `accept`：立即结束并返回当前应答。
  - `reject [rcode]`：返回错误码（例如 `reject 3` → NXDOMAIN）。
  - `return`：结束当前 sequence，返回上层调用点。
  - `goto <seq_tag>`：跳转到另一个 sequence，并不再返回。
  - `jump <seq_tag>`：调用另一个 sequence，执行完后回到当前 sequence 的下一条 rule。

官方推荐的常见套路就是：

- sequence 内：
  - 先按域名/IP/类型做广告拦截、黑白名单；
  - 再做缓存命中检查；
  - 最后转发到上游。
- server 插件只负责把请求送入某个 sequence。

---

## 6. HTTP API、Metrics 与 pprof

当配置中设置了 `api.http`，mosdns 会启动一个 HTTP 服务器，提供：

- 插件 API（实验性）：
  - 路径形如：`http://<api_addr>/plugins/<plugin_tag>/...`。
  - 每个插件可以在初始化时注册自己的子路由，实现管理/控制 API，例如：
    - cache 的统计查看；
    - 某些 data_provider 的规则热更新接口；
    - 其它运维数据输出。
- Prometheus 指标：
  - `http://<api_addr>/metrics`，包含基础 runtime 指标和一些插件指标。
- pprof：
  - `http://<api_addr>/debug/pprof/` 及其子路径（`profile` / `trace` / `symbol` 等），用于性能和内存分析。

官方文档把这些 API 都标记为“实验性”（不保证长期兼容），实际路径以具体版本为准。

---

## 7. 系统服务与命令行工具

### 7.1 系统服务（service）

`mosdns service` 子命令基于 `kardianos/service` 实现，目标是快速把 mosdns 装成系统级服务。

官方说明：

- 理论支持：Windows、Linux（systemd / Upstart / SysV）、macOS / Launchd 等。
- 已实测可用：Windows、Ubuntu、Debian 等。
- OpenWrt 不支持该方式（推荐使用专门的 OpenWrt 集成包）。

典型使用：

- 安装服务：

```bash
mosdns service install -d /path/to/working_dir -c /path/to/config.yaml
```

- 启动 / 停止 / 卸载：

```bash
mosdns service start
mosdns service stop
mosdns service uninstall
```

### 7.2 工具子命令（tools）

官方内置两类辅助命令：

- `mosdns config`：
  - `gen`：生成模板配置（如最小转发配置）。
  - `conv`：转换配置格式（YAML/JSON/TOML）。
- `mosdns probe`：
  - `idle-timeout {tcp|tls}://server`：探测上游空闲超时。
  - `conn-reuse {tcp|tls}://server`：检查是否支持 RFC 1035 连接复用。
  - `pipeline {tcp|tls}://server`：检查是否支持 RFC 7766 query pipelining。

---

## 8. OpenWrt 生态与第三方集成（官方链接）

官方文档中对 OpenWrt 只提供简单引用，不直接支持系统服务安装。常见的第三方集成有：

- `sbwml/luci-app-mosdns`：在 OpenWrt 上提供 Luci 管理界面。
- `QiuSimons/openwrt-mos`：另一套 OpenWrt 集成方案。

这些项目通常把 mosdns 打包为 OpenWrt 软件包，并提供 Web 前端/脚本集成，但与 mosdns v5 核心的配置/插件模型保持兼容。

---

本文件仅描述 upstream 原版 v5 的行为。本仓库在此基础上增加了大量 UI、审计、在线更新、全局 overrides 以及新插件，详见 `docs/changes-in-this-fork.md`。

