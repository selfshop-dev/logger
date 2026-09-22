package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestNew(t *testing.T) {
	t.Run("creates logger with configured writer and level", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer

		c := Config{
			MinLevel: slog.LevelInfo,
			Format:   FormatJSON,
		}

		var encoderCalled bool

		encoder := defaultEncoder(nil, nil)(FormatJSON)

		// Act
		l, level, out := New(
			c,
			WithWriter(&buf),
			WithEncoder(func(f Format) zapcore.Encoder {
				encoderCalled = true
				assert.Equal(t, FormatJSON, f)

				return encoder
			}),
		)

		l.Info("hello")

		// Assert
		require.NotNil(t, l)
		require.NotNil(t, out)

		assert.True(t, encoderCalled)
		assert.Equal(t, zap.InfoLevel, level.Level())

		var got map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

		assert.Equal(t, "hello", got["message"])
	})

	t.Run("uses stdout when writer is not configured", func(t *testing.T) {
		// Arrange
		c := Config{
			MinLevel: slog.LevelInfo,
			Format:   FormatJSON,
		}

		// Act
		l, level, out := New(c)

		// Assert
		require.NotNil(t, l)
		require.NotNil(t, out)
		assert.Equal(t, zap.InfoLevel, level.Level())
	})

	t.Run("creates normal handler without sampling", func(t *testing.T) {
		// Arrange
		c := Config{
			MinLevel: slog.LevelInfo,
			Format:   FormatJSON,
		}

		// Act
		l, _, _ := New(c)

		// Assert
		handler, ok := l.Handler().(*dualHandler)
		require.True(t, ok)

		assert.False(t, handler.sampled)
		assert.NotNil(t, handler.normal)
		assert.Nil(t, handler.always)
	})

	t.Run("creates always handler when sampling is enabled", func(t *testing.T) {
		// Arrange
		c := Config{
			MinLevel: slog.LevelInfo,
			Format:   FormatJSON,
			Sampler: SamplerConfig{
				Tick:       time.Second,
				First:      1,
				Thereafter: 1,
			},
		}

		// Act
		l, _, _ := New(c)

		// Assert
		handler, ok := l.Handler().(*dualHandler)
		require.True(t, ok)

		assert.True(t, handler.sampled)
		assert.NotNil(t, handler.normal)
		assert.NotNil(t, handler.always)
	})
}

func TestNew_WithNamed(t *testing.T) {
	// Arrange
	var buf bytes.Buffer

	c := Config{
		MinLevel: slog.LevelInfo,
		Format:   FormatJSON,
	}

	// Act
	l, _, _ := New(
		c,
		WithWriter(&buf),
		WithNamed(" service . api "),
	)

	l.Info("hello")

	// Assert
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	assert.Equal(t, "service.api", got["logger"])
}

func TestNew_WithAttrs(t *testing.T) {
	// Arrange
	var buf bytes.Buffer

	c := Config{
		MinLevel: slog.LevelInfo,
		Format:   FormatJSON,
	}

	// Act
	l, _, _ := New(
		c,
		WithWriter(&buf),
		WithAttrs(
			slog.String("service", "users"),
			"version",
			"1.0",
		),
	)

	l.Info("hello")

	// Assert
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	assert.Equal(t, "users", got["service"])
	assert.Equal(t, "1.0", got["version"])
}

func TestNew_WithDevelopment(t *testing.T) {
	t.Run("auto resolves to console", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer

		c := Config{
			MinLevel: slog.LevelInfo,
			Format:   FormatAuto,
		}

		// Act
		l, _, _ := New(
			c,
			WithWriter(&buf),
			WithDevelopment(),
		)

		l.Info("hello")

		// Assert
		output := buf.String()

		assert.Contains(t, output, "INFO")
		assert.Contains(t, output, "hello")
		assert.NotEmpty(t, 0, len(output))
	})

	t.Run("json is still json in development", func(t *testing.T) {
		// Arrange
		var buf bytes.Buffer

		c := Config{
			MinLevel: slog.LevelInfo,
			Format:   FormatJSON,
		}

		// Act
		l, _, _ := New(
			c,
			WithWriter(&buf),
			WithDevelopment(),
		)

		l.Info("hello")

		// Assert
		var got map[string]any
		require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

		assert.Equal(t, "hello", got["message"])
	})
}

func TestNew_WithResolveFormat(t *testing.T) {
	// Arrange
	var buf bytes.Buffer

	c := Config{
		MinLevel: slog.LevelInfo,
		Format:   FormatConsole,
	}

	// Act
	l, _, _ := New(
		c,
		WithWriter(&buf),
		WithResolveFormat(func(development bool) Format {
			assert.False(t, development)
			return FormatJSON
		}),
	)

	l.Info("hello")

	// Assert
	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	assert.Equal(t, "hello", got["message"])
}

