module github.com/ggsrc/gglib/app

go 1.27.0

require (
	github.com/ggsrc/gglib/grpc v0.0.0-20261013145132-b474e434e730
	github.com/ggsrc/gglib/health v0.0.0-20261013145132-b474e434e730
	github.com/ggsrc/gglib/metric v0.0.0-20261013145132-b474e434e730
	github.com/ggsrc/gglib/resource v0.0.0-20261013145132-b474e434e730
	github.com/ggsrc/gglib/zerolog v0.0.0-20261013145132-b474e434e730
	github.com/uptrace/uptrace-go v1.47.0
	google.golang.org/grpc v1.85.0
)

require (
	github.com/agoda-com/opentelemetry-go/otelzerolog v0.1.0 // indirect
	github.com/agoda-com/opentelemetry-logs-go v0.6.0 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/bytedance/gopkg v0.1.4 // indirect
	github.com/bytedance/sonic v1.15.5 // indirect
	github.com/bytedance/sonic/loader v0.5.3 // indirect
	github.com/cenkalti/backoff/v4 v4.3.0 // indirect
	github.com/cenkalti/backoff/v5 v5.0.3 // indirect
	github.com/cespare/xxhash/v2 v2.3.0 // indirect
	github.com/cloudwego/base64x v0.1.7 // indirect
	github.com/ggsrc/gglib/env v0.0.0-20261013145132-b474e434e730 // indirect
	github.com/ggsrc/gglib/interceptor v0.0.0-20261013145132-b474e434e730 // indirect
	github.com/ggsrc/gglib/mctx v0.0.0-20261013145132-b474e434e730 // indirect
	github.com/go-logr/logr v1.4.4 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/grpc-ecosystem/go-grpc-middleware/v2 v2.3.4 // indirect
	github.com/grpc-ecosystem/go-grpc-prometheus v1.2.0 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.33.0 // indirect
	github.com/kelseyhightower/envconfig v1.4.0 // indirect
	github.com/klauspost/cpuid/v2 v2.4.1 // indirect
	github.com/mattn/go-colorable v0.1.16 // indirect
	github.com/mattn/go-isatty v0.0.24 // indirect
	github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822 // indirect
	github.com/pandatix/go-abnf v0.5.5 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/prometheus/client_golang v1.25.0 // indirect
	github.com/prometheus/client_model v0.6.3 // indirect
	github.com/prometheus/common v0.73.1 // indirect
	github.com/prometheus/procfs v0.23.0 // indirect
	github.com/rs/zerolog v1.35.1 // indirect
	github.com/twitchyliquid64/golang-asm v0.15.1 // indirect
	go.opentelemetry.io/auto/sdk v1.2.1 // indirect
	go.opentelemetry.io/contrib/instrumentation/google.golang.org/grpc/otelgrpc v0.72.0 // indirect
	go.opentelemetry.io/contrib/instrumentation/runtime v0.72.0 // indirect
	go.opentelemetry.io/contrib/processors/minsev v0.17.0 // indirect
	go.opentelemetry.io/otel v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlplog/otlploghttp v0.23.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp v1.47.0 // indirect
	go.opentelemetry.io/otel/exporters/stdout/stdouttrace v1.47.0 // indirect
	go.opentelemetry.io/otel/log v1.47.0 // indirect
	go.opentelemetry.io/otel/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/log v1.47.0 // indirect
	go.opentelemetry.io/otel/sdk/metric v1.47.0 // indirect
	go.opentelemetry.io/otel/trace v1.47.0 // indirect
	go.opentelemetry.io/proto/otlp v1.11.1 // indirect
	golang.org/x/arch v0.32.0 // indirect
	golang.org/x/net v0.62.0 // indirect
	golang.org/x/sync v0.24.0 // indirect
	golang.org/x/sys v0.49.0 // indirect
	golang.org/x/text v0.43.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20261026165753-2ffab32bce7e // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20261026165753-2ffab32bce7e // indirect
	google.golang.org/protobuf v1.36.12 // indirect
)

// Sibling modules in this repository (scripts/replaces.sh).
replace (
	github.com/ggsrc/gglib/env => ../env
	github.com/ggsrc/gglib/grpc => ../grpc
	github.com/ggsrc/gglib/health => ../health
	github.com/ggsrc/gglib/interceptor => ../interceptor
	github.com/ggsrc/gglib/mctx => ../mctx
	github.com/ggsrc/gglib/metric => ../metric
	github.com/ggsrc/gglib/resource => ../resource
	github.com/ggsrc/gglib/zerolog => ../zerolog
)
