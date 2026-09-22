package logger

import (
	"context"
	"log/slog"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"go.uber.org/zap/zaptest/observer"
)

func TestWrapHandler_AddCallerSkip(t *testing.T) {
	t.Run("zero skip returns same handler", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		got := h.AddCallerSkip(0)
		assert.Same(t, h, got)
	})

	t.Run("positive skip returns copy", func(t *testing.T) {
		h := &wrapHandler{
			core:       observerCore(),
			callerSkip: 2,
		}
		got := h.AddCallerSkip(3)

		result, ok := got.(*wrapHandler)
		require.True(t, ok)

		assert.NotSame(t, h, result)
		assert.Equal(t, 2, h.callerSkip)
		assert.Equal(t, 5, result.callerSkip)
	})

	t.Run("negative skip cannot make value negative", func(t *testing.T) {
		h := &wrapHandler{
			core:       observerCore(),
			callerSkip: 2,
		}
		got := h.AddCallerSkip(-10)

		result, ok := got.(*wrapHandler)
		require.True(t, ok)

		assert.Equal(t, 2, h.callerSkip)
		assert.Equal(t, 0, result.callerSkip)
	})

	t.Run("negative skip subtracts from existing value", func(t *testing.T) {
		h := &wrapHandler{
			core:       observerCore(),
			callerSkip: 5,
		}
		got := h.AddCallerSkip(-2)

		result, ok := got.(*wrapHandler)
		require.True(t, ok)

		assert.Equal(t, 5, h.callerSkip)
		assert.Equal(t, 3, result.callerSkip)
	})
}

func TestWrapHandler_WithName(t *testing.T) {
	t.Run("empty segment returns same handler", func(t *testing.T) {
		h := &wrapHandler{
			core: observerCore(),
			name: "base",
		}
		got := h.WithName("   ")
		assert.Same(t, h, got)
	})

	t.Run("segment is trimmed", func(t *testing.T) {
		h := &wrapHandler{
			core: observerCore(),
			name: "base",
		}
		got := h.WithName(" service ")

		result, ok := got.(*wrapHandler)
		require.True(t, ok)

		assert.NotSame(t, h, result)
		assert.Equal(t, "base.service", result.name)
		assert.Equal(t, "base", h.name)
	})

	t.Run("adds name to empty name", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		got := h.WithName(" service ")

		result, ok := got.(*wrapHandler)
		require.True(t, ok)
		assert.Equal(t, "service", result.name)
	})
}

