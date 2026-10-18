package gobinary

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"time"
)

// BinaryDecoder is implemented by types that decode themselves from the start
// of a buffer. The returned int is the number of bytes consumed.
type BinaryDecoder interface {
	BinaryDecode([]byte) (int, error)
}

func decode(buf []byte, v reflect.Value, tag Tag) (int, error) {
	d, pv := indirect(v)
	if d != nil {
		return d.BinaryDecode(buf)
	}
	v = pv

	// time.Time and time.Duration need special handling.
	if v.Kind() == reflect.Struct && v.Type() == timeType {
		return decodeTime(buf, v, tag)
	}
	if v.Kind() == reflect.Int64 && v.Type() == durationType {
		return decodeTime(buf, v, tag)
	}
	if v.Kind() != reflect.Struct {
		return decodeValue(buf, v, tag)
	}

	pos := 0
	fields := fields(v.Type())
	for i, meta := range fields {
		// Unexported fields cannot be written through reflection.
		if !meta.exported {
			continue
		}
		if meta.err != nil {
			return pos, fmt.Errorf("field [%s]: %w", v.Type().Field(i).Name, meta.err)
		}

		tag := meta.tag
		var err error
		if tag, err = resolveLengthRef(v, tag, meta.lengthRef); err != nil {
			return pos, fmt.Errorf("field [%s]: %w", v.Type().Field(i).Name, err)
		}

		n, err := decode(buf[pos:], v.Field(i), tag)
		pos += n
		if err != nil {
			return pos, fmt.Errorf("field [%s]: %w", v.Type().Field(i).Name, err)
		}

		if tag.Skip != nil {
			pos += skip(len(buf)-pos, *tag.Skip)
		}
		if tag.Align && tag.ElementLength != nil && *tag.ElementLength > 0 {
			pos += padSize(len(buf)-pos, *tag.ElementLength)
		}
	}

	return pos, nil
}

func decodeValue(buf []byte, v reflect.Value, tag Tag) (int, error) {
	// The indirect walk only matters for BinaryDecoder, pointer and interface values.
	if k := v.Kind(); k == reflect.Pointer || k == reflect.Interface || canDecode(v.Type()) {
		d, pv := indirect(v)
		if d != nil {
			return d.BinaryDecode(buf)
		}
		v = pv
	}

	switch v.Kind() {
	case reflect.Struct:
		return decode(buf, v, tag)

	case reflect.Slice, reflect.Array:
		return decodeSequence(buf, v, tag)

	case reflect.Bool:
		if len(buf) < 1 {
			return 0, ErrTruncated
		}
		v.SetBool(buf[0] != 0)
		return 1, nil

	case reflect.Uint8:
		if len(buf) < 1 {
			return 0, ErrTruncated
		}
		v.SetUint(uint64(buf[0]))
		return 1, nil
	case reflect.Uint16:
		if len(buf) < 2 {
			return 0, ErrTruncated
		}
		v.SetUint(uint64(binary.LittleEndian.Uint16(buf)))
		return 2, nil
	case reflect.Uint32:
		if len(buf) < 4 {
			return 0, ErrTruncated
		}
		v.SetUint(uint64(binary.LittleEndian.Uint32(buf)))
		return 4, nil
	case reflect.Uint64:
		if len(buf) < 8 {
			return 0, ErrTruncated
		}
		v.SetUint(binary.LittleEndian.Uint64(buf))
		return 8, nil

	case reflect.Int8:
		if len(buf) < 1 {
			return 0, ErrTruncated
		}
		v.SetInt(int64(int8(buf[0])))
		return 1, nil
	case reflect.Int16:
		if len(buf) < 2 {
			return 0, ErrTruncated
		}
		v.SetInt(int64(int16(binary.LittleEndian.Uint16(buf))))
		return 2, nil
	case reflect.Int32:
		if len(buf) < 4 {
			return 0, ErrTruncated
		}
		v.SetInt(int64(int32(binary.LittleEndian.Uint32(buf))))
		return 4, nil
	case reflect.Int64:
		if len(buf) < 8 {
			return 0, ErrTruncated
		}
		v.SetInt(int64(binary.LittleEndian.Uint64(buf)))
		return 8, nil

	case reflect.Float32:
		if len(buf) < 4 {
			return 0, ErrTruncated
		}
		v.SetFloat(float64(math.Float32frombits(binary.LittleEndian.Uint32(buf))))
		return 4, nil
	case reflect.Float64:
		if len(buf) < 8 {
			return 0, ErrTruncated
		}
		v.SetFloat(math.Float64frombits(binary.LittleEndian.Uint64(buf)))
		return 8, nil

	case reflect.String:
		s, n, err := decodeString(buf, tag)
		if err != nil {
			return 0, err
		}
		v.SetString(s)
		return n, nil
	}

	return 0, fmt.Errorf("%w: %s", ErrUnsupportedType, v.Type())
}

