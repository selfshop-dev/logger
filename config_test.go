package logger_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/selfshop-dev/logger"
)

func TestConfig_ResolveFormat(t *testing.T) {
	// Arrange
	testCases := [...]struct {
		name        string
		format      logger.Format
		development bool
		want        logger.Format
	}{
		{
			name:   "console",
			format: logger.FormatConsole,
			want:   logger.FormatConsole,
		},
		{
			name:        "auto in development",
			format:      logger.FormatAuto,
			development: true,
			want:        logger.FormatConsole,
		},
		{
			name:   "auto in production",
			format: logger.FormatAuto,
			want:   logger.FormatJSON,
		},
		{
			name:        "json in development",
			format:      logger.FormatJSON,
			development: true,
			want:        logger.FormatJSON,
		},
		{
			name:   "unknown",
			format: logger.Format(100),
			want:   logger.FormatJSON,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := (logger.Config{Format: tc.format}).ResolveFormat(tc.development)

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}
