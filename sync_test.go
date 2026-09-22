package logger_test

import (
	"errors"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/selfshop-dev/logger"
)

func TestSafeSync_Nil(t *testing.T) {
	// Act
	err := logger.SafeSync(nil)

	// Assert
	assert.NoError(t, err)
}

func TestSafeSync_Success(t *testing.T) {
	// Arrange
	out := testWriteSyncer{
		syncFn: func() error { return nil },
	}

	// Act
	err := logger.SafeSync(out)

	// Assert
	assert.NoError(t, err)
}

func TestSafeSync_UnexpectedError(t *testing.T) {
	// Arrange
	out := testWriteSyncer{
		syncFn: func() error { return errors.New("boom") },
	}

	// Act
	err := logger.SafeSync(out)

	// Assert
	require.EqualError(t, err, "sync: unexpected error: boom")
}

func TestSafeSync_IgnoredErrors(t *testing.T) {
	// Arrange
	testCases := [...]struct {
		name string
		err  error
	}{
		{
			name: "EINVAL",
			err: &os.PathError{
				Op:   "sync",
				Path: "stdout",
				Err:  syscall.EINVAL,
			},
		},
		{
			name: "ENOTTY",
			err: &os.PathError{
				Op:   "sync",
				Path: "stdout",
				Err:  syscall.ENOTTY,
			},
		},
		{
			name: "invalid argument",
			err: &os.PathError{
				Op:   "sync",
				Path: "stdout",
				Err:  errors.New("Invalid Argument"),
			},
		},
		{
			name: "inappropriate ioctl",
			err: &os.PathError{
				Op:   "sync",
				Path: "stdout",
				Err:  errors.New("Inappropriate Ioctl for Device"),
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			err := logger.SafeSync(testWriteSyncer{
				syncFn: func() error { return tc.err },
			})

			// Assert
			assert.NoError(t, err)
		})
	}
}

func TestSafeSync_OtherPathError(t *testing.T) {
	// Arrange
	want := &os.PathError{
		Op:   "sync",
		Path: "stdout",
		Err:  errors.New("permission denied"),
	}

	// Act
	err := logger.SafeSync(testWriteSyncer{
		syncFn: func() error { return want },
	})

	// Assert
	require.Error(t, err)
	assert.True(t, strings.HasPrefix(err.Error(), "sync:"))
	assert.ErrorIs(t, err, want)
}

type testWriteSyncer struct {
	syncFn func() error
}

func (testWriteSyncer) Write(p []byte) (int, error) {
	return len(p), nil
}

func (s testWriteSyncer) Sync() error {
	if s.syncFn == nil {
		return nil
	}
	return s.syncFn()
}
