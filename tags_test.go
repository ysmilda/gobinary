package gobinary

import (
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestInvalidTags(t *testing.T) {
	tests := []struct {
		name string
		typ  any
	}{
		{"unknown token", struct {
			A uint16 `binary:"zeroTarminated"`
		}{}},
		{"duplicate token", struct {
			A []byte `binary:"length:1,length:2"`
		}{}},
		{"length without value", struct {
			A []byte `binary:"length"`
		}{}},
		{"flag with value", struct {
			A []byte `binary:"length:1,tillEnd:2"`
		}{}},
		{"unknown unit", struct {
			A time.Duration `binary:"length:4,unit:fortnights"`
		}{}},
		{"elementLength without length", struct {
			A []uint16 `binary:"elementLength:2"`
		}{}},
		{"align without elementLength", struct {
			A []uint16 `binary:"length:2,align"`
		}{}},
		{"align4 without length", struct {
			A []uint16 `binary:"align4"`
		}{}},
		{"align4 on byte slice", struct {
			A []byte `binary:"length:4,align4"`
		}{}},
		{"tillEnd not last", struct {
			A []byte `binary:"tillEnd"`
			B uint16
		}{}},
		{"zeroTerminated on scalar", struct {
			A uint32 `binary:"zeroTerminated"`
		}{}},
		{"tillEnd on string", struct {
			A string `binary:"tillEnd"`
		}{}},
		{"unit on scalar", struct {
			A uint32 `binary:"length:4,unit:seconds"`
		}{}},
		{"length on scalar", struct {
			A uint32 `binary:"length:4"`
		}{}},
		{"time without length", struct {
			A time.Duration `binary:"unit:seconds"`
		}{}},
		{"time with bad length", struct {
			A time.Duration `binary:"length:3"`
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

func TestDecoderIgnoresTags(t *testing.T) {
	type wrapped struct {
		Raw hunk `binary:"length:4,unit:seconds"`
	}

	buf := []byte{0xAA, 0xBB}
	var got wrapped
	if _, err := Decode(buf, &got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(got.Raw.fixed) != string([]byte{0xAA, 0xBB}) {
		t.Errorf("raw bytes % X, want AA BB", got.Raw.fixed)
	}
}