func TestNew_WiresHooksAndExtractors(t *testing.T) {
	// Arrange
	before := HookFunc(func(context.Context, *slog.Record) error {
		return nil
	})

	extractor := ExtractorFunc(func(context.Context) []slog.Attr {
		return nil
	})

	c := Config{
		MinLevel: slog.LevelInfo,
		Format:   FormatJSON,
	}

	// Act
	l, _, _ := New(
		c,
		WithHooks(before),
		WithExtractors(extractor),
	)

	// Assert
	handler, ok := l.Handler().(*dualHandler)
	require.True(t, ok)

	assert.Len(t, handler.hooks, 1)
	assert.Len(t, handler.extractors, 1)
}

func TestNew_WithCaller(t *testing.T) {
	t.Run("caller disabled", func(t *testing.T) {
		// Arrange
		c := Config{
			MinLevel: slog.LevelInfo,
			Format:   FormatJSON,
			Caller: CallerConfig{
				Disabled: true,
			},
		}

		// Act
		l, _, _ := New(c)

		// Assert
		handler, ok := l.Handler().(*dualHandler)
		require.True(t, ok)

		wrap, ok := handler.normal.(*wrapHandler)
		require.True(t, ok)

		assert.False(t, wrap.addCaller)
	})

	t.Run("caller enabled with skip", func(t *testing.T) {
		// Arrange
		c := Config{
			MinLevel: slog.LevelInfo,
			Format:   FormatJSON,
			Caller: CallerConfig{
				AddSkip: 3,
			},
		}

		// Act
		l, _, _ := New(c)

		// Assert
		handler, ok := l.Handler().(*dualHandler)
		require.True(t, ok)

		wrap, ok := handler.normal.(*wrapHandler)
		require.True(t, ok)

		assert.True(t, wrap.addCaller)
		assert.Equal(t, 3, wrap.callerSkip)
	})
}

func TestNew_WithStackAt(t *testing.T) {
	// Arrange
	stackAt := slog.LevelError

	c := Config{
		MinLevel: slog.LevelInfo,
		Format:   FormatJSON,
		StackAt:  &stackAt,
	}

	// Act
	l, _, _ := New(c)

	// Assert
	handler, ok := l.Handler().(*dualHandler)
	require.True(t, ok)

	wrap, ok := handler.normal.(*wrapHandler)
	require.True(t, ok)

	assert.Equal(t, &stackAt, wrap.stackAt)
}

func TestNewHandler(t *testing.T) {
	t.Run("caller disabled", func(t *testing.T) {
		// Arrange
		c := Config{
			Caller: CallerConfig{
				Disabled: true,
				AddSkip:  5,
			},
			StackAt: func() *slog.Level {
				level := slog.LevelError
				return &level
			}(),
		}

		core := zapcore.NewNopCore()

		// Act
		got := newHandler(c, core, "service")

		// Assert
		h, ok := got.(*wrapHandler)
		require.True(t, ok)

		assert.Equal(t, "service", h.name)
		assert.False(t, h.addCaller)
		assert.Equal(t, 0, h.callerSkip)
		assert.Equal(t, c.StackAt, h.stackAt)
		assert.Equal(t, core, h.core)
	})

	t.Run("caller enabled", func(t *testing.T) {
		// Arrange
		c := Config{
			Caller: CallerConfig{
				Disabled: false,
				AddSkip:  3,
			},
		}

		core := zapcore.NewNopCore()

		// Act
		got := newHandler(c, core, "service")

		// Assert
		h, ok := got.(*wrapHandler)
		require.True(t, ok)

		assert.Equal(t, "service", h.name)
		assert.True(t, h.addCaller)
		assert.Equal(t, 3, h.callerSkip)
		assert.Equal(t, c.StackAt, h.stackAt)
		assert.Equal(t, core, h.core)
	})
}

