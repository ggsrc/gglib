package resource

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
)

// Resource is a shared dependency with a lifecycle, such as a database pool.
// Start must not block. Stop should be safe to call on a resource that was
// never started.
type Resource interface {
	Name() string
	Init(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	OK(ctx context.Context) error
}

// ResourceManager runs the lifecycle of a group of resources.
type ResourceManager interface {
	AddResource(resource Resource) error
	Init(ctx context.Context) error
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	OK(ctx context.Context) error
}

type resourceManager struct {
	mu          sync.Mutex
	resources   []Resource
	initialized int // resources[:initialized] have been initialized
	stopped     bool
}

// NewResourceManager returns a ResourceManager for resources. Resources are
// initialized and started in the order given and stopped in reverse order, so
// register a dependency (a database pool) before the resources that use it (a
// worker that writes to it): the worker then stops first.
func NewResourceManager(resources []Resource) ResourceManager {
	return &resourceManager{
		resources: append([]Resource(nil), resources...),
	}
}

// AddResource registers one more resource. It is safe for concurrent use and
// always returns nil.
func (rm *resourceManager) AddResource(resource Resource) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.resources = append(rm.resources, resource)
	return nil
}

func (rm *resourceManager) snapshot() []Resource {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	return append([]Resource(nil), rm.resources...)
}

// stopOrder is the order Stop uses: the reverse of registration, so a resource
// stops before the resources registered ahead of it, which it may use.
func (rm *resourceManager) stopOrder() []Resource {
	rs := rm.snapshot()
	slices.Reverse(rs)
	return rs
}

// Init initializes, one by one and in order, the resources that are not
// initialized yet, and returns the first error, prefixed with the resource
// name. After a failure, calling Init again continues with the resource that
// failed, so a resource whose Init can be retried (it keeps no state when it
// fails) can be retried until it succeeds.
func (rm *resourceManager) Init(ctx context.Context) error {
	rm.mu.Lock()
	done := rm.initialized
	pending := append([]Resource(nil), rm.resources[done:]...)
	rm.mu.Unlock()
	for _, r := range pending {
		if err := r.Init(ctx); err != nil {
			return fmt.Errorf("resource %s: init: %w", r.Name(), err)
		}
		rm.mu.Lock()
		rm.initialized++
		rm.mu.Unlock()
	}
	return nil
}

// Start starts the resources one by one, in order, with ctx. If one fails,
// Start stops every resource and returns the error, prefixed with the
// resource name.
func (rm *resourceManager) Start(ctx context.Context) error {
	for _, r := range rm.snapshot() {
		if err := r.Start(ctx); err != nil {
			startErr := fmt.Errorf("resource %s: start: %w", r.Name(), err)
			return errors.Join(startErr, rm.Stop(ctx))
		}
	}
	return nil
}

// Stop stops every resource in reverse registration order, even when some
// fail, and returns their errors joined, each prefixed with the resource name.
// Only the first call stops anything; later calls return nil.
func (rm *resourceManager) Stop(ctx context.Context) error {
	rm.mu.Lock()
	if rm.stopped {
		rm.mu.Unlock()
		return nil
	}
	rm.stopped = true
	rm.mu.Unlock()

	var errs []error
	for _, r := range rm.stopOrder() {
		if err := r.Stop(ctx); err != nil {
			errs = append(errs, fmt.Errorf("resource %s: stop: %w", r.Name(), err))
		}
	}
	return errors.Join(errs...)
}

// OK checks every resource and returns their errors joined, each prefixed
// with the resource name.
func (rm *resourceManager) OK(ctx context.Context) error {
	var errs []error
	for _, r := range rm.snapshot() {
		if err := r.OK(ctx); err != nil {
			errs = append(errs, fmt.Errorf("resource %s: %w", r.Name(), err))
		}
	}
	return errors.Join(errs...)
}
