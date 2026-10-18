package gobinary

import (
	"fmt"
	"strconv"
	"strings"
)

// Tag contains all available tag values. Depending on the type of the field
// some of them have to be set for encoding and decoding to succeed.
type Tag struct {
	// Length is the byte length of a string, or the element count of a
	// sequence. Used as length:number in the struct tag.
	Length *int
	// ElementLength is the encoded size of one element, counting Length in
	// elements and fixing the width of variable sized elements.
	// Used as elementLength:number in the struct tag.
	ElementLength *int
	// LengthRef names another field holding the length, as an alternative to
	// Length. Used as lengthRef:OtherField in the struct tag.
	LengthRef *string
	// TimeUnit is the unit of a time.Time or time.Duration field, defaulting
	// to milliseconds. Used as unit:nanoseconds|microseconds|milliseconds|seconds|minutes|hours|days
	// in the struct tag.
	TimeUnit *string
	// Skip is the amount of bytes to skip after the field.
	// Used as skip:number in the struct tag.
	Skip *int
	// ZeroTerminated makes a string read up to the first NUL or Length bytes
	// and writes a NUL terminator. On a sequence it applies to every element.
	// Used as zeroTerminated in the struct tag.
	ZeroTerminated bool
	// TillEnd consumes all remaining bytes into a byte slice, only on the last
	// field. Used as tillEnd in the struct tag.
	TillEnd bool
	// Align pads the field to a multiple of ElementLength for the naturally
	// aligned offsets of the ADS upload. Used as align in the struct tag.
	Align bool
	// align4 pads every element of a sequence to four bytes. The name is
	// misspelled to stay compatible with the m1 reference implementation.
	// Used as align4 in the struct tag.
	align4 bool
}

// bareTags are the tags that stand on their own, without a value.
var bareTags = map[string]bool{
	"zeroTerminated": true,
	"tillEnd":        true,
	"align":          true,
	"align4":         true,
}

// valueTags are the tags that require a value after a colon.
var valueTags = map[string]bool{
	"length":        true,
	"elementLength": true,
	"lengthRef":     true,
	"skip":          true,
	"unit":          true,
}

// validUnits is the set of units a unit tag may hold.
var validUnits = map[string]bool{
	"nanoseconds":  true,
	"ns":           true,
	"microseconds": true,
	"us":           true,
	"milliseconds": true,
	"ms":           true,
	"seconds":      true,
	"s":            true,
	"minutes":      true,
	"m":            true,
	"hours":        true,
	"h":            true,
	"days":         true,
	"d":            true,
}

// parseTag parses a struct tag into a Tag and rejects unknown or malformed parts.
func parseTag(s string) (Tag, error) {
	if s == "" {
		return Tag{}, nil
	}

	var tag Tag
	seen := make(map[string]bool)
	for part := range strings.SplitSeq(s, ",") {
		name, value, hasValue := strings.Cut(part, ":")
		if name == "" {
			return Tag{}, errTag("empty tag")
		}
		if seen[name] {
			return Tag{}, errTag(fmt.Sprintf("duplicate tag %q", name))
		}
		seen[name] = true

		switch {
		case bareTags[name]:
			if hasValue {
				return Tag{}, errTag(fmt.Sprintf("%s takes no value", name))
			}
		case valueTags[name]:
			if !hasValue || value == "" {
				return Tag{}, errTag(fmt.Sprintf("%s requires a value", name))
			}
		default:
			return Tag{}, errTag(fmt.Sprintf("unknown tag %q", name))
		}

		switch name {
		case "zeroTerminated":
			tag.ZeroTerminated = true
		case "tillEnd":
			tag.TillEnd = true
		case "align":
			tag.Align = true
		case "align4":
			tag.align4 = true
		case "length":
			n, err := tagInt(name, value)
			if err != nil {
				return Tag{}, err
			}
			tag.Length = &n
		case "elementLength":
			n, err := tagInt(name, value)
			if err != nil {
				return Tag{}, err
			}
			tag.ElementLength = &n
		case "skip":
			n, err := tagInt(name, value)
			if err != nil {
				return Tag{}, err
			}
			tag.Skip = &n
		case "lengthRef":
			tag.LengthRef = &value
		case "unit":
			if !validUnits[value] {
				return Tag{}, errTag(fmt.Sprintf("unit: %q is not a known unit", value))
			}
			tag.TimeUnit = &value
		}
	}

	if tag.Length != nil && tag.LengthRef != nil {
		return Tag{}, errTag("only one of length and lengthRef may be set")
	}
	if tag.TillEnd && tag.Length != nil {
		return Tag{}, errTag("tillEnd may not be combined with length")
	}
	if tag.TillEnd && tag.LengthRef != nil {
		return Tag{}, errTag("tillEnd may not be combined with lengthRef")
	}
	if tag.TillEnd && tag.Skip != nil {
		return Tag{}, errTag("tillEnd may not be combined with skip")
	}
	if tag.ElementLength != nil && tag.Length == nil && tag.LengthRef == nil {
		return Tag{}, errTag("elementLength requires length or lengthRef")
	}
	if tag.Align && tag.ElementLength == nil {
		return Tag{}, errTag("align requires elementLength")
	}
	if tag.align4 && tag.Length == nil && tag.LengthRef == nil {
		return Tag{}, errTag("align4 requires length or lengthRef")
	}

	return tag, nil
}

// tagInt parses an integer tag value and rejects negative sizes.
func tagInt(name, value string) (int, error) {
	n, err := strconv.Atoi(value)
	if err != nil {
		return 0, errTag(fmt.Sprintf("%s: %q is not a valid number", name, value))
	}
	if n < 0 {
		return 0, errTag(fmt.Sprintf("%s: %q must not be negative", name, value))
	}
	return n, nil
}
