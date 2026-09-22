package logger_test

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/selfshop-dev/ctxval"

	"github.com/selfshop-dev/logger"
)

func ExampleNew() {
	var buf bytes.Buffer

	l, _, out := logger.New(
		logger.Config{
			Format: logger.FormatJSON,
			Caller: logger.CallerConfig{Disabled: true},
		},
		logger.WithWriter(&buf),
	)
	defer func() { _ = logger.SafeSync(out) }()

	l.Info("server started", "addr", ":8080")

	line := buf.String()
	fmt.Println(strings.Contains(line, `"message":"server started"`))
	fmt.Println(strings.Contains(line, `"addr":":8080"`))

	// Output:
	// true
	// true
}

func ExampleIntoContext() {
	var buf bytes.Buffer

	l := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	ctx := logger.IntoContext(context.Background(), l)

	logger.FromContext(ctx).Info("request finished")

	fmt.Println(strings.Contains(buf.String(), "request finished"))

	// Output:
	// true
}

func ExampleExtractFrom() {
	type RequestID string

	var buf bytes.Buffer
	l, _, out := logger.New(
		logger.Config{
			Format: logger.FormatJSON,
			Caller: logger.CallerConfig{Disabled: true},
		},
		logger.WithWriter(&buf),
		logger.WithExtractors(logger.ExtractFrom[RequestID]("request_id")),
	)
	defer func() { _ = logger.SafeSync(out) }()

	ctx := ctxval.With(context.Background(), RequestID("req-42"))
	l.InfoContext(ctx, "request started")

	fmt.Println(strings.Contains(buf.String(), `"request_id":"req-42"`))

	// Output:
	// true
}

func ExampleNamed() {
	var buf bytes.Buffer

	l, _, out := logger.New(
		logger.Config{
			Format: logger.FormatJSON,
			Caller: logger.CallerConfig{Disabled: true},
		},
		logger.WithWriter(&buf),
	)
	defer func() { _ = logger.SafeSync(out) }()

	l = logger.Named(l, "http")
	l = logger.Named(l, "server")
	l.Info("listening")

	fmt.Println(strings.Contains(buf.String(), `"logger":"http.server"`))

	// Output:
	// true
}

func ExampleBeforeHook() {
	var buf bytes.Buffer

	l, _, out := logger.New(
		logger.Config{
			Format: logger.FormatJSON,
			Caller: logger.CallerConfig{Disabled: true},
		},
		logger.WithWriter(&buf),
		logger.WithHooks(
			logger.BeforeHook(func(_ context.Context, r *slog.Record) error {
				r.AddAttrs(slog.Bool("handled", true))
				return nil
			}),
		),
	)
	defer func() { _ = logger.SafeSync(out) }()

	l.Info("processed")

	fmt.Println(strings.Contains(buf.String(), `"handled":true`))

	// Output:
	// true
}
