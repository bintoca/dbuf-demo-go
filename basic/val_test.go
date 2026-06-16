package basic

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"

	"github.com/x448/float16"
)

func TestGetU64(t *testing.T) {
	tests := []struct {
		name    string
		input   DbufVal
		want    uint64
		wantErr bool
	}{
		{"UintSmall", Uint(12345), 12345, false},
		{"IntSmall", Integer([]byte{0x01, 0x02}), 0, true},
		{"IntBig", Integer([]byte{0x00, 0x81, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}), 0x8102030405060708, false},
		{"IntBigErr", Integer([]byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}), 0x0102030405060708, true},
		{"IntMedium", Integer([]byte{0x00, 0xFF, 0x02, 0x03, 0x04}), 0xFF020304, false},
		{"Negative", Integer([]byte{0x80}), 0, true},
		{"Over9", Integer(make([]byte, 10)), 0, true},
		{"Big9", Integer([]byte{1, 255, 1, 2, 3, 4, 5, 6, 7}), 0, true},
		{"LeadingZero9", Integer([]byte{0, 1, 1, 2, 3, 4, 5, 6, 7}), 0, true},
		{"Empty", Integer([]byte{}), 0, true},
		{"InvalidType", Text("not a number"), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.GetU64()
			if (err != nil) != tt.wantErr {
				t.Errorf("%s: GetU64() error = %v, wantErr %v", tt.name, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("%s: GetU64() = %d, want %d", tt.name, got, tt.want)
			}
		})
	}
}

func TestFloat64(t *testing.T) {
	tests := []struct {
		name  string
		input float64
		want  []byte
	}{
		{"Pi", math.Pi, []byte{0x40, 0x09, 0x21, 0xFB, 0x54, 0x44, 0x2D, 0x18}},
		{"v64_short", 10000000.125, []byte{0x41, 0x63, 0x12, 0xD0, 0x04}},
		{"v32", 1000.125, []byte{0x44, 0x7A, 0x08}},
		{"PosInf", math.Inf(1), []byte{0x7C}},
		{"NegInf", math.Inf(-1), []byte{0xFC}},
		{"NaN", math.NaN(), []byte{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Float64(tt.input)
			if got.Type != Parse_type_float {
				t.Errorf("Float64 type = %d, want %d", got.Type, Parse_type_float)
			}
			if !bytes.Equal(got.Slice, tt.want) {
				t.Errorf("Float64 slice = %x, want %x", got.Slice, tt.want)
			}
		})
	}
}

func TestUint64(t *testing.T) {
	tests := []struct {
		name  string
		input uint64
		want  DbufVal
	}{
		{"Zero", 0, Uint(0)},
		{"MaxU32", 0xFFFFFFFF, Uint(0xFFFFFFFF)},
		{"AboveU32", 0x100000000, Integer([]byte{0x01, 0x00, 0x00, 0x00, 0x00})},
		{"AboveU32NeedsZero", 0x8000000000, Integer([]byte{0x00, 0x80, 0x00, 0x00, 0x00, 0x00})},
		{"MaxU63", 0x7FFFFFFFFFFFFFFF, Integer([]byte{0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})},
		{"AboveU63", 0x8000000000000000, Integer([]byte{0x00, 0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})},
		{"MaxU64", 0xFFFFFFFFFFFFFFFF, Integer([]byte{0x00, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Uint64(tt.input)
			if got.Type != tt.want.Type || got.Value != tt.want.Value || !bytes.Equal(got.Slice, tt.want.Slice) {
				t.Errorf("Uint64(%v) = {Type:%d, Val:%d, Slice:%x}, want {Type:%d, Val:%d, Slice:%x}",
					tt.input, got.Type, got.Value, got.Slice, tt.want.Type, tt.want.Value, tt.want.Slice)
			}
		})
	}
}
func TestInt64(t *testing.T) {
	tests := []struct {
		name  string
		input int64
		want  DbufVal
	}{
		{"Zero", 0, Uint(0)},
		{"PosSmall", 123, Uint(123)},
		{"PosMaxU32", 0xFFFFFFFF, Uint(0xFFFFFFFF)},
		{"PosAboveU32", 0x100000000, Integer([]byte{0x01, 0x00, 0x00, 0x00, 0x00})},
		{"PosNeedsLeadingZero", 0x8000000000, Integer([]byte{0x00, 0x80, 0x00, 0x00, 0x00, 0x00})},
		{"NegOne", -1, Integer([]byte{})},
		{"NegSmall", -128, Integer([]byte{0x80})},
		{"NegNeedsLeadingFF", -129, Integer([]byte{0xFF, 0x7F})},
		{"MaxInt64", 0x7FFFFFFFFFFFFFFF, Integer([]byte{0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF})},
		{"MinInt64", -0x8000000000000000, Integer([]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00})},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Int64(tt.input)
			if got.Type != tt.want.Type || got.Value != tt.want.Value || !bytes.Equal(got.Slice, tt.want.Slice) {
				t.Errorf("Int64(%v) = {Type:%d, Val:%d, Slice:%x}, want {Type:%d, Val:%d, Slice:%x}",
					tt.input, got.Type, got.Value, got.Slice, tt.want.Type, tt.want.Value, tt.want.Slice)
			}
		})
	}
}

