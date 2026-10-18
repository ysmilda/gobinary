package gobinary

import (
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"time"
)

// BinaryEncoder is implemented by types that encode themselves. It overrides
// the reflection based encoding of the type.
type BinaryEncoder interface {
	BinaryEncode() ([]byte, error)
}

// encoderOf returns v as a BinaryEncoder, if it or its pointer implements it.
func encoderOf(v reflect.Value) (BinaryEncoder, bool) {
	t := v.Type()
	if !canEncode(t) {
		return nil, false
	}
	if v.CanInterface() {
		if e, ok := v.Interface().(BinaryEncoder); ok {
			return e, true
		}
	}
	if v.CanAddr() {
		if e, ok := v.Addr().Interface().(BinaryEncoder); ok {
			return e, true
		}
	}
	// Only the pointer method set implements the interface, so check an
	// addressable copy.
	copied := reflect.New(t).Elem()
	copied.Set(v)
	if e, ok := copied.Addr().Interface().(BinaryEncoder); ok {
		return e, true
	}
	return nil, false
}

func encode(v reflect.Value, tag Tag) ([]byte, error) {
	buf := make([]byte, 0, sizeOf(v, tag))
	return appendTo(buf, v, tag)
}

// appendTo appends the encoded form of v to dst.
func appendTo(dst []byte, v reflect.Value, tag Tag) ([]byte, error) {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return dst, fmt.Errorf("%w: cannot encode nil pointer %s", ErrUnsupportedType, v.Type())
		}
	}

	if e, ok := encoderOf(v); ok {
		data, err := e.BinaryEncode()
		if err != nil {
			return dst, err
		}
		return append(dst, data...), nil
	}

	if v.Kind() == reflect.Pointer {
		return appendTo(dst, v.Elem(), tag)
	}

	// time.Time and time.Duration need special handling.
	switch v.Kind() {
	case reflect.Struct:
		if v.Type() == timeType {
			return appendTime(dst, v, tag)
		}
		return appendStruct(dst, v)
	case reflect.Int64:
		if v.Type() == durationType {
			return appendTime(dst, v, tag)
		}
	}
	return appendValue(dst, v, tag)
}

func appendStruct(dst []byte, v reflect.Value) ([]byte, error) {
	// Length fields come before the data they describe, so fill them in first.
	if err := fillLengthRefs(v); err != nil {
		return dst, err
	}

	fields := fields(v.Type())
	for i, meta := range fields {
		// Unexported fields cannot be read through reflection.
		if !meta.exported {
			continue
		}
		if meta.err != nil {
			return dst, fmt.Errorf("field [%s]: %w", v.Type().Field(i).Name, meta.err)
		}

		tag := meta.tag
		var err error
		if tag, err = resolveLengthRef(v, tag, meta.lengthRef); err != nil {
			return dst, fmt.Errorf("field [%s]: %w", v.Type().Field(i).Name, err)
		}

		if dst, err = appendTo(dst, v.Field(i), tag); err != nil {
			return dst, fmt.Errorf("field [%s]: %w", v.Type().Field(i).Name, err)
		}

		if tag.Skip != nil {
			dst = appendZero(dst, *tag.Skip)
		}
		if tag.Align && tag.ElementLength != nil && *tag.ElementLength > 0 {
			dst = pad(dst, *tag.ElementLength)
		}
	}

	return dst, nil
}