func TestWrapHandler_Enabled(t *testing.T) {
	testCases := [...]struct {
		name string
		core zapcore.Level
		l    slog.Level
		want bool
	}{
		{name: "debug enabled", core: zapcore.DebugLevel, l: slog.LevelDebug, want: true},
		{name: "info enabled", core: zapcore.InfoLevel, l: slog.LevelInfo, want: true},
		{name: "info disabled by warn core", core: zapcore.WarnLevel, l: slog.LevelInfo, want: false},
		{name: "warn enabled", core: zapcore.WarnLevel, l: slog.LevelWarn, want: true},
		{name: "error enabled", core: zapcore.ErrorLevel, l: slog.LevelError, want: true},
		{name: "debug disabled by info core", core: zapcore.InfoLevel, l: slog.LevelDebug, want: false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			h := &wrapHandler{core: observerCoreAt(tc.core)}
			got := h.Enabled(context.Background(), tc.l)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestWrapHandler_WouldWrite(t *testing.T) {
	t.Run("returns true when core accepts record", func(t *testing.T) {
		h := &wrapHandler{core: observerCoreAt(zapcore.InfoLevel)}
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		assert.True(t, h.WouldWrite(record))
	})

	t.Run("returns false when core rejects record", func(t *testing.T) {
		h := &wrapHandler{core: observerCoreAt(zapcore.WarnLevel)}
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		assert.False(t, h.WouldWrite(record))
	})
}

func TestWrapHandler_WithAttrs(t *testing.T) {
	t.Run("empty attrs returns same handler", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		got := h.WithAttrs(nil)
		assert.Same(t, h, got)
	})

	t.Run("regular attrs are stored", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		attrs := []slog.Attr{
			slog.String("service", "users"),
			slog.Int("port", 8080),
		}
		got := h.WithAttrs(attrs)

		result, ok := got.(*wrapHandler)
		require.True(t, ok)
		require.Len(t, result.stored, 2)

		assert.Equal(t, "service", result.stored[0].field.Key)
		assert.Equal(t, "users", result.stored[0].field.String)
		assert.Equal(t, "port", result.stored[1].field.Key)
		assert.Equal(t, int64(8080), result.stored[1].field.Integer)
	})

	t.Run("naming attr changes logger name and is not stored", func(t *testing.T) {
		h := &wrapHandler{
			core: observerCore(),
			name: "base",
		}
		got := h.WithAttrs([]slog.Attr{LogNamingAttr(" service.api ")})

		result, ok := got.(*wrapHandler)
		require.True(t, ok)
		assert.Equal(t, "service.api", result.name)
		assert.Empty(t, result.stored)
	})

	t.Run("internal attrs are skipped", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		attrs := []slog.Attr{
			{Key: attrPrefix + "internal", Value: slog.StringValue("secret")},
			slog.String("visible", "value"),
		}
		got := h.WithAttrs(attrs)

		result, ok := got.(*wrapHandler)
		require.True(t, ok)
		require.Len(t, result.stored, 1)
		assert.Equal(t, "visible", result.stored[0].field.Key)
	})

	t.Run("previous stored attrs are preserved", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		first := h.WithAttrs([]slog.Attr{slog.String("first", "one")})
		second := first.WithAttrs([]slog.Attr{slog.String("second", "two")})

		result, ok := second.(*wrapHandler)
		require.True(t, ok)
		require.Len(t, result.stored, 2)
		assert.Equal(t, "first", result.stored[0].field.Key)
		assert.Equal(t, "second", result.stored[1].field.Key)

		assert.Empty(t, h.stored)
	})

	t.Run("only skipped attrs returns same handler", func(t *testing.T) {
		t.Run("only internal", func(t *testing.T) {
			h := &wrapHandler{core: observerCore(), name: "base"}
			got := h.WithAttrs([]slog.Attr{
				{Key: attrPrefix + "secret", Value: slog.StringValue("x")},
			})
			assert.Same(t, h, got)
		})

		t.Run("only empty naming", func(t *testing.T) {
			h := &wrapHandler{core: observerCore(), name: "base"}
			got := h.WithAttrs([]slog.Attr{LogNamingAttr("   ")})
			assert.Same(t, h, got)
			assert.Equal(t, "base", h.name)
		})

		t.Run("mixed skipped with existing group", func(t *testing.T) {
			h := &wrapHandler{
				core:      observerCore(),
				name:      "base",
				groupPath: []string{"req"},
				grouped:   true,
			}
			got := h.WithAttrs([]slog.Attr{
				{Key: attrPrefix + "x", Value: slog.StringValue("y")},
				LogNamingAttr(""),
			})
			assert.Same(t, h, got)
		})
	})

	t.Run("multiple naming attrs - last wins", func(t *testing.T) {
		h := &wrapHandler{core: observerCore(), name: "base"}
		got := h.WithAttrs([]slog.Attr{
			LogNamingAttr("first"),
			slog.String("k", "v"),
			LogNamingAttr(" second "),
		}).(*wrapHandler)

		assert.Equal(t, "second", got.name)
		require.Len(t, got.stored, 1)
		assert.Equal(t, "k", got.stored[0].field.Key)
	})
}

func TestWrapHandler_WithGroup(t *testing.T) {
	t.Run("empty group returns same handler", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		got := h.WithGroup("   ")
		assert.Same(t, h, got)
	})

	t.Run("group is trimmed and stored", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		got := h.WithGroup(" request ")

		result, ok := got.(*wrapHandler)
		require.True(t, ok)

		assert.NotSame(t, h, result)
		assert.Equal(t, []string{"request"}, result.groupPath)
		assert.True(t, result.grouped)
		assert.Empty(t, h.groupPath)
		assert.False(t, h.grouped)
	})

	t.Run("multiple groups are appended", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		got := h.WithGroup("request").WithGroup("http")

		result, ok := got.(*wrapHandler)
		require.True(t, ok)
		assert.Equal(t, []string{"request", "http"}, result.groupPath)
		assert.True(t, result.grouped)
	})

	t.Run("does not mutate original group path", func(t *testing.T) {
		h := &wrapHandler{
			core:      observerCore(),
			groupPath: []string{"base"},
			grouped:   true,
		}
		got := h.WithGroup("child")

		result, ok := got.(*wrapHandler)
		require.True(t, ok)

		assert.Equal(t, []string{"base"}, h.groupPath)
		assert.Equal(t, []string{"base", "child"}, result.groupPath)
	})
}

