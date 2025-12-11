package resource

import (
	"context"
	"fmt"

	"github.com/pkg/errors"
	"golang.org/x/sync/errgroup"
)

type Resource interface {
	Name() string
	Init(ctx context.Context) error
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
	Init(ctx context.Context) error
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

func (rm *resourceManager) Init(ctx context.Context) error {
	for _, r := range rm.resources {
		err := r.Init(ctx)
		if err != nil {
			return err
		}
	}
	return nil
}

func (rm *resourceManager) Start(ctx context.Context) error {
	g, errCtx := errgroup.WithContext(ctx)
	for _, r := range rm.resources {
		g.Go(func() error {
			err := r.Start(errCtx)
			if err != nil {
				fmt.Printf("failed to start resource:%s, err:%v", r.Name(), err)
				return errors.Wrapf(err, "failed to start resource:%s", r.Name())
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		_ = rm.Stop(ctx)
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
