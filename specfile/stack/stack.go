package stack

import (
	"github.com/pkg/errors"
)

// Enable adds stack to error
func Enable(err error) error {
	if err == nil {
		return nil
	}
	if !hasLocalStackTrace(err) {
		return errors.WithStack(err)
	}
	return err
}

func hasLocalStackTrace(err error) bool {
	wrapped, ok := err.(interface {
		Unwrap() error
	})
	if ok && hasLocalStackTrace(wrapped.Unwrap()) {
		return true
	}

	_, ok = err.(interface {
		StackTrace() errors.StackTrace
	})
	return ok
}