func TestWrapHandler_ConsumeAttr(t *testing.T) {
	t.Run("regular attr is stored without group", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		name, grouped, stored := h.consumeAttr(
			slog.String("service", "users"),
			"base",
			false,
			nil,
		)

		assert.Equal(t, "base", name)
		assert.False(t, grouped)
		require.Len(t, stored, 1)
		assert.Equal(t, "service", stored[0].field.Key)
		assert.Equal(t, "users", stored[0].field.String)
		assert.Empty(t, stored[0].path)
	})

	t.Run("attr inside group enables grouped mode", func(t *testing.T) {
		h := &wrapHandler{
			core:      observerCore(),
			groupPath: []string{"request"},
		}
		name, grouped, stored := h.consumeAttr(
			slog.String("method", "GET"),
			"base",
			false,
			nil,
		)

		assert.Equal(t, "base", name)
		assert.True(t, grouped)
		require.Len(t, stored, 1)
		assert.Equal(t, []string{"request"}, stored[0].path)
		assert.Equal(t, "method", stored[0].field.Key)
	})

	t.Run("naming attr changes name and is skipped", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		name, grouped, stored := h.consumeAttr(
			LogNamingAttr("service.api"),
			"base",
			false,
			nil,
		)

		assert.Equal(t, "service.api", name)
		assert.False(t, grouped)
		assert.Empty(t, stored)
	})

	t.Run("internal attr is skipped", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		attr := slog.Attr{
			Key:   attrPrefix + "test",
			Value: slog.StringValue("value"),
		}
		name, grouped, stored := h.consumeAttr(attr, "base", false, nil)

		assert.Equal(t, "base", name)
		assert.False(t, grouped)
		assert.Empty(t, stored)
	})
}

func TestWrapHandler_ProcessAttr(t *testing.T) {
	t.Run("regular attr is appended to fields without groups", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		fields := make([]zapcore.Field, 0)

		name := h.processAttr(slog.String("service", "users"), "base", nil, &fields)

		assert.Equal(t, "base", name)
		require.Len(t, fields, 1)
		assert.Equal(t, "service", fields[0].Key)
		assert.Equal(t, "users", fields[0].String)
	})

	t.Run("naming attr changes name", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		fields := make([]zapcore.Field, 0)

		name := h.processAttr(LogNamingAttr("service.api"), "base", nil, &fields)

		assert.Equal(t, "service.api", name)
		assert.Empty(t, fields)
	})

	t.Run("internal attr is skipped", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		fields := make([]zapcore.Field, 0)
		attr := slog.Attr{
			Key:   attrPrefix + "secret",
			Value: slog.StringValue("hidden"),
		}

		name := h.processAttr(attr, "base", nil, &fields)

		assert.Equal(t, "base", name)
		assert.Empty(t, fields)
	})
}

func TestClassifyAttr(t *testing.T) {
	t.Run("regular attr", func(t *testing.T) {
		name, skip, field := classifyAttr(slog.String("service", "users"), "base")
		assert.Equal(t, "base", name)
		assert.False(t, skip)
		assert.Equal(t, "service", field.Key)
		assert.Equal(t, "users", field.String)
	})

	t.Run("valid naming attr", func(t *testing.T) {
		name, skip, field := classifyAttr(LogNamingAttr(" service.api "), "base")
		assert.Equal(t, "service.api", name)
		assert.True(t, skip)
		assert.Equal(t, zapcore.Field{}, field)
	})

	t.Run("empty naming attr keeps previous name", func(t *testing.T) {
		name, skip, field := classifyAttr(LogNamingAttr("   "), "base")
		assert.Equal(t, "base", name)
		assert.True(t, skip)
		assert.Equal(t, zapcore.Field{}, field)
	})

	t.Run("internal attr is skipped", func(t *testing.T) {
		name, skip, field := classifyAttr(slog.Attr{
			Key:   attrPrefix + "internal",
			Value: slog.StringValue("value"),
		}, "base")
		assert.Equal(t, "base", name)
		assert.True(t, skip)
		assert.Equal(t, zapcore.Field{}, field)
	})
}

