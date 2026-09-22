// Package logger provides structured logging with a log/slog API and a zap
// backend.
//
// The package is designed around a small set of composable features: a single
// constructor, immutable configuration options, context-aware values,
// extractors, hooks, logger naming, format selection, and optional sampling.
//
// # Basic usage
//
// Create a logger from Config and optional functional options:
//
//	l, _, out := logger.New(logger.Config{},
//		logger.WithWriter(os.Stdout),
//	)
//	defer logger.SafeSync(out)
//
//	l.Info("server started", "addr", ":8080")
//
// New returns a *slog.Logger together with the underlying zap.AtomicLevel and
// zapcore.WriteSyncer. The logger is immediately ready for use. The returned
// WriteSyncer can be passed to [SafeSync] when the application needs to flush
// buffered output without treating unsupported Sync operations (for example,
// some terminal writers) as fatal errors.
//
// # Configuration
//
// A zero Config is valid. Its effective minimum level is [slog.LevelInfo], the
// format is resolved as JSON unless development mode is enabled, caller
// information is enabled, and sampling is disabled.
//
// [Config.Format] accepts [FormatAuto], [FormatJSON], and [FormatConsole]. In
// auto mode, [WithDevelopment] selects the console encoder; otherwise JSON is
// selected. [WithResolveFormat] and [WithEncoder] allow applications to
// override this decision or provide a completely custom zap encoder.
//
// Caller information is enabled by default. Set [Config.Caller].Disabled to
// true to disable it, or use [Config.Caller].AddSkip when a wrapper adds an
// additional call frame that should not be reported as the caller.
//
// # Context integration
//
// [IntoContext] stores a logger in a [context.Context]. [FromContext] returns
// the stored logger, or [slog.Default] when none is present. [FromContextOr]
// behaves the same way but uses the supplied fallback before falling back to
// [slog.Default]. A nil logger passed to [IntoContext] leaves the context
// unchanged.
//
//	ctx = logger.IntoContext(ctx, l)
//	logger.FromContext(ctx).Info("request finished")
//
// # Extractors
//
// Extractors turn typed values already carried by a context into slog
// attributes before a record reaches hooks and the output handler.
// [ExtractFrom] creates an extractor for any type whose underlying type is
// string:
//
//	type RequestID string
//
//	ex := logger.ExtractFrom[RequestID]("request_id")
//	l, _, _ := logger.New(logger.Config{}, logger.WithExtractors(ex))
//
// Only a non-empty value is emitted. The type-safe value lookup itself is
// provided by the `ctxval` package used by this package.
//
// Multiple extractors are evaluated in registration order. Nil extractors are
// ignored.
//
// # Hooks
//
// Hooks can inspect or modify a [slog.Record] before it is handled and can run
// cleanup or post-processing afterwards. A before hook is registered with
// [BeforeHook]; an after hook is registered with [AfterHook].
//
// Before hooks run in registration order. If a before hook returns an error,
// later before hooks are not executed, the already executed hooks receive an
// After call, and the error is returned from the logging handler. After hooks
// run in reverse order for the hooks whose Before method completed
// successfully.
//
//	logger, _, _ := logger.New(logger.Config{}, logger.WithHooks(
//		logger.BeforeHook(func(_ context.Context, r *slog.Record) error {
//			r.AddAttrs(slog.Bool("handled", true))
//			return nil
//		}),
//	))
//
// # Naming
//
// [WithNamed] assigns an initial logger name. [Named] derives a logger with an
// additional name segment. Segments are trimmed, empty segments are ignored,
// and non-empty segments are joined with a dot.
//
//	l := logger.Named(logger, "http")
//	l = logger.Named(l, "server")
//
// The resulting logger name is `http.server`.
//
// # Attributes
//
// [WithAttrs] adds attributes to every record produced by the logger. It
// accepts either slog.Attr values or the standard slog key/value form. Empty
// attribute keys are ignored, and a dangling key without a value is discarded
// instead of being passed to slog.
//
// # Sampling
//
// Sampling is enabled when both [SamplerConfig.Tick] and
// [SamplerConfig.First] are positive. The first First records in a sampling
// interval are emitted; [SamplerConfig.Thereafter] controls the subsequent
// budget and negative values are treated as zero.
//
// [SamplerConfig.AlwaysLevel] splits records by level: records at or above the
// configured level bypass sampling, while lower-level records are sampled.
// Independently of that threshold, [AlwaysLogAttr] marks a record to use the
// unsampled backend when sampling is active.
//
// [AlwaysLogAttr] is an internal control attribute and is not emitted as a
// normal user-facing field. [LogNamingAttr] is likewise consumed by the
// handler to set the logger name for a record and is not emitted as an ordinary
// attribute.
//
// # Formats
//
// [ParseFormat] parses `auto`, `json`, and `console` case-insensitively.
// [Format.MarshalText] rejects values outside the supported range so invalid
// formats are not silently serialized into configuration.
//
// JSON output uses production-style zap encoding. Console output uses a
// development-style encoder with full logger names and millisecond-resolution
// local time formatting. Encoder configuration can be customized with
// [WithConsoleEncoder], [WithJSONEncoder], or replaced entirely with
// [WithEncoder].
//
// # Synchronization
//
// [SafeSync] flushes the returned output writer and normalizes platform-specific
// errors commonly produced when syncing stdout or stderr is unsupported. An
// unexpected sync error is wrapped and returned to the caller.
//
// The package does not mutate a caller's context or slog logger in place:
// operations such as [IntoContext], [Named], and slog's With methods derive
// new values according to the standard library's immutable-style API.
package logger
