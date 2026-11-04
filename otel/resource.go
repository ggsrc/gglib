package otel

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/sdk/resource"

	"github.com/ggsrc/gglib/env"
)

// newResource builds the resource, lowest precedence first: values from the
// process environment (gglib/env, POD_NAME, POD_NAMESPACE), then the SDK's
// detectors and OTEL_SERVICE_NAME / OTEL_RESOURCE_ATTRIBUTES, then options.
func newResource(ctx context.Context, cfg *config) (*resource.Resource, error) {
	var base []attribute.KeyValue
	if name := env.ServiceName(); name != env.UnknownServiceName {
		base = append(base, attribute.String("service.name", name))
	}
	if v := env.ServiceVersion(); v != "" {
		base = append(base, attribute.String("service.version", v))
	}
	base = append(base, environmentAttrs(env.Env())...)
	if pod := os.Getenv("POD_NAME"); pod != "" {
		base = append(base, attribute.String("k8s.pod.name", pod))
	}
	if ns := os.Getenv("POD_NAMESPACE"); ns != "" {
		base = append(base, attribute.String("k8s.namespace.name", ns))
	}

	// Lowest layer: the SDK's default service name and SDK attributes
	// (resource.Default is not used because it caches the environment of its
	// first call). gglib's values replace them; the host detector and
	// OTEL_SERVICE_NAME / OTEL_RESOURCE_ATTRIBUTES replace those.
	sdk, err := resource.New(ctx, resource.WithTelemetrySDK(),
		resource.WithAttributes(attribute.String("service.name", "unknown_service:"+filepath.Base(os.Args[0]))))
	if err != nil {
		return nil, fmt.Errorf("otel: resource: %w", err)
	}
	res, err := resource.Merge(sdk, resource.NewSchemaless(base...))
	if err != nil {
		return nil, fmt.Errorf("otel: resource: %w", err)
	}
	detected, err := resource.New(ctx, resource.WithHost(), resource.WithFromEnv())
	if err != nil {
		return nil, fmt.Errorf("otel: resource: %w", err)
	}
	if res, err = resource.Merge(res, detected); err != nil {
		return nil, fmt.Errorf("otel: resource: %w", err)
	}

	var overrides []attribute.KeyValue
	if cfg.serviceName != "" {
		overrides = append(overrides, attribute.String("service.name", cfg.serviceName))
	}
	if cfg.serviceVersion != "" {
		overrides = append(overrides, attribute.String("service.version", cfg.serviceVersion))
	}
	if cfg.environment != "" {
		overrides = append(overrides, environmentAttrs(cfg.environment)...)
	}
	overrides = append(overrides, cfg.attributes...)
	res, err = resource.Merge(res, resource.NewSchemaless(overrides...))
	if err != nil {
		return nil, fmt.Errorf("otel: resource: %w", err)
	}
	return res, nil
}

// environmentAttrs sets the current semantic-convention name and the older
// deployment.environment, which some backends still read.
func environmentAttrs(e string) []attribute.KeyValue {
	return []attribute.KeyValue{
		attribute.String("deployment.environment.name", e),
		attribute.String("deployment.environment", e),
	}
}