// decodeSequence decodes a slice or array. A byte slice is a blob sized by
// Length, or by the remaining buffer; any other element type is decoded per
// element with Length counting elements.
func decodeSequence(buf []byte, v reflect.Value, tag Tag) (int, error) {
	elem := v.Type().Elem()

	if elem.Kind() == reflect.Uint8 && (v.Kind() == reflect.Array || tag.ElementLength == nil) {
		return decodeBytes(buf, v, tag)
	}

	// The buffer decides how many elements are really there, so a truncated
	// stream decodes what fits instead of failing.
	count := len(buf)
	switch {
	case tag.TillEnd:
	case tag.Length != nil:
		if *tag.Length < 0 {
			return 0, fmt.Errorf("%w: negative length %d", ErrInvalidTag, *tag.Length)
		}
		count = *tag.Length
	default:
		count = len(buf) / max(elementSize(elem), 1)
	}

	return decodeList(buf, v, elem, count, tag)
}

// decodeList decodes up to count elements into a slice or an array.
func decodeList(buf []byte, v reflect.Value, elem reflect.Type, count int, tag Tag) (int, error) {
	// align4 is a property of the sequence, so it is kept out of the element
	// tag handed to the per element decode.
	align4 := tag.align4

	elemTag := tag
	elemTag.Length = tag.ElementLength
	elemTag.LengthRef = nil
	elemTag.Skip = nil
	elemTag.Align = false
	elemTag.align4 = false
	elemTag.TillEnd = false

	pos := 0

	if v.Kind() == reflect.Slice {
		slice := reflect.MakeSlice(v.Type(), 0, min(count, len(buf)))
		for range count {
			if pos >= len(buf) {
				break
			}
			item := reflect.New(elem).Elem()
			n, err := decode(buf[pos:], item, elemTag)
			if err != nil {
				return pos, err
			}
			slice = reflect.Append(slice, item)
			pos += n
			if align4 {
				pos += padSize(pos, 4)
			}
		}
		v.Set(slice)
		return pos, nil
	}

	for i := range v.Len() {
		if pos >= len(buf) {
			break
		}
		n, err := decode(buf[pos:], v.Index(i), elemTag)
		if err != nil {
			return pos, err
		}
		pos += n
		if align4 {
			pos += padSize(pos, 4)
		}
	}
	return pos, nil
}

// decodeBytes decodes a raw blob into a byte slice or a byte array.
func decodeBytes(buf []byte, v reflect.Value, tag Tag) (int, error) {
	length := len(buf)
	if tag.Length != nil {
		if *tag.Length < 0 {
			return 0, fmt.Errorf("%w: negative length %d", ErrInvalidTag, *tag.Length)
		}
		// A length past the buffer means the frame was cut short, which has to
		// be an error rather than a short value that looks complete.
		if *tag.Length > len(buf) {
			return 0, fmt.Errorf("%w: %d bytes announced, %d available", ErrTruncated, *tag.Length, len(buf))
		}
		length = *tag.Length
	}

	if v.Kind() == reflect.Slice {
		v.SetBytes(bytes.Clone(buf[:length]))
		return length, nil
	}

	for i := range min(length, v.Len()) {
		v.Index(i).SetUint(uint64(buf[i]))
	}
	return length, nil
}