func appendValue(dst []byte, v reflect.Value, tag Tag) ([]byte, error) {
	switch v.Kind() {
	case reflect.Struct, reflect.Pointer:
		return appendTo(dst, v, tag)

	case reflect.Slice, reflect.Array:
		return appendSequence(dst, v, tag)

	case reflect.Bool:
		if v.Bool() {
			return append(dst, 1), nil
		}
		return append(dst, 0), nil

	case reflect.Uint8:
		return append(dst, byte(v.Uint())), nil
	case reflect.Uint16:
		return binary.LittleEndian.AppendUint16(dst, uint16(v.Uint())), nil
	case reflect.Uint32:
		return binary.LittleEndian.AppendUint32(dst, uint32(v.Uint())), nil
	case reflect.Uint64:
		return binary.LittleEndian.AppendUint64(dst, v.Uint()), nil

	case reflect.Int8:
		return append(dst, byte(v.Int())), nil
	case reflect.Int16:
		return binary.LittleEndian.AppendUint16(dst, uint16(v.Int())), nil
	case reflect.Int32:
		return binary.LittleEndian.AppendUint32(dst, uint32(v.Int())), nil
	case reflect.Int64:
		return binary.LittleEndian.AppendUint64(dst, uint64(v.Int())), nil

	case reflect.Float32:
		return binary.LittleEndian.AppendUint32(dst, math.Float32bits(float32(v.Float()))), nil
	case reflect.Float64:
		return binary.LittleEndian.AppendUint64(dst, math.Float64bits(v.Float())), nil

	case reflect.String:
		return appendString(dst, v.String(), tag), nil
	}

	return dst, fmt.Errorf("%w: %s", ErrUnsupportedType, v.Type())
}

// appendSequence encodes a slice or array. A byte slice is a blob sized by
// Length; any other element type is encoded per element with Length counting
// elements.
func appendSequence(dst []byte, v reflect.Value, tag Tag) ([]byte, error) {
	elem := v.Type().Elem()

	if elem.Kind() == reflect.Uint8 && (v.Kind() == reflect.Array || tag.ElementLength == nil) {
		return appendBytes(dst, bytesOf(v), tag), nil
	}

	count, padding := elementCount(v.Len(), tag)

	elemTag := tag
	elemTag.Length = tag.ElementLength
	elemTag.LengthRef = nil
	elemTag.Skip = nil
	elemTag.Align = false
	elemTag.align4 = false
	elemTag.TillEnd = false

	start := len(dst)
	for i := range count {
		var err error
		if dst, err = appendTo(dst, v.Index(i), elemTag); err != nil {
			return dst, fmt.Errorf("element %d: %w", i, err)
		}
		// align4 starts every element on a four byte boundary.
		if tag.align4 {
			dst = appendZero(dst, padSize(len(dst)-start, 4))
		}
	}

	// Pad up to the requested element count.
	if padding > 0 {
		size := elementSize(elem)
		if tag.align4 {
			size += padSize(size, 4)
		}
		dst = appendZero(dst, padding*size)
	}
	return dst, nil
}

// appendTime encodes a time.Time or time.Duration as an integer of the tag's
// width and unit. A time.Time is the duration since the Unix epoch.
func appendTime(dst []byte, v reflect.Value, tag Tag) ([]byte, error) {
	width, ok := timeWidth(tag.Length)
	if !ok {
		return dst, fmt.Errorf("%w: a time field requires a length of 1, 2, 4 or 8", ErrInvalidTag)
	}

	var d time.Duration
	if v.Type() == durationType {
		d = time.Duration(v.Int())
	} else {
		d = v.Interface().(time.Time).Sub(time.Unix(0, 0))
	}
	raw := int64(d / unitDuration(tag.TimeUnit))

	switch width {
	case 1:
		return append(dst, byte(raw)), nil
	case 2:
		return binary.LittleEndian.AppendUint16(dst, uint16(raw)), nil
	case 4:
		return binary.LittleEndian.AppendUint32(dst, uint32(raw)), nil
	default:
		return binary.LittleEndian.AppendUint64(dst, uint64(raw)), nil
	}
}

// appendBytes copies data, truncating or zero padding it to Length.
func appendBytes(dst, data []byte, tag Tag) []byte {
	if tag.Length == nil {
		return append(dst, data...)
	}

	length := *tag.Length
	if length <= len(data) {
		return append(dst, data[:length]...)
	}
	dst = append(dst, data...)
	return appendZero(dst, length-len(data))
}

// appendString encodes a string, padding or truncating it to Length, or adding
// a NUL terminator when ZeroTerminated.
func appendString(dst []byte, s string, tag Tag) []byte {
	length := len(s)
	if tag.Length != nil {
		length = *tag.Length
	} else if tag.ZeroTerminated {
		length = len(s) + 1
	}

	if length <= len(s) {
		return append(dst, s[:length]...)
	}
	dst = append(dst, s...)
	return appendZero(dst, length-len(s))
}