func TestBuilder_PrepareCore(t *testing.T) {
	t.Run("uses configured encoder and writer", func(t *testing.T) {
		// Arrange
		writer := &bytes.Buffer{}
		encoder := defaultEncoder(nil, nil)(FormatJSON)

		var gotFormat Format

		b := &builder{
			writer: writer,
			encoder: func(f Format) zapcore.Encoder {
				gotFormat = f
				return encoder
			},
			resolveFormat: func(development bool) Format {
				assert.True(t, development)
				return FormatJSON
			},
			development: true,
		}

		c := Config{
			MinLevel: slog.LevelWarn,
			Format:   FormatConsole,
		}

		// Act
		gotEncoder, gotOut, gotLevel := b.prepareCore(c)

		// Assert
		require.NotNil(t, gotEncoder)
		require.NotNil(t, gotOut)

		assert.Equal(t, FormatJSON, gotFormat)
		assert.Equal(t, zap.WarnLevel, gotLevel.Level())

		_, err := gotOut.Write([]byte("test"))
		require.NoError(t, err)

		assert.Equal(t, "test", writer.String())
	})

	t.Run("uses config resolver when builder resolver is nil", func(t *testing.T) {
		// Arrange
		b := &builder{
			development: true,
		}

		c := Config{
			MinLevel: slog.LevelInfo,
			Format:   FormatAuto,
		}

		// Act
		gotEncoder, gotOut, gotLevel := b.prepareCore(c)

		// Assert
		require.NotNil(t, gotEncoder)
		require.NotNil(t, gotOut)
		require.NotNil(t, b.encoder)
		require.NotNil(t, b.writer)

		assert.Equal(t, zap.InfoLevel, gotLevel.Level())
	})
}

func TestBuildCore(t *testing.T) {
	t.Run("sampling disabled when tick is not positive", func(t *testing.T) {
		// Arrange
		encoder := testEncoder()
		out := &testWriteSyncer{}
		level := zap.NewAtomicLevelAt(zap.InfoLevel)

		sampler := SamplerConfig{
			Tick:       0,
			First:      1,
			Thereafter: 0,
		}

		// Act
		core, sampled := buildCore(encoder, sampler, level, out)

		// Assert
		assert.False(t, sampled)

		writeCoreEntry(t, core, zap.InfoLevel, "one")
		writeCoreEntry(t, core, zap.InfoLevel, "two")

		assert.Equal(t, 2, out.writes)
	})

	t.Run("sampling disabled when first is not positive", func(t *testing.T) {
		// Arrange
		encoder := testEncoder()
		out := &testWriteSyncer{}
		level := zap.NewAtomicLevelAt(zap.InfoLevel)

		sampler := SamplerConfig{
			Tick:       time.Second,
			First:      0,
			Thereafter: 0,
		}

		// Act
		core, sampled := buildCore(encoder, sampler, level, out)

		// Assert
		assert.False(t, sampled)

		writeCoreEntry(t, core, zap.InfoLevel, "one")
		writeCoreEntry(t, core, zap.InfoLevel, "two")

		assert.Equal(t, 2, out.writes)
	})

	t.Run("sampling is enabled", func(t *testing.T) {
		// Arrange
		encoder := testEncoder()
		out := &testWriteSyncer{}
		level := zap.NewAtomicLevelAt(zap.InfoLevel)

		sampler := SamplerConfig{
			Tick:       time.Hour,
			First:      1,
			Thereafter: 0,
		}

		// Act
		core, sampled := buildCore(encoder, sampler, level, out)

		// Assert
		assert.True(t, sampled)

		writeCoreEntry(t, core, zap.InfoLevel, "same")
		writeCoreEntry(t, core, zap.InfoLevel, "same")
		writeCoreEntry(t, core, zap.InfoLevel, "same")

		assert.Equal(t, 1, out.writes)
	})

	t.Run("negative thereafter is clamped to zero", func(t *testing.T) {
		// Arrange
		encoder := testEncoder()
		out := &testWriteSyncer{}
		level := zap.NewAtomicLevelAt(zap.InfoLevel)

		sampler := SamplerConfig{
			Tick:       time.Hour,
			First:      1,
			Thereafter: -100,
		}

		// Act
		core, sampled := buildCore(encoder, sampler, level, out)

		// Assert
		assert.True(t, sampled)

		writeCoreEntry(t, core, zap.InfoLevel, "same")
		writeCoreEntry(t, core, zap.InfoLevel, "same")

		assert.Equal(t, 1, out.writes)
	})

	t.Run("always level bypasses sampler", func(t *testing.T) {
		// Arrange
		encoder := testEncoder()
		out := &testWriteSyncer{}
		level := zap.NewAtomicLevelAt(zap.InfoLevel)

		alwaysLevel := slog.LevelWarn

		sampler := SamplerConfig{
			Tick:        time.Hour,
			First:       1,
			Thereafter:  0,
			AlwaysLevel: &alwaysLevel,
		}

		// Act
		core, sampled := buildCore(encoder, sampler, level, out)

		// Assert
		assert.True(t, sampled)

		writeCoreEntry(t, core, zap.InfoLevel, "info")
		writeCoreEntry(t, core, zap.InfoLevel, "info")

		writeCoreEntry(t, core, zap.WarnLevel, "warn")
		writeCoreEntry(t, core, zap.WarnLevel, "warn")

		assert.Equal(t, 3, out.writes)
	})

	t.Run("respects minimum level with always level", func(t *testing.T) {
		// Arrange
		encoder := testEncoder()
		out := &testWriteSyncer{}
		level := zap.NewAtomicLevelAt(zap.WarnLevel)

		alwaysLevel := slog.LevelError

		sampler := SamplerConfig{
			Tick:        time.Hour,
			First:       1,
			Thereafter:  0,
			AlwaysLevel: &alwaysLevel,
		}

		// Act
		core, sampled := buildCore(encoder, sampler, level, out)

		// Assert
		assert.True(t, sampled)

		writeCoreEntry(t, core, zap.InfoLevel, "info")
		writeCoreEntry(t, core, zap.WarnLevel, "warn")
		writeCoreEntry(t, core, zap.ErrorLevel, "error")

		assert.Equal(t, 2, out.writes)
	})
}

