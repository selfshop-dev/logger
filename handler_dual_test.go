package logger

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInternalKey_Bool(t *testing.T) {
	// Arrange
	key := internalKey{"test"}

	// Act
	got := key.Bool(true)

	// Assert
	assert.Equal(t, "test", got.Key)
	assert.Equal(t, slog.BoolValue(true), got.Value)
}

func TestInternalKey_Attr(t *testing.T) {
	// Arrange
	key := internalKey{"test"}
	value := struct {
		ID int
	}{
		ID: 123,
	}

	// Act
	got := key.Attr(value)

	// Assert
	assert.Equal(t, "test", got.Key)
	assert.Equal(t, slog.AnyValue(value), got.Value)
}

func TestInternalKey_String(t *testing.T) {
	// Arrange
	key := internalKey{"test"}

	// Act
	got := key.String("value")

	// Assert
	assert.Equal(t, "test", got.Key)
	assert.Equal(t, slog.StringValue("value"), got.Value)
}

func TestLogNamingAttr(t *testing.T) {
	// Act
	got := LogNamingAttr("service.api")

	// Assert
	assert.Equal(t, logNaming.key, got.Key)
	assert.Equal(t, slog.StringValue("service.api"), got.Value)
}

func TestAlwaysLogAttr(t *testing.T) {
	// Act
	got := AlwaysLogAttr()

	// Assert
	assert.Equal(t, alwaysLog.key, got.Key)
	assert.Equal(t, slog.BoolValue(true), got.Value)
}

func TestDualHandler_AddCallerSkip(t *testing.T) {
	t.Run("zero skip returns same handler", func(t *testing.T) {
		// Arrange
		normal := newTestTransformHandler()
		h := &dualHandler{
			normal:  normal,
			sampled: false,
		}

		// Act
		got := h.AddCallerSkip(0)

		// Assert
		assert.Same(t, h, got)
	})

	t.Run("unsupported handler returns same handler", func(t *testing.T) {
		// Arrange
		normal := &testPlainHandler{}
		h := &dualHandler{
			normal: normal,
		}

		// Act
		got := h.AddCallerSkip(2)

		// Assert
		assert.Same(t, h, got)
	})

	t.Run("unsampled transforms normal handler", func(t *testing.T) {
		// Arrange
		normal := newTestTransformHandler()
		h := &dualHandler{
			normal:  normal,
			sampled: false,
		}

		// Act
		got := h.AddCallerSkip(3)

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)
		assert.NotSame(t, h, result)

		normalResult, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		assert.Equal(t, 3, normalResult.callerSkip)
	})

	t.Run("sampled transforms both handlers", func(t *testing.T) {
		// Arrange
		normal := newTestTransformHandler()
		always := newTestTransformHandler()

		h := &dualHandler{
			normal:  normal,
			always:  always,
			sampled: true,
		}

		// Act
		got := h.AddCallerSkip(3)

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)
		assert.NotSame(t, h, result)
		assert.True(t, result.sampled)

		normalResult, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		alwaysResult, ok := result.always.(*testTransformHandler)
		require.True(t, ok)

		assert.Equal(t, 3, normalResult.callerSkip)
		assert.Equal(t, 3, alwaysResult.callerSkip)
	})
}

