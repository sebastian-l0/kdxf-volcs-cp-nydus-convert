package ratelimit

import (
	"context"
	"testing"
	"time"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

func TestNewFixedIntervalLimiter(t *testing.T) {
	l, err := NewFixedIntervalLimiter(100)
	if err != nil {
		t.Fatal(err)
	}
	if l.Interval() != 600*time.Millisecond {
		t.Fatalf("interval=%s", l.Interval())
	}
	l, err = NewFixedIntervalLimiter(60)
	if err != nil {
		t.Fatal(err)
	}
	if l.Interval() != time.Second {
		t.Fatalf("interval=%s", l.Interval())
	}
	if _, err := NewFixedIntervalLimiter(0); apperrors.CodeOf(err) != apperrors.CodeInvalidConfig {
		t.Fatalf("qpm0 code=%q", apperrors.CodeOf(err))
	}
	if _, err := NewFixedIntervalLimiter(101); apperrors.CodeOf(err) != apperrors.CodeInvalidConfig {
		t.Fatalf("qpm101 code=%q", apperrors.CodeOf(err))
	}
}

func TestFixedIntervalLimiterWait(t *testing.T) {
	now := time.Unix(0, 0)
	var slept time.Duration
	l, err := NewFixedIntervalLimiterWithHooks(100, func() time.Time { return now }, func(ctx context.Context, d time.Duration) error {
		slept += d
		now = now.Add(d)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if waited, err := l.Wait(context.Background()); err != nil || waited != 0 {
		t.Fatalf("first waited=%s err=%v", waited, err)
	}
	if waited, err := l.Wait(context.Background()); err != nil || waited != 600*time.Millisecond {
		t.Fatalf("second waited=%s err=%v", waited, err)
	}
	if slept != 600*time.Millisecond {
		t.Fatalf("slept=%s", slept)
	}
}

func TestFixedIntervalLimiterCancel(t *testing.T) {
	now := time.Unix(0, 0)
	l, err := NewFixedIntervalLimiterWithHooks(100, func() time.Time { return now }, func(ctx context.Context, d time.Duration) error { return context.Canceled })
	if err != nil {
		t.Fatal(err)
	}
	_, _ = l.Wait(context.Background())
	_, err = l.Wait(context.Background())
	if apperrors.CodeOf(err) != apperrors.CodeRateLimitWaitCanceled {
		t.Fatalf("code=%q err=%v", apperrors.CodeOf(err), err)
	}
}
