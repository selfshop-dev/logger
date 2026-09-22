package logger

import (
	"context"
	"log/slog"

	"github.com/selfshop-dev/ctxval"
)

type (
	// ExtractorFunc is a function that extracts zero or more attributes from
	// a context.
	//
	// A nil ExtractorFunc is treated as an extractor that produces no
	// attributes.
	ExtractorFunc func(context.Context) []slog.Attr

	// Extractor extracts zero or more attributes from a context.
	//
	// Implementations should return nil or an empty slice when there are no
	// attributes to add to the log record.
	Extractor interface {
		Extract(context.Context) []slog.Attr
	}
)

// ExtractFrom returns an extractor that reads a value of type T from ctx
// and converts it into a string log attribute named name.
//
// T must have an underlying string type. If ctx contains a non-empty value
// of type T, the returned extractor produces a single [slog.String]
// attribute using name as its key.
//
// If ctx does not contain a value of type T, or the stored value is an empty
// string, the extractor returns nil.
//
// The lookup is performed using [ctxval.Get], so values stored by
// [ctxval.With] for the same type T are eligible for extraction.
func ExtractFrom[T ~string](name string) ExtractorFunc {
	return func(ctx context.Context) []slog.Attr {
		v, ok := ctxval.Get[T](ctx)
		if ok && v != "" {
			return []slog.Attr{slog.String(name, string(v))}
		}
		return nil
	}
}

// Extract invokes fn with ctx and returns the attributes produced by it.
//
// A nil ExtractorFunc returns nil without invoking a function.
//
// Extract does not otherwise modify or normalize ctx. In particular, a nil
// context is passed to fn as-is; callers whose extractor requires a non-nil
// context are responsible for handling it.
func (fn ExtractorFunc) Extract(ctx context.Context) []slog.Attr {
	if fn == nil {
		return nil
	}
	return fn(ctx)
}

type extractors []Extractor

// ExtractAttrs extracts attributes from all configured extractors and adds
// them to r.
//
// Nil extractors are ignored. Extractors that return no attributes are also
// ignored.
//
// Attributes are added to r in extractor order. If multiple extractors
// produce attributes with the same key, all of them are added; no
// deduplication or replacement is performed here.
//
// The caller must provide a non-nil [slog.Record].
func (exs extractors) ExtractAttrs(ctx context.Context, r *slog.Record) {
	for _, ex := range exs {
		if ex == nil {
			continue
		}
		if attrs := ex.Extract(ctx); len(attrs) > 0 {
			r.AddAttrs(attrs...)
		}
	}
}