func TestDualHandler_WithName(t *testing.T) {
	t.Run("empty segment returns same handler", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal: &testTransformHandler{},
		}

		// Act
		got := h.WithName("   ")

		// Assert
		assert.Same(t, h, got)
	})

	t.Run("trims segment", func(t *testing.T) {
		// Arrange
		normal := newTestTransformHandler()
		h := &dualHandler{
			normal: normal,
		}

		// Act
		got := h.WithName(" service ")

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)

		normalResult, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		assert.Equal(t, "service", normalResult.name)
	})

	t.Run("unsupported handler returns same handler", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal: &testPlainHandler{},
		}

		// Act
		got := h.WithName("service")

		// Assert
		assert.Same(t, h, got)
	})

	t.Run("unsampled transforms normal handler", func(t *testing.T) {
		// Arrange
		normal := newTestTransformHandler()
		h := &dualHandler{
			normal:  normal,
			sampled: false,
		}

		// Act
		got := h.WithName("service")

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)

		normalResult, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		assert.Equal(t, "service", normalResult.name)
		assert.False(t, result.sampled)
	})

	t.Run("sampled transforms both handlers", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal:  newTestTransformHandler(),
			always:  newTestTransformHandler(),
			sampled: true,
		}

		// Act
		got := h.WithName("service")

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)

		normalResult, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		alwaysResult, ok := result.always.(*testTransformHandler)
		require.True(t, ok)

		assert.Equal(t, "service", normalResult.name)
		assert.Equal(t, "service", alwaysResult.name)
	})
}

func TestDualHandler_Enabled(t *testing.T) {
	t.Run("unsampled uses normal handler", func(t *testing.T) {
		// Arrange
		normal := &testEnabledHandler{
			enabled: true,
		}

		h := &dualHandler{
			normal:  normal,
			sampled: false,
		}

		// Act
		got := h.Enabled(context.Background(), slog.LevelInfo)

		// Assert
		assert.True(t, got)
		assert.Equal(t, 1, normal.enabledCalls)
	})

	t.Run("sampled returns true when normal is enabled", func(t *testing.T) {
		// Arrange
		normal := &testEnabledHandler{
			enabled: true,
		}
		always := &testEnabledHandler{
			enabled: false,
		}

		h := &dualHandler{
			normal:  normal,
			always:  always,
			sampled: true,
		}

		// Act
		got := h.Enabled(context.Background(), slog.LevelInfo)

		// Assert
		assert.True(t, got)

		// normal || always uses short-circuit evaluation.
		assert.Equal(t, 1, normal.enabledCalls)
		assert.Equal(t, 0, always.enabledCalls)
	})

	t.Run("sampled returns true when always is enabled", func(t *testing.T) {
		// Arrange
		normal := &testEnabledHandler{
			enabled: false,
		}
		always := &testEnabledHandler{
			enabled: true,
		}

		h := &dualHandler{
			normal:  normal,
			always:  always,
			sampled: true,
		}

		// Act
		got := h.Enabled(context.Background(), slog.LevelInfo)

		// Assert
		assert.True(t, got)
		assert.Equal(t, 1, normal.enabledCalls)
		assert.Equal(t, 1, always.enabledCalls)
	})

	t.Run("sampled returns false when both disabled", func(t *testing.T) {
		// Arrange
		normal := &testEnabledHandler{
			enabled: false,
		}
		always := &testEnabledHandler{
			enabled: false,
		}

		h := &dualHandler{
			normal:  normal,
			always:  always,
			sampled: true,
		}

		// Act
		got := h.Enabled(context.Background(), slog.LevelInfo)

		// Assert
		assert.False(t, got)
		assert.Equal(t, 1, normal.enabledCalls)
		assert.Equal(t, 1, always.enabledCalls)
	})
}

func TestDualHandler_WithGroup(t *testing.T) {
	t.Run("empty group returns same handler", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal: &testTransformHandler{},
		}

		// Act
		got := h.WithGroup("   ")

		// Assert
		assert.Same(t, h, got)
	})

	t.Run("unsampled applies group to normal", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal:  newTestTransformHandler(),
			sampled: false,
		}

		// Act
		got := h.WithGroup("request")

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)

		normal, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		assert.Equal(t, []string{"request"}, normal.groups)
	})

	t.Run("sampled applies group to both handlers", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal:  newTestTransformHandler(),
			always:  newTestTransformHandler(),
			sampled: true,
		}

		// Act
		got := h.WithGroup(" request ")

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)

		normal, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		always, ok := result.always.(*testTransformHandler)
		require.True(t, ok)

		// WithGroup trims the incoming segment first.
		assert.Equal(t, []string{"request"}, normal.groups)
		assert.Equal(t, []string{"request"}, always.groups)
	})
}

