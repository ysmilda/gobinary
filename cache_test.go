package gobinary

import (
	"sync"
	"testing"
)

func TestConcurrentAccess(t *testing.T) {
	type shared struct {
		ID   uint32
		Name string `binary:"zeroTerminated"`
		Raw  []byte `binary:"lengthRef:ID"`
	}

	var wg sync.WaitGroup
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			encoded, err := Encode(shared{ID: 2, Name: "n", Raw: []byte{7, 8}})
			if err != nil {
				t.Errorf("encode: %v", err)
				return
			}

			var got shared
			if _, err := Decode(encoded, &got); err != nil {
				t.Errorf("decode: %v", err)
				return
			}
			if got.ID != 2 || got.Name != "n" || len(got.Raw) != 2 {
				t.Errorf("got %+v", got)
			}
		}()
	}
	wg.Wait()
}
