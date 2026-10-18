package gobinary

import (
	"fmt"
	"reflect"
	"sync"
)

// fieldMeta is the parsed description of a single struct field, cached per type.
type fieldMeta struct {
	exported  bool
	tag       Tag
	err       error
	lengthRef []int
}

// typeCache caches the parsed field metadata per struct type.
var typeCache sync.Map

// decoderType is the reflect form of the BinaryDecoder interface.
var decoderType = reflect.TypeFor[BinaryDecoder]()

// decoderCache caches whether the pointer to a type implements BinaryDecoder.
var decoderCache sync.Map

// canDecode reports whether the address of a value of type t implements
// BinaryDecoder.
func canDecode(t reflect.Type) bool {
	if cached, ok := decoderCache.Load(t); ok {
		return cached.(bool)
	}
	ok := reflect.PointerTo(t).Implements(decoderType)
	decoderCache.Store(t, ok)
	return ok
}

// encoderType is the reflect form of the BinaryEncoder interface.
var encoderType = reflect.TypeFor[BinaryEncoder]()

// encoderCache caches whether a type or its pointer implements BinaryEncoder.
var encoderCache sync.Map

// canEncode reports whether a value of type t or its pointer implements
// BinaryEncoder.
func canEncode(t reflect.Type) bool {
	if cached, ok := encoderCache.Load(t); ok {
		return cached.(bool)
	}
	ok := t.Implements(encoderType) || reflect.PointerTo(t).Implements(encoderType)
	encoderCache.Store(t, ok)
	return ok
}

// elementSizeCache caches the encoded size of one element per type.
var elementSizeCache sync.Map

// fields returns the cached field metadata of a struct type.
func fields(t reflect.Type) []fieldMeta {
	if cached, ok := typeCache.Load(t); ok {
		return cached.([]fieldMeta)
	}

	// tillEnd consumes everything left, so it only makes sense on the last
	// exported field.
	lastExported := -1
	for i := range t.NumField() {
		if t.Field(i).IsExported() {
			lastExported = i
		}
	}

	metas := make([]fieldMeta, t.NumField())
	for i := range t.NumField() {
		sf := t.Field(i)

		tag, err := parseTag(sf.Tag.Get(tagKey))
		if err == nil {
			err = validateTag(sf.Type, tag)
		}
		if err == nil && tag.TillEnd && i != lastExported {
			err = errTag("tillEnd may only be used on the last field")
		}

		meta := fieldMeta{
			exported:  sf.IsExported(),
			tag:       tag,
			err:       err,
			lengthRef: nil,
		}
		if tag.LengthRef != nil {
			if ref, ok := t.FieldByName(*tag.LengthRef); ok {
				meta.lengthRef = ref.Index
			}
		}
		metas[i] = meta
	}

	actual, _ := typeCache.LoadOrStore(t, metas)
	return actual.([]fieldMeta)
}

// validateTag rejects tag combinations that do not fit the Go type of the
// field.
func validateTag(t reflect.Type, tag Tag) error {
	// A type that encodes or decodes itself ignores every tag.
	if canDecode(t) || canEncode(t) {
		return nil
	}

	if tag.TimeUnit != nil && t != timeType && t != durationType {
		return errTag("unit is only valid on time.Time and time.Duration")
	}

	switch {
	case t == timeType || t == durationType:
		if _, ok := timeWidth(tag.Length); !ok {
			return errTag("a time field requires a length of 1, 2, 4 or 8")
		}
		if tag.ElementLength != nil || tag.TillEnd || tag.align4 || tag.Align {
			return errTag("elementLength, align, tillEnd and align4 are not valid on a time field")
		}

	case t.Kind() == reflect.String:
		if tag.TillEnd {
			return errTag("tillEnd is only valid on a sequence")
		}
		if tag.align4 {
			return errTag("align4 is only valid on a sequence")
		}

	case t.Kind() == reflect.Slice || t.Kind() == reflect.Array:
		// align4 only has an effect on the per element path of a byte sequence.
		if tag.align4 && t.Elem().Kind() == reflect.Uint8 && (t.Kind() == reflect.Array || tag.ElementLength == nil) {
			return errTag("align4 is not valid on a byte sequence")
		}

	default:
		if tag.Length != nil {
			return errTag(fmt.Sprintf("length is only valid on strings, sequences and time fields, not on %s", t))
		}
		if tag.ZeroTerminated || tag.TillEnd || tag.align4 {
			return errTag(fmt.Sprintf("zeroTerminated, tillEnd and align4 are not valid on %s", t))
		}
	}

	return nil
}