func TestDefaultEncoder_JSON(t *testing.T) {
	// Arrange
	var called bool

	encoder := defaultEncoder(
		nil,
		func(cfg *zapcore.EncoderConfig) {
			called = true
			cfg.MessageKey = "msg"
		},
	)(FormatJSON)

	entry := zapcore.Entry{
		Level:      zap.InfoLevel,
		Time:       time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		LoggerName: "service.api",
		Message:    "hello",
	}

	// Act
	buf, err := encoder.EncodeEntry(entry, []zapcore.Field{
		zap.String("request_id", "123"),
	})

	// Assert
	require.NoError(t, err)
	assert.True(t, called)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	assert.Equal(t, "hello", got["msg"])
	assert.Equal(t, "123", got["request_id"])
	assert.Equal(t, "service.api", got["logger"])
	assert.Equal(t, "2026-01-02T03:04:05Z", got["timestamp"])
}

func TestDefaultEncoder_Console(t *testing.T) {
	// Arrange
	var called bool

	encoder := defaultEncoder(
		func(cfg *zapcore.EncoderConfig) {
			called = true
			cfg.ConsoleSeparator = " | "
		},
		nil,
	)(FormatConsole)

	entry := zapcore.Entry{
		Level:      zap.InfoLevel,
		Time:       time.Date(2026, 1, 2, 3, 4, 5, 123000000, time.UTC),
		LoggerName: "service.api",
		Message:    "hello",
	}

	// Act
	buf, err := encoder.EncodeEntry(entry, []zapcore.Field{
		zap.String("request_id", "123"),
	})

	// Assert
	require.NoError(t, err)
	assert.True(t, called)

	output := buf.String()

	assert.Contains(t, output, "03:04:05.123")
	assert.Contains(t, output, "INFO")
	assert.Contains(t, output, "service.api")
	assert.Contains(t, output, "hello")
	assert.Contains(t, output, "request_id")
	assert.Contains(t, output, "123")
	assert.Contains(t, output, " | ")
}

func TestDefaultEncoder_AutoUsesJSON(t *testing.T) {
	// Arrange
	encoder := defaultEncoder(nil, nil)(FormatAuto)

	entry := zapcore.Entry{
		Level:   zap.InfoLevel,
		Time:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Message: "hello",
	}

	// Act
	buf, err := encoder.EncodeEntry(entry, nil)

	// Assert
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	assert.Equal(t, "hello", got["message"])
}

func TestDefaultEncoder_UnknownFormatUsesJSON(t *testing.T) {
	// Arrange
	encoder := defaultEncoder(nil, nil)(Format(100))

	entry := zapcore.Entry{
		Level:   zap.InfoLevel,
		Time:    time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		Message: "hello",
	}

	// Act
	buf, err := encoder.EncodeEntry(entry, nil)

	// Assert
	require.NoError(t, err)

	var got map[string]any
	require.NoError(t, json.Unmarshal(buf.Bytes(), &got))

	assert.Equal(t, "hello", got["message"])
}

func testEncoder() zapcore.Encoder {
	cfg := zap.NewProductionEncoderConfig()
	cfg.MessageKey = "message"

	return zapcore.NewJSONEncoder(cfg)
}

type testWriteSyncer struct {
	writes int
	data   bytes.Buffer
}

func (w *testWriteSyncer) Write(p []byte) (int, error) {
	w.writes++

	return w.data.Write(p)
}

func (w *testWriteSyncer) Sync() error {
	return nil
}

func writeCoreEntry(
	t *testing.T,
	core zapcore.Core,
	level zapcore.Level,
	message string,
) {
	t.Helper()

	entry := zapcore.Entry{
		Level:   level,
		Time:    time.Now(),
		Message: message,
	}

	checked := core.Check(entry, nil)
	if checked != nil {
		checked.Write()
	}
}
