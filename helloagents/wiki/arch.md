# 架构设计

## 总体架构
```mermaid
flowchart LR
    Client -->|UDP/TCP 53| Entry[udp_all/tcp_all]
    Entry --> Sequence6666[sequence_6666]
    Sequence6666 --> SequenceAll[sequence_all -> sequence_main]
    SequenceMain -->|mark 11| Local[sequence_local]
    SequenceMain -->|mark 22/777| FakeIP[sequence_fakeip]
    SequenceMain -->|not in list| NotInList[sequence_not_in_list_*]
    SequenceMain -->|mark 33| Google[sequence_google]
    NotInList --> Google
    Local --> ForwardLocal[forward_local -> 国内上游]
    Google --> ForwardGoogle[forward_google -> 国外上游]
    FakeIP --> ForwardFakeIP[forward_fakeip -> 127.0.0.1:6666]
```

## 技术栈
- **CLI:** Cobra + Viper + zap，命令 `mosdns start` 提供运行入口。
- **插件:** YAML 中的 `plugins` 会注册成 `tag`，通过 `sequence`、`fallback` 等组合形成逻辑树。
- **网络:** miekg/dns 提供 UDP/TCP 服务端，forward 插件支持 DoH/DoQ、SOCKS5、ECS。
- **缓存:** 多层 cache 插件（全局/国内/国外/节点）配合 dump 文件，实现断电恢复。

## 核心流程
```mermaid
sequenceDiagram
    participant Client
    participant udp_all
    participant seq6666 as sequence_6666
    participant seqMain as sequence_main
    participant fwdLocal as forward_local
    participant fwdGoogle as forward_google
    Client->>udp_all: 查询 baidu.com A
    udp_all->>seq6666: 调用入口缓存序列
    seq6666->>seqMain: 命中缓存或进入主分流
    seqMain->>fwdLocal: 国内域名 + A 记录 → forward_local
    fwdLocal-->>seqMain: 解析结果
    seqMain-->>Client: 过滤/标记后返回
```

## 重大架构决策
| adr_id | title | date | status | affected_modules | details |
|--------|-------|------|--------|------------------|---------|
| *暂无* | - | - | - | - | - |
