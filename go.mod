module github.com/xtls/xray-core

go 1.26

require (
	// REALITY 服务端/客户端核心（修改版 crypto/tls fork），版本保持上游不变
	github.com/xtls/reality v0.0.0-20260322125925-9234c772ba8f
	// uTLS：REALITY 依赖的 TLS 指纹/类型定义
	github.com/refraction-networking/utls v1.8.3-0.20260301010127-aa6edf4b11af
	// mldsa65（后量子签名，REALITY mldsa65Verify 用）
	github.com/cloudflare/circl v1.6.5
	// VLESS 加密/x25519 子命令使用
	golang.org/x/crypto v0.55.0
	lukechampine.com/blake3 v1.4.1

	// 协议与传输基础设施
	google.golang.org/protobuf v1.36.12 // protobuf 生成代码运行时
	github.com/pires/go-proxyproto v0.15.0 // VLESS fallbacks 的 PROXY protocol (xver)
	github.com/miekg/dns v1.1.73 // transport/internet/tls 内部使用
	github.com/apernet/quic-go v0.61.1-0.20260806010916-184d081eef3e // 仅使用 quicvarint（common/protocol/quic）
	github.com/klauspost/cpuid/v2 v2.4.0 // blake3 指令集派发
	go4.org/netipx v0.0.0-20231129151722-fdeea329fbba // common/geodata 私网匹配

	// x/net (http2, REALITY spider 遗留公共类型) / x/sys / x/sync
	golang.org/x/net v0.58.0
	golang.org/x/sys v0.47.0
	golang.org/x/sync v0.22.0
)

require (
	github.com/andybalholm/brotli v1.0.6 // indirect
	github.com/juju/ratelimit v1.0.2 // indirect
	github.com/klauspost/compress v1.17.4 // indirect
	github.com/kr/text v0.2.0 // indirect
	github.com/stretchr/testify v1.12.1 // indirect
	golang.org/x/text v0.41.0 // indirect
)
