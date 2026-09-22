package logger

import (
	"log/slog"
	"os"
	"time"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// New creates a configured [slog.Logger] backed by zap.
//
// Configuration is taken from c. Optional [Option] values are applied in the
// order in which they are provided; nil options are ignored.
//
// By default, logs are written to stdout and the output format is resolved
// from [Config.Format] and the development flag configured through options.
// The minimum enabled level, caller information, stack traces, extractors,
// hooks, and other behavior are configured from c and opts.
//
// When sampling is enabled through [SamplerConfig], ordinary records are
// processed by the sampled backend. Records marked with [AlwaysLogAttr] are
// routed through an unsampled backend so that they are emitted regardless of
// the sampler's decision.
//
// New returns the logger together with the atomic level controlling the
// underlying zap cores and the write syncer used by those cores.
//
// The returned [zap.AtomicLevel] can be used to change the minimum enabled
// level dynamically. The returned [zapcore.WriteSyncer] can be used by the
// caller when the underlying output needs to be flushed or otherwise
// synchronized.
//
// The returned logger already contains any attributes configured through
// options.
func New(c Config, opts ...Option) (*slog.Logger, zap.AtomicLevel, zapcore.WriteSyncer) {
	b := &builder{}
	for _, opt := range opts {
		if opt != nil {
			opt.apply(b)
		}
	}
	encoder, out, level := b.prepareCore(c)
	normalCore, sampled := buildCore(encoder, c.Sampler, level, out)

	d := &dualHandler{
		extractors: b.extractors,
		hooks:      b.hooks,
		sampled:    sampled,
		normal:     newHandler(c, normalCore, b.name),
	}
	if sampled {
		// Sampling is active: AlwaysLog-tagged records are routed
		// around the sampler through a second, unsampled backend.
		d.always = newHandler(
			c, zapcore.NewCore(encoder, out, level), b.name,
		)
	}
	return slog.New(d).With(b.attrs...), level, out
}

// newHandler creates a slog handler backed by core and configured according
// to conf.
//
// The handler uses name as its initial logger name.
//
// Caller information is enabled unless disabled through
// [CallerConfig.Disabled]. When enabled, [CallerConfig.AddSkip] is used as
// the additional caller offset.
//
// Stack trace capture is configured from [Config.StackAt].
func newHandler(
	conf Config,
	core zapcore.Core, name string,
) slog.Handler {
	w := &wrapHandler{
		core:    core,
		name:    name,
		stackAt: conf.StackAt,
	}
	if !conf.Caller.Disabled {
		w.addCaller = true
		w.callerSkip = conf.Caller.AddSkip
	}
	return w
}

// prepareCore initializes the encoder, output writer, and atomic log level
// used by the logger.
//
// An encoder explicitly supplied through options takes precedence over the
// default encoder. Likewise, a custom output writer takes precedence over
// stdout.
//
// If no format resolver is supplied, [Config.ResolveFormat] is used.
//
// The returned write syncer wraps the configured writer with
// [zapcore.Lock] so that concurrent writes are serialized.
func (b *builder) prepareCore(c Config) (
	zapcore.Encoder,
	zapcore.WriteSyncer, zap.AtomicLevel,
) {
	if b.encoder == nil {
		b.encoder = defaultEncoder(b.consoleEncoder, b.jsonEncoder)
	}
	if b.resolveFormat == nil {
		b.resolveFormat = c.ResolveFormat
	}
	encoder := b.encoder(
		b.resolveFormat(b.development),
	)
	if b.writer == nil {
		b.writer = os.Stdout
	}
	sync := zapcore.AddSync(b.writer)
	return encoder,
		zapcore.Lock(sync),
		zap.NewAtomicLevelAt(convertSlogToZap(c.MinLevel))
}

// buildCore constructs the backend used for ordinary log records and reports
// whether sampling is active.
//
// Sampling is enabled only when both SamplerConfig.Tick and
// SamplerConfig.First are greater than zero. Otherwise, the returned core is
// an ordinary unsampled zap core and the second AlwaysLog backend is not
// required.
//
// Thereafter values below zero are clamped to zero before constructing the
// zap sampler.
//
// When AlwaysLevel is nil, all records are subject to the configured sampler.
//
// When AlwaysLevel is set, records below that level are sampled while records
// at or above that level bypass sampling. The returned sampled flag is true
// in both cases so that [AlwaysLogAttr] can route explicitly marked records
// through the unsampled backend.
func buildCore(
	encoder zapcore.Encoder,
	sampler SamplerConfig, level zap.AtomicLevel, out zapcore.WriteSyncer,
) (zapcore.Core, bool /* sampled */) {
	core := zapcore.NewCore(encoder, out, level)
	if sampler.Tick <= 0 || sampler.First <= 0 {
		return core, false
	}

	// zapcore.NewSamplerWithOptions silently misbehaves with a negative
	// budget (every record after the first batch is dropped): clamp it.
	thereafter := max(sampler.Thereafter, 0)

	if sampler.AlwaysLevel == nil {
		return zapcore.NewSamplerWithOptions(
			core,
			sampler.Tick,
			sampler.First, thereafter,
		), true
	}
	alwaysLevel := convertSlogToZap(*sampler.AlwaysLevel)

	lo := zap.LevelEnablerFunc(func(l zapcore.Level) bool { return level.Enabled(l) && l < alwaysLevel })
	hi := zap.LevelEnablerFunc(func(l zapcore.Level) bool { return level.Enabled(l) && l >= alwaysLevel })

	loCore := zapcore.NewCore(encoder, out, lo)
	hiCore := zapcore.NewCore(encoder, out, hi)

	return zapcore.NewTee(
		hiCore,
		zapcore.NewSamplerWithOptions(
			loCore,
			sampler.Tick,
			sampler.First, thereafter,
		),
	), true
}

// timeEncoder encodes a timestamp using the local
// "15:04:05.000" time representation.
var timeEncoder = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("15:04:05.000"))
}

// defaultEncoder returns an encoder factory for the requested [Format].
//
// [FormatConsole] uses zap's development encoder with full logger names,
// millisecond-resolution local timestamps, and colorized log levels.
//
// All other formats use zap's production JSON encoder with millisecond
// durations, RFC3339 timestamps, and "message"/"timestamp" field names.
//
// Optional console and JSON encoder customization functions are applied after
// the package's default encoder configuration and can therefore override
// individual encoder settings.
func defaultEncoder(
	console func(*zapcore.EncoderConfig), json func(*zapcore.EncoderConfig),
) func(Format) zapcore.Encoder {
	return func(f Format) zapcore.Encoder {
		switch f {
		case FormatConsole:
			ec := zap.NewDevelopmentEncoderConfig()
			ec.EncodeName = zapcore.FullNameEncoder
			ec.EncodeTime = timeEncoder
			ec.EncodeLevel = zapcore.CapitalColorLevelEncoder
			if console != nil {
				console(&ec)
			}
			return zapcore.NewConsoleEncoder(ec)
		default:
			ec := zap.NewProductionEncoderConfig()
			ec.EncodeDuration = zapcore.MillisDurationEncoder
			ec.EncodeTime = zapcore.RFC3339TimeEncoder
			ec.MessageKey = "message"
			ec.TimeKey = "timestamp"
			if json != nil {
				json(&ec)
			}
			return zapcore.NewJSONEncoder(ec)
		}
	}
}
