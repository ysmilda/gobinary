package gobinary

import "testing"

func wideValue() wideHeader {
	return wideHeader{
		Type:   0x0102,
		Flags:  0x03,
		Name:   "sensor",
		Count:  3,
		Items:  []uint16{1, 2, 3},
		Start:  0x1122334455667788,
		Ratio:  1.5,
		Active: true,
		Label:  "ok",
	}
}

type wideHeader struct {
	Type     uint16
	Flags    uint8
	Reserved uint8  `binary:"skip:2"`
	Name     string `binary:"length:16,zeroTerminated"`
	Count    uint32
	Items    []uint16 `binary:"lengthRef:Count"`
	Start    uint64
	Ratio    float64
	Active   bool
	Label    string `binary:"zeroTerminated"`
}

func BenchmarkEncodeSmall(b *testing.B) {
	v := request{Group: 0x4040, Offset: 0x10, RLength: 4, Data: []byte{1, 2, 3}}

	b.ReportAllocs()
	for b.Loop() {
		if _, err := Encode(v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkEncodeWide(b *testing.B) {
	v := wideValue()

	b.ReportAllocs()
	for b.Loop() {
		if _, err := Encode(v); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeSmall(b *testing.B) {
	buf := []byte{0, 0, 0, 0, 4, 0, 0, 0, 7, 8, 9, 10}

	b.ReportAllocs()
	for b.Loop() {
		var got reply
		if _, err := Decode(buf, &got); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkDecodeWide(b *testing.B) {
	encoded, err := Encode(wideValue())
	if err != nil {
		b.Fatal(err)
	}

	b.ReportAllocs()
	for b.Loop() {
		var got wideHeader
		if _, err := Decode(encoded, &got); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkSizeSmall(b *testing.B) {
	v := request{Group: 0x4040, Offset: 0x10, RLength: 4, Data: []byte{1, 2, 3}}

	b.ReportAllocs()
	for b.Loop() {
		if Size(v) == 0 {
			b.Fatal("zero size")
		}
	}
}

func BenchmarkSizeWide(b *testing.B) {
	v := wideValue()

	b.ReportAllocs()
	for b.Loop() {
		if Size(v) == 0 {
			b.Fatal("zero size")
		}
	}
}
