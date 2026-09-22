package logger

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNamed(t *testing.T) {
	t.Run("nil logger", func(t *testing.T) {
		// Act
		got := Named(nil, "service")

		// Assert
		require.NotNil(t, got)
		assert.Same(t, slog.Default(), got)
	})

	t.Run("empty segment", func(t *testing.T) {
		// Arrange
		l := slog.New(slog.NewTextHandler(nil, nil))

		testCases := [...]struct {
			name    string
			segment string
		}{
			{
				name:    "empty",
				segment: "",
			},
			{
				name:    "spaces",
				segment: "   ",
			},
			{
				name:    "tabs",
				segment: "\t",
			},
			{
				name:    "spaces and tabs",
				segment: " \t ",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Act
				got := Named(l, tc.segment)

				// Assert
				assert.Same(t, l, got)
			})
		}
	})

	t.Run("handler supports WithName", func(t *testing.T) {
		// Arrange
		handler := &testNamedHandler{}
		l := slog.New(handler)

		// Act
		got := Named(l, " service ")

		// Assert
		require.NotNil(t, got)
		assert.NotSame(t, l, got)

		assert.Equal(t, "service", handler.name)

		gotHandler, ok := got.Handler().(*testNamedHandler)
		require.True(t, ok)
		assert.Same(t, handler, gotHandler)
	})

	t.Run("handler does not support WithName", func(t *testing.T) {
		// Arrange
		handler := &testHandlerWithoutName{}
		l := slog.New(handler)

		// Act
		got := Named(l, "service")

		// Assert
		assert.Same(t, l, got)
	})
}

func TestNormalizeName(t *testing.T) {
	testCases := [...]struct {
		name string
		in   string
		want string
	}{
		{
			name: "empty",
			in:   "",
			want: "",
		},
		{
			name: "spaces",
			in:   "   ",
			want: "",
		},
		{
			name: "simple",
			in:   "service",
			want: "service",
		},
		{
			name: "leading spaces",
			in:   "  service",
			want: "service",
		},
		{
			name: "trailing spaces",
			in:   "service  ",
			want: "service",
		},
		{
			name: "leading and trailing dots",
			in:   ".service.",
			want: "service",
		},
		{
			name: "multiple dots",
			in:   "service..api...handler",
			want: "service.api.handler",
		},
		{
			name: "spaces around segments",
			in:   " service . api . handler ",
			want: "service.api.handler",
		},
		{
			name: "empty segments",
			in:   "service...api",
			want: "service.api",
		},
		{
			name: "dots and spaces",
			in:   " . service . . api . ",
			want: "service.api",
		},
		{
			name: "only dots",
			in:   "...",
			want: "",
		},
		{
			name: "only dots and spaces",
			in:   " . . . ",
			want: "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := normalizeName(tc.in)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestJoinName(t *testing.T) {
	testCases := [...]struct {
		name    string
		base    string
		segment string
		want    string
	}{
		{
			name:    "both empty",
			base:    "",
			segment: "",
			want:    "",
		},
		{
			name:    "empty base",
			base:    "",
			segment: "service",
			want:    "service",
		},
		{
			name:    "empty segment",
			base:    "service",
			segment: "",
			want:    "service",
		},
		{
			name:    "both values",
			base:    "service",
			segment: "api",
			want:    "service.api",
		},
		{
			name:    "normalizes base",
			base:    ".service..api.",
			segment: "handler",
			want:    "service.api.handler",
		},
		{
			name:    "normalizes segment",
			base:    "service",
			segment: ".api..handler.",
			want:    "service.api.handler",
		},
		{
			name:    "normalizes both",
			base:    " . service . . api . ",
			segment: " . handler . ",
			want:    "service.api.handler",
		},
		{
			name:    "base becomes empty",
			base:    "...",
			segment: "service",
			want:    "service",
		},
		{
			name:    "segment becomes empty",
			base:    "service",
			segment: "...",
			want:    "service",
		},
		{
			name:    "both normalize to empty",
			base:    "...",
			segment: " . . ",
			want:    "",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := joinName(tc.base, tc.segment)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}

type testNamedHandler struct {
	name string
}

func (h *testNamedHandler) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *testNamedHandler) Handle(context.Context, slog.Record) error {
	return nil
}

func (h *testNamedHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *testNamedHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *testNamedHandler) WithName(segment string) slog.Handler {
	h.name = segment
	return h
}

type testHandlerWithoutName struct{}

func (h *testHandlerWithoutName) Enabled(context.Context, slog.Level) bool {
	return true
}

func (h *testHandlerWithoutName) Handle(context.Context, slog.Record) error {
	return nil
}

func (h *testHandlerWithoutName) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *testHandlerWithoutName) WithGroup(string) slog.Handler {
	return h
}
