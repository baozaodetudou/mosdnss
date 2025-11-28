# mosdns 请求流程解析


## 1. 启动阶段

### 1.1 CLI 与工作目录
1. 在仓库根目录执行 `mosdns start --dir demo/mosdns1`。
2. `coremain.NewServer` 将工作目录切换到 `demo/mosdns1`，令相对路径（include、dump、rule）均以该目录为基准。
3. `loadConfig` 读取 `config.yaml`，基于 `include` 顺序依次加载子配置，确保依赖先于引用被注册。
4. 注册 HTTP API：`api.http: 0.0.0.0:9099`；日志写入 `/tmp/mosdns.log`。

### 1.2 include 顺序与职责
| 顺序 | 文件 | 作用 |
|------|------|------|
| 1 | `sub_config/adguard.yaml` | AdGuard 插件定义（按需启用） |
| 2 | `domain_output.yaml` | 运行中生成 `gen/*.txt` 的统计/规则插件 |
| 3 | `rule_set.yaml` | 所有 `domain_set`/`ip_set`/`geosite` 定义 |
| 4 | `cache.yaml` | 各层缓存及 dump 策略 |
| 5-9 | `forward_local.yaml` / `_nocn.yaml` / `_nocn_ecs.yaml` / `forward_1.yaml` / `forward_2.yaml` | 定义国内/国外/节点上游与复用序列 |
| 10 | `con_match.yaml` | 并发匹配与 blackhole 标记逻辑 |
| 11-15 | `switch.yaml`、`not_in_list_*`、`main.yaml`、`for_singbox.yaml` | 配置动态开关、主分流、下游导出 |
| 16-17 | `forward_2.yaml`、`webinfo.yaml`、`requery.yaml` | 暴露主分流为内部 upstream、前端信息与审计刷新 |

### 1.3 入口插件
- `sequence_all_single`: 直连 `sequence_main`，可改为上游 `forward_all_in`。
- `sequence_all`: `fallback` 包装，primary/secondary 均指向 `sequence_all_single`，阈值 300ms，用于主流程失效时快速重试。
- `sequence_6666`: 面向客户端的综合序列（过滤 -> client 保护 -> 缓存 -> 主分流）。
- `udp_all` / `tcp_all`: 监听 `:53`，entry 均为 `sequence_6666`，且 `enable_audit` 打开，可在 `mosdns.log` 中看到审计记录。

## 2. 查询生命周期（以 `nslookup baidu.com` 为例）

### 2.1 客户端 → 入口
1. `nslookup` 通过 UDP 53 将请求发往 `127.0.0.1`。
2. `udp_all`/`tcp_all` 监听 `:53`，将报文送入 `sequence_6666`。
3. `sequence_6666` 会同步写审计日志，便于复盘所有过滤/标记。

### 2.2 `sequence_6666` 顺序判定
> 默认开关值来自 `rule/switch*.txt`：`switch1=B`、`switch2=A`、`switch3=A`、`switch4=A`、`switch5=A`、`switch6=B`、`switch7=A`。

| 步骤 | 触发条件 | 动作 | 备注 |
|------|----------|------|------|
| 1. 屏蔽 AAAA | `switch6=A` 且 `qtype=AAAA` | 直接 `reject` (NXDOMAIN) | 对标“屏蔽 IPv6 解析”。默认 `B` 不拦截。 |
| 2. 屏蔽特殊类型 | `switch5=A` 且 `qtype ∈ {SOA, PTR, HTTPS}` | `reject` | 即指令中的 SOA/PTR/HTTPS 屏蔽。 |
| 3. DDNS 优先 | `qname ∈ $ddnslist` | 标记 `mark1` → `$forward_local` → 返回 | 满足“直接走国内 DNS 并结束”。 |
| 4. 黑名单过滤 | `switch1=A` | 依次检查 `nov4list`、`nov6list`、`blocklist`，命中即 `reject` | 对应“无 V4/V6/黑名单”三步判定。 |
| 5. AdGuard | `switch7=A` | 先查本地 `$adguard`，再查在线广告列表；命中即 `reject` | 说明文中两次提及的“在线广告域名”合并于此。 |
| 6. 客户端白名单 | `switch2=A` 且 `client_ip.txt` 不含当前源 IP | 设置 `mark3`，强制 `$sequence_local`，随后 `accept` | 仅允许名单内客户端走后续完整分流。 |
| 7. rewrite & 类型检查 | 见 §2.3 | 继续深入主分流前的最后一步。 |

> 若希望体验完整分流，可把发起端 IP 写入 `client_ip.txt` 或将 `switch2` 改为 `B` 并重启。

