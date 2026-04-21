module github.com/theairblow/turnable

go 1.25.0

require (
	github.com/google/uuid v1.6.0
	github.com/gorilla/websocket v1.5.3
	github.com/pion/dtls/v3 v3.1.2
	github.com/pion/logging v0.2.4
	github.com/pion/sctp v1.9.4
	github.com/pion/sdp/v3 v3.0.18
	github.com/pion/turn/v5 v5.0.3
	github.com/spf13/cobra v1.10.2
	github.com/xtaci/kcp-go/v5 v5.6.72
)

// fixes building on android locally inside termux
replace github.com/wlynxg/anet => github.com/BieHDC/anet v0.0.6-0.20241226223613-d47f8b766b3c

require (
	github.com/0xERR0R/expiration-cache v0.1.0 // indirect
	github.com/RackSec/srslog v0.0.0-20180709174129-a4725f04ec91 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cisco/go-hpke v0.0.0-20230407100446-246075f83609 // indirect
	github.com/cisco/go-tls-syntax v0.0.0-20200617162716-46b0cfb76b9b // indirect
	github.com/cloudflare/circl v1.6.3 // indirect
	github.com/cloudflare/odoh-go v1.0.1-0.20230926114050-f39fa019b017 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/folbricht/routedns v0.1.155 // indirect
	github.com/hashicorp/golang-lru v1.0.2 // indirect
	github.com/inconshreveable/mousetrap v1.1.0 // indirect
	github.com/jtacoma/uritemplates v1.0.0 // indirect
	github.com/klauspost/cpuid/v2 v2.3.0 // indirect
	github.com/klauspost/reedsolomon v1.13.3 // indirect
	github.com/miekg/dns v1.1.69 // indirect
	github.com/oschwald/maxminddb-golang v1.13.1 // indirect
	github.com/patrickmn/go-cache v2.1.0+incompatible // indirect
	github.com/pion/randutil v0.1.0 // indirect
	github.com/pion/rtcp v1.2.16 // indirect
	github.com/pion/rtp v1.10.1 // indirect
	github.com/pion/srtp/v3 v3.0.10 // indirect
	github.com/pion/stun/v3 v3.1.2 // indirect
	github.com/pion/transport/v4 v4.0.1 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/quic-go/qpack v0.6.0 // indirect
	github.com/quic-go/quic-go v0.57.1 // indirect
	github.com/redis/go-redis/v9 v9.17.2 // indirect
	github.com/spf13/pflag v1.0.10 // indirect
	github.com/tjfoc/gmsm v1.4.1 // indirect
	github.com/txthinking/runnergroup v0.0.0-20250224021307-5864ffeb65ae // indirect
	github.com/txthinking/socks5 v0.0.0-20251011041537-5c31f201a10e // indirect
	github.com/wlynxg/anet v0.0.5 // indirect
	github.com/yuin/gopher-lua v1.1.2-0.20241109074121-ccacf662c9d2 // indirect
	golang.org/x/crypto v0.50.0 // indirect
	golang.org/x/mod v0.34.0 // indirect
	golang.org/x/net v0.53.0 // indirect
	golang.org/x/sync v0.20.0 // indirect
	golang.org/x/sys v0.43.0 // indirect
	golang.org/x/text v0.36.0 // indirect
	golang.org/x/time v0.15.0 // indirect
	golang.org/x/tools v0.43.0 // indirect
)
