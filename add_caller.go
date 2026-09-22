package logger

import "log/slog"

// callerSkipHandler is a [slog.Handler] that supports adjusting the number
// of stack frames skipped when resolving the caller of a log record.
//
// Implementations typically use the skip value to account for additional
// wrapper functions between the logging call and the actual caller.
type callerSkipHandler interface {
	slog.Handler
	AddCallerSkip(skip int) slog.Handler
}

// AddCallerSkip returns a logger whose handler skips skip additional stack
// frames when determining the source caller of a log record.
//
// If l is nil, [slog.Default] is returned.
//
// If skip is zero, l is returned unchanged.
//
// If the logger's handler implements callerSkipHandler, AddCallerSkip creates
// a new logger using the handler returned by AddCallerSkip. The original
// logger is not modified.
//
// If the logger's handler does not support caller skipping, l is returned
// unchanged. This makes AddCallerSkip safe to use with arbitrary
// [slog.Handler] implementations without requiring callers to know whether
// the underlying handler supports caller information adjustment.
//
// A positive skip value moves the reported caller further up the call stack.
// The exact interpretation of skip is determined by the underlying handler.
//
// AddCallerSkip is useful when logging is performed through wrapper functions
// and the caller reported by the handler would otherwise point at the wrapper
// rather than the code that initiated the log call.
func AddCallerSkip(l *slog.Logger, skip int) *slog.Logger {
	if l == nil || skip == 0 {
		if l == nil {
			return slog.Default()
		}
		return l
	}

	h, ok := l.Handler().(callerSkipHandler)
	if ok {
		return slog.New(h.AddCallerSkip(skip))
	}

	return l
}
