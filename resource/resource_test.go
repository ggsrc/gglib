package resource

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type recorder struct {
	mu     sync.Mutex
	events []string
}

func (r *recorder) add(e string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) list() []string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]string(nil), r.events...)
}

type fake struct {
	name                       string
	rec                        *recorder
	initErr, startErr, stopErr error
	okErr                      error
	startCtx                   context.Context
}

func (f *fake) Name() string { return f.name }
func (f *fake) Init(context.Context) error {
	f.rec.add("init " + f.name)
	return f.initErr
}

func (f *fake) Start(ctx context.Context) error {
	f.rec.add("start " + f.name)
	f.startCtx = ctx
	return f.startErr
}

func (f *fake) Stop(context.Context) error {
	f.rec.add("stop " + f.name)
	return f.stopErr
}
func (f *fake) OK(context.Context) error { return f.okErr }

func newFakes(rec *recorder, names ...string) []*fake {
	out := make([]*fake, len(names))
	for i, n := range names {
		out[i] = &fake{name: n, rec: rec}
	}
	return out
}

func manager(fs ...*fake) ResourceManager {
	rs := make([]Resource, len(fs))
	for i, f := range fs {
		rs[i] = f
	}
	return NewResourceManager(rs)
}

func TestLifecycleOrder(t *testing.T) {
	rec := &recorder{}
	fs := newFakes(rec, "db", "cache")
	rm := manager(fs...)

	require.NoError(t, rm.Init(t.Context()))
	require.NoError(t, rm.Start(t.Context()))
	require.NoError(t, rm.OK(t.Context()))
	require.NoError(t, rm.Stop(t.Context()))
	assert.Equal(t, []string{"init db", "init cache", "start db", "start cache", "stop cache", "stop db"}, rec.list())
}

func TestInitStopsAtFirstError(t *testing.T) {
	rec := &recorder{}
	fs := newFakes(rec, "db", "cache", "queue")
	fs[1].initErr = errors.New("unreachable")

	err := manager(fs...).Init(t.Context())
	require.ErrorIs(t, err, fs[1].initErr)
	require.EqualError(t, err, "resource cache: init: unreachable")
	assert.Equal(t, []string{"init db", "init cache"}, rec.list())
}

// A failed Init used to start over from the first resource on the next call,
// so retrying failed on the resources already initialized.
func TestInitResumesAfterAFailure(t *testing.T) {
	rec := &recorder{}
	fs := newFakes(rec, "db", "cache", "queue")
	fs[1].initErr = errors.New("unreachable")
	rm := manager(fs...)

	require.Error(t, rm.Init(t.Context()))
	fs[1].initErr = nil
	require.NoError(t, rm.Init(t.Context()))
	require.NoError(t, rm.Init(t.Context()), "nothing left to initialize")
	assert.Equal(t, []string{"init db", "init cache", "init cache", "init queue"}, rec.list())

	require.NoError(t, rm.AddResource(&fake{name: "late", rec: rec}))
	require.NoError(t, rm.Init(t.Context()))
	assert.Equal(t, "init late", rec.list()[4], "a resource added later is initialized by the next Init")
}

// Start used to pass an errgroup context that was canceled as soon as Start
// returned, and ran the resources in parallel.
func TestStartUsesCallerContext(t *testing.T) {
	rec := &recorder{}
	fs := newFakes(rec, "db", "worker")
	require.NoError(t, manager(fs...).Start(t.Context()))
	for _, f := range fs {
		require.NoError(t, f.startCtx.Err(), "%s: start context still alive after Start", f.name)
	}
	assert.Equal(t, []string{"start db", "start worker"}, rec.list())
}

func TestStartFailureStopsEverything(t *testing.T) {
	rec := &recorder{}
	fs := newFakes(rec, "db", "cache", "worker")
	fs[1].startErr = errors.New("bad")
	fs[0].stopErr = errors.New("db close")

	err := manager(fs...).Start(t.Context())
	require.ErrorIs(t, err, fs[1].startErr)
	require.ErrorIs(t, err, fs[0].stopErr)
	assert.Contains(t, err.Error(), "resource cache: start: bad")
	assert.Equal(t, []string{"start db", "start cache", "stop worker", "stop cache", "stop db"}, rec.list())
}

// Stop used to run in registration order, so a database pool registered before
// the worker that writes to it was closed while the worker was still running.
func TestStopRunsInReverseRegistrationOrder(t *testing.T) {
	rec := &recorder{}
	rm := manager(newFakes(rec, "db", "cache")...)
	require.NoError(t, rm.AddResource(&fake{name: "worker", rec: rec}))

	require.NoError(t, rm.Stop(t.Context()))
	assert.Equal(t, []string{"stop worker", "stop cache", "stop db"}, rec.list())
}

// Stop used to give up at the first error, leaking the rest.
func TestStopContinuesAfterErrors(t *testing.T) {
	rec := &recorder{}
	fs := newFakes(rec, "db", "cache", "worker")
	fs[0].stopErr = errors.New("e0")
	fs[2].stopErr = errors.New("e2")
	rm := manager(fs...)

	err := rm.Stop(t.Context())
	require.ErrorIs(t, err, fs[0].stopErr)
	require.ErrorIs(t, err, fs[2].stopErr)
	assert.Contains(t, err.Error(), "resource db: stop: e0")
	assert.Contains(t, err.Error(), "resource worker: stop: e2")
	assert.Len(t, rec.list(), 3, "every resource was stopped")

	require.NoError(t, rm.Stop(t.Context()), "a second Stop does nothing")
	assert.Len(t, rec.list(), 3)
}

func TestOKReportsEveryFailureByName(t *testing.T) {
	rec := &recorder{}
	fs := newFakes(rec, "db", "cache")
	fs[0].okErr = errors.New("ping db")
	fs[1].okErr = errors.New("ping cache")

	err := manager(fs...).OK(t.Context())
	require.ErrorIs(t, err, fs[0].okErr)
	require.ErrorIs(t, err, fs[1].okErr)
	assert.Contains(t, err.Error(), "resource db: ping db")
	assert.Contains(t, err.Error(), "resource cache: ping cache")
}

func TestAddResourceConcurrently(t *testing.T) {
	rec := &recorder{}
	rm := NewResourceManager(nil)
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() { require.NoError(t, rm.AddResource(&fake{name: "r", rec: rec})) })
	}
	wg.Wait()
	require.NoError(t, rm.Init(t.Context()))
	assert.Len(t, rec.list(), 20)
}

func TestNewResourceManagerCopiesTheSlice(t *testing.T) {
	rec := &recorder{}
	rs := []Resource{&fake{name: "a", rec: rec}}
	rm := NewResourceManager(rs)
	rs[0] = &fake{name: "b", rec: rec}
	require.NoError(t, rm.Init(t.Context()))
	assert.Equal(t, []string{"init a"}, rec.list())
}
