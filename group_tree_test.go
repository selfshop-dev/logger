package logger

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func TestGroupTree_Add(t *testing.T) {
	t.Run("adds field to root", func(t *testing.T) {
		// Arrange
		g := &groupTree{}
		field := zap.String("key", "value")

		// Act
		g.add(nil, field)

		// Assert
		assert.Len(t, g.fields, 1)
		assert.Equal(t, field.Key, g.fields[0].Key)
		assert.Nil(t, g.groups)
		assert.Nil(t, g.order)
	})

	t.Run("adds field to one group", func(t *testing.T) {
		// Arrange
		g := &groupTree{}
		field := zap.String("key", "value")

		// Act
		g.add([]string{"request"}, field)

		// Assert
		require.Len(t, g.order, 1)
		assert.Equal(t, "request", g.order[0])

		require.Contains(t, g.groups, "request")

		child := g.groups["request"]
		require.NotNil(t, child)

		require.Len(t, child.fields, 1)
		assert.Equal(t, field.Key, child.fields[0].Key)
	})

	t.Run("adds field to nested groups", func(t *testing.T) {
		// Arrange
		g := &groupTree{}
		field := zap.String("key", "value")

		// Act
		g.add([]string{"http", "request"}, field)

		// Assert
		require.Equal(t, []string{"http"}, g.order)

		httpGroup := g.groups["http"]
		require.NotNil(t, httpGroup)

		require.Equal(t, []string{"request"}, httpGroup.order)

		requestGroup := httpGroup.groups["request"]
		require.NotNil(t, requestGroup)

		require.Len(t, requestGroup.fields, 1)
		assert.Equal(t, field.Key, requestGroup.fields[0].Key)
	})

	t.Run("preserves group order", func(t *testing.T) {
		// Arrange
		g := &groupTree{}

		// Act
		g.add([]string{"first"}, zap.String("key", "1"))
		g.add([]string{"second"}, zap.String("key", "2"))
		g.add([]string{"third"}, zap.String("key", "3"))

		// Assert
		assert.Equal(t, []string{
			"first",
			"second",
			"third",
		}, g.order)
	})

	t.Run("does not duplicate existing groups", func(t *testing.T) {
		// Arrange
		g := &groupTree{}

		// Act
		g.add([]string{"request"}, zap.String("first", "1"))
		g.add([]string{"request"}, zap.String("second", "2"))

		// Assert
		assert.Equal(t, []string{"request"}, g.order)
		assert.Len(t, g.groups, 1)

		requestGroup := g.groups["request"]
		require.NotNil(t, requestGroup)

		assert.Len(t, requestGroup.fields, 2)
		assert.Equal(t, "first", requestGroup.fields[0].Key)
		assert.Equal(t, "second", requestGroup.fields[1].Key)
	})

	t.Run("preserves field order", func(t *testing.T) {
		// Arrange
		g := &groupTree{}

		// Act
		g.add(nil, zap.String("first", "1"))
		g.add(nil, zap.String("second", "2"))
		g.add(nil, zap.String("third", "3"))

		// Assert
		require.Len(t, g.fields, 3)

		assert.Equal(t, "first", g.fields[0].Key)
		assert.Equal(t, "second", g.fields[1].Key)
		assert.Equal(t, "third", g.fields[2].Key)
	})
}

func TestGroupTree_ToFields(t *testing.T) {
	t.Run("empty tree", func(t *testing.T) {
		// Arrange
		g := &groupTree{}

		// Act
		got := g.toFields()

		// Assert
		assert.Nil(t, got)
	})

	t.Run("root fields", func(t *testing.T) {
		// Arrange
		g := &groupTree{}
		g.add(nil, zap.String("request_id", "123"))
		g.add(nil, zap.Int("attempt", 2))

		// Act
		got := g.toFields()

		// Assert
		assert.Len(t, got, 2)

		encoded := encodeFields(t, got)

		assert.Equal(t, map[string]any{
			"request_id": "123",
			"attempt":    int64(2),
		}, encoded)
	})

	t.Run("one group", func(t *testing.T) {
		// Arrange
		g := &groupTree{}
		g.add([]string{"request"}, zap.String("method", "GET"))
		g.add([]string{"request"}, zap.String("path", "/users"))

		// Act
		got := g.toFields()

		// Assert
		encoded := encodeFields(t, got)

		assert.Equal(t, map[string]any{
			"request": map[string]any{
				"method": "GET",
				"path":   "/users",
			},
		}, encoded)
	})

	t.Run("nested groups", func(t *testing.T) {
		// Arrange
		g := &groupTree{}

		g.add(
			[]string{"http", "request"},
			zap.String("method", "GET"),
		)
		g.add(
			[]string{"http", "request"},
			zap.String("path", "/users"),
		)
		g.add(
			[]string{"http", "response"},
			zap.Int("status", 200),
		)

		// Act
		got := g.toFields()

		// Assert
		encoded := encodeFields(t, got)

		assert.Equal(t, map[string]any{
			"http": map[string]any{
				"request": map[string]any{
					"method": "GET",
					"path":   "/users",
				},
				"response": map[string]any{
					"status": int64(200),
				},
			},
		}, encoded)
	})

	t.Run("root fields and groups", func(t *testing.T) {
		// Arrange
		g := &groupTree{}

		g.add(nil, zap.String("request_id", "123"))
		g.add([]string{"request"}, zap.String("method", "GET"))
		g.add(nil, zap.String("level", "info"))

		// Act
		got := g.toFields()

		// Assert
		// Root fields go first, groups are appended afterwards.
		require.Len(t, got, 3)

		encoded := encodeFields(t, got)

		assert.Equal(t, map[string]any{
			"request_id": "123",
			"level":      "info",
			"request": map[string]any{
				"method": "GET",
			},
		}, encoded)
	})

	t.Run("skips empty groups", func(t *testing.T) {
		// Arrange
		g := &groupTree{
			order: []string{"empty", "request"},
			groups: map[string]*groupTree{
				"empty": {},
				"request": {
					fields: []zapcore.Field{zap.String("method", "GET")},
				},
			},
		}

		// Act
		got := g.toFields()

		// Assert
		encoded := encodeFields(t, got)

		assert.Equal(t, map[string]any{
			"request": map[string]any{
				"method": "GET",
			},
		}, encoded)
	})
}

func TestFieldsMarshaler_MarshalLogObject(t *testing.T) {
	// Arrange
	fields := fieldsMarshaler{
		zap.String("request_id", "123"),
		zap.Int("attempt", 2),
		zap.Bool("cached", true),
	}

	enc := zapcore.NewMapObjectEncoder()

	// Act
	err := fields.MarshalLogObject(enc)

	// Assert
	require.NoError(t, err)

	assert.Equal(t, map[string]any{
		"request_id": "123",
		"attempt":    int64(2),
		"cached":     true,
	}, enc.Fields)
}

func encodeFields(t *testing.T, fields []zapcore.Field) map[string]any {
	t.Helper()
	enc := zapcore.NewMapObjectEncoder()

	for _, field := range fields {
		field.AddTo(enc)
	}
	return enc.Fields
}