### 2.3 `sequence_all` 进入 `sequence_main`
1. **重写 (`exec: $rewrite`)**：命中 `rule/rewrite.txt` 则改写域名并立即查询 rewrite 上游，满足“重定向域名”需求。
2. **类型分支**：
   - **非 A/AAAA**：打 `mark66`，若 `geosite_cn` 匹配则标 `mark99` 并走 `$sequence_local`，否则视为国外域名，走 `$sequence_google`。
   - **A/AAAA**：继续执行名单判定（灰/白名单、自生成/在线列表等）。
3. **灰名单 (`$greylist`)**：`mark22` 直接触发 `sequence_fakeip`（sing-box fakeip），返回后写 `mark777`/`mark999` 并在 `gen/fakeiplist.txt` 记录。
4. **白名单 (`$whitelist`)**：`mark11` → `$sequence_local`；若结果缺少 A 或 AAAA，分别写入 `nov4list`/`nov6list`；再检查 `CNtoMihomo`，决定是向 mihomo fakeip（国内代理）还是返回 real IP。
5. **realip 列表 (`$realiplist`)**：`mark33` 直接改走 `$sequence_google`，确保特殊域名永远使用国外上游。

### 2.4 自生成/在线清单与副作用
1. **生成黑洞标签**：`exec: $gen_conc` 将 geosite/domains 匹配结果转换为 `127.0.0.x/::x`，并在 `resp_ip` 处置：
   - `127.0.0.1` → `mark888`（认定为国内） → `$sequence_local`
   - `127.0.0.2` → `mark999`（fakeip） → `$sequence_fakeip`
   - `127.0.0.3` → `mark666`（未知） → 转 `$conc_lookup` 后根据真实 IP 决策。
2. **自生成/在线名单写入**：
   - 命中在线国外清单且不在自生成清单时，会追加到 `gen/foreignlist.txt`；国内同理写 `gen/domesticlist.txt`。
   - 缺少 A/AAAA 时分别写入 `gen/nov4.txt`、`gen/nov6.txt`，用于后续无解析快速拦截。
3. **FakeIP 与 mihomo**：`mark777` 会触发 `my_fakeiplist` 输出，可供 sing-box/mihomo 复用。若 `CNtoMihomo` 开启且域名判定为国内，将由 mihomo 负责 fakeip，国外域名仍交给 sing-box。

### 2.5 列表外域名与 `sequence_not_in_list_*`
| 模式 | 初始动作 | 兜底逻辑 | fakeip 条件 |
|------|----------|----------|-------------|
| 泄露 (`sequence_not_in_list_leak`) | `mark68` → `$sequence_local`（国内） | 国内返回 rcode0 无 IP 或 rcode3 时打 `mark123`，转 `$sequence_google`；若最终 IP 不属中国 (`mark89`)，交给 `$sequence_fakeip` | `mark89` 成立即进入 fakeip；若 IP 属中国则继续本地 real IP 或 mihomo。 |
| 不泄露 (`sequence_not_in_list_noleak`) | 直接 `$sequence_google_node`（带 ECS） | 国外查询 rcode2/5 用 `$sequence_local` 兜底并写 `mark456`；其余同上 | 同样通过 `mark89` 触发 fakeip。 |

### 2.6 兼容模式 vs 安全模式
> 指令中的“兼容模式（国内优先）”与“安全模式（国外优先）”即 `switch3` 控制的两条泳道。

#### 兼容模式（国内优先）
1. 首次查询命中国内上游（含 `cache_cn`）。
2. 若 `rcode0` 但无 AAAA，或 `rcode3`，将域名写入 `nov6list`；若 `rcode0` 无 A，则 fallback 到国外上游，再次失败则写入 `nov4list`。
3. 国内返回的 IP 若不在中国网段，则 `mark89`→`sequence_fakeip`；若是中国 IP，则检查 `CNtoMihomo`：开启则交给 mihomo fakeip，否则直接返回 real IP。

#### 安全模式（国外优先）
1. 首次查询使用 `sequence_google_node`，强制携带国内 ECS（8.8.8.8 + ECS）。
2. 若 `rcode0` 无 AAAA 或 `rcode3`，也将域名写入 `nov6list`；V4 缺失则 fallback 回国内 DNS，再失败写入 `nov4list`。
3. 国外返回 IP 若不在中国，则直接 fakeip；若是中国 IP，再次查看 `CNtoMihomo` 决定由 mihomo fakeip 还是返回 real IP。

### 2.7 `baidu.com` 与 `google.com` 场景
- **`baidu.com`（国内域名）**：如未被 `switch2` 拦截，会落入 `sequence_main` → `gen_conc` 标记国内 → `mark888` → `$sequence_local` → 国内上游返回 → `cache_cn`/`cache_all` 回写 → 客户端得到 real IP。
- **`google.com`（灰名单）**：默认因 `switch2=A` 被提前放行（直接国内查询结束）。若允许该客户端进入主分流，则：
  1. `qname $greylist` → `mark22` → `sequence_fakeip`（sing-box fakeip，`mark777`/`mark999` 记录）。
  2. 若移出灰名单，则作为“列表外”域名，按 `switch3` 选择泄露/不泄露模式，通常因返回 IP 非中国触发 `mark89` → fakeip。