func TestConvertSlogToZap(t *testing.T) {
	testCases := [...]struct {
		name string
		in   slog.Level
		want zapcore.Level
	}{
		{name: "debug", in: slog.LevelDebug, want: zapcore.DebugLevel},
		{name: "info", in: slog.LevelInfo, want: zapcore.InfoLevel},
		{name: "warn", in: slog.LevelWarn, want: zapcore.WarnLevel},
		{name: "error", in: slog.LevelError, want: zapcore.ErrorLevel},
		{name: "below debug", in: slog.Level(-1), want: zapcore.DebugLevel},
		{name: "between info and warn", in: slog.Level(2), want: zapcore.InfoLevel},
		{name: "between warn and error", in: slog.Level(5), want: zapcore.WarnLevel},
		{name: "above error", in: slog.Level(20), want: zapcore.ErrorLevel},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.want, convertSlogToZap(tc.in))
		})
	}
}

func TestWrapHandler_BuildTree(t *testing.T) {
	t.Run("not grouped returns nil", func(t *testing.T) {
		h := &wrapHandler{core: observerCore()}
		assert.Nil(t, h.buildTree())
	})

	t.Run("grouped returns tree", func(t *testing.T) {
		h := &wrapHandler{
			core:    observerCore(),
			grouped: true,
			stored: []storedField{
				{path: []string{"request"}, field: zap.String("method", "GET")},
				{path: []string{"request"}, field: zap.Int("status", 200)},
			},
		}
		got := h.buildTree()
		require.NotNil(t, got)

		require.Len(t, got.order, 1)
		assert.Equal(t, "request", got.order[0])
		require.Contains(t, got.groups, "request")

		child := got.groups["request"]
		require.Len(t, child.fields, 2)
		assert.Equal(t, "method", child.fields[0].Key)
		assert.Equal(t, "status", child.fields[1].Key)
	})
}

