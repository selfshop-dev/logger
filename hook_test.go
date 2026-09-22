package logger

import (
	"context"
	"errors"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHookFunc_Before(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		var fn HookFunc
		err := fn.Before(context.Background(), nil)
		require.NoError(t, err)
	})

	t.Run("calls function", func(t *testing.T) {
		type testKey struct{}

		ctx := context.WithValue(context.Background(), testKey{}, "test-value")
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		var called bool
		fn := HookFunc(func(gotCtx context.Context, gotRecord *slog.Record) error {
			called = true
			assert.Equal(t, ctx, gotCtx)
			assert.Equal(t, "test", gotRecord.Message)
			return nil
		})

		err := fn.Before(ctx, &record)
		require.NoError(t, err)
		assert.True(t, called)
	})

	t.Run("returns error", func(t *testing.T) {
		wantErr := errors.New("hook failed")
		fn := HookFunc(func(context.Context, *slog.Record) error {
			return wantErr
		})

		err := fn.Before(context.Background(), nil)
		require.ErrorIs(t, err, wantErr)
	})
}

func TestHookFunc_After(t *testing.T) {
	var fn HookFunc
	ctx := context.Background()
	record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

	fn.After(ctx, record, errors.New("error"))
}

func TestBeforeHook(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		got := BeforeHook(nil)
		assert.Nil(t, got)
	})

	t.Run("returns hook", func(t *testing.T) {
		called := false
		fn := func(context.Context, *slog.Record) error {
			called = true
			return nil
		}

		got := BeforeHook(fn)
		require.NotNil(t, got)

		err := got.Before(context.Background(), nil)
		require.NoError(t, err)
		assert.True(t, called)
	})
}

func TestAfterHook_Before(t *testing.T) {
	h := afterHook{
		fn: func(context.Context, slog.Record, error) {},
	}
	err := h.Before(context.Background(), nil)
	require.NoError(t, err)
}

func TestAfterHook_After(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		h := afterHook{}
		h.After(
			context.Background(),
			slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0),
			nil,
		)
	})

	t.Run("calls function", func(t *testing.T) {
		type testKey struct{}

		ctx := context.WithValue(context.Background(), testKey{}, "test-value")
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		wantErr := errors.New("some error")

		var (
			called    bool
			gotCtx    context.Context
			gotRecord slog.Record
			gotErr    error
		)

		h := afterHook{
			fn: func(ctx context.Context, r slog.Record, err error) {
				called = true
				gotCtx = ctx
				gotRecord = r
				gotErr = err
			},
		}

		h.After(ctx, record, wantErr)

		require.True(t, called)
		assert.Equal(t, ctx, gotCtx)
		assert.Equal(t, record.Message, gotRecord.Message)
		assert.Equal(t, wantErr, gotErr)
	})
}

func TestAfterHook(t *testing.T) {
	t.Run("nil", func(t *testing.T) {
		got := AfterHook(nil)
		assert.Nil(t, got)
	})

	t.Run("returns hook", func(t *testing.T) {
		called := false
		fn := func(context.Context, slog.Record, error) {
			called = true
		}

		got := AfterHook(fn)
		require.NotNil(t, got)

		got.After(
			context.Background(),
			slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0),
			nil,
		)
		assert.True(t, called)
	})
}

