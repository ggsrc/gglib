// Package safego starts goroutines that recover panics, so a bug in
// concurrent code fails the work that owns it instead of crashing the
// process. A panic in a goroutine that does not recover it ends the whole
// program, whatever the code that started the goroutine does.
//
// There is one entry point for each shape of concurrent work:
//
//   - [Go] starts detached background work; a panic is logged.
//   - [Async] starts one call whose error the caller receives on a channel;
//     a panic is logged and delivered as [ErrPanic].
//   - [Group] runs a fan-out the caller waits for. A panic is the error of the
//     goroutine that panicked: [Group.Go] returns it from [Group.Wait];
//     [Group.GoInto] stores it in that branch's own error variable.
//   - [Run] is the synchronous boundary for one isolated unit of work: an
//     iteration of a long-lived loop, an item of a batch, a cleanup step. A
//     panic is logged and returned, and the caller continues.
//
// Each panic is logged once, at error level, with the panic value, the stack
// and the label, through the logger of the caller's context
// ([github.com/ggsrc/gglib/zerolog/log.Ctx]). Callers receive [ErrPanic],
// which carries neither the panic value nor the label, so a panic message
// cannot leak into an API response.
//
// Cancellation stays with the caller: these functions never cancel or
// detach the context they are given. Pass context.WithoutCancel(ctx) for work
// that must outlive a request.
package safego

import (
	"context"
	"errors"
	"runtime/debug"

	"golang.org/x/sync/errgroup"

	"github.com/ggsrc/gglib/zerolog/log"
)

// ErrPanic is the error that replaces a recovered panic.
var ErrPanic = errors.New("panic recovered")

// Run calls fn and returns its error. If fn panics, Run logs the panic with
// label and returns [ErrPanic]. A nil fn counts as a panic.
//
// label identifies the call site in logs, for example "billing.sendInvoice".
func Run(ctx context.Context, label string, fn func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			log.Ctx(ctx).Error().
				Interface("panic", r).
				Bytes("stack", debug.Stack()).
				Str("label", label).
				Msg("safego: panic recovered")
			err = ErrPanic
		}
	}()
	return fn()
}

// Go runs fn in a new goroutine and returns immediately. If fn panics, the
// panic is logged with label; nothing else observes it. ctx is used only for
// logging.
func Go(ctx context.Context, label string, fn func()) {
	go func() {
		_ = Run(ctx, label, func() error {
			fn()
			return nil
		})
	}()
}

// Async runs fn in a new goroutine and delivers its error, or [ErrPanic] if
// fn panicked, on the returned channel. The channel has room for the one
// value, so the goroutine finishes even if the caller stops waiting.
func Async(ctx context.Context, label string, fn func() error) <-chan error {
	done := make(chan error, 1)
	go func() { done <- Run(ctx, label, fn) }()
	return done
}

// Group is an [errgroup.Group] whose goroutines recover panics: a panic is
// logged and becomes [ErrPanic], the error of the goroutine that panicked.
//
// Create a Group with [NewGroup] or [WithContext]; the zero value is not
// usable.
type Group struct {
	ctx   context.Context
	label string
	group *errgroup.Group
}

// NewGroup returns a Group that waits for all its goroutines and cancels
// none of them. ctx is used only for logging; label names the group in logs.
func NewGroup(ctx context.Context, label string) *Group {
	return &Group{ctx: ctx, label: label, group: new(errgroup.Group)}
}

// WithContext returns a Group and a context derived from ctx. The derived
// context is canceled when a goroutine started with [Group.Go] returns an
// error or panics, or when [Group.Wait] returns.
func WithContext(ctx context.Context, label string) (*Group, context.Context) {
	group, groupCtx := errgroup.WithContext(ctx)
	return &Group{ctx: ctx, label: label, group: group}, groupCtx
}

// SetLimit limits the number of active goroutines in the group to n; a
// negative n means no limit. See [errgroup.Group.SetLimit].
func (g *Group) SetLimit(n int) { g.group.SetLimit(n) }

// Go runs fn in a new goroutine owned by the group. Its error, or
// [ErrPanic], fails the group: [Group.Wait] returns the first such error, and
// a group from [WithContext] cancels its context.
//
// With a limit set, Go blocks until a goroutine slot is free.
func (g *Group) Go(fn func() error) {
	g.group.Go(g.wrap(fn))
}

// TryGo is [Group.Go] that does not block: it starts fn only if the group is
// below its limit, and reports whether it did.
func (g *Group) TryGo(fn func() error) bool {
	return g.group.TryGo(g.wrap(fn))
}

// GoInto runs fn in a new goroutine owned by the group and stores its error,
// or [ErrPanic], in *errp instead of failing the group. Use it for a branch
// whose failure the caller handles itself, such as an optional source or one
// item of a batch, and read *errp after [Group.Wait]. A nil errp discards the
// error.
func (g *Group) GoInto(errp *error, fn func() error) {
	g.group.Go(func() error {
		err := Run(g.ctx, g.label, fn)
		if errp != nil {
			*errp = err
		}
		return nil
	})
}

// Wait blocks until every goroutine in the group has returned, then returns
// the first error from [Group.Go], or nil.
func (g *Group) Wait() error { return g.group.Wait() }

func (g *Group) wrap(fn func() error) func() error {
	return func() error { return Run(g.ctx, g.label, fn) }
}
