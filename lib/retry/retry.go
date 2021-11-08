package retry

import (
	"context"
	"errors"
	"time"

	"github.com/wojciech-malota-wojcik/logger"
	"go.uber.org/zap"
)

// Retryable returns retryable error
func Retryable(err error) error {
	if err == nil {
		return nil
	}
	return ErrRetryable{err: err}
}

// ErrRetryable represents retryable error
type ErrRetryable struct {
	err error
}

// Error returns string representation of error
func (e ErrRetryable) Error() string {
	return e.err.Error()
}

// Unwrap returns next error
func (e ErrRetryable) Unwrap() error {
	return e.err
}

// Do retries running function until it returns non-retryable error
func Do(ctx context.Context, retryAfter time.Duration, fn func() error) error {
	log := logger.Get(ctx)
	var lastMessage string
	var r ErrRetryable
	for {
		if err := fn(); !errors.As(err, &r) {
			return err
		}
		if errors.Is(r.err, ctx.Err()) {
			return r.err
		}

		newMessage := r.err.Error()
		if lastMessage != newMessage {
			log.Info("Will retry", zap.Error(r.err))
			lastMessage = newMessage
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(retryAfter):
		}
	}
}
