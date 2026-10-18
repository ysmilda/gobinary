# Gobinary

This package offers a declarative binary encoder and decoder for Go structs. The layout of a value on the wire is described with struct tags, so a struct can be serialised and deserialised without any hand written packing code.

The codec is not tied to a single protocol. Values are read and written as little endian scalars, strings and sequences, and the tags describe how they map onto a buffer.

## Installation

The package can be installed using the `go get` command:

```bash
go get github.com/ysmilda/gobinary
```

## Usage

Tags are read from the `binary` key. `Encode` serialises a value, `Decode` reads it back and `Size` reports the encoded size.

```go
type request struct {
	Group   uint32
	Offset  uint32
	RLength uint32
	WLength uint32
	Data    []byte `binary:"lengthRef:WLength"`
}

encoded, err := gobinary.Encode(request{Group: 0x4040, Offset: 0x10, RLength: 4, Data: []byte{1, 2, 3}})
if err != nil {
	// ...
}

var out request
if _, err := gobinary.Decode(encoded, &out); err != nil {
	// ...
}
```

The write length is filled in from the data slice while encoding, so the caller can leave it at zero. The value passed to `Encode` is never modified and `Size` is guaranteed to agree with `Encode`, which lets a caller pre-allocate a buffer.

## Tags

| Tag              | Value  | Description                                                                                                                                     |
| ---------------- | ------ | ----------------------------------------------------------------------------------------------------------------------------------------------- |
| `length`         | number | Fixed byte length of a string, or the length of an array or slice. A byte sequence is measured in bytes, every other sequence in elements.     |
| `elementLength`  | number | Encoded size of a single element. Makes `length`/`lengthRef` count elements, and reads and writes variable sized elements at a fixed width.     |
| `lengthRef`      | field  | Names another field of the struct that holds the length. That field is filled in automatically when encoding.                                   |
| `unit`           | unit   | Unit of a `time.Time` or `time.Duration` field: `nanoseconds`, `microseconds`, `milliseconds`, `seconds`, `minutes`, `hours` or `days`.         |
| `skip`           | number | Number of bytes to skip after the field.                                                                                                        |
| `zeroTerminated` | -      | Reads a string up to the first NUL byte or `length` bytes, whichever comes first, and writes a NUL terminator. Applies to every element of a sequence. |
| `tillEnd`        | -      | Reads the remaining bytes of the buffer into a byte slice. Only allowed on the last field of a struct.                                          |
| `align`          | -      | Pads the field to a multiple of `elementLength`.                                                                                                |
| `align4`        | -      | Pads every element of a sequence to a multiple of four bytes. The name is misspelled to stay compatible with the m1 reference implementation.  |

Supported field types are booleans, signed and unsigned integers, floats, strings, byte slices, slices and arrays of any of these, structs, pointers and `time.Time` / `time.Duration`.

Tags are validated eagerly. An unknown or misplaced tag, an unknown unit or a `lengthRef` to a missing field is reported with `ErrInvalidTag` instead of silently changing the layout of a field.

## Custom types

A type can take over its own wire format by implementing `BinaryEncoder` and `BinaryDecoder`. Such a type is written and read verbatim and any tags on it are ignored. `BinaryDecode` reads from the start of the buffer and returns the number of bytes it consumed.

```go
type raw struct {
	data []byte
}

func (r *raw) BinaryEncode() ([]byte, error) {
	return r.data, nil
}

func (r *raw) BinaryDecode(buf []byte) (int, error) {
	if len(buf) < 4 {
		return 0, gobinary.ErrTruncated
	}
	r.data = buf[:4]
	return 4, nil
}
```

`Size` calls `BinaryEncode` to determine the encoded length, so an encoder has to be deterministic.

The full documentation can be found at [pkg.go.dev](https://pkg.go.dev/github.com/ysmilda/gobinary).
