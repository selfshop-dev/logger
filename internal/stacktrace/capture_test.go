package stacktrace

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCapture(t *testing.T) {
	t.Run("captures caller", func(t *testing.T) {
		// Act
		stack := captureDirect()
		defer stack.Free()

		// Assert
		require.Positive(t, stack.Count(), 0)

		frame, more := stack.Next()

		assert.Contains(t, frame.Function, "captureDirect")
		assert.NotEmpty(t, frame.File)
		assert.NotZero(t, frame.Line)
		assert.NotZero(t, frame.PC)
		assert.True(t, more)
	})

	t.Run("skip skips caller", func(t *testing.T) {
		// Act
		stack := captureWithSkip()
		defer stack.Free()

		// Assert
		require.Positive(t, stack.Count(), 0)

		frame, _ := stack.Next()

		assert.Contains(t, frame.Function, "TestCapture")
		assert.NotContains(t, frame.Function, "captureWithSkip")
	})

	t.Run("captures deep stack", func(t *testing.T) {
		// Arrange
		const depth = initialDepth + 32

		// Act
		stack := captureDeep(depth)
		defer stack.Free()

		// Assert
		assert.Greater(t, stack.Count(), initialDepth)
	})

	t.Run("next iterates all frames", func(t *testing.T) {
		// Arrange
		stack := captureDirect()
		defer stack.Free()

		wantCount := stack.Count()

		// Act
		var gotCount int

		for {
			_, more := stack.Next()
			gotCount++

			if !more {
				break
			}
		}

		// Assert
		assert.Equal(t, wantCount, gotCount)

		frame, more := stack.Next()

		assert.False(t, more)
		assert.Equal(t, runtime.Frame{}, frame)
		assert.Equal(t, wantCount, stack.Count())
	})
}

func TestStack_Free(t *testing.T) {
	// Arrange
	stack := captureDirect()
	require.NotNil(t, stack)

	storageLen := len(stack.storage)

	// Act
	stack.Free()

	// Assert
	assert.Nil(t, stack.pcs)
	assert.Nil(t, stack.frames)
	assert.Len(t, storageLen, len(stack.storage))
}

func captureDirect() *stack {
	return Capture(0)
}

func captureWithSkip() *stack {
	return Capture(1)
}

func captureDeep(depth int) *stack {
	if depth == 0 {
		return Capture(0)
	}

	return captureDeep(depth - 1)
}
