package resource

import (
	"context"

	"golang.org/x/sync/errgroup"
)

type Resource interface {
	Name() string
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	OK(ctx context.Context) error
}

type HealthStatus struct {
	Status bool
	Error  error
}

type ResourceManager interface {
	AddResource(resource Resource) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	OK(ctx context.Context) error
}

type resourceManager struct {
	resources []Resource
}

func NewResourceManager(resources []Resource) ResourceManager {
	return &resourceManager{
		resources: resources,
	}
}

func (rm *resourceManager) AddResource(resource Resource) error {
	rm.resources = append(rm.resources, resource)
	return nil
}

func (rm *resourceManager) Start(ctx context.Context) error {
	g, errCtx := errgroup.WithContext(ctx)
	for _, r := range rm.resources {
		g.Go(func() error {
			return r.Start(errCtx)
		})
	}

	if err := g.Wait(); err != nil {
		rm.Stop(ctx)
		return err
	}
	return nil
}

func (rm *resourceManager) Stop(ctx context.Context) error {
	for _, r := range rm.resources {
		err := r.Stop(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (rm *resourceManager) OK(ctx context.Context) error {
	for _, r := range rm.resources {
		err := r.OK(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}
