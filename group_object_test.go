package logger

import (
	"errors"
	"log/slog"
	"testing"
	"time"
	"unsafe"

	"go.uber.org/zap/zapcore"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestConvertAttrToField(t *testing.T) {
	testDuration := 1500 * time.Millisecond
	testErr := errors.New("test error")
	testTime := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

	testCases := [...]struct {
		name string
		attr slog.Attr
		want map[string]any
	}{
		{
			name: "string",
			attr: slog.String("key", "value"),
			want: map[string]any{
				"key": "value",
			},
		},
		{
			name: "int64",
			attr: slog.Int64("key", 42),
			want: map[string]any{
				"key": int64(42),
			},
		},
		{
			name: "int",
			attr: slog.Int("key", 42),
			want: map[string]any{
				"key": int64(42),
			},
		},
		{
			name: "uint64",
			attr: slog.Uint64("key", 42),
			want: map[string]any{
				"key": uint64(42),
			},
		},
		{
			name: "uint",
			attr: slog.Uint64("key", uint64(42)),
			want: map[string]any{
				"key": uint64(42),
			},
		},
		{
			name: "float64",
			attr: slog.Float64("key", 42.5),
			want: map[string]any{
				"key": 42.5,
			},
		},
		{
			name: "bool",
			attr: slog.Bool("key", true),
			want: map[string]any{
				"key": true,
			},
		},
		{
			name: "duration",
			attr: slog.Duration("key", testDuration),
			want: map[string]any{
				"key": testDuration,
			},
		},
		{
			name: "time",
			attr: slog.Time("key", testTime),
			want: map[string]any{
				"key": testTime,
			},
		},
		{
			name: "any",
			attr: slog.Any("key", 123),
			want: map[string]any{
				"key": int64(123),
			},
		},
		{
			name: "error",
			attr: slog.Any("error", testErr),
			want: map[string]any{
				"error": testErr.Error(),
			},
		},
		{
			name: "stringer",
			attr: slog.Any("key", testStringer("hello")),
			want: map[string]any{
				"key": "hello",
			},
		},
		{
			name: "object marshaler",
			attr: slog.Any("key", testObjectMarshaler{
				Value: "hello",
			}),
			want: map[string]any{
				"key": map[string]any{
					"value": "hello",
				},
			},
		},
		{
			name: "array marshaler",
			attr: slog.Any("key", testArrayMarshaler{
				Values: []string{"one", "two"},
			}),
			want: map[string]any{
				"key": []any{
					"one",
					"two",
				},
			},
		},
		{
			name: "log valuer",
			attr: slog.Any("key", testLogValuer{
				Value: "resolved",
			}),
			want: map[string]any{
				"key": "resolved",
			},
		},
		{
			name: "group",
			attr: slog.Group(
				"request",
				slog.String("method", "GET"),
				slog.String("path", "/users"),
			),
			want: map[string]any{
				"request": map[string]any{
					"method": "GET",
					"path":   "/users",
				},
			},
		},
		{
			name: "inline group",
			attr: slog.Group(
				"",
				slog.String("method", "GET"),
				slog.Int("status", 200),
			),
			want: map[string]any{
				"method": "GET",
				"status": int64(200),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			field := convertAttrToField(tc.attr)

			// Assert
			got := encodeField(t, field)

			assert.Equal(t, tc.want, got)
		})
	}
}

func TestAttrToField_KindAny_Default(t *testing.T) {
	type testAny struct {
		Value string
	}

	// Arrange
	attr := slog.Any("key", testAny{Value: "value"})

	// Act
	field := convertAttrToField(attr)

	// Assert
	assert.Equal(t, "key", field.Key)
	assert.Equal(t, zapcore.ReflectType, field.Type)
}

func TestAttrToField_EmptyGroup(t *testing.T) {
	// Arrange
	attr := slog.Group("request")

	// Act
	field := convertAttrToField(attr)

	// Assert
	assert.Equal(t, zapcore.SkipType, field.Type)
}

func TestAttrToField_LogValuer(t *testing.T) {
	// Arrange
	attr := slog.Any("key", testLogValuer{
		Value: "resolved",
	})

	// Act
	field := convertAttrToField(attr)

	// Assert
	assert.Equal(t, zapcore.StringType, field.Type)
	assert.Equal(t, "key", field.Key)

	got := encodeField(t, field)
	assert.Equal(t, map[string]any{
		"key": "resolved",
	}, got)
}

func TestAttrToField_LogValuer_ResolvesNestedValue(t *testing.T) {
	// Arrange
	attr := slog.Any("outer", testNestedLogValuer{})

	// Act
	field := convertAttrToField(attr)

	// Assert
	got := encodeField(t, field)

	assert.Equal(t, map[string]any{
		"outer": "resolved",
	}, got)
}

func TestAttrToField_UnknownKind_FallsToDefault(t *testing.T) {
	// Arrange
	type value struct {
		_   [0]func()
		num uint64
		any any
	}
	attr := slog.Attr{
		Key:   "unknown",
		Value: *(*slog.Value)(unsafe.Pointer(new(value{any: slog.Kind(100)}))), // #nosec G103 -- test intentionally constructs an invalid slog.Value to exercise the unknown-kind fallback
	}

	// Act + Assert
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic from unknown kind in default branch")
		}
	}()

	_ = convertAttrToField(attr)
}

func TestGroupObject_MarshalLogObject(t *testing.T) {
	// Arrange
	group := groupObject{
		slog.String("method", "GET"),
		slog.Int("status", 200),
		slog.Bool("cached", true),
	}

	enc := zapcore.NewMapObjectEncoder()

	// Act
	err := group.MarshalLogObject(enc)

	// Assert
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"method": "GET",
		"status": int64(200),
		"cached": true,
	}, enc.Fields)
}

func TestGroupObject_MarshalLogObject_NestedGroup(t *testing.T) {
	// Arrange
	group := groupObject{
		slog.String("method", "GET"),
		slog.Group(
			"request",
			slog.String("path", "/users"),
			slog.Int("attempt", 2),
		),
	}

	enc := zapcore.NewMapObjectEncoder()

	// Act
	err := group.MarshalLogObject(enc)

	// Assert
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"method": "GET",
		"request": map[string]any{
			"path":    "/users",
			"attempt": int64(2),
		},
	}, enc.Fields)
}

type testStringer string

func (s testStringer) String() string {
	return string(s)
}

type testObjectMarshaler struct {
	Value string
}

func (m testObjectMarshaler) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	enc.AddString("value", m.Value)
	return nil
}

type testArrayMarshaler struct {
	Values []string
}

func (m testArrayMarshaler) MarshalLogArray(enc zapcore.ArrayEncoder) error {
	for _, value := range m.Values {
		enc.AppendString(value)
	}
	return nil
}

type testLogValuer struct {
	Value string
}

func (v testLogValuer) LogValue() slog.Value {
	return slog.StringValue(v.Value)
}

type testNestedLogValuer struct{}

func (testNestedLogValuer) LogValue() slog.Value {
	return slog.AnyValue(testLogValuer{
		Value: "resolved",
	})
}

func encodeField(t *testing.T, field zapcore.Field) map[string]any {
	t.Helper()

	enc := zapcore.NewMapObjectEncoder()
	field.AddTo(enc)

	return enc.Fields
}
