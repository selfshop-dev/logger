package logger

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/selfshop-dev/ctxval"
)

func TestExtractFrom(t *testing.T) {
	type requestID string
	type traceID string

	testCases := [...]struct {
		name      string
		extractor ExtractorFunc
		ctx       context.Context
		want      []slog.Attr
	}{
		{
			name:      "value present",
			extractor: ExtractFrom[requestID]("request_id"),
			ctx:       ctxval.With(context.Background(), requestID("req-123")),
			want: []slog.Attr{
				slog.String("request_id", "req-123"),
			},
		},
		{
			name:      "value absent",
			extractor: ExtractFrom[requestID]("request_id"),
			ctx:       context.Background(),
			want:      nil,
		},
		{
			name:      "value empty",
			extractor: ExtractFrom[requestID]("request_id"),
			ctx:       ctxval.With(context.Background(), requestID("")),
			want:      nil,
		},
		{
			name:      "different type present",
			extractor: ExtractFrom[requestID]("request_id"),
			ctx:       ctxval.With(context.Background(), traceID("trace-123")),
			want:      nil,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := tc.extractor(tc.ctx)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractorFunc_Extract(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		// Arrange
		var fn ExtractorFunc

		// Act
		got := fn.Extract(context.Background())

		// Assert
		assert.Nil(t, got)
	})

	t.Run("calls function", func(t *testing.T) {
		// Arrange
		ctx := context.Background()
		called := false

		fn := ExtractorFunc(func(gotCtx context.Context) []slog.Attr {
			called = true
			assert.Equal(t, ctx, gotCtx)

			return []slog.Attr{
				slog.String("request_id", "req-123"),
			}
		})

		// Act
		got := fn.Extract(ctx)

		// Assert
		require.True(t, called)
		assert.Equal(t, []slog.Attr{
			slog.String("request_id", "req-123"),
		}, got)
	})
}

func TestExtractors_ExtractAttrs(t *testing.T) {
	testCases := [...]struct {
		name       string
		extractors extractors
		want       []slog.Attr
	}{
		{
			name:       "empty",
			extractors: nil,
			want:       nil,
		},
		{
			name: "extractor returns nil",
			extractors: extractors{
				ExtractorFunc(func(context.Context) []slog.Attr {
					return nil
				}),
			},
			want: nil,
		},
		{
			name: "one extractor",
			extractors: extractors{
				ExtractorFunc(func(context.Context) []slog.Attr {
					return []slog.Attr{
						slog.String("request_id", "req-123"),
					}
				}),
			},
			want: []slog.Attr{
				slog.String("request_id", "req-123"),
			},
		},
		{
			name: "multiple extractors",
			extractors: extractors{
				ExtractorFunc(func(context.Context) []slog.Attr {
					return []slog.Attr{
						slog.String("request_id", "req-123"),
					}
				}),
				ExtractorFunc(func(context.Context) []slog.Attr {
					return nil
				}),
				ExtractorFunc(func(context.Context) []slog.Attr {
					return []slog.Attr{
						slog.String("trace_id", "trace-456"),
						slog.Int("attempt", 2),
					}
				}),
			},
			want: []slog.Attr{
				slog.String("request_id", "req-123"),
				slog.String("trace_id", "trace-456"),
				slog.Int("attempt", 2),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			record := slog.NewRecord(
				time.Unix(0, 0),
				slog.LevelInfo,
				"test", 0,
			)

			// Act
			tc.extractors.ExtractAttrs(context.Background(), &record)

			// Assert
			var got []slog.Attr
			record.Attrs(func(attr slog.Attr) bool {
				got = append(got, attr)
				return true
			})

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestExtractors_ExtractAttrs_SkipsNil(t *testing.T) {
	// Arrange
	var order []string

	exs := extractors{
		nil,
		ExtractorFunc(func(context.Context) []slog.Attr {
			order = append(order, "first")
			return []slog.Attr{slog.String("a", "1")}
		}),
		nil,
		ExtractorFunc(func(context.Context) []slog.Attr {
			order = append(order, "second")
			return []slog.Attr{slog.String("b", "2")}
		}),
		nil,
	}

	record := slog.NewRecord(time.Unix(0, 0), slog.LevelInfo, "test", 0)

	// Act
	exs.ExtractAttrs(context.Background(), &record)

	// Assert
	assert.Equal(t, []string{"first", "second"}, order)

	var got []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		got = append(got, attr)
		return true
	})

	assert.Equal(t, []slog.Attr{
		slog.String("a", "1"),
		slog.String("b", "2"),
	}, got)
}

func TestExtractors_ExtractAttrs_PassesContext(t *testing.T) {
	// Arrange
	ctx := context.Background()
	called := 0

	exs := extractors{
		ExtractorFunc(func(gotCtx context.Context) []slog.Attr {
			called++
			assert.Equal(t, ctx, gotCtx)

			return []slog.Attr{
				slog.String("key", "value"),
			}
		}),
		ExtractorFunc(func(gotCtx context.Context) []slog.Attr {
			called++
			assert.Equal(t, ctx, gotCtx)

			return []slog.Attr{
				slog.String("key2", "value2"),
			}
		}),
	}

	record := slog.NewRecord(
		time.Unix(0, 0),
		slog.LevelInfo,
		"test",
		0,
	)

	// Act
	exs.ExtractAttrs(ctx, &record)

	// Assert
	assert.Equal(t, 2, called)

	var got []slog.Attr
	record.Attrs(func(attr slog.Attr) bool {
		got = append(got, attr)
		return true
	})

	assert.Equal(t, []slog.Attr{
		slog.String("key", "value"),
		slog.String("key2", "value2"),
	}, got)
}