func TestDualHandler_Handle(t *testing.T) {
	t.Run("before hook error stops handling", func(t *testing.T) {
		// Arrange
		wantErr := errors.New("before failed")

		normal := &testHandleHandler{}

		afterCalled := false

		before := HookFunc(func(context.Context, *slog.Record) error {
			return wantErr
		})

		after := afterHook{
			fn: func(context.Context, slog.Record, error) {
				afterCalled = true
			},
		}

		h := &dualHandler{
			normal: normal,
			hooks: hooks{
				before,
				after,
			},
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		// Act
		err := h.Handle(context.Background(), record)

		// Assert
		require.ErrorIs(t, err, wantErr)

		// The failing Before hook is not added to "executed",
		// therefore its After hook is not run.
		assert.False(t, afterCalled)

		assert.Equal(t, 0, normal.handleCalls)
	})
}

func TestDualHandler_Handle_SkipsRecord(t *testing.T) {
	// Arrange
	normal := &testWouldWriteHandleHandler{
		wouldWrite: false,
	}
	always := &testWouldWriteHandleHandler{
		wouldWrite: true,
	}

	extractorCalled := false
	beforeCalled := false
	afterCalled := false

	extractor := ExtractorFunc(func(context.Context) []slog.Attr {
		extractorCalled = true
		return nil
	})

	before := HookFunc(func(context.Context, *slog.Record) error {
		beforeCalled = true
		return nil
	})

	after := afterHook{
		fn: func(context.Context, slog.Record, error) {
			afterCalled = true
		},
	}

	h := &dualHandler{
		normal:     normal,
		always:     always,
		extractors: extractors{extractor},
		hooks:      hooks{before, after},
		sampled:    true,
	}

	record := slog.NewRecord(
		time.Time{},
		slog.LevelInfo,
		"test",
		0,
	)

	// Act
	err := h.Handle(context.Background(), record)

	// Assert
	require.NoError(t, err)

	assert.Equal(t, 1, normal.wouldWriteCalls)
	assert.Equal(t, 0, always.wouldWriteCalls)

	assert.Equal(t, 0, normal.handleCalls)
	assert.Equal(t, 0, always.handleCalls)

	assert.False(t, extractorCalled)
	assert.False(t, beforeCalled)
	assert.False(t, afterCalled)
}

func TestDualHandler_Handle_AlwaysLogUsesAlwaysHandler(t *testing.T) {
	// Arrange
	normal := &testHandleHandler{}
	always := &testHandleHandler{}

	h := &dualHandler{
		normal:  normal,
		always:  always,
		sampled: true,
	}

	record := slog.NewRecord(
		time.Time{},
		slog.LevelInfo,
		"test",
		0,
	)
	record.AddAttrs(AlwaysLogAttr())

	// Act
	err := h.Handle(context.Background(), record)

	// Assert
	require.NoError(t, err)

	assert.Equal(t, 0, normal.handleCalls)
	assert.Equal(t, 1, always.handleCalls)
}

func TestDualHandler_Apply(t *testing.T) {
	t.Run("normal transform fails", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal: &testPlainHandler{},
		}

		// Act
		got := h.apply(func(slog.Handler) (slog.Handler, bool) {
			return nil, false
		})

		// Assert
		assert.Same(t, h, got)
	})

	t.Run("unsampled applies only normal", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal:  &testTransformHandler{},
			sampled: false,
		}

		// Act
		got := h.apply(func(handler slog.Handler) (slog.Handler, bool) {
			return &testTransformHandler{
				name: "transformed",
			}, true
		})

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)

		transformed, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		assert.Equal(t, "transformed", transformed.name)
		assert.False(t, result.sampled)
	})

	t.Run("sampled returns same handler when always transform fails", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal:  &testTransformHandler{},
			always:  &testPlainHandler{},
			sampled: true,
		}

		// Act
		got := h.apply(func(handler slog.Handler) (slog.Handler, bool) {
			if _, ok := handler.(*testPlainHandler); ok {
				return nil, false
			}

			return &testTransformHandler{
				name: "transformed",
			}, true
		})

		// Assert
		assert.Same(t, h, got)
	})

	t.Run("sampled transforms both handlers", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal:  &testTransformHandler{},
			always:  &testTransformHandler{},
			sampled: true,
		}

		// Act
		got := h.apply(func(handler slog.Handler) (slog.Handler, bool) {
			assert.IsType(t, &testTransformHandler{}, handler)

			return &testTransformHandler{
				name: "transformed",
			}, true
		})

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)

		assert.True(t, result.sampled)
		assert.Equal(
			t,
			"transformed",
			result.normal.(*testTransformHandler).name,
		)
		assert.Equal(
			t,
			"transformed",
			result.always.(*testTransformHandler).name,
		)
	})
}

