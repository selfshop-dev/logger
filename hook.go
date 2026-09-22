package logger

import (
	"context"
	"log/slog"
	"slices"
)

type (
	// HookFunc is a function that can be used as a before-hook.
	//
	// The function receives the logging context and a mutable log record.
	// Returning an error stops execution of subsequent before-hooks.
	//
	// HookFunc implements [Hook] by using the function for [Hook.Before] and
	// providing a no-op implementation of [Hook.After].
	HookFunc func(context.Context, *slog.Record) error

	// Hook participates in the log record processing lifecycle.
	//
	// Before hooks are executed in registration order. If a Before method
	// returns an error, execution stops and hooks that have not yet executed
	// are not called.
	//
	// After is called for hooks whose Before method completed successfully.
	// After hooks are executed in reverse order.
	Hook interface {
		Before(context.Context, *slog.Record) error
		After(context.Context, slog.Record, error)
	}
)

// Before invokes fn for the before-hook phase.
//
// A nil HookFunc is treated as a no-op and returns nil.
//
// The record is passed to fn as-is and may be modified by the function.
// Any error returned by fn is propagated to the caller.
func (fn HookFunc) Before(ctx context.Context, r *slog.Record) error {
	if fn == nil {
		return nil
	}
	return fn(ctx, r)
}

// After implements [Hook] for HookFunc.
//
// HookFunc has no after-hook behavior, so After always does nothing.
func (HookFunc) After(context.Context, slog.Record, error) {}

// BeforeHook converts a function into a [Hook] that runs during the before
// phase.
//
// A nil function returns nil and can therefore be safely included in a hook
// collection.
func BeforeHook(fn func(context.Context, *slog.Record) error) Hook {
	if fn == nil {
		return nil
	}
	return HookFunc(fn)
}

// afterHook adapts a function to the after-hook part of [Hook].
//
// Its Before method is a no-op; the configured function is invoked only during
// the after phase.
type afterHook struct {
	fn func(context.Context, slog.Record, error)
}

// Before implements [Hook] for afterHook and does nothing.
func (h afterHook) Before(context.Context, *slog.Record) error {
	return nil
}

// After invokes the configured after-hook function.
//
// A nil function is treated as a no-op.
func (h afterHook) After(ctx context.Context, r slog.Record, err error) {
	if h.fn == nil {
		return
	}
	h.fn(ctx, r, err)
}

// AfterHook converts a function into a [Hook] that runs during the after
// phase.
//
// The function receives the final log record and the error produced during
// processing, if any.
//
// A nil function returns nil and can therefore be safely included in a hook
// collection.
func AfterHook(fn func(context.Context, slog.Record, error)) Hook {
	if fn == nil {
		return nil
	}
	return afterHook{fn: fn}
}

// hooks is an ordered collection of [Hook] values.
//
// Before hooks are executed from first to last. After hooks are executed from
// last to first, matching the usual nested-resource lifecycle semantics.
type hooks []Hook

// RunBefore executes the before phase for all hooks in registration order.
//
// Nil hooks are ignored.
//
// If a hook returns an error, execution stops immediately and the error is
// returned. The returned executed collection contains only hooks whose Before
// method completed successfully.
//
// The returned executed collection is intended to be passed to [hooks.RunAfter]
// so that only successfully entered hooks receive the corresponding after
// notification.
func (hs hooks) RunBefore(ctx context.Context, r *slog.Record) (executed hooks, err error) {
	executed = make(hooks, 0, len(hs))
	for _, h := range hs {
		if h == nil {
			continue
		}
		if err = h.Before(ctx, r); err != nil {
			return executed, err
		}
		executed = append(executed, h)
	}
	return executed, nil
}

// RunAfter executes the after phase for the supplied hooks in reverse order.
//
// Nil hooks are ignored.
//
// err is the error produced by the preceding log processing. It is passed
// unchanged to every after-hook.
//
// Hooks are executed in reverse order so that the lifecycle is symmetrical
// with RunBefore: the last successfully entered hook is the first one to
// receive the after notification.
func (hs hooks) RunAfter(ctx context.Context, r slog.Record, err error) {
	for _, h := range slices.Backward(hs) {
		if h != nil {
			h.After(ctx, r, err)
		}
	}
}
