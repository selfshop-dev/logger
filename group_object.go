package logger

import (
	"fmt"
	"log/slog"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func convertAttrToField(a slog.Attr) zapcore.Field {
	switch a.Value.Kind() {
	case slog.KindString:
		return zap.String(a.Key, a.Value.String())
	case slog.KindInt64:
		return zap.Int64(a.Key, a.Value.Int64())
	case slog.KindUint64:
		return zap.Uint64(a.Key, a.Value.Uint64())
	case slog.KindFloat64:
		return zap.Float64(a.Key, a.Value.Float64())
	case slog.KindBool:
		return zap.Bool(a.Key, a.Value.Bool())
	case slog.KindDuration:
		return zap.Duration(a.Key, a.Value.Duration())
	case slog.KindTime:
		return zap.Time(a.Key, a.Value.Time())
	case slog.KindAny:
		switch v := a.Value.Any().(type) {
		case error:
			// Match the standard slog handlers: a single field under
			// the attribute key. zap.NamedError would emit an extra
			// "<key>Error" field.
			return zap.String(a.Key, v.Error())
		case zapcore.ObjectMarshaler:
			return zap.Object(a.Key, v)
		case zapcore.ArrayMarshaler:
			return zap.Array(a.Key, v)
		case fmt.Stringer:
			return zap.Stringer(a.Key, v)
		default:
			return zap.Any(a.Key, v)
		}
	case slog.KindLogValuer:
		return convertAttrToField(slog.Attr{
			Key:   a.Key,
			Value: a.Value.Resolve(),
		})
	case slog.KindGroup:
		attrs := a.Value.Group()
		if len(attrs) == 0 {
			return zap.Skip()
		}
		if a.Key == "" {
			return zap.Inline(groupObject(attrs))
		}
		return zap.Object(a.Key, groupObject(attrs))
	default:
		// unreachable: all current slog.Kind values are handled above.
		// Kept as a safety net for future Go versions.
		return zap.Any(a.Key, a.Value.Any())
	}
}

type groupObject []slog.Attr

func (g groupObject) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	for _, a := range g {
		convertAttrToField(a).AddTo(enc)
	}
	return nil
}
