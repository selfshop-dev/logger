package stacktrace

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTake(t *testing.T) {
	t.Run("returns formatted stack", func(t *testing.T) {
		// Act
		got := Take(0)

		// Assert
		require.NotEmpty(t, got)

		lines := strings.Split(got, "\n")
		require.GreaterOrEqual(t, len(lines), 2)

		assert.Contains(t, lines[0], "TestTake")
		assert.True(t, strings.HasPrefix(lines[1], "\t"))
		assert.Contains(t, got, ".go:")
	})

	t.Run("first frame is included", func(t *testing.T) {
		// Act
		got := takeFromHelper()

		// Assert
		require.NotEmpty(t, got)

		lines := strings.Split(got, "\n")
		require.GreaterOrEqual(t, len(lines), 2)

		assert.Contains(t, lines[0], "takeFromHelper")
	})

	t.Run("final frame is excluded", func(t *testing.T) {
		// Act
		got := Take(0)

		// Assert
		require.NotEmpty(t, got)

		assert.NotContains(t, got, "runtime.goexit")
	})

	t.Run("different skip changes first frame", func(t *testing.T) {
		// Arrange
		got0 := Take(0)
		got1 := Take(1)

		// Act
		first0 := strings.Split(got0, "\n")[0]
		first1 := strings.Split(got1, "\n")[0]

		// Assert
		assert.NotEqual(t, first0, first1)
		assert.Contains(t, first0, "TestTake")
	})
}

func TestTake_EmptyStack(t *testing.T) {
	// Arrange
	const skip = 1_000_000

	// Act
	got := Take(skip)

	// Assert
	assert.Empty(t, got)
}

func takeFromHelper() string {
	return Take(0)
}
