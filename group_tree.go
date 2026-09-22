package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type groupTree struct {
	order  []string
	fields []zapcore.Field
	groups map[string]*groupTree
}

func (g *groupTree) add(path []string, f zapcore.Field) {
	for _, name := range path {
		if g.groups == nil {
			g.groups = make(map[string]*groupTree)
		}
		child, ok := g.groups[name]
		if !ok {
			child = &groupTree{}
			g.groups[name] = child
			g.order = append(g.order, name)
		}
		g = child
	}
	g.fields = append(g.fields, f)
}

func (g *groupTree) toFields() []zapcore.Field {
	out := make(
		[]zapcore.Field,
		len(g.fields),
		len(g.fields)+len(g.order),
	)
	copy(out, g.fields)

	for _, name := range g.order {
		fields := g.groups[name].toFields()
		if len(fields) == 0 {
			continue
		}
		out = append(
			out,
			zap.Object(name, fieldsMarshaler(fields)),
		)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

type fieldsMarshaler []zapcore.Field

func (f fieldsMarshaler) MarshalLogObject(enc zapcore.ObjectEncoder) error {
	for i := range f {
		f[i].AddTo(enc)
	}
	return nil
}
