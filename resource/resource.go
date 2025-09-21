package resource

import (
	"context"
)

type Resource interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	HealthCheck(ctx context.Context) error
}

type ResourceManager interface {
	AddResource(resource Resource) error
	GetResource(name string) (Resource, bool)

	StartAll(ctx context.Context) error
	StopAll(ctx context.Context) error
	HealthCheckAll(ctx context.Context) error
}
