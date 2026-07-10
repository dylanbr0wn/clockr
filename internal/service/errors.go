package service

import (
	"errors"
	"fmt"
)

var (
	ErrInvalidInput       = errors.New("invalid input")
	ErrFailedPrecondition = errors.New("failed precondition")
)

func invalidInputf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrInvalidInput, fmt.Sprintf(format, args...))
}

func failedPreconditionf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", ErrFailedPrecondition, fmt.Sprintf(format, args...))
}
