package gobinary

import "reflect"

// sizeOf returns the encoded size of v, taking the tag into account.
func sizeOf(v reflect.Value, tag Tag) int {
	if v.Kind() == reflect.Pointer {
		if v.IsNil() {
			return 0
		}
	}

	if e, ok := encoderOf(v); ok {
		data, err := e.BinaryEncode()
		if err != nil {
			return 0
		}
		return len(data)
	}

	if v.Kind() == reflect.Pointer {
		return sizeOf(v.Elem(), tag)
	}

	switch v.Kind() {
	case reflect.Struct:
		// A time field is a fixed width integer.
		if v.Type() == timeType {
			if tag.Length != nil {
				return *tag.Length
			}
			return 8
		}

		size := 0
		fields := fields(v.Type())
		for i, meta := range fields {
			if !meta.exported || meta.err != nil {
				continue
			}

			size += sizeOf(v.Field(i), meta.tag)
			if meta.tag.Skip != nil {
				size += *meta.tag.Skip
			}
			if meta.tag.Align && meta.tag.ElementLength != nil && *meta.tag.ElementLength > 0 {
				size += padSize(size, *meta.tag.ElementLength)
			}
		}
		return size

	case reflect.Slice, reflect.Array:
		elem := v.Type().Elem()

		// A byte sequence is a blob sized by Length, or by the value.
		if elem.Kind() == reflect.Uint8 && (v.Kind() == reflect.Array || tag.ElementLength == nil) {
			if tag.Length != nil {
				return *tag.Length
			}
			return v.Len()
		}

		// Mirror appendSequence: Length truncates or pads the element count.
		count, padding := elementCount(v.Len(), tag)
		elemTag := tag
		elemTag.Length = tag.ElementLength
		elemTag.LengthRef = nil
		elemTag.Skip = nil
		elemTag.Align = false
		elemTag.align4 = false
		elemTag.TillEnd = false

		size := 0
		for i := range count {
			size += sizeOf(v.Index(i), elemTag)
			if tag.align4 {
				size += padSize(size, 4)
			}
		}
		if padding > 0 {
			per := elementSize(elem)
			if tag.align4 {
				per += padSize(per, 4)
			}
			size += padding * per
		}
		return size

	case reflect.String:
		// A fixed size string is sized by Length, a variable one by the value.
		if tag.Length != nil {
			return *tag.Length
		}
		if tag.ZeroTerminated {
			return v.Len() + 1
		}
		return v.Len()

	case reflect.Bool, reflect.Uint8, reflect.Int8:
		return 1
	case reflect.Uint16, reflect.Int16:
		return 2
	case reflect.Uint32, reflect.Int32, reflect.Float32:
		return 4
	case reflect.Int64:
		// A time.Duration uses the tag width.
		if v.Type() == durationType && tag.Length != nil {
			return *tag.Length
		}
		return 8
	case reflect.Uint64, reflect.Float64:
		return 8
	}

	return 0
}

// elementSize returns the encoded size of one element of type t.
func elementSize(t reflect.Type) int {
	if cached, ok := elementSizeCache.Load(t); ok {
		return cached.(int)
	}

	size := sizeOf(reflect.Zero(t), Tag{})
	elementSizeCache.Store(t, size)
	return size
}
