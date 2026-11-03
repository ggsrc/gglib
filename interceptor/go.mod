module github.com/ggsrc/gglib/interceptor

go 1.27.0

require (
	github.com/bytedance/sonic v1.15.5
	github.com/getsentry/sentry-go v0.50.0
	github.com/ggsrc/gglib/mctx v0.0.0-20261013145132-b474e434e730
	github.com/ggsrc/gglib/zerolog v0.0.0-20261013145132-b474e434e730
	github.com/jinzhu/copier v0.4.0
	github.com/rs/zerolog v1.35.1
	github.com/stretchr/testify v1.12.1
	go.opentelemetry.io/otel v1.47.0
	go.opentelemetry.io/otel/trace v1.47.0
	google.golang.org/grpc v1.85.0
	google.golang.org/protobuf v1.36.12
)

require (
	github.com/bytedance/gopkg v0.1.4 // indirect
	github.com/bytedance/sonic/loader v0.5.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.7 // indirect
	github.com/klauspost/cpuid/v2 v2.4.1 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/pingcap/errors v0.11.5-0.20211224045212-9687c2b0f87c // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	go.uber.org/atomic v1.10.0 // indirect
	go.yaml.in/yaml/v3 v3.0.6 // indirect
	golang.org/x/arch v0.32.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261026165753-2ffab32bce7e // indirect
)

// Sibling modules in this repository (scripts/replaces.sh).
replace (
	github.com/ggsrc/gglib/env => ../env
	github.com/ggsrc/gglib/mctx => ../mctx
	github.com/ggsrc/gglib/zerolog => ../zerolog
)
