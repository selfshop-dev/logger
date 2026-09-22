package logger

import (
	"context"
	"io"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap/zapcore"
)

func TestOptionFunc_Apply(t *testing.T) {
	t.Run("nil function", func(t *testing.T) {
		// Arrange
		var fn optionFunc
		b := &builder{}

		// Act
		fn.apply(b)

		// Assert
		assert.Equal(t, &builder{}, b)
	})

	t.Run("calls function", func(t *testing.T) {
		// Arrange
		called := false

		fn := optionFunc(func(b *builder) {
			called = true
			b.name = "test"
		})

		b := &builder{}

		// Act
		fn.apply(b)

		// Assert
		assert.True(t, called)
		assert.Equal(t, "test", b.name)
	})
}

func TestWithWriter(t *testing.T) {
	t.Run("sets writer", func(t *testing.T) {
		// Arrange
		writer := &testWriter{}
		b := &builder{}

		// Act
		WithWriter(writer).apply(b)

		// Assert
		assert.Same(t, writer, b.writer)
	})

	t.Run("nil writer does not change value", func(t *testing.T) {
		// Arrange
		writer := &testWriter{}
		b := &builder{
			writer: writer,
		}

		// Act
		WithWriter(nil).apply(b)

		// Assert
		assert.Same(t, writer, b.writer)
	})
}

func TestWithNamed(t *testing.T) {
	t.Run("sets normalized name", func(t *testing.T) {
		// Arrange
		b := &builder{}

		// Act
		WithNamed(" . service . . api . ").apply(b)

		// Assert
		assert.Equal(t, "service.api", b.name)
	})

	t.Run("empty name does not change value", func(t *testing.T) {
		// Arrange
		b := &builder{
			name: "existing",
		}

		// Act
		WithNamed("   ...   ").apply(b)

		// Assert
		assert.Equal(t, "existing", b.name)
	})
}

func TestWithExtractors(t *testing.T) {
	t.Run("adds non-nil extractors", func(t *testing.T) {
		// Arrange
		var order []int

		ex1 := ExtractorFunc(func(context.Context) []slog.Attr {
			order = append(order, 1)
			return nil
		})
		ex2 := ExtractorFunc(func(context.Context) []slog.Attr {
			order = append(order, 2)
			return nil
		})

		b := &builder{}

		// Act
		WithExtractors(nil, ex1, nil, ex2).apply(b)

		// Assert
		assert.Len(t, b.extractors, 2)

		ctx := context.Background()
		_ = b.extractors[0].Extract(ctx)
		_ = b.extractors[1].Extract(ctx)

		assert.Equal(t, []int{1, 2}, order)
	})

	t.Run("preserves existing extractors", func(t *testing.T) {
		// Arrange
		var order []int

		existing := ExtractorFunc(func(context.Context) []slog.Attr {
			order = append(order, 1)
			return nil
		})
		added := ExtractorFunc(func(context.Context) []slog.Attr {
			order = append(order, 2)
			return nil
		})

		b := &builder{
			extractors: []Extractor{existing},
		}

		// Act
		WithExtractors(added).apply(b)

		// Assert
		assert.Len(t, b.extractors, 2)

		ctx := context.Background()
		_ = b.extractors[0].Extract(ctx)
		_ = b.extractors[1].Extract(ctx)

		assert.Equal(t, []int{1, 2}, order)
	})
}

func TestWithHooks(t *testing.T) {
	t.Run("adds non-nil hooks", func(t *testing.T) {
		// Arrange
		var order []int

		h1 := HookFunc(func(context.Context, *slog.Record) error {
			order = append(order, 1)
			return nil
		})
		h2 := HookFunc(func(context.Context, *slog.Record) error {
			order = append(order, 2)
			return nil
		})

		b := &builder{}

		// Act
		WithHooks(nil, h1, nil, h2).apply(b)

		// Assert
		assert.Len(t, b.hooks, 2)

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		executed, err := hooks(b.hooks).RunBefore(
			context.Background(),
			&record,
		)

		require.NoError(t, err)
		assert.Len(t, executed, 2)
		assert.Equal(t, []int{1, 2}, order)
	})

	t.Run("preserves existing hooks", func(t *testing.T) {
		// Arrange
		var order []int

		existing := HookFunc(func(context.Context, *slog.Record) error {
			order = append(order, 1)
			return nil
		})
		added := HookFunc(func(context.Context, *slog.Record) error {
			order = append(order, 2)
			return nil
		})

		b := &builder{
			hooks: []Hook{existing},
		}

		// Act
		WithHooks(added).apply(b)

		// Assert
		assert.Len(t, b.hooks, 2)

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		executed, err := hooks(b.hooks).RunBefore(
			context.Background(),
			&record,
		)

		require.NoError(t, err)
		assert.Len(t, executed, 2)
		assert.Equal(t, []int{1, 2}, order)
	})
}

func TestWithAttrs(t *testing.T) {
	t.Run("adds sanitized attrs", func(t *testing.T) {
		// Arrange
		b := &builder{}

		attrs := []any{
			slog.String("attr", "value"),
			"key",
			"value",
			"",
			"ignored",
			123,
			"not-a-string-key",
			"ignored-value",
			"valid",
			42,
		}

		// Act
		WithAttrs(attrs...).apply(b)

		// Assert
		assert.Equal(t, []any{
			slog.String("attr", "value"),
			"key",
			"value",
			"ignored-value",
			"valid",
		}, b.attrs)
	})
}