func TestDualHandler_WithAttrs(t *testing.T) {
	t.Run("empty attrs returns same handler", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal: &testTransformHandler{},
		}

		// Act
		got := h.WithAttrs(nil)

		// Assert
		assert.Same(t, h, got)
	})

	t.Run("unsampled applies attrs to normal", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal:  newTestTransformHandler(),
			sampled: false,
		}

		attrs := []slog.Attr{
			slog.String("service", "users"),
		}

		// Act
		got := h.WithAttrs(attrs)

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)
		assert.NotSame(t, h, result)

		normal, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		assert.Equal(t, attrs, normal.attrs)
		assert.False(t, result.sampled)
	})

	t.Run("sampled applies attrs to both handlers", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal:  newTestTransformHandler(),
			always:  newTestTransformHandler(),
			sampled: true,
		}

		attrs := []slog.Attr{
			slog.String("service", "users"),
		}

		// Act
		got := h.WithAttrs(attrs)

		// Assert
		result, ok := got.(*dualHandler)
		require.True(t, ok)

		normal, ok := result.normal.(*testTransformHandler)
		require.True(t, ok)

		always, ok := result.always.(*testTransformHandler)
		require.True(t, ok)

		assert.Equal(t, attrs, normal.attrs)
		assert.Equal(t, attrs, always.attrs)
	})
}

