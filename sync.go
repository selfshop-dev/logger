package logger

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"syscall"

	"go.uber.org/zap/zapcore"
)

// SafeSync flushes the supplied write syncer and suppresses errors that
// indicate that the underlying output does not support synchronization.
//
// A nil write syncer is treated as a no-op and returns nil.
//
// Some output destinations, most notably terminals and standard output/error
// streams, may return EINVAL, ENOTTY, or equivalent platform-specific
// "invalid argument"/"inappropriate ioctl" errors from Sync even though the
// output itself is usable. SafeSync treats these errors as non-fatal.
//
// All other errors are returned wrapped with the "sync:" prefix. Unexpected
// errors that are not [os.PathError] values are wrapped as
// "sync: unexpected error: ...".
//
// SafeSync is intended for shutdown or flush paths where synchronization is
// desirable but must not fail merely because the output destination does not
// implement Sync in the conventional filesystem sense.
func SafeSync(out zapcore.WriteSyncer) error {
	if out == nil {
		return nil
	}

	err := out.Sync()
	if err == nil {
		return nil
	}

	var pe *os.PathError
	if !errors.As(err, &pe) {
		return fmt.Errorf("sync: unexpected error: %w", err)
	}

	switch {
	case errors.Is(pe.Err, syscall.EINVAL),
		errors.Is(pe.Err, syscall.ENOTTY):
		return nil
	}

	msg := strings.ToLower(pe.Err.Error())
	if strings.Contains(msg, "invalid argument") ||
		strings.Contains(msg, "inappropriate ioctl") {
		return nil
	}
	return fmt.Errorf("sync: %w", err)
}
