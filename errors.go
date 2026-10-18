package gobinary

import (
	"errors"
	"fmt"
)

var (
	// ErrUnsupportedType is returned when a field has a Go type that cannot be
	// represented on the wire.
	ErrUnsupportedType = errors.New("unsupported field type")
	// ErrTruncated is returned when a buffer ends in the middle of a field.
	ErrTruncated = errors.New("unexpected end of buffer")
	// ErrInvalidTag is returned when a struct tag is malformed.
	ErrInvalidTag = errors.New("invalid struct tag")
)

func errTag(msg string) error {
	return fmt.Errorf("%w: %s", ErrInvalidTag, msg)
}