func TestHasAlwaysLog(t *testing.T) {
	testCases := [...]struct {
		name  string
		attrs []slog.Attr
		want  bool
	}{
		{
			name:  "not present",
			attrs: nil,
			want:  false,
		},
		{
			name: "true",
			attrs: []slog.Attr{
				AlwaysLogAttr(),
			},
			want: true,
		},
		{
			name: "false",
			attrs: []slog.Attr{
				alwaysLog.Bool(false),
			},
			want: false,
		},
		{
			name: "wrong key",
			attrs: []slog.Attr{
				slog.Bool("always_log", true),
			},
			want: false,
		},
		{
			name: "wrong type",
			attrs: []slog.Attr{
				slog.String(alwaysLog.key, "true"),
			},
			want: false,
		},
		{
			name: "true after other attrs",
			attrs: []slog.Attr{
				slog.String("key", "value"),
				AlwaysLogAttr(),
			},
			want: true,
		},
		{
			name: "false followed by true",
			attrs: []slog.Attr{
				alwaysLog.Bool(false),
				AlwaysLogAttr(),
			},
			want: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
			record.AddAttrs(tc.attrs...)

			// Act
			got := hasAlwaysLog(record)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestDualHandler_ShouldSkip(t *testing.T) {
	t.Run("not sampled never skips", func(t *testing.T) {
		// Arrange
		target := &testWouldWriteHandler{
			wouldWrite: false,
		}

		h := &dualHandler{
			normal:  target,
			sampled: false,
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		// Act
		got := h.shouldSkip(record)

		// Assert
		assert.False(t, got)
		assert.Equal(t, 0, target.wouldWriteCalls)
	})

	t.Run("sampled skips when normal would not write", func(t *testing.T) {
		// Arrange
		target := &testWouldWriteHandler{
			wouldWrite: false,
		}

		h := &dualHandler{
			normal:  target,
			sampled: true,
			always:  &testWouldWriteHandler{wouldWrite: true},
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		// Act
		got := h.shouldSkip(record)

		// Assert
		assert.True(t, got)
		assert.Equal(t, 1, target.wouldWriteCalls)
	})

	t.Run("does not skip when normal would write", func(t *testing.T) {
		// Arrange
		target := &testWouldWriteHandler{
			wouldWrite: true,
		}

		h := &dualHandler{
			normal:  target,
			sampled: true,
			always:  &testWouldWriteHandler{wouldWrite: false},
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		// Act
		got := h.shouldSkip(record)

		// Assert
		assert.False(t, got)
		assert.Equal(t, 1, target.wouldWriteCalls)
	})

	t.Run("always log uses always handler", func(t *testing.T) {
		// Arrange
		normal := &testWouldWriteHandler{
			wouldWrite: false,
		}
		always := &testWouldWriteHandler{
			wouldWrite: true,
		}

		h := &dualHandler{
			normal:  normal,
			always:  always,
			sampled: true,
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		record.AddAttrs(AlwaysLogAttr())

		// Act
		got := h.shouldSkip(record)

		// Assert
		assert.False(t, got)
		assert.Equal(t, 0, normal.wouldWriteCalls)
		assert.Equal(t, 1, always.wouldWriteCalls)
	})

	t.Run("handler without WouldWrite does not skip", func(t *testing.T) {
		// Arrange
		h := &dualHandler{
			normal:  &testPlainHandler{},
			sampled: true,
			always:  &testPlainHandler{},
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		// Act
		got := h.shouldSkip(record)

		// Assert
		assert.False(t, got)
	})
}

func TestDualHandler_Handle_SuccessPath(t *testing.T) {
	// Arrange
	normal := &testHandleHandler{}

	var order []string

	extractor := ExtractorFunc(func(context.Context) []slog.Attr {
		order = append(order, "extractor")
		return []slog.Attr{slog.String("request_id", "123")}
	})

	before := HookFunc(func(_ context.Context, r *slog.Record) error {
		order = append(order, "before")
		return nil
	})

	after := afterHook{
		fn: func(_ context.Context, r slog.Record, err error) {
			order = append(order, "after")
			require.NoError(t, err)
			assert.Equal(t, "test", r.Message)
		},
	}

	h := &dualHandler{
		normal:     normal,
		extractors: extractors{extractor},
		hooks:      hooks{before, after},
		sampled:    false,
	}

	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

	// Act
	err := h.Handle(context.Background(), record)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, []string{"extractor", "before", "after"}, order)
	assert.Equal(t, 1, normal.handleCalls)

	// extractor attrs должны попасть в record
	require.Len(t, normal.records, 1)
	var gotAttrs []slog.Attr
	normal.records[0].Attrs(func(a slog.Attr) bool {
		gotAttrs = append(gotAttrs, a)
		return true
	})
	assert.Equal(t, []slog.Attr{slog.String("request_id", "123")}, gotAttrs)
}

func TestDualHandler_Handle_HandlerErrorGoesToAfter(t *testing.T) {
	// Arrange
	wantErr := errors.New("handle failed")
	normal := &testHandleHandler{handleErr: wantErr}

	var gotErr error
	after := afterHook{
		fn: func(_ context.Context, _ slog.Record, err error) {
			gotErr = err
		},
	}

	h := &dualHandler{
		normal: normal,
		hooks:  hooks{after},
	}

	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

	// Act
	err := h.Handle(context.Background(), record)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, wantErr, gotErr)
	assert.Equal(t, 1, normal.handleCalls)
}

func TestDualHandler_ShouldSkip_AlwaysLogWouldNotWrite(t *testing.T) {
	// Arrange
	normal := &testWouldWriteHandler{wouldWrite: true}
	always := &testWouldWriteHandler{wouldWrite: false}

	h := &dualHandler{
		normal:  normal,
		always:  always,
		sampled: true,
	}

	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
	record.AddAttrs(AlwaysLogAttr())

	// Act
	got := h.shouldSkip(record)

	// Assert
	assert.True(t, got)
	assert.Equal(t, 0, normal.wouldWriteCalls)
	assert.Equal(t, 1, always.wouldWriteCalls)
}

func TestDualHandler_WithAttrs_PreservesExtractorsAndHooks(t *testing.T) {
	// Arrange
	extractor := ExtractorFunc(func(context.Context) []slog.Attr { return nil })
	hook := HookFunc(func(context.Context, *slog.Record) error { return nil })

	h := &dualHandler{
		normal:     newTestTransformHandler(),
		extractors: extractors{extractor},
		hooks:      hooks{hook},
		sampled:    false,
	}

	// Act
	got := h.WithAttrs([]slog.Attr{slog.String("k", "v")}).(*dualHandler)

	// Assert
	assert.Equal(t, h.extractors, got.extractors)
	assert.Equal(t, h.hooks, got.hooks)
}

type testTransformHandler struct {
	name       string
	callerSkip int
	attrs      []slog.Attr
	groups     []string
}

func newTestTransformHandler() *testTransformHandler {
	return &testTransformHandler{}
}

func (*testTransformHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (*testTransformHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (h *testTransformHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	cp := *h
	cp.attrs = append(append([]slog.Attr(nil), h.attrs...), attrs...)
	return &cp
}

func (h *testTransformHandler) WithGroup(group string) slog.Handler {
	cp := *h
	cp.groups = append(append([]string(nil), h.groups...), group)
	return &cp
}

func (h *testTransformHandler) AddCallerSkip(skip int) slog.Handler {
	cp := *h
	cp.callerSkip += skip
	return &cp
}

func (h *testTransformHandler) WithName(name string) slog.Handler {
	cp := *h
	cp.name = name
	return &cp
}

type testPlainHandler struct{}

func (*testPlainHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (*testPlainHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (*testPlainHandler) WithAttrs([]slog.Attr) slog.Handler {
	return &testPlainHandler{}
}

func (*testPlainHandler) WithGroup(string) slog.Handler {
	return &testPlainHandler{}
}

type testEnabledHandler struct {
	enabled      bool
	enabledCalls int
}

func (h *testEnabledHandler) Enabled(context.Context, slog.Level) bool {
	h.enabledCalls++
	return h.enabled
}

func (*testEnabledHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (h *testEnabledHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *testEnabledHandler) WithGroup(string) slog.Handler {
	return h
}

type testWouldWriteHandler struct {
	wouldWrite      bool
	wouldWriteCalls int
}

func (h *testWouldWriteHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (*testWouldWriteHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (h *testWouldWriteHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *testWouldWriteHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *testWouldWriteHandler) WouldWrite(slog.Record) bool {
	h.wouldWriteCalls++
	return h.wouldWrite
}

type testHandleHandler struct {
	handleCalls int
	records     []slog.Record
	handleErr   error
}

func (*testHandleHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *testHandleHandler) Handle(_ context.Context, r slog.Record) error {
	h.handleCalls++
	h.records = append(h.records, r)
	return h.handleErr
}

func (h *testHandleHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *testHandleHandler) WithGroup(string) slog.Handler {
	return h
}

type testWouldWriteHandleHandler struct {
	handleCalls     int
	wouldWrite      bool
	wouldWriteCalls int
}

func (h *testWouldWriteHandleHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *testWouldWriteHandleHandler) Handle(context.Context, slog.Record) error {
	h.handleCalls++
	return nil
}

func (h *testWouldWriteHandleHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *testWouldWriteHandleHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *testWouldWriteHandleHandler) WouldWrite(slog.Record) bool {
	h.wouldWriteCalls++
	return h.wouldWrite
}
