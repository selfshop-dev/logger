package logger

import (
	"context"
	"log/slog"

	"github.com/selfshop-dev/ctxval"
)

// IntoContext returns a derived context carrying l as its logger.
//
// If ctx is nil, [context.Background] is used as the parent context.
//
// If l is nil, ctx is returned unchanged and no logger is stored.
//
// Otherwise, IntoContext returns a child context from which the logger can
// later be retrieved with [FromContext] or [FromContextOr].
//
// IntoContext does not modify the parent context.
func IntoContext(ctx context.Context, l *slog.Logger) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if l == nil {
		return ctx
	}
	return ctxval.With(ctx, l)
}

// FromContext returns the logger stored in ctx.
//
// If ctx contains a non-nil logger previously stored by [IntoContext],
// that logger is returned.
//
// If ctx is nil or does not contain a non-nil logger, [slog.Default] is
// returned.
func FromContext(ctx context.Context) *slog.Logger {
	return FromContextOr(ctx, nil)
}

// FromContextOr returns the logger stored in ctx, or fallback when no logger
// is available.
//
// If ctx contains a non-nil logger previously stored by [IntoContext], that
// logger is returned and fallback is ignored.
//
// If ctx is nil or does not contain a non-nil logger, a non-nil fallback is
// returned when provided.
//
// If neither ctx nor fallback provides a logger, [slog.Default] is returned.
//
// This function always returns a non-nil logger.
func FromContextOr(ctx context.Context, fallback *slog.Logger) *slog.Logger {
	if ctx != nil {
		if l, ok := ctxval.Get[*slog.Logger](ctx); ok && l != nil {
			return l
		}
	}
	if fallback != nil {
		return fallback
	}
	return slog.Default()
}
