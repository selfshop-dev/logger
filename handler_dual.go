package logger

import (
	"context"
	"log/slog"
	"strings"
)

// internalKey identifies an internal log attribute.
//
// Internal attributes use the reserved "@__" prefix and are intended for
// communication between logger components rather than for application log
// output.
type internalKey struct{ key string }

// Bool returns a boolean attribute using the internal key.
func (k internalKey) Bool(v bool) slog.Attr { return slog.Bool(k.key, v) }

// Attr returns an attribute using the internal key and the value's dynamic
// type.
func (k internalKey) Attr(v any) slog.Attr { return slog.Any(k.key, v) }

// String returns a string attribute using the internal key.
func (k internalKey) String(v string) slog.Attr { return slog.String(k.key, v) }

const attrPrefix = "@__"

var (
	alwaysLog = internalKey{attrPrefix + "always_log"}
	logNaming = internalKey{attrPrefix + "log_naming"}
)

// LogNamingAttr returns an internal attribute that associates a naming value
// with the current log record.
//
// The attribute is intended for use by logger internals and handlers. Its key
// uses the logger's reserved internal attribute prefix.
func LogNamingAttr(s string) slog.Attr { return logNaming.String(s) }

// AlwaysLogAttr returns an internal marker attribute that prevents a sampled
// log record from being handled by the normal sampling path.
//
// When sampling is enabled, a record containing this attribute is routed to
// the handler responsible for records that must always be emitted.
//
// The marker is an internal implementation detail and should generally not
// be included directly in application-facing log attributes.
func AlwaysLogAttr() slog.Attr { return alwaysLog.Bool(true) }

// dualHandler is a [slog.Handler] that coordinates normal and unconditional
// log handling.
//
// When sampling is disabled, only normal is used.
//
// When sampling is enabled, normal handles sampled records while always
// handles records marked with [AlwaysLogAttr].
//
// extractors add context-derived attributes before the record is passed to
// hooks. hooks can inspect or modify the record before it is handled.
type dualHandler struct {
	normal slog.Handler
	always slog.Handler // nil when sampled == false

	extractors extractors
	hooks      hooks
	sampled    bool
}

// AddCallerSkip returns a handler whose underlying handlers skip skip
// additional caller stack frames when resolving the log caller.
//
// If skip is zero, h is returned unchanged.
//
// When sampling is enabled, the operation is applied to both normal and
// always. If either underlying handler does not support caller skipping,
// h is returned unchanged.
//
// When sampling is disabled, only normal must support caller skipping.
// If it does not, h is returned unchanged.
func (h *dualHandler) AddCallerSkip(skip int) slog.Handler {
	if skip == 0 {
		return h
	}
	return h.apply(func(handler slog.Handler) (slog.Handler, bool) {
		n, ok := handler.(callerSkipHandler)
		if ok {
			return n.AddCallerSkip(skip), true
		}
		return nil, false
	})
}

// WithName returns a handler with segment appended to the handler's name.
//
// Leading and trailing whitespace in segment is removed. An empty segment
// after trimming leaves h unchanged.
//
// The operation is applied to both underlying handlers when sampling is
// enabled. If any required underlying handler does not support named
// handlers, h is returned unchanged.
func (h *dualHandler) WithName(segment string) slog.Handler {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return h
	}
	return h.apply(func(handler slog.Handler) (slog.Handler, bool) {
		n, ok := handler.(namedHandler)
		if ok {
			return n.WithName(segment), true
		}
		return nil, false
	})
}

// Enabled reports whether a record at level l may be handled.
//
// When sampling is disabled, Enabled delegates to normal.
//
// When sampling is enabled, the record is considered enabled when either
// normal or always reports the level as enabled.
//
// The context is passed to the underlying handlers unchanged.
func (h *dualHandler) Enabled(ctx context.Context, l slog.Level) bool {
	if h.sampled {
		return h.normal.Enabled(ctx, l) || h.always.Enabled(ctx, l)
	}
	return h.normal.Enabled(ctx, l)
}