func TestWrapHandler_Handle(t *testing.T) {
	t.Run("writes record", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := &wrapHandler{core: core, name: "service"}

		record := slog.NewRecord(
			time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
			slog.LevelInfo,
			"hello",
			0,
		)
		record.AddAttrs(
			slog.String("request_id", "abc"),
			slog.Int("status", 200),
		)

		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())

		entry := logs.All()[0]
		assert.Equal(t, "hello", entry.Message)
		assert.Equal(t, "service", entry.LoggerName)
		require.Len(t, entry.Context, 2)
		assert.Equal(t, "request_id", entry.Context[0].Key)
		assert.Equal(t, "abc", entry.Context[0].String)
		assert.Equal(t, "status", entry.Context[1].Key)
		assert.Equal(t, int64(200), entry.Context[1].Integer)
	})

	t.Run("disabled record is not written", func(t *testing.T) {
		core, logs := observer.New(zapcore.WarnLevel)
		h := &wrapHandler{core: core}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "ignored", 0)
		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		assert.Equal(t, 0, logs.Len())
	})

	t.Run("stored attrs are written", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := (&wrapHandler{core: core}).WithAttrs([]slog.Attr{
			slog.String("service", "users"),
			slog.Int("version", 2),
		}).(*wrapHandler)

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "started", 0)
		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())

		fields := logs.All()[0].Context
		require.Len(t, fields, 2)
		assert.Equal(t, "service", fields[0].Key)
		assert.Equal(t, "users", fields[0].String)
		assert.Equal(t, "version", fields[1].Key)
		assert.Equal(t, int64(2), fields[1].Integer)
	})

	t.Run("record attrs are processed", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := &wrapHandler{core: core, name: "base"}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "started", 0)
		record.AddAttrs(
			slog.String("request_id", "123"),
			slog.String("method", "GET"),
		)

		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())

		fields := logs.All()[0].Context
		require.Len(t, fields, 2)
		assert.Equal(t, "request_id", fields[0].Key)
		assert.Equal(t, "method", fields[1].Key)
	})

	t.Run("record naming attr changes logger name", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := &wrapHandler{core: core, name: "base"}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		record.AddAttrs(LogNamingAttr("service.api"))

		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())
		assert.Equal(t, "service.api", logs.All()[0].LoggerName)
	})

	t.Run("internal attrs are not written", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := &wrapHandler{core: core}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		record.AddAttrs(
			slog.String("visible", "yes"),
			slog.String(attrPrefix+"internal", "no"),
		)

		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())

		fields := logs.All()[0].Context
		require.Len(t, fields, 1)
		assert.Equal(t, "visible", fields[0].Key)
	})

	t.Run("stored and record attrs are combined", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := (&wrapHandler{core: core}).WithAttrs([]slog.Attr{
			slog.String("service", "users"),
		}).(*wrapHandler)

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		record.AddAttrs(slog.String("request_id", "123"))

		err := h.Handle(context.Background(), record)
		require.NoError(t, err)

		fields := logs.All()[0].Context
		require.Len(t, fields, 2)
		assert.Equal(t, "service", fields[0].Key)
		assert.Equal(t, "request_id", fields[1].Key)
	})

	t.Run("caller is added", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := &wrapHandler{
			core:       core,
			addCaller:  true,
			callerSkip: 0,
		}

		doLog := func(handler slog.Handler) {
			var pcs [1]uintptr
			runtime.Callers(1, pcs[:])
			rec := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", pcs[0])
			_ = handler.Handle(context.Background(), rec)
		}

		wrap1 := func(h slog.Handler) { doLog(h) }
		wrap2 := func(h slog.Handler) { wrap1(h) }
		wrap3 := func(h slog.Handler) { wrap2(h) }

		wrap3(h)

		require.Equal(t, 1, logs.Len())
		caller := logs.All()[0].Caller
		assert.True(t, caller.Defined)
		assert.NotEmpty(t, caller.File)
		assert.NotZero(t, caller.Line)
		assert.NotEmpty(t, caller.Function)
	})

	t.Run("caller is not added when disabled", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := &wrapHandler{core: core, addCaller: false}

		var pcs [1]uintptr
		n := runtime.Callers(1, pcs[:])
		require.Equal(t, 1, n)

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", pcs[0])
		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())
		assert.False(t, logs.All()[0].Caller.Defined)
	})

	t.Run("caller is not added when record PC is zero", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := &wrapHandler{core: core, addCaller: true}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())
		assert.False(t, logs.All()[0].Caller.Defined)
	})

	t.Run("stack is added at configured level", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		stackAt := slog.LevelError
		h := &wrapHandler{
			core:    core,
			stackAt: &stackAt,
		}

		doLog := func() {
			rec := slog.NewRecord(time.Time{}, slog.LevelError, "failure", 0)
			_ = h.Handle(context.Background(), rec)
		}
		wrap1 := func() { doLog() }
		wrap2 := func() { wrap1() }
		wrap3 := func() { wrap2() }

		wrap3()

		require.Equal(t, 1, logs.Len())
		assert.NotEmpty(t, logs.All()[0].Stack)
	})

	t.Run("stack is not added below configured level", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		stackAt := slog.LevelError
		h := &wrapHandler{core: core, stackAt: &stackAt}

		record := slog.NewRecord(time.Time{}, slog.LevelWarn, "warning", 0)
		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())
		assert.Empty(t, logs.All()[0].Stack)
	})

	t.Run("stack is not added when stackAt is nil", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := &wrapHandler{core: core}

		record := slog.NewRecord(time.Time{}, slog.LevelError, "failure", 0)
		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())
		assert.Empty(t, logs.All()[0].Stack)
	})

	t.Run("grouped attrs are written as nested objects", func(t *testing.T) {
		core, logs := observer.New(zapcore.DebugLevel)
		h := &wrapHandler{
			core:      core,
			groupPath: []string{"request", "http"},
			grouped:   true,
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "request", 0)
		record.AddAttrs(
			slog.String("method", "GET"),
			slog.Int("status", 200),
		)

		err := h.Handle(context.Background(), record)
		require.NoError(t, err)
		require.Equal(t, 1, logs.Len())

		fields := logs.All()[0].Context
		require.Len(t, fields, 1)
		assert.Equal(t, "request", fields[0].Key)
		assert.Equal(t, zapcore.ObjectMarshalerType, fields[0].Type)

		requestEncoder := zapcore.NewMapObjectEncoder()
		err = fields[0].Interface.(zapcore.ObjectMarshaler).MarshalLogObject(requestEncoder)
		require.NoError(t, err)

		httpValue, ok := requestEncoder.Fields["http"].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "GET", httpValue["method"])
		assert.Equal(t, int64(200), httpValue["status"])
	})
}

