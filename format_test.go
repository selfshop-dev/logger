package logger_test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/selfshop-dev/logger"
)

func TestParseFormat(t *testing.T) {
	// Arrange
	testCases := [...]struct {
		name    string
		text    string
		want    logger.Format
		wantErr string
	}{
		{
			name: "auto",
			text: "auto",
			want: logger.FormatAuto,
		},
		{
			name: "json",
			text: "json",
			want: logger.FormatJSON,
		},
		{
			name: "console",
			text: "console",
			want: logger.FormatConsole,
		},
		{
			name: "empty",
			text: "",
			want: logger.FormatJSON,
		},
		{
			name: "case insensitive",
			text: "JSON",
			want: logger.FormatJSON,
		},
		{
			name:    "invalid",
			text:    "invalid",
			wantErr: `unknown format "invalid"`,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, err := logger.ParseFormat(tc.text)

			// Assert
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFormat_String(t *testing.T) {
	// Arrange
	testCases := [...]struct {
		name string
		f    logger.Format
		want string
	}{
		{
			name: "auto",
			f:    logger.FormatAuto,
			want: "auto",
		},
		{
			name: "json",
			f:    logger.FormatJSON,
			want: "json",
		},
		{
			name: "console",
			f:    logger.FormatConsole,
			want: "console",
		},
		{
			name: "unknown",
			f:    logger.Format(100),
			want: "unknown(100)",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got := tc.f.String()

			// Assert
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFormat_Set(t *testing.T) {
	// Arrange
	var f logger.Format

	// Act
	err := f.Set("console")

	// Assert
	require.NoError(t, err)
	assert.Equal(t, logger.FormatConsole, f)
}

func TestFormat_Get(t *testing.T) {
	// Arrange
	f := logger.FormatConsole

	// Act
	got := f.Get()

	// Assert
	assert.Equal(t, f, got)
}

func TestFormat_MarshalText(t *testing.T) {
	// Arrange
	testCases := [...]struct {
		name    string
		f       logger.Format
		want    []byte
		wantErr string
	}{
		{
			name: "auto",
			f:    logger.FormatAuto,
			want: []byte("auto"),
		},
		{
			name: "json",
			f:    logger.FormatJSON,
			want: []byte("json"),
		},
		{
			name: "console",
			f:    logger.FormatConsole,
			want: []byte("console"),
		},
		{
			name:    "invalid",
			f:       logger.Format(100),
			wantErr: "invalid format 100",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Act
			got, err := tc.f.MarshalText()

			// Assert
			if tc.wantErr != "" {
				require.EqualError(t, err, tc.wantErr)
				assert.Nil(t, got)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.want, got)
		})
	}
}

func TestFormat_UnmarshalText(t *testing.T) {
	// Arrange
	testCases := [...]struct {
		name string
		text string
		want logger.Format
	}{
		{
			name: "auto",
			text: "auto",
			want: logger.FormatAuto,
		},
		{
			name: "json",
			text: "json",
			want: logger.FormatJSON,
		},
		{
			name: "console",
			text: "console",
			want: logger.FormatConsole,
		},
		{
			name: "empty",
			text: "",
			want: logger.FormatJSON,
		},
		{
			name: "upper case",
			text: "CONSOLE",
			want: logger.FormatConsole,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange
			var f logger.Format

			// Act
			err := f.UnmarshalText([]byte(tc.text))

			// Assert
			require.NoError(t, err)
			assert.Equal(t, tc.want, f)
		})
	}
}

func TestFormat_UnmarshalText_Invalid(t *testing.T) {
	// Arrange
	f := logger.FormatConsole

	// Act
	err := f.UnmarshalText([]byte("invalid"))

	// Assert
	require.EqualError(t, err, `unknown format "invalid"`)
	assert.Equal(t, logger.InvalidFormat, f)
}

func TestFormat_UnmarshalText_NilReceiver(t *testing.T) {
	// Arrange
	var f *logger.Format

	// Act
	err := f.UnmarshalText([]byte("json"))

	// Assert
	require.EqualError(t, err, "nil Format")
}

func TestFormat_JSONMarshal(t *testing.T) {
	// Arrange
	value := struct {
		Format logger.Format `json:"format"`
	}{
		Format: logger.FormatConsole,
	}

	// Act
	got, err := json.Marshal(value)

	// Assert
	require.NoError(t, err)
	assert.JSONEq(t, `{"format":"console"}`, string(got))
}

func TestFormat_Set_Invalid(t *testing.T) {
	// Arrange
	f := logger.FormatConsole

	// Act
	err := f.Set("invalid")

	// Assert
	require.EqualError(t, err, `unknown format "invalid"`)
	assert.Equal(t, logger.InvalidFormat, f)
}

func TestFormat_JSONUnmarshal(t *testing.T) {
	// Arrange
	input := []byte(`{"format":"console"}`)
	var value struct {
		Format logger.Format `json:"format"`
	}

	// Act
	err := json.Unmarshal(input, &value)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, logger.FormatConsole, value.Format)
}