func TestSanitizeAttrs(t *testing.T) {
	testCases := [...]struct {
		name string
		in   []any
		want []any
	}{
		{
			name: "empty",
			in:   nil,
			want: []any{},
		},
		{
			name: "valid attr",
			in: []any{
				slog.String("key", "value"),
			},
			want: []any{
				slog.String("key", "value"),
			},
		},
		{
			name: "empty attr key",
			in: []any{
				slog.String("", "value"),
				slog.String("valid", "value"),
			},
			want: []any{
				slog.String("valid", "value"),
			},
		},
		{
			name: "valid key value pair",
			in: []any{
				"key",
				"value",
			},
			want: []any{
				"key",
				"value",
			},
		},
		{
			name: "empty key value pair",
			in: []any{
				"",
				"value",
				"valid",
				"value",
			},
			want: []any{
				"valid",
				"value",
			},
		},
		{
			name: "non-string key",
			in: []any{
				123,
				"value",
				"valid",
				"value",
			},
			want: []any{
				"valid",
				"value",
			},
		},
		{
			name: "dangling key",
			in: []any{
				"key",
			},
			want: []any{},
		},
		{
			name: "mixed attrs",
			in: []any{
				slog.String("first", "1"),
				"second",
				"2",
				slog.String("", "ignored"),
				"",
				"ignored",
				"third",
				3,
			},
			want: []any{
				slog.String("first", "1"),
				"second",
				"2",
				"third",
				3,
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := sanitizeAttrs(tc.in)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestWithDevelopment(t *testing.T) {
	// Arrange
	b := &builder{}

	// Act
	WithDevelopment().apply(b)

	// Assert
	assert.True(t, b.development)
}

func TestWithResolveFormat(t *testing.T) {
	t.Run("sets function", func(t *testing.T) {
		// Arrange
		called := false

		fn := func(development bool) Format {
			called = true
			assert.True(t, development)
			return FormatJSON
		}

		b := &builder{}

		// Act
		WithResolveFormat(fn).apply(b)

		// Assert
		require.NotNil(t, b.resolveFormat)
		assert.Equal(t, FormatJSON, b.resolveFormat(true))
		assert.True(t, called)
	})

	t.Run("nil function does not change value", func(t *testing.T) {
		// Arrange
		fn := func(bool) Format {
			return FormatConsole
		}

		b := &builder{
			resolveFormat: fn,
		}

		// Act
		WithResolveFormat(nil).apply(b)

		// Assert
		require.NotNil(t, b.resolveFormat)
		assert.Equal(t, FormatConsole, b.resolveFormat(false))
	})
}

func TestWithEncoder(t *testing.T) {
	t.Run("sets function", func(t *testing.T) {
		// Arrange
		called := false

		fn := func(format Format) zapcore.Encoder {
			called = true
			assert.Equal(t, FormatJSON, format)
			return nil
		}

		b := &builder{}

		// Act
		WithEncoder(fn).apply(b)

		// Assert
		require.NotNil(t, b.encoder)
		assert.Nil(t, b.encoder(FormatJSON))
		assert.True(t, called)
	})

	t.Run("nil function does not change value", func(t *testing.T) {
		// Arrange
		fn := func(Format) zapcore.Encoder {
			return nil
		}

		b := &builder{
			encoder: fn,
		}

		// Act
		WithEncoder(nil).apply(b)

		// Assert
		require.NotNil(t, b.encoder)
		assert.Nil(t, b.encoder(FormatJSON))
	})
}

func TestWithConsoleEncoder(t *testing.T) {
	t.Run("sets function", func(t *testing.T) {
		// Arrange
		called := false

		fn := func(cfg *zapcore.EncoderConfig) {
			called = true
			cfg.MessageKey = "msg"
		}

		b := &builder{}

		// Act
		WithConsoleEncoder(fn).apply(b)

		// Assert
		require.NotNil(t, b.consoleEncoder)

		cfg := &zapcore.EncoderConfig{}
		b.consoleEncoder(cfg)

		assert.True(t, called)
		assert.Equal(t, "msg", cfg.MessageKey)
	})

	t.Run("nil function does not change value", func(t *testing.T) {
		// Arrange
		fn := func(cfg *zapcore.EncoderConfig) {
			cfg.MessageKey = "existing"
		}

		b := &builder{
			consoleEncoder: fn,
		}

		// Act
		WithConsoleEncoder(nil).apply(b)

		// Assert
		require.NotNil(t, b.consoleEncoder)

		cfg := &zapcore.EncoderConfig{}
		b.consoleEncoder(cfg)

		assert.Equal(t, "existing", cfg.MessageKey)
	})
}

func TestWithJSONEncoder(t *testing.T) {
	t.Run("sets function", func(t *testing.T) {
		// Arrange
		called := false

		fn := func(cfg *zapcore.EncoderConfig) {
			called = true
			cfg.MessageKey = "message"
		}

		b := &builder{}

		// Act
		WithJSONEncoder(fn).apply(b)

		// Assert
		require.NotNil(t, b.jsonEncoder)

		cfg := &zapcore.EncoderConfig{}
		b.jsonEncoder(cfg)

		assert.True(t, called)
		assert.Equal(t, "message", cfg.MessageKey)
	})

	t.Run("nil function does not change value", func(t *testing.T) {
		// Arrange
		fn := func(cfg *zapcore.EncoderConfig) {
			cfg.MessageKey = "existing"
		}

		b := &builder{
			jsonEncoder: fn,
		}

		// Act
		WithJSONEncoder(nil).apply(b)

		// Assert
		require.NotNil(t, b.jsonEncoder)

		cfg := &zapcore.EncoderConfig{}
		b.jsonEncoder(cfg)

		assert.Equal(t, "existing", cfg.MessageKey)
	})
}

type testWriter struct{}

func (*testWriter) Write(p []byte) (int, error) {
	return io.Discard.Write(p)
}
