package gobinary

import (
	"errors"
	"reflect"
	"testing"
)

type request struct {
	Group   uint32
	Offset  uint32
	RLength uint32
	WLength uint32
	Data    []byte `binary:"lengthRef:WLength"`
}

type reply struct {
	Result uint32
	Length uint32
	Data   []byte `binary:"lengthRef:Length"`
}

func TestLengthRef(t *testing.T) {
	encoded, err := Encode(request{Group: 0x4040, Offset: 0x10, RLength: 4, Data: []byte{1, 2, 3}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	want := []byte{0x40, 0x40, 0x00, 0x00, 0x10, 0x00, 0x00, 0x00, 0x04, 0x00, 0x00, 0x00, 0x03, 0x00, 0x00, 0x00, 1, 2, 3}
	if string(encoded) != string(want) {
		t.Errorf("encoded % X, want % X", encoded, want)
	}
}

func TestLengthRefCountsElements(t *testing.T) {
	type frame struct {
		Count uint8
		Data  []uint16 `binary:"lengthRef:Count"`
	}

	encoded, err := Encode(frame{Data: []uint16{0x00FF, 0x00AA}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	want := []byte{0x02, 0xFF, 0x00, 0xAA, 0x00}
	if string(encoded) != string(want) {
		t.Fatalf("encoded % X, want % X", encoded, want)
	}

	var got frame
	if _, err := Decode(encoded, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(got.Data, []uint16{0x00FF, 0x00AA}) {
		t.Errorf("round trip data %v, want [255 170]", got.Data)
	}
}

func TestRoundTripWide(t *testing.T) {
	in := wideValue()

	encoded, err := Encode(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if got := Size(in); got != len(encoded) {
		t.Errorf("Size is %d, encoded is %d", got, len(encoded))
	}

	var out wideHeader
	if _, err := Decode(encoded, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
}

func TestSliceOfStringsElementLength(t *testing.T) {
	type strs struct {
		Strs []string `binary:"length:2,elementLength:5,zeroTerminated"`
	}
	in := strs{Strs: []string{"test", "other"}}

	encoded, err := Encode(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := []byte{'t', 'e', 's', 't', 0, 'o', 't', 'h', 'e', 'r'}
	if !reflect.DeepEqual(encoded, want) {
		t.Errorf("encoded % X, want % X", encoded, want)
	}
	if got := Size(in); got != len(encoded) {
		t.Errorf("Size is %d, encoded is %d", got, len(encoded))
	}

	var out strs
	if _, err := Decode(encoded, &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !reflect.DeepEqual(in, out) {
		t.Errorf("round trip mismatch:\n in=%+v\nout=%+v", in, out)
	}
}

func TestDecode(t *testing.T) {
	buf := []byte{0, 0, 0, 0, 4, 0, 0, 0, 7, 8, 9, 10}

	var got reply
	n, err := Decode(buf, &got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if n != len(buf) {
		t.Errorf("consumed %d bytes, want %d", n, len(buf))
	}
	if got.Length != 4 {
		t.Errorf("length %d, want 4", got.Length)
	}
	if string(got.Data) != string([]byte{7, 8, 9, 10}) {
		t.Errorf("data % X, want 07 08 09 0A", got.Data)
	}
}

func TestLittleEndian(t *testing.T) {
	type scalars struct {
		A uint16
		B uint32
		C int64
	}

	encoded, err := Encode(scalars{A: 0x0102, B: 0x03040506, C: -1})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	want := []byte{0x02, 0x01, 0x06, 0x05, 0x04, 0x03, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	if string(encoded) != string(want) {
		t.Errorf("encoded % X, want % X", encoded, want)
	}
}

func TestZeroTerminatedString(t *testing.T) {
	type named struct {
		Pad  uint32
		Name string `binary:"zeroTerminated"`
	}

	encoded, err := Encode(named{Name: "GVL.x"})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if want := "\x00\x00\x00\x00" + "GVL.x" + "\x00"; string(encoded) != want {
		t.Errorf("encoded % X, want % X", encoded, want)
	}

	var got named
	n, err := Decode(append(encoded, 0xFF), &got)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if n != len(encoded) {
		t.Errorf("consumed %d bytes, want %d", n, len(encoded))
	}
	if got.Name != "GVL.x" {
		t.Errorf("got %q, want %q", got.Name, "GVL.x")
	}
}

func TestFixedStringIsTrimmed(t *testing.T) {
	type named struct {
		Name string `binary:"length:16"`
	}

	var got named
	padded := append([]byte("Plc"), make([]byte, 13)...)
	if _, err := Decode(padded, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Name != "Plc" {
		t.Errorf("got %q, want %q", got.Name, "Plc")
	}
}

func TestSliceWithoutLength(t *testing.T) {
	type row struct {
		Values []uint16
	}

	encoded, err := Encode(row{Values: []uint16{1, 2, 3}})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(encoded) != 6 {
		t.Fatalf("encoded %d bytes, want 6", len(encoded))
	}

	var got row
	if _, err := Decode(encoded, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(got.Values) != 3 || got.Values[2] != 3 {
		t.Errorf("got %v", got.Values)
	}
}

func TestSkip(t *testing.T) {
	type padded struct {
		ID    uint16 `binary:"skip:2"`
		Value uint16
	}

	buf := []byte{0x11, 0x11, 0xFF, 0xFF, 0x22, 0x22}
	var got padded
	if _, err := Decode(buf, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != 0x1111 || got.Value != 0x2222 {
		t.Errorf("got {ID:%x Value:%x}, want {1111 2222}", got.ID, got.Value)
	}
}

func TestTruncatedBuffer(t *testing.T) {
	var got reply
	if _, err := Decode([]byte{0, 0, 0, 0, 100, 0, 0, 0}, &got); err == nil {
		t.Error("a buffer that is too short was accepted")
	}
}

func TestSize(t *testing.T) {
	if got := Size(reply{}); got != 8 {
		t.Errorf("size of an empty reply is %d, want 8", got)
	}
	if got := Size(request{Data: []byte{1, 2, 3, 4, 5}}); got != 21 {
		t.Errorf("size is %d, want 21", got)
	}
}

type hunk struct {
	fixed []byte
}

func (h *hunk) BinaryDecode(buf []byte) (int, error) {
	if len(buf) < 2 {
		return 0, ErrTruncated
	}
	h.fixed = buf[:2]
	return 2, nil
}

func (h *hunk) BinaryEncode() ([]byte, error) {
	return h.fixed, nil
}

func TestDecoderHook(t *testing.T) {
	type wrapped struct {
		ID   uint16
		Raw  hunk
		Rest uint16
	}

	buf := []byte{0x11, 0x11, 0xAA, 0xBB, 0x22, 0x22}
	var got wrapped
	if _, err := Decode(buf, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.ID != 0x1111 {
		t.Errorf("id %x, want 1111", got.ID)
	}
	if string(got.Raw.fixed) != string([]byte{0xAA, 0xBB}) {
		t.Errorf("raw bytes % X, want AA BB", got.Raw.fixed)
	}
	if got.Rest != 0x2222 {
		t.Errorf("rest %x, want 2222", got.Rest)
	}

	encoded, err := Encode(got)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if string(encoded) != string(buf) {
		t.Errorf("encoded % X, want % X", encoded, buf)
	}
	if size := Size(got); size != len(buf) {
		t.Errorf("size %d, want %d", size, len(buf))
	}
}

func TestTopLevelEncoder(t *testing.T) {
	in := hunk{fixed: []byte{0x01, 0x02}}

	encoded, err := Encode(in)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if string(encoded) != string([]byte{0x01, 0x02}) {
		t.Errorf("encoded % X, want 01 02", encoded)
	}
	if size := Size(in); size != 2 {
		t.Errorf("size %d, want 2", size)
	}
}

func TestPointerField(t *testing.T) {
	type optional struct {
		Flag uint8
		Val  *uint32
	}

	buf := []byte{1, 0xC0, 0xDE, 0x00, 0x00}
	var got optional
	if _, err := Decode(buf, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Val == nil {
		t.Fatal("pointer field stayed nil")
	}
	if *got.Val != 0xDEC0 {
		t.Errorf("got %x, want DEC0", *got.Val)
	}

	encoded, err := Encode(optional{Flag: 1, Val: &val42})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(encoded) != 5 {
		t.Errorf("encoded %d bytes, want 5", len(encoded))
	}
}

var val42 uint32 = 42

func TestNilPointerErrors(t *testing.T) {
	type optional struct {
		Val *uint32
	}

	if _, err := Encode(optional{}); !errors.Is(err, ErrUnsupportedType) {
		t.Errorf("encode of nil pointer: %v, want ErrUnsupportedType", err)
	}
}

func TestUnexportedFieldsAreSkipped(t *testing.T) {
	type mixed struct {
		A uint16
		_ uint16
		B uint16
	}

	encoded, err := Encode(mixed{A: 1, B: 3})
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if len(encoded) != 4 {
		t.Errorf("encoded %d bytes, want 4", len(encoded))
	}
}

func TestMalformedNumericTag(t *testing.T) {
	tests := []struct {
		name string
		typ  any
	}{
		{"length", struct {
			Raw []byte `binary:"length:abc"`
		}{}},
		{"elementLength", struct {
			Raw []uint16 `binary:"length:2,elementLength:x"`
		}{}},
		{"skip", struct {
			A uint16 `binary:"skip:"`
			B uint16
		}{}},
		{"negative", struct {
			Raw []byte `binary:"length:-1"`
		}{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := Encode(tt.typ); !errors.Is(err, ErrInvalidTag) {
				t.Errorf("encode error = %v, want ErrInvalidTag", err)
			}
			ptr := reflect.New(reflect.TypeOf(tt.typ)).Interface()
			if _, err := Decode(make([]byte, 16), ptr); !errors.Is(err, ErrInvalidTag) {
				t.Errorf("decode error = %v, want ErrInvalidTag", err)
			}
		})
	}
}
