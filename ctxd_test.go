package logger_test

import (
	"bytes"
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/selfshop-dev/logger"
)

func TestIntoContext_FromContext(t *testing.T) {
	// Arrange
	l := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	// Act
	ctx := logger.IntoContext(context.Background(), l)

	// Assert
	assert.Same(t, l, logger.FromContext(ctx))
}

func TestIntoContext_NilContext(t *testing.T) {
	// Arrange
	l := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	// Act
	ctx := logger.IntoContext(nil, l) //nolint:staticcheck // intentional: verifies nil context is replaced with context.Background()

	// Assert
	assert.NotNil(t, ctx)
	assert.Same(t, l, logger.FromContext(ctx))
}

func TestIntoContext_NilLogger(t *testing.T) {
	// Arrange
	type unrelatedKey struct{}

	ctx := context.WithValue(context.Background(), unrelatedKey{}, "value")

	// Act
	got := logger.IntoContext(ctx, nil)

	// Assert
	assert.Same(t, ctx, got)
	assert.Same(t, slog.Default(), logger.FromContext(ctx))
}

func TestFromContext_NilContext(t *testing.T) {
	// Act
	got := logger.FromContext(nil) //nolint:staticcheck // intentional: verifies FromContext handles nil context

	// Assert
	assert.Same(t, slog.Default(), got)
}

func TestFromContextOr_ContextLogger(t *testing.T) {
	// Arrange
	l := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	ctx := logger.IntoContext(context.Background(), l)
	fallback := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	// Act
	got := logger.FromContextOr(ctx, fallback)

	// Assert
	assert.Same(t, l, got)
}

func TestFromContextOr_Fallback(t *testing.T) {
	// Arrange
	fallback := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	// Act
	got := logger.FromContextOr(context.Background(), fallback)

	// Assert
	assert.Same(t, fallback, got)
}

func TestFromContextOr_Default(t *testing.T) {
	// Arrange
	original := slog.Default()
	defer slog.SetDefault(original)

	custom := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	slog.SetDefault(custom)

	// Act
	got := logger.FromContextOr(context.Background(), nil)

	// Assert
	assert.Same(t, custom, got)
}

func TestIntoContext_BothNil(t *testing.T) {
	// Arrange
	// Act
	got := logger.IntoContext(nil, nil) //nolint:staticcheck // intentional: verifies nil-context and nil-logger boundary

	// Assert
	assert.NotNil(t, got)
	assert.Same(t, slog.Default(), logger.FromContext(got))
}

func TestFromContextOr_NilLoggerInContext(t *testing.T) {
	// Arrange
	ctx := logger.IntoContext(context.Background(), (*slog.Logger)(nil))
	fallback := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))

	// Act
	got := logger.FromContextOr(ctx, fallback)

	// Assert
	assert.Same(t, fallback, got)
}
