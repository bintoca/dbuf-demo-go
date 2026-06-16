package basic

import (
	"reflect"
	"strings"
	"testing"
)

func TestMagicNumber(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want uint32
	}{
		{"M1", []byte{0xDE, 0xDE, 0xDE, 0xDE}, Registry_magic_number},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DecoderState{Data: tt.data}
			ini := InitialDecode{}
			d.DecodeInitial(&ini)
			if ini.Type != Parse_type_registry {
				t.Errorf("MagicNumber() = %v, want %v", ini.Type, Parse_type_registry)
			}
			if ini.Value != tt.want {
				t.Errorf("MagicNumber() = %v, want %v", ini.Value, tt.want)
			}
		})
	}
}
func TestDecode(t *testing.T) {
	tests := []struct {
		name      string
		data      []byte
		wantType  uint32
		wantVal   uint32
		wantSlice []byte
		wantErr   bool
	}{
		{"Uint", []byte{UintByte(5)}, Parse_type_uint, 5, nil, false},
		{"Uint32", []byte{UintByte(31), 0xF1, 0x02, 0x03, 0x04}, Parse_type_uint, 4043440900, nil, false},
		{"Registry", []byte{RegistryByte(10)}, Parse_type_registry, 10, nil, false},
		{"Registry32", []byte{RegistryByte(31), 0xF1, 0x02, 0x03, 0x04}, Parse_type_registry, 4043440900, nil, false},
		{"String", []byte{TextByte(3), 'a', 'b', 'c'}, Parse_type_text, 0, []byte("abc"), false},
		{"Bytes", []byte{BytesByte(2), 0x01, 0x02}, Parse_type_bytes, 0, []byte{0x01, 0x02}, false},
		{"Array", []byte{ArrayByte(2), 0x01, 0x02}, Parse_type_array, 0, []byte{0x01, 0x02}, false},
		{"Map", []byte{MapByte(2), 0x01, 0x02}, Parse_type_map, 0, []byte{0x01, 0x02}, false},
		{"Integer", []byte{IntegerByte(5), 0x01, 0x02, 0x03, 0x04, 0x05}, Parse_type_integer, 0, []byte{0x01, 0x02, 0x03, 0x04, 0x05}, false},
		{"Float", []byte{FloatByte(5), 0x01, 0x02, 0x03, 0x04, 0x05}, Parse_type_float, 0, []byte{0x01, 0x02, 0x03, 0x04, 0x05}, false},
		{"Incomplete", []byte{TextByte(3), 'a', 'b'}, 0, 0, nil, true},
		{"IncompleteVal", []byte{TextByte(31), 0x01}, 0, 0, nil, true},
		{"Incomplete0", []byte{}, 0, 0, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := DecoderState{Data: tt.data}
			v, bytes, err := d.Decode()
			if tt.wantErr && !strings.Contains(err.Error(), "R:70") {
				t.Errorf("Decode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				got := v.GetDbufVal(bytes)
				if got.GetType() != tt.wantType {
					t.Errorf("Decode() Type = %v, want %v", got.GetType(), tt.wantType)
				}
				if got.GetValue() != tt.wantVal {
					t.Errorf("Decode() Val = %v, want %v", got.GetValue(), tt.wantVal)
				}
				if !reflect.DeepEqual(got.Slice, tt.wantSlice) {
					t.Errorf("Decode() Slice = %v, want %v", got.Slice, tt.wantSlice)
				}
			}
		})
	}
}

func TestDecodeSequence(t *testing.T) {
	data := []byte{UintByte(5), RegistryByte(10)}
	d := DecoderState{Data: data}
	seq, err := d.DecodeSequence()
	if err != nil {
		t.Fatalf("DecodeSequence() error = %v", err)
	}
	if len(seq) != 2 {
		t.Errorf("DecodeSequence() len = %d, want 2", len(seq))
	}
	if seq[0].GetType() != Parse_type_uint {
		t.Errorf("Item 0 Type = %v", seq[0].GetType())
	}
	if seq[0].GetValue() != 5 {
		t.Errorf("Item 0 Val = %v", seq[0].GetValue())
	}
	if seq[1].GetType() != Parse_type_registry {
		t.Errorf("Item 1 Type = %v", seq[1].GetType())
	}
	if seq[1].GetValue() != 10 {
		t.Errorf("Item 1 Val = %v", seq[1].GetValue())
	}
}

