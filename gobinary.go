// Package gobinary converts Go values to and from the little endian binary
// layout described by struct tags, for example `binary:"length:16,zeroTerminated"`.
// See Tag for the available options.
package gobinary

import (
	"fmt"
	"reflect"
)

// tagKey is the struct tag key the package reads.
const tagKey = "binary"

// Decode reads a record from the start of buf into v, which has to be a non nil
// pointer. It returns the number of bytes consumed.
func Decode(buf []byte, v any) (int, error) {
	rv := reflect.ValueOf(v)
	if !rv.IsValid() || rv.Kind() != reflect.Pointer || rv.IsNil() {
		return 0, fmt.Errorf("%w: Decode requires a non nil pointer, got %T", ErrUnsupportedType, v)
	}

	return decode(buf, rv.Elem(), Tag{})
}

// Encode serialises v. A struct is copied first, because a length field is
// filled in from the data it describes, so the caller's value stays untouched.
func Encode(v any) ([]byte, error) {
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Struct && !rv.CanAddr() {
		copied := reflect.New(rv.Type()).Elem()
		copied.Set(rv)
		rv = copied
	}

	return encode(rv, Tag{})
}

// Size returns the number of bytes Encode occupies for v, so a caller can
// pre-allocate a buffer. The size is taken from the value.
func Size(v any) int {
	if v == nil {
		return 0
	}
	return sizeOf(reflect.ValueOf(v), Tag{})
}