// Handle processes a log record.
//
// Before handling, the record is checked against the underlying handler's
// WouldWrite method when sampling is enabled. Records that would not be
// written are discarded early.
//
// Context-derived attributes are then added through configured extractors,
// followed by the before-hooks.
//
// A record containing [AlwaysLogAttr] is routed to always when sampling is
// enabled; all other records are routed to normal.
//
// After handling, registered after-hooks are executed with the result of the
// selected handler.
//
// If a before-hook returns an error, after-hooks are still executed with that
// error and the record is not passed to the underlying handler.
func (h *dualHandler) Handle(ctx context.Context, r slog.Record) error {
	if h.shouldSkip(r) {
		return nil
	}
	h.extractors.ExtractAttrs(ctx, &r)

	executed, err := h.hooks.RunBefore(ctx, &r)
	if err != nil {
		executed.RunAfter(ctx, r, err)
		return err
	}

	handler := h.normal
	if h.sampled && hasAlwaysLog(r) {
		handler = h.always
	}

	executed.RunAfter(ctx, r, handler.Handle(ctx, r))
	return nil
}

// WithAttrs returns a new handler with attrs attached to all subsequently
// handled records.
//
// Empty attribute slices leave h unchanged.
//
// When sampling is enabled, the attributes are attached to both normal and
// always so that records routed through either path retain the same
// attributes. The original handler is not modified.
func (h *dualHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	d := &dualHandler{
		extractors: h.extractors,
		hooks:      h.hooks,
		sampled:    h.sampled,
		normal:     h.normal.WithAttrs(attrs),
	}
	if h.sampled {
		d.always = h.always.WithAttrs(attrs)
	}
	return d
}

// WithGroup returns a new handler that groups subsequent attributes under
// group.
//
// Leading and trailing whitespace in group is removed. An empty group after
// trimming leaves h unchanged.
//
// When sampling is enabled, the group is applied to both normal and always.
// The original handler is not modified.
func (h *dualHandler) WithGroup(group string) slog.Handler {
	if group = strings.TrimSpace(group); group == "" {
		return h
	}
	d := &dualHandler{
		extractors: h.extractors,
		hooks:      h.hooks,
		sampled:    h.sampled,
		normal:     h.normal.WithGroup(group),
	}
	if h.sampled {
		d.always = h.always.WithGroup(group)
	}
	return d
}

// hasAlwaysLog reports whether r contains the internal always-log marker.
//
// The marker is recognized only when its value is exactly the boolean true.
// Attribute traversal stops as soon as a matching marker is found.
func hasAlwaysLog(r slog.Record) bool {
	expected := slog.BoolValue(true)
	found := false
	r.Attrs(func(a slog.Attr) bool {
		found = a.Key == alwaysLog.key && a.Value.Equal(expected)
		return !found
	})
	return found
}

// shouldSkip reports whether r can be discarded before the normal handling
// pipeline is executed.
//
// When sampling is disabled, records are never skipped here.
//
// When sampling is enabled, the target handler is selected according to the
// [AlwaysLogAttr] marker. If that handler implements
// WouldWrite(slog.Record) bool, its result determines whether the record
// should be handled.
//
// If the selected handler does not implement WouldWrite, the record is not
// skipped.
func (h *dualHandler) shouldSkip(r slog.Record) bool {
	if !h.sampled {
		return false
	}
	target := h.normal
	if hasAlwaysLog(r) {
		target = h.always
	}
	p, ok := target.(interface{ WouldWrite(slog.Record) bool })
	return ok && !p.WouldWrite(r)
}

// apply applies transform to the underlying handler or handlers and returns
// the resulting dual handler.
//
// When sampling is disabled, transform is applied only to normal.
//
// When sampling is enabled, transform must succeed for both normal and
// always; otherwise the original handler is returned unchanged.
//
// This all-or-nothing behavior prevents normal and always from acquiring
// inconsistent handler configuration.
func (h *dualHandler) apply(
	transform func(slog.Handler) (slog.Handler, bool),
) slog.Handler {
	n, ok := transform(h.normal)
	if !ok {
		return h
	}
	if !h.sampled {
		cp := *h
		cp.normal = n
		return &cp
	}
	a, ok := transform(h.always)
	if !ok {
		return h
	}
	return &dualHandler{
		extractors: h.extractors,
		hooks:      h.hooks,
		sampled:    true,
		normal:     n,
		always:     a,
	}
}
