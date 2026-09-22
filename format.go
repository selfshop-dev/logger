package logger

import (
	"bytes"
	"fmt"
)

// Format specifies the output format used by the logger.
type Format int8

const (
	// FormatAuto selects the output format based on the current environment.
	// See [Config.ResolveFormat] for the resolution rules.
	FormatAuto Format = iota - 1

	// FormatJSON configures structured JSON log output.
	//
	// This is the zero value of Format and is also the fallback format for
	// unknown configuration values handled by [Config.ResolveFormat].
	FormatJSON

	// FormatConsole configures human-readable console log output.
	FormatConsole

	_minFormat = FormatAuto
	_maxFormat = FormatConsole

	// InvalidFormat is assigned internally when text cannot be parsed as a
	// valid Format value.
	//
	// InvalidFormat is not a valid value for configuration and cannot be
	// serialized by [Format.MarshalText].
	InvalidFormat = _maxFormat + 1
)

// ParseFormat parses text into a [Format].
//
// The accepted values are "auto", "json", and "console". Parsing is
// case-insensitive, so values such as "JSON" and "Console" are accepted.
//
// An empty string is treated as "json".
//
// ParseFormat returns an error when text does not represent a valid format.
func ParseFormat(text string) (Format, error) {
	var f Format
	err := f.UnmarshalText([]byte(text))
	return f, err
}

// String returns the textual representation of f.
//
// Valid formats are represented as "auto", "json", or "console".
//
// For an invalid Format value, String returns "unknown(N)", where N is the
// underlying integer value.
func (f Format) String() string {
	switch f {
	case FormatAuto:
		return "auto"
	case FormatJSON:
		return "json"
	case FormatConsole:
		return "console"
	default:
		return fmt.Sprintf("unknown(%d)", int(f))
	}
}

// Set implements a string-based configuration setter.
//
// It has the same parsing semantics as [Format.UnmarshalText].
func (f *Format) Set(s string) error {
	return f.UnmarshalText([]byte(s))
}

// Get returns f as an [any] value.
//
// This method is intended for configuration libraries that use a generic
// getter interface.
func (f Format) Get() any {
	return f
}

// MarshalText implements [encoding.TextMarshaler].
//
// It returns the canonical textual representation of a valid Format:
// "auto", "json", or "console".
//
// Invalid Format values result in an error and are not serialized.
func (f Format) MarshalText() ([]byte, error) {
	if f < _minFormat ||
		f > _maxFormat {
		return nil, fmt.Errorf("invalid format %d", int(f))
	}
	return []byte(f.String()), nil
}

// UnmarshalText implements [encoding.TextUnmarshaler].
//
// It accepts "auto", "json", and "console" as valid values. Parsing is
// case-insensitive.
//
// An empty string is treated as FormatJSON.
//
// If text does not represent a valid format, UnmarshalText returns an error
// and sets f to [InvalidFormat].
//
// Calling UnmarshalText on a nil *Format returns an error.
func (f *Format) UnmarshalText(text []byte) error {
	if f == nil {
		return fmt.Errorf("nil Format")
	}
	if f.unmarshalText(text) ||
		f.unmarshalText(bytes.ToLower(text)) {
		return nil
	}
	return fmt.Errorf("unknown format %q", text)
}

func (f *Format) unmarshalText(text []byte) bool {
	switch string(text) {
	case "auto":
		*f = FormatAuto
	case "json", "": // allow empty string
		*f = FormatJSON
	case "console":
		*f = FormatConsole
	default:
		*f = InvalidFormat
		return false
	}
	return true
}
