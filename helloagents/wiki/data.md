# 数据模型

## 配置文件结构
```yaml
log:
  level: warn
  file: /tmp/mosdns.log
api:
  http: 0.0.0.0:9099
include:
  - sub_config/adguard.yaml
  - ...
plugins:
  - tag: sequence_all
    type: fallback
    args: { primary: sequence_all_single, secondary: sequence_all_single }
```

- `include` 顺序即插件依赖顺序，先声明基础数据、cache、forward，再封装高层序列。
- `plugins` 顶层段在主配置和子配置中都生效，同时引用 `$tag` 调用其他插件。

## 规则与缓存数据
| 名称 | 文件/目录 | 描述 |
|------|-----------|------|
| `rule/*.txt` | 静态域名或开关定义 | 提供白名单、黑名单、开关初始值 |
| `gen/*.txt` | 运行期生成的规则 | 由 `domain_output`、`con_match` 等插件写入 |
| `cache_*.dump` | 缓存文件 | `cache` 插件周期性 dump 便于重启后加载 |
| `srs/*.json` | geosite/geoip 镜像 | 缺省走 SOCKS5 更新，也支持本地文件 |

## API 状态存储
- `sub_config/webinfo.yaml` 负责把 Web UI 的配置写入 `webinfo/`。
- `client_ip.txt` 提供 `switch2` 所需的允许清单。

## 数据一致性
- 所有生成的规则都在 `domain_output.yaml` 中定义输出，便于其他 sequence 条件引用。
- 端口 53 的审计由 `enable_audit: true` 开启，记录写入 `mosdns.log`。