func TestHooks_RunBefore(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		hs := hooks{}
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		executed, err := hs.RunBefore(context.Background(), &record)

		require.NoError(t, err)
		assert.Empty(t, executed)
	})

	t.Run("executes all hooks", func(t *testing.T) {
		var order []int

		hooksList := hooks{
			HookFunc(func(context.Context, *slog.Record) error {
				order = append(order, 1)
				return nil
			}),
			HookFunc(func(context.Context, *slog.Record) error {
				order = append(order, 2)
				return nil
			}),
			HookFunc(func(context.Context, *slog.Record) error {
				order = append(order, 3)
				return nil
			}),
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		executed, err := hooksList.RunBefore(context.Background(), &record)

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2, 3}, order)
		assert.Len(t, executed, 3)
	})

	t.Run("stops on error", func(t *testing.T) {
		wantErr := errors.New("hook failed")
		var order []int

		hooksList := hooks{
			HookFunc(func(context.Context, *slog.Record) error {
				order = append(order, 1)
				return nil
			}),
			HookFunc(func(context.Context, *slog.Record) error {
				order = append(order, 2)
				return wantErr
			}),
			HookFunc(func(context.Context, *slog.Record) error {
				order = append(order, 3)
				return nil
			}),
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		executed, err := hooksList.RunBefore(context.Background(), &record)

		require.ErrorIs(t, err, wantErr)
		assert.Equal(t, []int{1, 2}, order)
		assert.Len(t, executed, 1)
	})

	t.Run("skips nil hooks", func(t *testing.T) {
		var order []int

		hooksList := hooks{
			HookFunc(func(context.Context, *slog.Record) error {
				order = append(order, 1)
				return nil
			}),
			HookFunc(func(context.Context, *slog.Record) error {
				order = append(order, 2)
				return nil
			}),
			nil,
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		executed, err := hooksList.RunBefore(context.Background(), &record)

		require.NoError(t, err)
		assert.Equal(t, []int{1, 2}, order)
		assert.Len(t, executed, 2)
	})

	t.Run("nil hooks only", func(t *testing.T) {
		hooksList := hooks{nil, nil, nil}
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		executed, err := hooksList.RunBefore(context.Background(), &record)

		require.NoError(t, err)
		assert.Empty(t, executed)
	})
}

func TestHooks_RunAfter(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		hs := hooks{}
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		hs.RunAfter(context.Background(), record, nil)
	})

	t.Run("executes in reverse order", func(t *testing.T) {
		var order []int

		hooksList := hooks{
			AfterHook(func(context.Context, slog.Record, error) {
				order = append(order, 1)
			}),
			AfterHook(func(context.Context, slog.Record, error) {
				order = append(order, 2)
			}),
			AfterHook(func(context.Context, slog.Record, error) {
				order = append(order, 3)
			}),
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		hooksList.RunAfter(context.Background(), record, nil)

		assert.Equal(t, []int{3, 2, 1}, order)
	})

	t.Run("passes error", func(t *testing.T) {
		wantErr := errors.New("original error")
		var gotErr error

		hooksList := hooks{
			AfterHook(func(_ context.Context, _ slog.Record, err error) {
				gotErr = err
			}),
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		hooksList.RunAfter(context.Background(), record, wantErr)

		assert.Equal(t, wantErr, gotErr)
	})

	t.Run("skips nil hooks", func(t *testing.T) {
		var order []int

		hooksList := hooks{
			nil,
			AfterHook(func(context.Context, slog.Record, error) {
				order = append(order, 1)
			}),
			nil,
			AfterHook(func(context.Context, slog.Record, error) {
				order = append(order, 2)
			}),
			nil,
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)
		hooksList.RunAfter(context.Background(), record, nil)

		assert.Equal(t, []int{2, 1}, order)
	})

	t.Run("nil hooks only", func(t *testing.T) {
		hooksList := hooks{nil, nil, nil}
		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		hooksList.RunAfter(context.Background(), record, nil)
	})

	t.Run("mixed before and after hooks", func(t *testing.T) {
		var order []string

		hooksList := hooks{
			BeforeHook(func(context.Context, *slog.Record) error {
				order = append(order, "before-1")
				return nil
			}),
			AfterHook(func(context.Context, slog.Record, error) {
				order = append(order, "after-1")
			}),
			BeforeHook(func(context.Context, *slog.Record) error {
				order = append(order, "before-2")
				return nil
			}),
			AfterHook(func(context.Context, slog.Record, error) {
				order = append(order, "after-2")
			}),
		}

		record := slog.NewRecord(time.Time{}, slog.LevelInfo, "test", 0)

		executed, err := hooksList.RunBefore(context.Background(), &record)
		require.NoError(t, err)
		assert.Equal(t, []string{"before-1", "before-2"}, order)

		order = nil
		executed.RunAfter(context.Background(), record, nil)
		assert.Equal(t, []string{"after-2", "after-1"}, order)
	})
}