// bytesOf returns the raw bytes of a byte slice or a byte array.
func bytesOf(v reflect.Value) []byte {
	if v.Kind() == reflect.Slice {
		return v.Bytes()
	}
	buf := make([]byte, v.Len())
	reflect.Copy(reflect.ValueOf(buf), v)
	return buf
}

// elementCount returns how many elements to encode and how many to pad.
func elementCount(available int, tag Tag) (count, padding int) {
	if tag.Length == nil {
		return available, 0
	}

	switch {
	case *tag.Length < available:
		return *tag.Length, 0
	case *tag.Length > available:
		return available, *tag.Length - available
	default:
		return available, 0
	}
}

// zeroes backs appendZero so padding does not allocate.
var zeroes [32]byte

// appendZero appends n zero bytes to dst.
func appendZero(dst []byte, n int) []byte {
	for n >= len(zeroes) {
		dst = append(dst, zeroes[:]...)
		n -= len(zeroes)
	}
	return append(dst, zeroes[:n]...)
}

// pad extends buf with zero bytes until its length is a multiple of alignment.
func pad(buf []byte, alignment int) []byte {
	if rem := len(buf) % alignment; rem != 0 {
		buf = appendZero(buf, alignment-rem)
	}
	return buf
}

// fillLengthRefs writes the size of each data field into its lengthRef field.
func fillLengthRefs(v reflect.Value) error {
	fields := fields(v.Type())
	for i, meta := range fields {
		if !meta.exported {
			continue
		}
		if meta.err != nil {
			return fmt.Errorf("field [%s]: %w", v.Type().Field(i).Name, meta.err)
		}
		if meta.tag.LengthRef == nil || len(meta.lengthRef) == 0 {
			continue
		}

		ref := v.FieldByIndex(meta.lengthRef)
		if !ref.IsValid() {
			return fmt.Errorf("field [%s]: %w: lengthRef %q does not exist", v.Type().Field(i).Name, ErrInvalidTag, *meta.tag.LengthRef)
		}

		length := measure(v.Field(i), meta.tag)

		switch ref.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			ref.SetUint(uint64(length))
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			ref.SetInt(int64(length))
		default:
			return fmt.Errorf("field [%s]: %w: lengthRef %q refers to a non integer field", v.Type().Field(i).Name, ErrInvalidTag, *meta.tag.LengthRef)
		}
	}
	return nil
}

// resolveLengthRef fills in Length from the field named by LengthRef.
func resolveLengthRef(v reflect.Value, tag Tag, index []int) (Tag, error) {
	if tag.LengthRef == nil {
		return tag, nil
	}

	ref := v.FieldByIndex(index)
	if !ref.IsValid() {
		return tag, fmt.Errorf("%w: lengthRef %q does not exist", ErrInvalidTag, *tag.LengthRef)
	}

	if ref.Kind() == reflect.Pointer {
		if ref.IsNil() {
			return tag, fmt.Errorf("%w: lengthRef %q is nil", ErrInvalidTag, *tag.LengthRef)
		}
		ref = ref.Elem()
	}

	var length int
	switch ref.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		length = int(ref.Uint())
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		length = int(ref.Int())
	default:
		return tag, fmt.Errorf("%w: lengthRef %q refers to a non integer field", ErrInvalidTag, *tag.LengthRef)
	}

	if length < 0 {
		return tag, fmt.Errorf("%w: lengthRef %q is negative (%d)", ErrInvalidTag, *tag.LengthRef, length)
	}

	tag.Length = &length
	return tag, nil
}

// measure returns the value a lengthRef has to hold: elements for a sequence,
// bytes for a string.
func measure(v reflect.Value, tag Tag) int {
	if !v.IsValid() {
		return 0
	}
	for v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return 0
		}
		v = v.Elem()
	}

	switch v.Kind() {
	case reflect.Slice, reflect.Array:
		return v.Len()
	case reflect.String:
		if tag.ZeroTerminated {
			return len(v.String()) + 1
		}
		return len(v.String())
	}

	return sizeOf(v, Tag{})
}
