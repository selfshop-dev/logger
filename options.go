package logger

import (
	"io"
	"log/slog"

	"go.uber.org/zap/zapcore"
)

// builder accumulates logger configuration while [New] processes options.
//
// A builder is intended for internal use and is not safe for concurrent use.
// Options are applied in the order supplied to [New].
type builder struct {
	hooks      []Hook
	attrs      []any
	extractors []Extractor

	encoder        func(Format) zapcore.Encoder
	consoleEncoder func(*zapcore.EncoderConfig)
	jsonEncoder    func(*zapcore.EncoderConfig)

	resolveFormat func(bool) Format

	name   string
	writer io.Writer

	development bool
}

type (
	// optionFunc adapts a function to the [Option] interface.
	optionFunc func(*builder)

	// Option configures a logger created by [New].
	//
	// Options are applied in the order in which they are passed to New.
	// When multiple options configure the same setting, a later option
	// generally replaces the corresponding earlier setting, while collection
	// options such as WithAttrs, WithHooks, and WithExtractors append to the
	// existing configuration.
	Option interface {
		apply(*builder)
	}
)

// apply applies the option to b.
//
// A nil option is ignored.
func (f optionFunc) apply(b *builder) {
	if f != nil {
		f(b)
	}
}

// WithWriter configures w as the destination for log output.
//
// A nil writer is ignored. If no non-nil writer is supplied, [New] writes
// logs to [os.Stdout].
func WithWriter(w io.Writer) Option {
	return optionFunc(func(b *builder) {
		if w != nil {
			b.writer = w
		}
	})
}

// WithNamed sets the initial logger name.
//
// The name is normalized using the package's logger-name normalization rules.
// An empty or otherwise invalid name is ignored.
//
// The configured name becomes the initial name of the handler created by
// [New]. It can subsequently be extended with [Named] or handler-specific
// naming operations.
func WithNamed(s string) Option {
	return optionFunc(func(b *builder) {
		if v := normalizeName(s); v != "" {
			b.name = v
		}
	})
}

// WithExtractors registers context attribute extractors.
//
// Extractors are executed in registration order when a log record is handled.
// Nil extractors are ignored.
//
// Multiple calls append to the existing extractor list.
func WithExtractors(extractors ...Extractor) Option {
	return optionFunc(func(b *builder) {
		for _, ex := range extractors {
			if ex != nil {
				b.extractors = append(b.extractors, ex)
			}
		}
	})
}

// WithHooks registers log processing hooks.
//
// Hooks are executed in registration order during the before phase and in
// reverse order during the after phase. See [Hook], [BeforeHook], and
// [AfterHook].
//
// Nil hooks are ignored.
//
// Multiple calls append to the existing hook list.
func WithHooks(hooks ...Hook) Option {
	return optionFunc(func(b *builder) {
		for _, h := range hooks {
			if h != nil {
				b.hooks = append(b.hooks, h)
			}
		}
	})
}

// WithAttrs adds attributes that are attached to the logger returned by [New].
//
// The arguments follow the same general representation accepted by
// [slog.Logger.With]: either [slog.Attr] values or key/value pairs.
//
// Invalid arguments are discarded rather than being allowed to trigger the
// panic that slog would produce for malformed key/value input. In particular,
// a trailing key without a value is ignored, non-string keys are ignored, and
// attributes with an empty key are ignored.
//
// Multiple calls append attributes to the existing list.
func WithAttrs(attrs ...any) Option {
	return optionFunc(func(b *builder) {
		b.attrs = append(b.attrs, sanitizeAttrs(attrs)...)
	})
}

// sanitizeAttrs filters the argument list accepted by WithAttrs into a form
// that can safely be passed to [slog.Logger.With].
//
// A non-empty [slog.Attr] is preserved as a single argument. For key/value
// arguments, only pairs whose key is a non-empty string are preserved.
//
// Invalid input is silently discarded. In particular:
//
//   - slog.Attr values with an empty key are discarded;
//   - a key without a following value is discarded;
//   - non-string keys are discarded together with their following value.
//
// The function preserves the order of all accepted arguments.
func sanitizeAttrs(attrs []any) []any {
	out := make([]any, 0, len(attrs))
	for i := 0; i < len(attrs); {
		// a lone Attr is a valid With argument
		if a, ok := attrs[i].(slog.Attr); ok {
			if a.Key != "" {
				out = append(out, a)
			}
			i++
			continue
		}
		if i+1 >= len(attrs) {
			// dangling key without a value: slog would panic
			break
		}
		key, ok := attrs[i].(string)
		if !ok || key == "" {
			i += 2
			continue
		}
		out = append(out, attrs[i], attrs[i+1])
		i += 2
	}
	return out
}

// WithDevelopment enables development-mode format resolution.
//
// When no custom format resolver is supplied, development mode causes
// [Config.ResolveFormat] to select [FormatConsole] for [FormatAuto].
// Explicitly configured formats are not overridden.
func WithDevelopment() Option {
	return optionFunc(func(b *builder) { b.development = true })
}

// WithResolveFormat overrides the function used to resolve the effective
// output format.
//
// The function receives the development-mode flag and returns the format to
// use by the encoder.
//
// A nil function is ignored.
//
// A later WithResolveFormat option replaces the resolver configured by an
// earlier one.
func WithResolveFormat(fn func(development bool) Format) Option {
	return optionFunc(func(b *builder) {
		if fn != nil {
			b.resolveFormat = fn
		}
	})
}

// WithEncoder overrides the encoder factory used to create the logger's
// output encoder.
//
// The function is called with the resolved [Format] and must return the
// corresponding [zapcore.Encoder].
//
// A nil function is ignored.
//
// A later WithEncoder option replaces the encoder factory configured by an
// earlier one.
func WithEncoder(fn func(Format) zapcore.Encoder) Option {
	return optionFunc(func(b *builder) {
		if fn != nil {
			b.encoder = fn
		}
	})
}

// WithConsoleEncoder customizes the default console encoder configuration.
//
// The function is called with the [zapcore.EncoderConfig] after the package's
// default console settings have been initialized. It may modify those
// settings before the encoder is created.
//
// A nil function is ignored.
//
// This option affects the package's default encoder only; it has no effect
// when a custom encoder factory is supplied through [WithEncoder].
func WithConsoleEncoder(fn func(*zapcore.EncoderConfig)) Option {
	return optionFunc(func(b *builder) {
		if fn != nil {
			b.consoleEncoder = fn
		}
	})
}

// WithJSONEncoder customizes the default JSON encoder configuration.
//
// The function is called with the [zapcore.EncoderConfig] after the package's
// default JSON settings have been initialized. It may modify those settings
// before the encoder is created.
//
// A nil function is ignored.
//
// This option affects the package's default encoder only; it has no effect
// when a custom encoder factory is supplied through [WithEncoder].
func WithJSONEncoder(fn func(*zapcore.EncoderConfig)) Option {
	return optionFunc(func(b *builder) {
		if fn != nil {
			b.jsonEncoder = fn
		}
	})
}