// decodeTime decodes a numeric time.Time or time.Duration in the tag's unit,
// defaulting to milliseconds, since the Unix epoch.
func decodeTime(buf []byte, v reflect.Value, tag Tag) (int, error) {
	width, ok := timeWidth(tag.Length)
	if !ok {
		return 0, fmt.Errorf("%w: a time field requires a length of 1, 2, 4 or 8", ErrInvalidTag)
	}
	if len(buf) < width {
		return 0, ErrTruncated
	}

	var raw uint64
	switch width {
	case 1:
		raw = uint64(buf[0])
	case 2:
		raw = uint64(binary.LittleEndian.Uint16(buf))
	case 4:
		raw = uint64(binary.LittleEndian.Uint32(buf))
	default:
		raw = binary.LittleEndian.Uint64(buf)
	}

	d := time.Duration(int64(raw)) * unitDuration(tag.TimeUnit)
	if v.Type() == durationType {
		v.SetInt(int64(d))
	} else {
		v.Set(reflect.ValueOf(time.UnixMicro(d.Microseconds())))
	}
	return width, nil
}

// decodeString decodes a string field. Without a Length tag the string is read
// up to the NUL terminator, or up to the end of the buffer.
func decodeString(buf []byte, tag Tag) (string, int, error) {
	if tag.Length != nil {
		if *tag.Length < 0 {
			return "", 0, fmt.Errorf("%w: negative length %d", ErrInvalidTag, *tag.Length)
		}
		if *tag.Length > len(buf) {
			return "", 0, ErrTruncated
		}
		// A fixed size string is zero padded, so cut it at the first NUL.
		field := buf[:*tag.Length]
		if i := bytes.IndexByte(field, 0); i >= 0 {
			field = field[:i]
		}
		return string(field), *tag.Length, nil
	}

	if !tag.ZeroTerminated {
		return string(buf), len(buf), nil
	}

	if i := bytes.IndexByte(buf, 0); i >= 0 {
		return string(buf[:i]), i + 1, nil
	}
	return string(buf), len(buf), nil
}

// indirect walks down v allocating pointers as needed, until it gets to a
// non-pointer. If it encounters a BinaryDecoder, indirect stops and returns that.
// Adapted from encoding/json.indirect.
func indirect(v reflect.Value) (BinaryDecoder, reflect.Value) {
	v0 := v
	haveAddr := false

	if v.Kind() != reflect.Pointer && v.Type().Name() != "" && v.CanAddr() {
		haveAddr = true
		v = v.Addr()
	}
	for {
		// Load value from interface, but only if the result will be
		// usefully addressable.
		if v.Kind() == reflect.Interface && !v.IsNil() {
			e := v.Elem()
			if e.Kind() == reflect.Pointer && !e.IsNil() {
				haveAddr = false
				v = e
				continue
			}
		}

		if v.Kind() != reflect.Pointer {
			break
		}

		// Prevent infinite loop if v is an interface pointing to its own address:
		//     var v interface{}
		//     v = &v
		if v.Elem().Kind() == reflect.Interface && v.Elem().Elem() == v {
			v = v.Elem()
			break
		}
		if v.IsNil() {
			v.Set(reflect.New(v.Type().Elem()))
		}
		if v.Type().NumMethod() > 0 && v.CanInterface() {
			if u, ok := v.Interface().(BinaryDecoder); ok {
				return u, reflect.Value{}
			}
		}

		if haveAddr {
			v = v0 // restore original value after round-trip Value.Addr().Elem()
			haveAddr = false
		} else {
			v = v.Elem()
		}
	}
	return nil, v
}

// skip returns how many of n padding bytes are left in the buffer.
func skip(remaining, n int) int {
	return min(max(n, 0), max(remaining, 0))
}

// padSize returns the amount of padding needed to align size to alignment.
func padSize(size, alignment int) int {
	if alignment <= 0 {
		return 0
	}
	if rem := size % alignment; rem != 0 {
		return alignment - rem
	}
	return 0
}