### 2.8 缓存与上游回顾
- **cache 层级**：`cache_all`（客户端第一层）、`cache_cn`/`cache_google`/`cache_node`/`cache_google_node`（分场景缓存），`switch4` 可整体关闭 lazy cache。
- **上游映射**：
  - 国内：`forward_local`（223.5.5.5、221.130.33.60，含 300ms 并行）
  - 国外：`forward_google`（HTTPS DoH，经 127.0.0.1:7891 SOCKS5）
  - 国外 ECS：`forward_google_ecs` 强制携带 `ecs 2408:8214:213::1`
  - FakeIP：`forward_fakeip` 监听 `udp://127.0.0.1:6666`，通常接入 sing-box，若 `CNtoMihomo` 为 A，则国内 fakeip 交给 mihomo
- **代理分工**：整体思路是“sing-box 负责国外域名 fakeip，mihomo 负责国内域名 fakeip（若开启 CNtoMihomo）”；两者使用不同 fakeip 网段，保证路由与策略可区分。

### 2.9 Web UI / API 触点
- `switch` 插件开放 `/plugins/switchN/post`，可在线切换 A/B。
- `domain_output` 会通过 `/api/v1/update/*` 系列接口回写 `gen/*.txt`，实现在线刷新列表。
- `/api/v1/system/*` 提供状态查询，可结合 `mosdns.log` 对照本文流程查证实际行为。


## 3. 附录

### 3.1 `mark` 语义速查
| 标记 | 说明 | 所在序列 |
|------|------|----------|
| 1 | DDNS 域名 → 强制国内查询 | `sequence_6666` |
| 2 | 黑名单/无解析请求检查 | `sequence_6666` |
| 3 | 非白名单客户端触发本地查询 | `sequence_6666` |
| 11 | 自定义白名单 → 国内查询 → TTL 延长/生成 nov4/nov6 | `sequence_main` |
| 22 | 灰名单 → fakeip 流程 | `sequence_main` |
| 33 | realiplist → 国外上游 | `sequence_main` |
| 66 | 非 A/AAAA 请求标记 | `sequence_main` |
| 68 | 列表外域名（泄露模式） | `sequence_not_in_list_leak` |
| 77x | FakeIP 回写标记 (`777` fakeip、`777`+`!999` 生成规则) | `sequence_main` |
| 88x | 黑洞匹配 (`888` 国内、`999` fakeip、`666` 未识别) | `sequence_main` |
| 123 | 国内返回空/污染，转查国外 | `sequence_not_in_list_*` |
| 456 | SERVFAIL → 国内兜底（非泄露模式） | `_noleak` |
| 777 | FakeIP 响应标记 | `sequence_main` |
| 888/999 | realip/fakeip 列表写入 | `sequence_main` |

### 3.2 `switch` 默认值与影响
| 开关 | 默认 | 影响 |
|------|------|------|
| switch1 | B | 不启用黑名单/空记录清洗；设为 A 可激活 `$blocklist`。 |
| switch2 | A | 启用“指定客户端才可走完整分流”；若 `client_ip.txt` 为空，则所有请求直接进入 `$sequence_local`。 |
| switch3 | A | 泄露版流程（先国内），B 切换为 `_noleak`。 |
| switch4 | A | 启用 lazy cache；B 仅保留节点缓存。 |
| switch5 | A | 屏蔽 qtype 6/12/65。 |
| switch6 | B | 默认允许 AAAA；设为 A 时可屏蔽。 |
| switch7 | A | 启用 AdGuard 列表。 |

### 3.3 关键文件索引
| 类别 | 路径 | 备注 |
|------|------|------|
| 主配置 | `demo/mosdns1/config.yaml` | 定义入口 `sequence_6666` 与 server。 |
| 子配置 | `demo/mosdns1/sub_config/*.yaml` | 各类 forward/cache/rule/switch。 |
| 用户配置 | `demo/mosdns1/rule/*.txt` | 开关、白/黑名单、DDNS、PCDN 列表。 |
| 运行产物 | `demo/mosdns1/gen/*.txt`, `cache_*.dump` | `domain_output` / `cache` 插件生成。 |
| 客户端清单 | `demo/mosdns1/client_ip.txt` | 配合 switch2 使用。 |

---
完成上述文档后，建议在修改配置或新增插件时同步更新，以保持 SSOT 一致性。
