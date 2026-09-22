package logger

import (
	"context"
	"log/slog"
	"runtime"
	"slices"
	"strings"

	"go.uber.org/zap/zapcore"

	"github.com/selfshop-dev/logger/internal/stacktrace"
)

// storedField is a zap field together with the group path that was active
// when the field was stored by WithAttrs.
//
// The path is used to reconstruct the final nested field structure when
// grouped logging is enabled.
type storedField struct {
	path  []string
	field zapcore.Field
}

// wrapHandler adapts a [zapcore.Core] to the [slog.Handler] interface.
//
// In addition to forwarding log records to the underlying zap core,
// wrapHandler maintains slog logger names and groups, supports caller and
// stack trace information, and stores attributes added through WithAttrs.
//
// The handler is immutable from the caller's perspective: methods such as
// WithAttrs, WithGroup, WithName, and AddCallerSkip return derived handlers
// without modifying the original handler.
type wrapHandler struct {
	core zapcore.Core
	name string // logger name

	stored []storedField

	groupPath []string
	grouped   bool

	stackAt *slog.Level

	addCaller  bool
	callerSkip int
}

// AddCallerSkip returns a derived handler that skips skip additional stack
// frames when resolving caller and stack trace information.
//
// A zero skip returns h unchanged.
//
// Negative values are allowed but cannot reduce the effective caller skip
// below zero.
//
// The original handler is not modified.
func (h *wrapHandler) AddCallerSkip(skip int) slog.Handler {
	if skip == 0 {
		return h
	}
	cp := *h
	cp.callerSkip += skip
	if cp.callerSkip < 0 {
		cp.callerSkip = 0
	}
	return &cp
}

// WithName returns a derived handler with segment appended to the current
// logger name.
//
// Leading and trailing whitespace is removed from segment. An empty segment
// leaves h unchanged.
//
// Names are combined using the package's logger-name joining rules.
func (h *wrapHandler) WithName(segment string) slog.Handler {
	segment = strings.TrimSpace(segment)
	if segment == "" {
		return h
	}
	cp := *h
	cp.name = joinName(h.name, segment)
	return &cp
}

// Enabled reports whether the underlying zap core is enabled for l.
//
// slog levels are mapped to the corresponding zap levels before querying the
// core.
func (h *wrapHandler) Enabled(_ context.Context, l slog.Level) bool {
	return h.core.Enabled(convertSlogToZap(l))
}

// WouldWrite reports whether the underlying zap core would accept r.
//
// This method performs the core's level and sampling check using the record's
// level and message. It is intended for handlers that need to decide whether
// a record should be processed before performing more expensive extraction or
// hook operations.
//
// The check does not write the record.
func (h *wrapHandler) WouldWrite(r slog.Record) bool {
	ent := zapcore.Entry{
		Level:   convertSlogToZap(r.Level),
		Message: r.Message,
	}
	return h.core.Check(ent, nil) != nil
}

// Handle converts r into a zap entry and writes it through the underlying
// zap core.
//
// Stored attributes and record attributes are converted to zap fields.
// When grouping is enabled, fields are collected into a group tree and
// materialized only after the underlying core accepts the entry.
//
// The internal log-naming attribute can change the logger name for the
// current record without being emitted as a regular field. Other attributes
// using the logger's reserved internal prefix are ignored.
//
// If the underlying core does not accept the record, Handle returns nil
// without writing the record.
//
// When caller information is enabled and the record contains a valid program
// counter, the caller is resolved using the configured caller skip.
//
// When stackAt is configured, a stack trace is captured for records whose
// level is greater than or equal to stackAt.
//
// Handle does not return errors produced by the underlying zap core after
// Check; zapcore.Entry.Write is used for the actual write operation.
func (h *wrapHandler) Handle(ctx context.Context, r slog.Record) error {
	ent := zapcore.Entry{
		Level:      convertSlogToZap(r.Level),
		LoggerName: h.name,
		Time:       r.Time,
		Message:    r.Message,
	}

	fields := make([]zapcore.Field, 0, len(h.stored)+r.NumAttrs())

	tree := h.buildTree()
	if tree == nil {
		for _, sf := range h.stored {
			fields = append(fields, sf.field)
		}
	}

	name := ent.LoggerName

	r.Attrs(func(a slog.Attr) bool {
		name = h.processAttr(a, name, tree, &fields)
		return true
	})

	ent.LoggerName = name

	ce := h.core.Check(ent, nil)
	if ce == nil {
		return nil
	}

	if tree != nil {
		fields = tree.toFields()
	}

	const baseSkip = 4
	skip := baseSkip + h.callerSkip

	if h.addCaller && r.PC != 0 {
		var pcs [1]uintptr
		n := runtime.Callers(skip+1, pcs[:])
		if n > 0 {
			f, _ := runtime.CallersFrames(pcs[:n]).Next()
			if f.PC != 0 {
				ce.Caller = zapcore.EntryCaller{
					Defined:  true,
					PC:       f.PC,
					File:     f.File,
					Line:     f.Line,
					Function: f.Function,
				}
			}
		}
	}

	if h.stackAt != nil &&
		r.Level >= *h.stackAt {
		ce.Stack = stacktrace.Take(skip)
	}

	ce.Write(fields...)
	return nil
}

