package logger

import (
	"log/slog"
	"strings"
)

// namedHandler is a [slog.Handler] that supports extending the logger name
// with an additional name segment.
type namedHandler interface {
	slog.Handler
	WithName(segment string) slog.Handler
}

// Named returns a logger with segment appended to its logger name.
//
// Leading and trailing whitespace in segment is removed. If segment is empty
// after trimming, l is returned unchanged.
//
// If l is nil, [slog.Default] is returned.
//
// When the underlying handler implements namedHandler, Named creates a new
// logger using the handler returned by WithName. The original logger is not
// modified.
//
// If the underlying handler does not support named loggers, l is returned
// unchanged.
func Named(l *slog.Logger, segment string) *slog.Logger {
	if l == nil {
		return slog.Default()
	}
	if segment = strings.TrimSpace(segment); segment == "" {
		return l
	}
	h, ok := l.Handler().(namedHandler)
	if ok {
		return slog.New(h.WithName(segment))
	}
	return l
}

// normalizeName normalizes a dotted logger name.
//
// Leading and trailing whitespace is removed from the complete name and from
// each individual segment. Leading and trailing dots are removed, and empty
// segments are discarded.
//
// For example, " api.. users. " is normalized to "api.users".
//
// An empty or whitespace-only name, as well as a name consisting only of
// dots, produces an empty string.
func normalizeName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, ".")

	if s == "" {
		return ""
	}
	parts := strings.Split(s, ".")
	out := parts[:0]

	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		out = append(out, part)
	}
	return strings.Join(out, ".")
}

// joinName combines base and segment into a normalized dotted logger name.
//
// Both values are normalized before they are combined. If either value is
// empty after normalization, the other value is returned unchanged.
// Otherwise, the values are joined with a single dot.
//
// For example, joinName("api.users", "repository") returns
// "api.users.repository".
func joinName(base, segment string) string {
	base = normalizeName(base)
	segment = normalizeName(segment)
	switch {
	case base == "":
		return segment
	case segment == "":
		return base
	default:
		return base + "." + segment
	}
}