func TestWrapHandler_Handle_GroupedStoredAttrs(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	h := (&wrapHandler{
		core:      core,
		groupPath: []string{"request"},
		grouped:   true,
	}).WithAttrs([]slog.Attr{
		slog.String("id", "123"),
	}).(*wrapHandler)

	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
	record.AddAttrs(slog.String("method", "GET"))

	err := h.Handle(context.Background(), record)
	require.NoError(t, err)
	require.Equal(t, 1, logs.Len())

	fields := logs.All()[0].Context
	require.Len(t, fields, 1)
	assert.Equal(t, "request", fields[0].Key)
	assert.Equal(t, zapcore.ObjectMarshalerType, fields[0].Type)

	requestEncoder := zapcore.NewMapObjectEncoder()
	err = fields[0].Interface.(zapcore.ObjectMarshaler).MarshalLogObject(requestEncoder)
	require.NoError(t, err)

	assert.Equal(t, "123", requestEncoder.Fields["id"])
	assert.Equal(t, "GET", requestEncoder.Fields["method"])
}

func TestWrapHandler_Handle_NamingAndAttrsTogether(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	h := &wrapHandler{core: core, name: "base"}

	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "started", 0)
	record.AddAttrs(
		slog.String("request_id", "123"),
		LogNamingAttr("api.users"),
		slog.String("method", "GET"),
	)

	err := h.Handle(context.Background(), record)
	require.NoError(t, err)
	require.Equal(t, 1, logs.Len())

	entry := logs.All()[0]
	assert.Equal(t, "api.users", entry.LoggerName)
	require.Len(t, entry.Context, 2)
	assert.Equal(t, "request_id", entry.Context[0].Key)
	assert.Equal(t, "method", entry.Context[1].Key)
}

func TestWrapHandler_Handle_NamingAttrLastWins(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)
	h := &wrapHandler{core: core, name: "base"}

	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
	record.AddAttrs(
		LogNamingAttr("first"),
		slog.String("key", "value"),
		LogNamingAttr("second"),
	)

	err := h.Handle(context.Background(), record)
	require.NoError(t, err)
	require.Equal(t, 1, logs.Len())

	assert.Equal(t, "second", logs.All()[0].LoggerName)
	require.Len(t, logs.All()[0].Context, 1)
	assert.Equal(t, "key", logs.All()[0].Context[0].Key)
}

func TestWrapHandler_Handle_DifferentStoredPathsAndRecordAttrs(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	h := &wrapHandler{
		core:      core,
		groupPath: []string{"request"},
		grouped:   true,
		stored: []storedField{
			{path: []string{"user"}, field: zap.String("id", "u-1")},
			{path: []string{"request"}, field: zap.String("id", "r-1")},
		},
	}

	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "msg", 0)
	record.AddAttrs(slog.String("method", "POST"))

	require.NoError(t, h.Handle(context.Background(), record))
	require.Equal(t, 1, logs.Len())

	fields := logs.All()[0].Context
	require.Len(t, fields, 2)

	var userObj, reqObj map[string]any
	for _, f := range fields {
		enc := zapcore.NewMapObjectEncoder()
		require.NoError(t, f.Interface.(zapcore.ObjectMarshaler).MarshalLogObject(enc))
		switch f.Key {
		case "user":
			userObj = enc.Fields
		case "request":
			reqObj = enc.Fields
		}
	}

	require.NotNil(t, userObj)
	require.NotNil(t, reqObj)
	assert.Equal(t, "u-1", userObj["id"])
	assert.Equal(t, "r-1", reqObj["id"])
	assert.Equal(t, "POST", reqObj["method"])
}