func TestValidateKeysUnique(t *testing.T) {
	// Small sequence
	seqSmall := []DbufVal{
		Uint(1), Uint(10),
		Uint(2), Uint(20),
		Uint(1), Uint(30), // Duplicate key Uint(1)
	}
	if err := ValidateKeysUnique(seqSmall); err == nil {
		t.Error("ValidateKeysUnique(small) expected error for duplicate keys")
	}

	seqSmallOk := []DbufVal{
		Uint(1), Uint(10),
		Uint(2), Uint(20),
	}
	if err := ValidateKeysUnique(seqSmallOk); err != nil {
		t.Errorf("ValidateKeysUnique(small) unexpected error: %v", err)
	}

	// Large sequence
	vals := []DbufVal{}
	for i := 0; i < 20; i++ {
		vals = append(vals, Uint(uint32(i)), Uint(uint32(i*10)))
	}
	// Add duplicate
	vals = append(vals, Uint(0), Uint(999))

	if err := ValidateKeysUnique(vals); err == nil {
		t.Error("ValidateKeysUnique(large) expected error for duplicate keys")
	}

	// Large sequence OK
	valsOk := []DbufVal{}
	for i := 0; i < 20; i++ {
		valsOk = append(valsOk, Uint(uint32(i)), Uint(uint32(i*10)))
	}
	// Add collision on sh (same type/length) but different content
	// String "abc" and "abd"
	s1 := Text("abc")
	s2 := Text("abd")
	valsOk = append(valsOk, s1, Uint(1), s2, Uint(2))

	if err := ValidateKeysUnique(valsOk); err != nil {
		t.Errorf("ValidateKeysUnique(large) unexpected error: %v", err)
	}
}

func TestCompare(t *testing.T) {
	v1 := Uint(10)
	v2 := Uint(10)
	v3 := Uint(20)
	v4 := Registry(10)
	v5 := Text("abc")
	v6 := Text("abd")

	if Compare(v1, v2) != 0 {
		t.Error("Compare(v1, v2) != 0")
	}
	if !v1.Equal(v2) {
		t.Error("v1.Equal(v2) is false")
	}
	if Compare(v1, v3) >= 0 {
		t.Error("Compare(v1, v3) >= 0")
	}
	if Compare(v1, v4) == 0 {
		t.Error("Compare(v1, v4) == 0 (different types)")
	}
	if Compare(v5, v6) >= 0 {
		t.Error("Compare(v5, v6) >= 0")
	}
	if Compare(v6, v5) <= 0 {
		t.Error("Compare(v5, v6) >= 0")
	}
}

func TestFindKey(t *testing.T) {
	seq := []DbufVal{
		Uint(1), Uint(10),
		Uint(2), Uint(20),
	}

	idx := FindKey(seq, Uint(2))
	if idx != 2 {
		t.Errorf("FindKey(2) index = %d, want 2", idx)
	}

	idx = FindKey(seq, Uint(3))
	if idx != -1 {
		t.Errorf("FindKey(3) index = %d, want -1", idx)
	}
}

