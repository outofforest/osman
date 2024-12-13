package parser

import (
	"github.com/pkg/errors"

	"github.com/outofforest/osman/specfile/stack"
)

// LocationError gives a location in source code that caused the error.
type LocationError struct {
	Location []Range
	error
}

// Unwrap unwraps to the next error.
func (e *LocationError) Unwrap() error {
	return e.error
}

// Range is a code section between two positions.
type Range struct {
	Start Position
	End   Position
}

// Position is a point in source code.
type Position struct {
	Line      int
	Character int
}

func withLocation(err error, start, end int) error {
	return WithLocation(err, toRanges(start, end))
}

// WithLocation extends an error with a source code location.
func WithLocation(err error, location []Range) error {
	if err == nil {
		return nil
	}
	var el *LocationError
	if errors.As(err, &el) {
		return err
	}
	return stack.Enable(&LocationError{
		error:    err,
		Location: location,
	})
}

func toRanges(start, end int) []Range {
	r := []Range{}
	if end <= start {
		end = start
	}
	for i := start; i <= end; i++ {
		r = append(r, Range{Start: Position{Line: i}, End: Position{Line: i}})
	}
	return r
}