func TestWrapHandler_WithAttrs_DoesNotMutateOriginal(t *testing.T) {
	h := &wrapHandler{
		core: observerCore(),
		stored: []storedField{
			{path: []string{"request"}, field: zap.String("id", "123")},
		},
	}

	got := h.WithAttrs([]slog.Attr{slog.String("method", "GET")})

	result, ok := got.(*wrapHandler)
	require.True(t, ok)

	require.Len(t, h.stored, 1)
	require.Len(t, result.stored, 2)

	assert.Equal(t, "id", h.stored[0].field.Key)
	assert.Equal(t, "id", result.stored[0].field.Key)
	assert.Equal(t, "method", result.stored[1].field.Key)
}

func TestWrapHandler_WithAttrs_DoesNotMutateStoredSlice(t *testing.T) {
	origStored := []storedField{{path: nil, field: zap.String("a", "1")}}
	h := &wrapHandler{
		core:   observerCore(),
		stored: origStored,
	}

	got := h.WithAttrs([]slog.Attr{slog.String("b", "2")}).(*wrapHandler)

	require.Len(t, h.stored, 1)
	require.Len(t, got.stored, 2)

	got.stored[0].field = zap.String("mutated", "x")
	assert.Equal(t, "a", h.stored[0].field.Key)
	assert.Equal(t, "a", origStored[0].field.Key)
}

func TestWrapHandler_WithAttrs_PreservesGroupPath(t *testing.T) {
	h := &wrapHandler{
		core:      observerCore(),
		groupPath: []string{"request"},
		grouped:   true,
	}

	got := h.WithAttrs([]slog.Attr{slog.String("id", "123")})

	result, ok := got.(*wrapHandler)
	require.True(t, ok)

	assert.Equal(t, []string{"request"}, result.groupPath)
	assert.True(t, result.grouped)
	require.Len(t, result.stored, 1)
	assert.Equal(t, []string{"request"}, result.stored[0].path)
}

func TestWrapHandler_AddCallerSkip_PreservesState(t *testing.T) {
	stackAt := slog.LevelError
	h := &wrapHandler{
		core:       observerCore(),
		name:       "service",
		groupPath:  []string{"request"},
		grouped:    true,
		stackAt:    &stackAt,
		addCaller:  true,
		callerSkip: 2,
		stored: []storedField{
			{path: []string{"request"}, field: zap.String("id", "123")},
		},
	}

	got := h.AddCallerSkip(3)

	result, ok := got.(*wrapHandler)
	require.True(t, ok)

	assert.Equal(t, "service", result.name)
	assert.Equal(t, []string{"request"}, result.groupPath)
	assert.True(t, result.grouped)
	assert.Equal(t, stackAt, *result.stackAt)
	assert.True(t, result.addCaller)
	assert.Equal(t, 5, result.callerSkip)

	require.Len(t, result.stored, 1)
	assert.Equal(t, "id", result.stored[0].field.Key)
}

func TestWrapHandler_AddCallerSkip_AffectsFrame(t *testing.T) {
	core, logs := observer.New(zapcore.DebugLevel)

	h := &wrapHandler{
		core:       core,
		addCaller:  true,
		callerSkip: 0,
	}

	logFromHelper := func(handler slog.Handler) {
		var pcs [1]uintptr
		runtime.Callers(1, pcs[:])
		rec := slog.NewRecord(time.Time{}, slog.LevelInfo, "from-helper", pcs[0])
		_ = handler.Handle(context.Background(), rec)
	}

	wrap1 := func(h slog.Handler) { logFromHelper(h) }
	wrap2 := func(h slog.Handler) { wrap1(h) }
	wrap3 := func(h slog.Handler) { wrap2(h) }

	wrap3(h)
	require.Equal(t, 1, logs.Len())
	caller0 := logs.All()[0].Caller
	require.True(t, caller0.Defined, "caller should be defined with deep enough stack")

	h2 := h.AddCallerSkip(1).(*wrapHandler)
	logs.TakeAll()

	wrap3(h2)
	require.Equal(t, 1, logs.Len())
	caller1 := logs.All()[0].Caller
	require.True(t, caller1.Defined)

	assert.NotEqual(t, caller0.Function, caller1.Function)
}

func observerCore() zapcore.Core {
	return observerCoreAt(zapcore.DebugLevel)
}

func observerCoreAt(level zapcore.Level) zapcore.Core {
	core, _ := observer.New(level)
	return core
}
