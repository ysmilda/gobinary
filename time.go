package gobinary

import (
	"reflect"
	"time"
)

var (
	timeType     = reflect.TypeFor[time.Time]()
	durationType = reflect.TypeFor[time.Duration]()
)

// defaultTimeUnit is the unit of a time field without an explicit unit.
const defaultTimeUnit = time.Millisecond

// unitDuration returns the time.Duration one unit of a time field stands for.
func unitDuration(unit *string) time.Duration {
	if unit == nil {
		return defaultTimeUnit
	}
	switch *unit {
	case "nanoseconds", "ns":
		return time.Nanosecond
	case "microseconds", "us":
		return time.Microsecond
	case "milliseconds", "ms":
		return time.Millisecond
	case "seconds", "s":
		return time.Second
	case "minutes", "m":
		return time.Minute
	case "hours", "h":
		return time.Hour
	case "days", "d":
		return 24 * time.Hour
	}
	return defaultTimeUnit
}

// timeWidth returns the width of a time field, which is 1, 2, 4 or 8 bytes.
func timeWidth(length *int) (int, bool) {
	if length == nil {
		return 0, false
	}
	switch *length {
	case 1, 2, 4, 8:
		return *length, true
	}
	return 0, false
}
