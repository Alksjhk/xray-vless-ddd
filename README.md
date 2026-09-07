# xray-vless — VLESS + REALITY 服务端最小裁剪版

从 [XTLS/Xray-core](https://github.com/XTLS/Xray-core) 源码裁剪出的最小可编译 Go 项目，
只保留 **VLESS 服务端 inbound + REALITY TLS + freedom/blackhole outbound**。
协议字节、REALITY 握手、认证失败透传（fallbacks）逻辑与上游完全一致，仅删除未引用代码。

- 基线版本：Xray-core `26.7.28`（core.Version() 输出保持不变）
- License：MPL-2.0（所有保留文件均保留原头部）
- 产物：单个静态二进制（`CGO_ENABLED=0`，Linux/Windows 均可编译），约 13.6 MB

---

## 1. 目录树（保留部分）

```
xray-vless/
├── main/main.go                  # 程序入口：run / x25519 / uuid / version 四个子命令
├── go.mod / go.sum               # 裁剪后的依赖（require 带功能分组注释）
├── core/                         # Xray 实例管理：core.New / 特性注册 / 配置装载接口
├── proxy/
│   ├── proxy.go                  # proxy.Inbound/Outbound 接口、Vision 缓冲读写、PROXY protocol
│   ├── vless/                    # VLESS：account.go、account.pb.go、validator.go、encryption/（VLESS Encryption）
│   │   ├── inbound/              # ★ 服务端 inbound handler（核心）
│   │   ├── encoding/             # ★ VLESS 头部编解码 + addons
│   │   └── outbound/             # ✂ 已删除（客户端 outbound）
│   ├── freedom/                  # ★ outbound：把解密后的流量转发到真实目标
│   └── blackhole/                # outbound：丢弃（block 场景，可选保留）
├── transport/
│   ├── pipe/ ...                 # common 级管道（被 dispatcher/mux 引用）
│   └── internet/
│       ├── internet.go 等        # 传输层接口、系统拨号器、sockopt、happy eyeballs
│       ├── tcp/                  # ★ TCP 监听/拨号（hub.go 调 reality.Server）
│       ├── tls/                  # TLS 公共辅助（REALITY 共用）
│       ├── reality/              # ★ REALITY 薄封装层（reality.go / config.go / config.pb.go）
│       ├── stat/                 # 连接统计包装（stat.Connection）
│       ├── udp/                  # UDP 会话分发（proxyman worker 引用）
│       ├── headers/              # TCP 伪装头（http/none）
│       └── finalmask/            # 仅保留根包（proxy/proxy.go 引用其类型），子包已删
├── common/                       # 全量保留：buf/net/protocol/serial/session/uuid/crypto/mux/errors/log/signal/task/...
├── features/                     # 全量保留（接口层）：inbound/outbound/routing/policy/stats/dns/extension/...
├── app/
│   ├── proxyman/                 # ★ inbound/outbound handler 管理器（worker 调 Process 并注入 Dispatcher）
│   ├── dispatcher/               # ★ DefaultDispatcher：routing.Dispatcher 实现，衔接 inbound→outbound
│   ├── log/                      # 日志 App（infra/conf 固定注入，core 启动链必需）
│   ├── stats/                    # 计数器（dispatcher 引用）
│   └── reverse/                  # VLESS 反向链路（proxy/vless/inbound 直接 import，协议的一部分）
├── infra/conf/                   # 配置解析：vless.go / freedom.go / blackhole.go / log.go /
│   │                             #   transport_{internet,method,security,sockopt}.go / xray.go（注册表）
│   └── serial/ + json/           # JSONC 配置装载（仅 JSON，TOML/YAML 已删）
└── main/confloader/              # core 依赖的配置文件读取器
```

## 2. 已删除模块

- `proxy/`：vmess、trojan、shadowsocks、shadowsocks_2022、wireguard、socks、http、
  dokodemo、loopback、dns、hysteria、tun，以及 **`proxy/vless/outbound`**（客户端侧）
- `app/`：router、dns、policy、commander、metrics、observatory、geodata、version，
  以及各 App 的 gRPC command 子包（app/log/command、app/proxyman/command、app/stats/command）
- `transport/internet/`：websocket、grpc、httpupgrade、splithttp、kcp、hysteria、
  browser_dialer、tagged；finalmask 的全部子包（noise/mkcp/sudoku/…，仅保留根包）
- `infra/`：vformat、vprotogen、conf 中被删协议的解析文件、cfgcommon（仅测试用）
- `main/`：原命令框架（main/commands、distro、run.go），重写为最小 `main/main.go`
- 全部 `_test.go`；serial 的 TOML/YAML 加载器
- 间接依赖随之清理：google.golang.org/grpc、quic-go 主体、gvisor、sing、wireguard、
  gorilla/websocket、go-toml、ghodss/yaml 等均从 go.mod 消失

## 3. 无法删除的 app 层（引用链说明）

裁剪清单原计划删除 app/dispatcher、app/proxyman 等，但引用链证明它们是服务端闭环的硬依赖：

- `proxy/vless/inbound/inbound.go` 顶层 import `app/dispatcher`（`dispatcher.WrapLink`）
  和 `app/reverse`（Rvs 反向链路 mux），并在 `Process(ctx, network, conn, dispatch routing.Dispatcher)`
  中通过 `dispatch.DispatchLink(...)` 转发流量
- `routing.Dispatcher` 的唯一生产实现是 `app/dispatcher`，由 `app/proxyman/inbound` 的
  tcpWorker 在 accept 连接后传入 `handler.Process(...)`
- `infra/conf/xray.go` 的 `Config.Build()` 固定向 core.Config 注入
  `dispatcher.Config` + `proxyman.InboundConfig` + `proxyman.OutboundConfig` + log App
- 保留 `app/reverse` 的代价极小（纯 Go，无第三方依赖），且删除会破坏 VLESS 协议中的
  Rvs 路径，违背"协议行为 100% 等价"原则

其余 app（router/policy/dns/…）确实可删：core.New 对 routing/policy/stats/dns
都有默认兜底实现（`routing.DefaultRouter`、`policy.DefaultManager`、`stats.NoopManager`、
`localdns`），配置里不写对应段落即不加载。

## 4. 密钥生成

```bash
./xray-vless x25519          # 输出 PrivateKey / Password (PublicKey) / Hash32，格式同完整版
./xray-vless x25519 -i "私钥" --std-encoding   # 从指定私钥派生，可选 std base64
./xray-vless uuid            # RFC 4122 UUIDv4
./xray-vless uuid -i example # UUIDv5（VLESS 确定性 UUID）
openssl rand -hex 8        # shortId（0~16 位偶数长度 hex，可多条）
```

填入位置：PrivateKey → 服务端 `realitySettings.privateKey`；
PublicKey → 客户端 `realitySettings.publicKey`；
UUID → 服务端 `settings.clients[].id`；shortId → 服务端 `realitySettings.shortIds[]`。

## 5. 最小可用配置（config.json）

字段名与 xray 原版完全一致（dest / serverNames / privateKey / shortIds / show）：

```json
{
  "log": { "loglevel": "warning" },
  "inbounds": [
    {
      "listen": "0.0.0.0",
      "port": 443,
      "protocol": "vless",
      "settings": {
        "clients": [
          { "id": "37ad80e9-bb55-45e3-9254-78cc71d42b71", "flow": "xtls-rprx-vision" }
        ],
        "decryption": "none"
      },
      "streamSettings": {
        "network": "tcp",
        "security": "reality",
        "realitySettings": {
          "show": false,
          "dest": "www.microsoft.com:443",
          "serverNames": ["www.microsoft.com"],
          "privateKey": "kKIKFRa_wGqVrVhBAwa63mj68ztiP5bWf2ALyngKjWg",
          "shortIds": ["0123456789abcdef"]
        }
      },
      "sniffing": { "enabled": false }
    }
  ],
  "outbounds": [
    { "protocol": "freedom", "tag": "direct" },
    { "protocol": "blackhole", "tag": "block" }
  ]
}
```

客户端（完整版 xray / v2rayN 等）按标准 VLESS+Reality 配置对接即可：
`publicKey` 填 x25519 输出的 PublicKey，`shortId` 从服务端 shortIds 中选一条。

## 6. 构建

```bash
# Linux / macOS
CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o xray-vless ./main

# Windows (PowerShell)
$env:CGO_ENABLED=0
go build -trimpath -ldflags "-s -w" -o xray-vless.exe ./main
```

运行：`./xray-vless run -c config.json`（缺省 `-c` 时自动找当前目录 `config.json`，
`run` 为默认子命令，也支持 stdin）。

### CI 自动构建（GitHub Actions）

工作流：`.github/workflows/build.yml`。推送任意分支即触发构建（也可手动触发）；
推送 `v*` tag 时自动创建对应 Release 并上传产物。只产出两个目标：

| 产物 | 说明 |
| --- | --- |
| `xray-vless-linux-x64` | `CGO_ENABLED=0` 纯静态 ELF（CI 会校验无 `PT_INTERP` / 无 `DT_NEEDED`），Linux x86_64 |
| `xray-vless-windows-x64.exe` | 控制台版 Windows x86_64 二进制 |

构建参数与上游一致（`-trimpath -buildvcs=false -gcflags="all=-l=4" -ldflags="-X github.com/xtls/xray-core/core.build=<commit> -s -w -buildid="`）。
产物为裸二进制、不做 zip 压缩，每个二进制附同名 `*.dgst`（md5/sha1/sha256/sha512）。
tag 构建的产物直接挂在 Release 上；普通分支构建的产物在 Actions 的 Artifacts 里下载（90 天过期）。

## 7. 与上游同步

保留文件均为上游原样（含 MPL-2.0 头），结构性改动集中在：
`main/main.go`（重写）、`infra/conf/{xray,transport_internet,transport_method,vless}.go`、
`infra/conf/serial/loader.go`（JSON-only）、`app/proxyman/inbound/worker.go`（去 hysteria）。
建议 fork 后以 git 分支维护本裁剪，便于 `git merge` 上游更新；
合并冲突只会集中出现在上述少数文件。