// WithAttrs returns a derived handler with attrs stored for subsequent
// records.
//
// Empty attrs leave h unchanged.
//
// Attributes are classified before being stored. The internal log-naming
// attribute changes the derived logger name rather than becoming a field,
// while other internal attributes are discarded.
//
// When grouping is active, each stored field retains the group path that was
// active when WithAttrs was called. This allows the same attributes to be
// materialized correctly when the record is eventually handled.
//
// The original handler is not modified.
func (h *wrapHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	if len(attrs) == 0 {
		return h
	}
	grouped := h.grouped
	name := h.name
	stored := make([]storedField, 0, len(attrs))

	for _, a := range attrs {
		name, grouped, stored = h.consumeAttr(a, name, grouped, stored)
	}
	if name == h.name &&
		grouped == h.grouped && len(stored) == 0 {
		return h
	}
	cp := *h
	cp.grouped = grouped
	cp.name = name

	if len(stored) > 0 {
		cp.stored = append(
			slices.Clone(h.stored),
			stored...,
		)
	}
	return &cp
}

// WithGroup returns a derived handler that places subsequent attributes under
// group.
//
// Leading and trailing whitespace is removed from group. An empty group
// leaves h unchanged.
//
// The group path is copied before the new segment is appended, so the
// original handler is not modified.
func (h *wrapHandler) WithGroup(group string) slog.Handler {
	if group = strings.TrimSpace(group); group == "" {
		return h
	}
	cp := *h
	cp.groupPath = append(slices.Clone(h.groupPath), group)
	cp.grouped = true
	return &cp
}

// convertSlogToZap converts an [slog.Level] into the corresponding
// [zapcore.Level].
//
// Levels at or above [slog.LevelError] map to zapcore.ErrorLevel; levels at
// or above [slog.LevelWarn] map to zapcore.WarnLevel; levels at or above
// [slog.LevelInfo] map to zapcore.InfoLevel. All lower levels map to
// zapcore.DebugLevel.
//
// slog levels are continuous, while this conversion intentionally reduces
// them to zap's four standard severity levels.
func convertSlogToZap(
	l slog.Level,
) zapcore.Level {
	switch {
	case l >= slog.LevelError:
		return zapcore.ErrorLevel
	case l >= slog.LevelWarn:
		return zapcore.WarnLevel
	case l >= slog.LevelInfo:
		return zapcore.InfoLevel
	default:
		return zapcore.DebugLevel
	}
}

// buildTree constructs the group tree required to materialize stored fields.
//
// If grouping is not active, buildTree returns nil and stored fields can be
// written directly without constructing an intermediate tree.
func (h *wrapHandler) buildTree() *groupTree {
	if !h.grouped {
		return nil
	}
	tree := &groupTree{}
	for _, sf := range h.stored {
		tree.add(sf.path, sf.field)
	}
	return tree
}

// processAttr classifies a record attribute and either updates the logger
// name, ignores the attribute, or adds the converted field to the current
// output.
//
// Internal log-naming attributes update name and are not emitted as fields.
// Other attributes using the reserved internal prefix are ignored.
//
// Regular attributes are converted to zap fields and added either directly
// to fields or to tree when grouping is active.
func (h *wrapHandler) processAttr(
	a slog.Attr, name string, tree *groupTree, fields *[]zapcore.Field,
) string {
	name, skip, f := classifyAttr(a, name)
	if skip {
		return name
	}
	if tree != nil {
		tree.add(h.groupPath, f)
	} else {
		*fields = append(*fields, f)
	}
	return name
}

// consumeAttr classifies an attribute while processing WithAttrs.
//
// Regular attributes are stored together with the current group path so that
// they can be materialized later when a record is handled.
//
// The log-naming attribute changes the derived logger name. Other internal
// attributes are ignored.
//
// The returned grouped value becomes true when the current group path is
// non-empty and a regular attribute is stored.
func (h *wrapHandler) consumeAttr(
	a slog.Attr, name string, grouped bool, stored []storedField,
) (
	string, bool, []storedField,
) {
	name, skip, f := classifyAttr(a, name)
	if skip {
		return name, grouped, stored
	}
	stored = append(stored, storedField{
		path:  h.groupPath,
		field: f,
	})
	if len(h.groupPath) > 0 {
		grouped = true
	}
	return name, grouped, stored
}

// classifyAttr determines how an attribute should be handled by the logger.
//
// The internal log-naming attribute changes the current logger name and is
// not emitted as a field.
//
// Any other attribute whose key starts with the reserved internal attribute
// prefix is ignored.
//
// All remaining attributes are converted to zap fields and returned for
// normal processing.
func classifyAttr(a slog.Attr, name string) (newName string, skip bool, field zapcore.Field) {
	var zero zapcore.Field
	switch {
	case a.Key == logNaming.key:
		if v := normalizeName(a.Value.String()); v != "" {
			return v, true, zero
		}
		return name, true, zero
	case strings.HasPrefix(a.Key, attrPrefix):
		return name, true, zero
	default:
		return name, false, convertAttrToField(a)
	}
}
