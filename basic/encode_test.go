package basic

import (
	"reflect"
	"testing"
)

func TestWriteBytes(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		bufSize int
		wantErr bool
	}{
		{"Simple", []byte{1, 2, 3}, 3, false},
		{"Empty", []byte{}, 0, false},
		{"Overflow", []byte{1, 2, 3}, 2, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			d := EncoderState{Data: make([]byte, tt.bufSize)}
			err := d.WriteBytes(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteBytes() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && !reflect.DeepEqual(d.Data[:d.Index], tt.data) {
				t.Errorf("WriteBytes() got = %v, want %v", d.Data[:d.Index], tt.data)
			}
		})
	}
}

func TestEncode(t *testing.T) {
	tests := []struct {
		name    string
		val     DbufValSeq
		want    []byte
		wantErr bool
		bufSize int // if 0, calculated from want
	}{
		{
			name: "Uint",
			val:  DbufValSeq{Val: Uint(5)},
			want: []byte{UintByte(5)},
		},
		{
			name: "Registry",
			val:  DbufValSeq{Val: Registry(10)},
			want: []byte{RegistryByte(10)},
		},
		{
			name: "String",
			val:  Text("abc").ToDbufSeq(),
			want: []byte{TextByte(3), 'a', 'b', 'c'},
		},
		{
			name: "Bytes",
			val:  Bytes([]byte{1, 2}).ToDbufSeq(),
			want: []byte{BytesByte(2), 0x01, 0x02},
		},
		{
			name: "Integer",
			val:  Integer([]byte{0x01, 0x02, 0x03, 0x04, 0x05}).ToDbufSeq(),
			want: []byte{IntegerByte(5), 0x01, 0x02, 0x03, 0x04, 0x05},
		},
		{
			name: "Float",
			val:  Float([]byte{0x01, 0x02, 0x03, 0x04, 0x05}).ToDbufSeq(),
			want: []byte{FloatByte(5), 0x01, 0x02, 0x03, 0x04, 0x05},
		},
		{
			name: "Array",
			val:  ArraySlice([]byte{1, 2}).ToDbufSeq(),
			want: []byte{ArrayByte(2), 0x01, 0x02},
		},
		{
			name: "Map",
			val:  MapSlice([]byte{1, 2}).ToDbufSeq(),
			want: []byte{MapByte(2), 0x01, 0x02},
		},
		{
			name: "ArraySequence",
			val: DbufValSeq{
				Val: Array(),
				Sequence: []DbufValSeq{
					{Val: Uint(1)},
					{Val: Uint(2)},
				},
			},
			want: []byte{ArrayByte(2), UintByte(1), UintByte(2)},
		},
		{
			name: "MapSequence",
			val: DbufValSeq{
				Val: Map(),
				Sequence: []DbufValSeq{
					{Val: Uint(1)},
					{Val: Uint(2)},
				},
			},
			want: []byte{MapByte(2), UintByte(1), UintByte(2)},
		},
		{
			name:    "BufferTooSmall",
			val:     DbufValSeq{Val: Uint(5)},
			want:    []byte{UintByte(5)},
			bufSize: 0, // Force error (size 0)
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sz := len(tt.want)
			if tt.bufSize == 0 && tt.wantErr {
				sz = 0
			}
			d := EncoderState{Data: make([]byte, sz)}
			err := d.EncodeSeq(tt.val)
			if (err != nil) != tt.wantErr {
				t.Errorf("Encode() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if !reflect.DeepEqual(d.Data[:d.Index], tt.want) {
					t.Errorf("Encode() got = %x, want %x", d.Data[:d.Index], tt.want)
				}
			}
		})
	}
}

func TestMeasure(t *testing.T) {
	if got := Measure(5, 24, false); got != 1 {
		t.Errorf("Measure(5, 24, false) = %d; want 1", got)
	}
	if got := Measure(5, 24, true); got != 6 {
		t.Errorf("Measure(5, 24, true) = %d; want 6", got)
	}
	if got := Measure(24, 24, false); got != 2 {
		t.Errorf("Measure(24, 24, false) = %d; want 2", got)
	}
	if got := Measure(24, 24, true); got != 26 {
		t.Errorf("Measure(24, 24, true) = %d; want 26", got)
	}
}

func TestMeasureValue(t *testing.T) {
	v1 := DbufValSeq{Val: Uint(5)}
	if got, _ := MeasureValue(v1, false); got != 1 {
		t.Errorf("MeasureValue(Uint(5)) = %d; want 1", got)
	}

	v2 := Text("abc").ToDbufSeq()
	if got, _ := MeasureValue(v2, false); got != 4 {
		t.Errorf("MeasureValue(String) = %d; want 4", got)
	}

	v3 := DbufValSeq{
		Val: Array(),
		Sequence: []DbufValSeq{
			{Val: Uint(1)},
			{Val: Uint(2)},
		},
	}
	if got, _ := MeasureValue(v3, false); got != 2 {
		t.Errorf("MeasureValue(Sequence, false) = %d; want 2", got)
	}
	if got, _ := MeasureValue(v3, true); got != 3 {
		t.Errorf("MeasureValue(Sequence, true) = %d; want 3", got)
	}
	v4 := DbufValSeq{
		Val:      Array(),
		Sequence: []DbufValSeq{},
	}
	if got, _ := MeasureValue(v4, false); got != 0 {
		t.Errorf("MeasureValue(EmptySequence, false) = %d; want 0", got)
	}
	if got, _ := MeasureValue(v4, true); got != 1 {
		t.Errorf("MeasureValue(EmptySequence, true) = %d; want 1", got)
	}

	v5 := Text("").ToDbufSeq()
	if got, _ := MeasureValue(v5, false); got != 1 {
		t.Errorf("MeasureValue(String Empty) = %d; want 4", got)
	}
}
