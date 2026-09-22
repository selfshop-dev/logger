package logger

import (
	"log/slog"
	"time"
)

// Config defines the logger configuration.
//
// MinLevel specifies the minimum log level that is enabled.
//
// Format controls the output format. Use [FormatAuto] to select the format
// based on the development flag passed to [Config.ResolveFormat].
//
// Caller controls caller information attached to log records.
//
// StackAt specifies the minimum log level at which stack traces are captured.
// A nil value disables stack trace capture.
//
// Sampler controls log sampling for records that are eligible for sampling.
type Config struct {
	MinLevel slog.Level    `json:"min_level" yaml:"min_level" koanf:"min_level"`
	Format   Format        `json:"format"    yaml:"format"    koanf:"format"`
	Caller   CallerConfig  `json:"caller"    yaml:"caller"    koanf:"caller"`
	StackAt  *slog.Level   `json:"stack_at"  yaml:"stack_at"  koanf:"stack_at"`
	Sampler  SamplerConfig `json:"sampler"   yaml:"sampler"   koanf:"sampler"`
}

// SamplerConfig defines log sampling parameters.
//
// AlwaysLevel specifies a level at or above which records are always emitted
// without sampling. A nil value means that no log level is explicitly
// excluded from sampling by this setting.
//
// Tick defines the time interval over which the sampling counters are tracked.
//
// First specifies how many records with the same sampling key are emitted
// immediately at the beginning of each sampling interval.
//
// Thereafter specifies how many additional records are emitted for every
// subsequent group of matching records after First has been reached.
//
// The exact sampling key and the treatment of individual fields are defined
// by the sampler implementation.
type SamplerConfig struct {
	AlwaysLevel *slog.Level   `json:"always_level" yaml:"always_level" koanf:"always_level"`
	Tick        time.Duration `json:"tick"         yaml:"tick"         koanf:"tick"`
	First       int           `json:"first"        yaml:"first"        koanf:"first"`
	Thereafter  int           `json:"thereafter"   yaml:"thereafter"   koanf:"thereafter"`
}

// CallerConfig controls caller information attached to log records.
//
// AddSkip specifies the number of additional caller stack frames to skip.
// This is useful when logging is performed through wrapper functions.
//
// Disabled disables caller information entirely when set to true.
type CallerConfig struct {
	AddSkip  int  `json:"add_skip" yaml:"add_skip" koanf:"add_skip"`
	Disabled bool `json:"disabled" yaml:"disabled" koanf:"disabled"`
}

// ResolveFormat resolves the effective log format for the given environment.
//
// If Format is [FormatConsole], ResolveFormat always returns FormatConsole.
//
// If Format is [FormatAuto], ResolveFormat returns FormatConsole when
// development is true and FormatJSON otherwise.
//
// Any other Format value, including the zero value, resolves to FormatJSON.
// This makes JSON the fallback format for unknown or unset configuration
// values.
func (c Config) ResolveFormat(development bool) Format {
	switch c.Format {
	case FormatConsole:
		return FormatConsole
	case FormatAuto:
		if development {
			return FormatConsole
		}
		return FormatJSON
	default:
		return FormatJSON
	}
}