func UintByte(b byte) byte {
	return (Parse_type_uint << 5) + b
}
func IntegerByte(b byte) byte {
	return (Parse_type_integer << 5) + b
}
func ArrayByte(b byte) byte {
	return (Parse_type_array << 5) + b
}
func MapByte(b byte) byte {
	return (Parse_type_map << 5) + b
}
func TextByte(b byte) byte {
	return (Parse_type_text << 5) + b
}
func BytesByte(b byte) byte {
	return (Parse_type_bytes << 5) + b
}
func RegistryByte(b byte) byte {
	return (Parse_type_registry << 5) + b
}
func FloatByte(b byte) byte {
	return (Parse_type_float << 5) + b
}
func TestCheckComplete(t *testing.T) {
	tests := []struct {
		name         string
		data         [][]byte
		lengthKnown  bool
		length       uint64
		useEndMarker bool
		want         byte
	}{
		{
			name:        "SimpleUint",
			data:        [][]byte{{UintByte(5)}},
			lengthKnown: true,
			length:      1,
			want:        CheckComplete_Done,
		},
		{
			name: "SimpleUintUnknownLength",
			data: [][]byte{{UintByte(5)}},
			want: CheckComplete_Continue,
		},
		{
			name:         "SimpleUintWithMarker",
			data:         [][]byte{{UintByte(5), RegistryByte(Single_byte_threshold), Registry_end_marker}},
			useEndMarker: true,
			want:         CheckComplete_Done,
		},
		{
			name:        "IncompleteUint",
			data:        [][]byte{{UintByte(31)}},
			lengthKnown: true,
			length:      2,
			want:        CheckComplete_Continue,
		},
		{
			name:        "SplitUintExtra",
			data:        [][]byte{{UintByte(Single_byte_threshold)}, {0x01}},
			lengthKnown: true,
			length:      2,
			want:        CheckComplete_NonCanonical_InitialDecode,
		},
		{
			name:        "ArrayDone",
			data:        [][]byte{{ArrayByte(2), UintByte(5), UintByte(5)}},
			lengthKnown: true,
			length:      3,
			want:        CheckComplete_Done,
		},
		{
			name:        "TextDone",
			data:        [][]byte{{TextByte(2), UintByte(5), UintByte(5)}},
			lengthKnown: true,
			length:      3,
			want:        CheckComplete_Done,
		},
		{
			name:        "TextSplitDone",
			data:        [][]byte{{TextByte(2)}, {31, 27}},
			lengthKnown: true,
			length:      3,
			want:        CheckComplete_Done,
		},
		{
			name:        "TextSplitDone2",
			data:        [][]byte{{TextByte(2), UintByte(5)}, {UintByte(5)}},
			lengthKnown: true,
			length:      3,
			want:        CheckComplete_Done,
		},
		{
			name:        "ArrayIncomplete",
			data:        [][]byte{{ArrayByte(2), UintByte(5)}},
			lengthKnown: true,
			length:      3,
			want:        CheckComplete_Continue,
		},
		{
			name:        "TotalLengthExceeded_InitialDecode",
			data:        [][]byte{{UintByte(Single_byte_threshold), 0x21}},
			lengthKnown: true,
			length:      1,
			want:        CheckComplete_TotalLengthExceeded_InitialDecode,
		},
		{
			name:        "TotalLengthExceeded_Slice",
			data:        [][]byte{{TextByte(2), 0x01}},
			lengthKnown: true,
			length:      1,
			want:        CheckComplete_TotalLengthExceeded_Slice,
		},
		{
			name: "NestedLengthExceeded_TopOfStackComplete",
			data: [][]byte{{ArrayByte(1), ArrayByte(1)}},
			want: CheckComplete_NestedLengthExceeded_TopOfStackComplete,
		},
		{
			name: "NestedLengthExceeded_AppendToStack",
			data: [][]byte{{ArrayByte(2), ArrayByte(2)}},
			want: CheckComplete_NestedLengthExceeded_AppendToStack,
		},
		{
			name: "NestedLengthExceeded_InitialDecode",
			data: [][]byte{{ArrayByte(2), UintByte(28), UintByte(5), UintByte(5)}},
			want: CheckComplete_NestedLengthExceeded_InitialDecode,
		},
		{
			name:        "NestingDone",
			data:        [][]byte{{ArrayByte(4), ArrayByte(3), ArrayByte(2), UintByte(5), UintByte(5)}},
			lengthKnown: true,
			length:      5,
			want:        CheckComplete_Done,
		},
		{
			name:        "NestingDone2",
			data:        [][]byte{{ArrayByte(5), ArrayByte(4), ArrayByte(3), TextByte(2), UintByte(5), UintByte(5)}},
			lengthKnown: true,
			length:      6,
			want:        CheckComplete_Done,
		},
		{
			name: "MapCountOdd",
			data: [][]byte{{MapByte(1), UintByte(5)}},
			want: CheckComplete_MapCountOdd,
		},
		{
			name: "MapCountOdd2",
			data: [][]byte{{MapByte(3), UintByte(5), UintByte(5), UintByte(5)}},
			want: CheckComplete_MapCountOdd,
		},
		{
			name: "MapCountOdd3",
			data: [][]byte{{MapByte(6), MapByte(5), UintByte(5), MapByte(3), UintByte(5), TextByte(1), 0x01}},
			want: CheckComplete_MapCountOdd,
		},
		{
			name:        "MapDone",
			data:        [][]byte{{MapByte(5), UintByte(5), MapByte(3), UintByte(5), TextByte(1), 0x01}},
			lengthKnown: true,
			length:      6,
			want:        CheckComplete_Done,
		},
		{
			name: "NonCanonical_InitialDecode",
			data: [][]byte{{UintByte(Single_byte_threshold), 0x01}},
			want: CheckComplete_NonCanonical_InitialDecode,
		},
		{
			name: "Invalid_UTF8",
			data: [][]byte{{TextByte(2), 0, 248}},
			want: CheckComplete_Invalid_UTF8,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := CheckCompleteState{
				UseLength:    tt.lengthKnown,
				Length:       tt.length,
				UseEndMarker: tt.useEndMarker,
			}
			var got byte
			for _, chunk := range tt.data {
				got = s.CheckComplete(chunk)
				if got != CheckComplete_Continue && got != CheckComplete_Done {
					break
				}
			}
			if got != tt.want {
				t.Errorf("CheckComplete() = %v, want %v", got, tt.want)
			}
		})
	}
}
