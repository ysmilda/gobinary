package gobinary

import (
	"reflect"
	"testing"
	"time"
)

func TestTimeDecode(t *testing.T) {
	tests := []struct {
		name string
		buf  []byte
		typ  any
		want any
	}{
		{
			name: "time length 1 defaults to milliseconds",
			buf:  []byte{123},
			typ: struct {
				V time.Time `binary:"length:1"`
			}{},
			want: time.UnixMicro((123 * time.Second).Milliseconds()),
		},
		{
			name: "time length 2 minutes",
			buf:  []byte{0x39, 0x30},
			typ: struct {
				V time.Time `binary:"length:2,unit:minutes"`
			}{},
			want: time.UnixMicro((12345 * time.Minute).Microseconds()),
		},
		{
			name: "time length 4 seconds",
			buf:  []byte{0x15, 0xCD, 0x5B, 0x07},
			typ: struct {
				V time.Time `binary:"length:4,unit:seconds"`
			}{},
			want: time.UnixMicro((123456789 * time.Second).Microseconds()),
		},
		{
			name: "time length 8 milliseconds",
			buf:  []byte{0xCB, 0x04, 0xFB, 0x71, 0x1F, 0x01, 0x00, 0x00},
			typ: struct {
				V time.Time `binary:"length:8,unit:milliseconds"`
			}{},
			want: time.UnixMicro((1234567890123 * time.Millisecond).Microseconds()),
		},
		{
			name: "duration hours",
			buf:  []byte{123},
			typ: struct {
				V time.Duration `binary:"length:1,unit:hours"`
			}{},
			want: 123 * time.Hour,
		},
		{
			name: "duration days",
			buf:  []byte{0x39, 0x30},
			typ: struct {
				V time.Duration `binary:"length:2,unit:days"`
			}{},
			want: 12345 * 24 * time.Hour,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.New(reflect.TypeOf(tt.typ))
			n, err := Decode(tt.buf, v.Interface())
			if err != nil {
				t.Fatalf("decode: %v", err)
			}
			if n != len(tt.buf) {
				t.Errorf("consumed %d, want %d", n, len(tt.buf))
			}
			got := v.Elem().Field(0).Interface()
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTimeRoundTrip(t *testing.T) {
	type timed struct {
		Stamp time.Time     `binary:"length:8,unit:milliseconds"`
		Dur   time.Duration `binary:"length:4,unit:seconds"`
	}
	in := timed{
		Stamp: time.UnixMilli(1700000000123),
		Dur:   12345 * time.Second,
	}

	encoded, err := Encode(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got := Size(in); got != len(encoded) {
		t.Errorf("Size is %d, encoded is %d", got, len(encoded))
	}
	if len(encoded) != 12 {
		t.Errorf("encoded %d bytes, want 12", len(encoded))
	}

	var out timed
	if _, err := Decode(encoded, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !out.Stamp.Equal(in.Stamp) || out.Dur != in.Dur {
		t.Errorf("round trip mismatch: got %+v, want %+v", out, in)
	}
}