func TestGetI64(t *testing.T) {
	tests := []struct {
		name    string
		input   DbufVal
		want    int64
		wantErr bool
	}{
		{"UintSmall", Uint(123), 123, false},
		{"IntPosSmall", Integer([]byte{0x7F}), 127, false},
		{"IntPosBoundary", Integer([]byte{0x00, 0x80}), 128, false},
		{"IntNegSmall", Integer([]byte{0x80}), -128, false},
		{"IntNegBoundary", Integer([]byte{0xFF, 0x7F}), -129, false},
		{"IntMax", Integer([]byte{0x7F, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}), 0x7FFFFFFFFFFFFFFF, false},
		{"IntMin", Integer([]byte{0x80, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}), -0x8000000000000000, false},
		{"Empty", Integer([]byte{}), -1, false},
		// Error cases
		{"RedundantZero", Integer([]byte{0x00, 0x01}), 0, true},
		{"RedundantFF", Integer([]byte{0xFF, 0x80}), 0, true},
		{"RedundantFFone", Integer([]byte{0xFF}), 0, true},
		{"TooLong", Integer(make([]byte, 9)), 0, true},
		{"InvalidType", Text("not a number"), 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.GetI64()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetI64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetI64() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("RoundTrip", func(t *testing.T) {
		val := int64(-9223372036854775808)
		dbuf := Int64(val)
		got, err := dbuf.GetI64()
		if err != nil || got != val {
			t.Errorf("RoundTrip failed: got %v, err %v", got, err)
		}
	})
}

func TestGetFloat64(t *testing.T) {
	// Helpers for constructing sub-precision floats
	f16Bytes := func(f float32) []byte {
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, float16.Fromfloat32(f).Bits())
		return b
	}
	f32Bytes := func(f float32) []byte {
		b := make([]byte, 4)
		binary.BigEndian.PutUint32(b, math.Float32bits(f))
		return b
	}

	tests := []struct {
		name     string
		input    DbufVal
		want     float64
		checkNaN bool
		wantErr  bool
	}{
		{"ValidFloat", Float64(1.2345), 1.2345, false, false},
		{"Zero", Float64(0.0), 0.0, false, false},
		{"Negative", Float64(-500.5), -500.5, false, false},
		{"EmptySliceNaN", Float(nil), 0, true, false},
		{"Float16", Float(f16Bytes(1.5)), 1.5, false, false},
		{"Float16_1Byte", Float([]byte{0x3C}), 1.0, false, false},
		{"Float32", Float(f32Bytes(1.25)), 1.25, false, false},
		{"Float32_3Bytes", Float([]byte{0x3F, 0xA0, 0x00}), 1.25, false, false},
		// Error cases
		{"InvalidType", Uint(123), 0, false, true},
		{"InvalidLengthLong", Float(make([]byte, 9)), 0, false, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := tt.input.GetFloat64()
			if (err != nil) != tt.wantErr {
				t.Errorf("GetFloat64() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.checkNaN {
				if !math.IsNaN(got) {
					t.Errorf("GetFloat64() = %v, want NaN", got)
				}
			} else if got != tt.want {
				t.Errorf("GetFloat64() = %v, want %v", got, tt.want)
			}
		})
	}

	t.Run("RoundTripSpecial", func(t *testing.T) {
		val := math.Pi
		dbuf := Float64(val)
		got, err := dbuf.GetFloat64()
		if err != nil || got != val {
			t.Errorf("RoundTrip failed: got %v, err %v", got, err)
		}
	})
}
