# mosdns1 请求流程解析

本文基于 `mosdns start --dir /Users/doumao/code/github/mosdns/demo/mosdns1` 启动的默认配置，对一次 `nslookup baidu.com 127.0.0.1` 的内部执行过程进行拆解，并附上关键配置速查。

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
2. 监听器 `udp_all` 接收报文，交给 `sequence_6666`。

### 2.2 `sequence_6666` 关键步骤
> 默认开关值来自 `rule/switch*.txt`：`switch1=B`、`switch2=A`、`switch3=A`、`switch4=A`、`switch5=A`、`switch6=B`、`switch7=A`。

1. **协议过滤**：
   - 若 `switch6=A` 时屏蔽 AAAA (`qtype 28`)；但默认 `B`，所以允许 AAAA。
   - `switch5=A`，禁止 `SOA/PTR/HTTPS`（qtype 6/12/65），与当前查询无关。
2. **DDNS 特例**：命中 `$ddnslist` 时 `mark 1` 并走 `$forward_local`，本例未命中。
3. **黑名单/空记录**：`switch1=B` ⇒ 不触发 `mark 2`，后续 blocklist 判断被跳过。
4. **AdGuard**：`switch7=A` + `$adguard` 列表时直接 `reject 3`；`baidu.com` 不在列表。
5. **客户端放行**：
   - `switch2=A` 且 `client_ip.txt` 默认为空 ⇒ `!client_ip $client_ip` 为真。
   - 设置 `mark 3`，立即执行 `$sequence_local`，随后 `accept`。
   - 因此在默认 demo 下，本地 `nslookup` 会在入口就被国内序列终止，后续 `cache_all`/`sequence_main` 不参与。
6. **（若客户端已列入白名单或将 switch2 置 B）后的路径**：
   - `switch3` 与 `switch4` 决定是否走 `cache_all` 或 `cache_all_noleak`。
   - 缓存未命中时落到 `$sequence_all`，进而进入 `sequence_main`。

> **提示**：若希望真实体验多层分流，可将 `rule/switch2.txt` 改为 `B` 或在 `client_ip.txt` 中写入发起机的局域网 IP，然后重新启动。

### 2.3 `sequence_all` → `sequence_main`
当请求进入主分流后，核心决策如下：

1. **前置动作**：
   - `exec: $rewrite` 根据 `rule/rewrite.txt` 重写域名（如需要）
   - 如果上一步已有响应（缓存命中），直接 `accept`。
2. **`mark 66` / `mark 99`**：非 A/AAAA 查询先打 `mark66`，再按 `$geosite_cn` 区分国内（`mark99`），继而走 `$sequence_local`。
3. **灰名单/白名单**：
   - `qname $greylist` ⇒ `mark22` ⇒ `sequence_fakeip`。
   - `qname $whitelist` ⇒ `mark11` ⇒ `sequence_local`，并附带生成 `nov4/nov6` 规则。
4. **realip 场景**：`qname $realiplist` ⇒ `mark33` ⇒ `sequence_google`（国外上游）。
5. **生成/匹配黑洞 IP**：
   - `exec: $gen_conc` 并发比对 geosite/domains，返回 `127.0.0.x/::x` 作为标签。
   - `resp_ip 127.0.0.1` ⇒ `mark888`（国内） → `$sequence_local`。
   - `resp_ip 127.0.0.2` ⇒ `mark999`（fakeip） → `$sequence_fakeip`。
   - `resp_ip 127.0.0.3` ⇒ `mark666`（未命中列表） → 丢弃并转入 `$conc_lookup`，再根据结果选择本地或 fakeip。
   - `mark777` 用于 fakeip 返回值的后续标注。
6. **泄露/不泄露逻辑**：
   - `switch3=A`（默认）调用 `sequence_not_in_list_leak`：表外域名先走国内、若异常再回落到 Google/FakeIP。
   - `switch3=B` 调用 `sequence_not_in_list_noleak`：优先走带 ECS 的国外上游，再必要时国内兜底。

对 `baidu.com`（属于 geosite_cn）的完整路径：
1. `sequence_6666` 若未被 `switch2` 拦截，会命中 `cache_all`（启用）→ 无命中 → 进入 `sequence_main`。
2. `gen_conc` 识别其为国内域名，写入 blackhole `127.0.0.1`。
3. `mark888` 成立，直接转入 `$sequence_local`。
4. `$sequence_local` 先走 `cache_cn` → miss → `forward_local`（223.5.5.5、221.130.33.60），返回结果后 `cname_remover` 清理尾部。
5. 响应回写各层缓存（`cache_cn`、`cache_all`）并返回客户端。

### 2.4 `sequence_not_in_list_*` 亮点
- **泄露模式 (`_leak`)**：
  1. 标记 `mark68` → 先走 `$sequence_local`。
  2. 若无 IPv6/IPv4，生成 `nov6/nov4` 规则以供后续匹配。
  3. `mark123` 代表本地返回为空/污染，触发 `$sequence_google` 重查。
  4. 若最终 IP 不在 `$geoip_cn`，记 `mark89`，交给 `$sequence_fakeip`。
- **不泄露模式 (`_noleak`)**：
  1. 直接使用 `$sequence_google_node`（带 ECS）查询 8.8.8.8。
  2. SERVFAIL (`rcode 2/5`) 时使用 `$sequence_local` 兜底。
  3. 与泄露模式一样，最终根据 `mark89` 决定 fakeip 或 realip。

### 2.5 缓存与上游行为
- `cache_all*`：面向客户端的第一层缓存，`size=20M`，`lazy_cache_ttl=3 天`，会 dump 到 `cache_all*.dump`。
- `cache_cn`, `cache_google`, `cache_google_node`, `cache_node`: 在对应序列中引用，保证国内/国外/节点解析可以离线延迟；`switch4=B` 时，仅节点缓存保留。
- 上游：
  - 国内：`forward_local` → `223.5.5.5` + `221.130.33.60`（第二条限时 300ms）。
  - 国外：`forward_google` 通过 HTTPS DoH，走 `127.0.0.1:7891` SOCKS5。
  - 国外（节点/ECS）：`forward_google_ecs` 强制携带 `ecs 2408:8214:213::1`。
  - FakeIP：`forward_fakeip` 连接本机 6666 端口，可挂载 sing-box。

### 2.6 Web UI / API 交互点
- `switch` 插件暴露 `/plugins/switchN/post` 接口，可在 Web UI 修改为 `A/B`。
- `domain_output` 通过 `domain_set_url` 回写 `gen/*.txt` 到 API，实现在线更新列表。
- `/api/v1/update/*`、`/api/v1/system/*` 提供状态查询，与当前流程无直接耦合。

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
