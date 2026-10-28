module github.com/gcash/bchd

go 1.26.0

require (
	github.com/bluele/gcache v0.0.2
	github.com/btcsuite/winsvc v1.0.0
	github.com/davecgh/go-spew v1.1.1
	github.com/dchest/siphash v1.2.3
	github.com/decred/dcrd/dcrec/secp256k1/v4 v4.4.1
	github.com/gcash/bchlog v0.0.0-20261027020323-7b5704c0a60f
	github.com/gcash/bchutil v0.0.0-20261028060706-e320ade87427
	github.com/gcash/goleveldb v1.1.0
	github.com/gcash/goslp v1.0.0
	github.com/golang/glog v1.2.5
	github.com/gorilla/mux v1.8.1
	github.com/gorilla/websocket v1.5.3
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.33.0
	github.com/jessevdk/go-flags v1.6.1
	github.com/jrick/logrotate v1.1.2
	github.com/prometheus/client_golang v1.25.0
	github.com/traefik/grpc-web v0.18.0
	github.com/xeipuuv/gojsonschema v1.2.0
	github.com/zquestz/grab v0.0.0-20190224022517-abcee96e61b1
	golang.org/x/crypto v0.58.0
	golang.org/x/net v0.62.0
	golang.org/x/text v0.43.0
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
	lukechampine.com/uint128 v1.3.0
)

require (
	github.com/aead/siphash v1.0.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/btcsuite/go-socks v0.0.0-20170105172521-4720035b7bfd // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/coder/websocket v1.8.15 // indirect
	github.com/desertbit/timer v1.0.1 // indirect
	github.com/golang/snappy v1.0.0 // indirect
	github.com/kkdai/bstream v1.0.0 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.72.1 // indirect
	github.com/prometheus/procfs v0.23.0 // indirect
	github.com/rs/cors v1.11.1 // indirect
	github.com/xeipuuv/gojsonpointer v0.0.0-20190905194746-02993c407bfb // indirect
	github.com/xeipuuv/gojsonreference v0.0.0-20180127040603-bd5ef7bd5415 // indirect
	go.yaml.in/yaml/v3 v3.0.5 // indirect
	golang.org/x/sys v0.49.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261026165753-2ffab32bce7e // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261026165753-2ffab32bce7e // indirect
	google.golang.org/grpc/cmd/protoc-gen-go-grpc v1.6.2 // indirect
)

tool (
	github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway
	github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2
	google.golang.org/grpc/cmd/protoc-gen-go-grpc
	google.golang.org/protobuf/cmd/protoc-gen-go
)
