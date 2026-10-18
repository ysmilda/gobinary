package gobinary

import (
	"reflect"
	"testing"
)

func TestAlign4(t *testing.T) {
	type aligned struct {
		Count uint8
		Items []uint16 `binary:"lengthRef:Count,align4"`
	}
	in := aligned{Count: 3, Items: []uint16{1, 2, 3}}

	encoded, err := Encode(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := []byte{3, 0x01, 0x00, 0x00, 0x00, 0x02, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00}
	if !reflect.DeepEqual(encoded, want) {
		t.Errorf("encoded % X, want % X", encoded, want)
	}
	if got := Size(in); got != len(encoded) {
		t.Errorf("Size is %d, encoded is %d", got, len(encoded))
	}

	var out aligned
	if _, err := Decode(encoded, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
}
