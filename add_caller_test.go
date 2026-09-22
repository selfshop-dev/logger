package logger_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/selfshop-dev/logger"
)

func TestAddCallerSkip(t *testing.T) {
	t.Run("nil logger", func(t *testing.T) {
		// Act
		got := logger.AddCallerSkip(nil, 1)

		// Assert
		require.NotNil(t, got)
		assert.Same(t, slog.Default(), got)
	})

	t.Run("zero skip", func(t *testing.T) {
		// Arrange
		l := slog.New(slog.NewTextHandler(nil, nil))

		// Act
		got := logger.AddCallerSkip(l, 0)

		// Assert
		assert.Same(t, l, got)
	})

	t.Run("handler supports caller skip", func(t *testing.T) {
		// Arrange
		handler := &testCallerSkipHandler{}
		l := slog.New(handler)

		// Act
		got := logger.AddCallerSkip(l, 2)

		// Assert
		require.NotNil(t, got)
		assert.NotSame(t, l, got)

		require.Equal(t, 2, handler.skip)

		gotHandler, ok := got.Handler().(*testCallerSkipHandler)
		require.True(t, ok)
		assert.Same(t, handler, gotHandler)
	})

	t.Run("handler does not support caller skip", func(t *testing.T) {
		// Arrange
		handler := &testHandler{}
		l := slog.New(handler)

		// Act
		got := logger.AddCallerSkip(l, 2)

		// Assert
		assert.Same(t, l, got)
	})

	t.Run("negative skip", func(t *testing.T) {
		// Arrange
		handler := &testCallerSkipHandler{skip: 5}
		l := slog.New(handler)

		// Act
		got := logger.AddCallerSkip(l, -3)

		// Assert
		require.NotNil(t, got)
		assert.Equal(t, 2, handler.skip) // 5 + (-3)
	})
}

type testCallerSkipHandler struct {
	skip int
}

func (h *testCallerSkipHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *testCallerSkipHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (h *testCallerSkipHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *testCallerSkipHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *testCallerSkipHandler) AddCallerSkip(skip int) slog.Handler {
	h.skip += skip
	return h
}

type testHandler struct{}

func (h *testHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *testHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (h *testHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *testHandler) WithGroup(string) slog.Handler {
	return h
}
