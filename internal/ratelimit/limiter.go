package ratelimit

import (
	"context"
	"time"

	apperrors "github.com/sebastian-l0/kdxf-volcs-cp-nydus-convert/internal/errors"
)

type Limiter interface {
	Wait(ctx context.Context) (time.Duration, error)
}

type FixedIntervalLimiter struct {
	interval time.Duration
	last     time.Time
	now      func() time.Time
	sleep    func(context.Context, time.Duration) error
}

func NewFixedIntervalLimiter(qpm int) (*FixedIntervalLimiter, error) {
	if qpm < 1 || qpm > 100 {
		return nil, apperrors.New(apperrors.CodeInvalidConfig, "--run-pipeline-qpm must be between 1 and 100")
	}
	return &FixedIntervalLimiter{
		interval: time.Minute / time.Duration(qpm),
		now:      time.Now,
		sleep:    sleepContext,
	}, nil
}

func NewFixedIntervalLimiterWithHooks(qpm int, now func() time.Time, sleep func(context.Context, time.Duration) error) (*FixedIntervalLimiter, error) {
	l, err := NewFixedIntervalLimiter(qpm)
	if err != nil {
		return nil, err
	}
	if now != nil {
		l.now = now
	}
	if sleep != nil {
		l.sleep = sleep
	}
	return l, nil
}

func (l *FixedIntervalLimiter) Wait(ctx context.Context) (time.Duration, error) {
	if l == nil {
		return 0, nil
	}
	now := l.now()
	if l.last.IsZero() {
		l.last = now
		return 0, nil
	}
	next := l.last.Add(l.interval)
	wait := next.Sub(now)
	if wait > 0 {
		if err := l.sleep(ctx, wait); err != nil {
			return 0, apperrors.Wrap(apperrors.CodeRateLimitWaitCanceled, "rate limit wait canceled", err)
		}
		now = l.now()
	} else {
		wait = 0
	}
	l.last = now
	return wait, nil
}

func (l *FixedIntervalLimiter) Interval() time.Duration {
	if l == nil {
		return 0
	}
	return l.interval
}

func sleepContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
