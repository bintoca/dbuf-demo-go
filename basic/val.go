package basic

import (
	"cmp"
	"encoding/binary"
	"fmt"
	"github.com/x448/float16"
	"math"
)

type Optional[T any] struct {
	HasValue bool
	Value    T
}

type InitialDecode struct {
	Value              uint32
	Type               byte
	MissingExtraLength byte
	ExtraLength        byte
}

type DbufVal struct {
	Value uint32
	Type  byte
	Slice []byte
}

type DbufValString struct {
	Value  uint32
	Type   byte
	String string
}

type DbufValSeq struct {
	Val      DbufVal
	Sequence []DbufValSeq
}

func (v InitialDecode) TypeHasSlice() bool {
	return v.Type != Parse_type_registry && v.Type != Parse_type_uint
}
func (v InitialDecode) TypeHasNesting() bool {
	return v.Type == Parse_type_array || v.Type == Parse_type_map
}
func (v InitialDecode) IsEndMarker() bool {
	return v.Type == Parse_type_registry && v.Value == Registry_end_marker
}
func (d InitialDecode) GetDbufVal(parseBytes []byte) DbufVal {
	if parseBytes != nil {
		d.Value = 0
	}
	return DbufVal{Type: d.Type, Value: d.Value, Slice: parseBytes}
}

func Compare(a, b DbufVal) int {
	c := cmp.Or(
		cmp.Compare(a.GetType(), b.GetType()),
		cmp.Compare(a.GetValue(), b.GetValue()),
		cmp.Compare(len(a.Slice), len(b.Slice)),
	)
	if c != 0 {
		return c
	}
	for i := 0; i < len(a.Slice); i++ {
		if a.Slice[i] != b.Slice[i] {
			return cmp.Compare(a.Slice[i], b.Slice[i])
		}
	}
	return 0
}
func (v DbufVal) Equal(other DbufVal) bool {
	return Compare(v, other) == 0
}
func SliceEqual(a, b []DbufVal) bool {
	if len(a) != len(b) {
		return false
	}
	for i := 0; i < len(a); i++ {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}
func (v DbufVal) GetType() uint32 {
	return uint32(v.Type)
}
func (v DbufValString) GetType() uint32 {
	return uint32(v.Type)
}
func (v DbufVal) GetValue() uint32 {
	return v.Value
}
func (v DbufVal) GetU64() (uint64, error) {
	switch v.GetType() {
	case Parse_type_integer:
		if len(v.Slice) == 0 ||
			len(v.Slice) > 9 ||
			v.Slice[0] > 127 ||
			(v.Slice[0] < 128 && len(v.Slice) < 5) ||
			(len(v.Slice) < 9 && v.Slice[0] == 0 && v.Slice[1] < 128) {
			return 0, DataValueNotAccepted()
		}
		if len(v.Slice) == 9 {
			if v.Slice[0] != 0 || v.Slice[1] < 128 {
				return 0, DataValueNotAccepted()
			}
		}
		start := 0
		if v.Slice[0] == 0 {
			start = 1
		}
		var u64 uint64
		for i := start; i < len(v.Slice); i++ {
			u64 = u64<<8 + uint64(v.Slice[i])
		}
		return u64, nil
	case Parse_type_uint:
		return uint64(v.GetValue()), nil
	default:
		return 0, DataTypeNotAccepted()
	}
}

func (v DbufVal) GetI64() (int64, error) {
	switch v.GetType() {
	case Parse_type_uint:
		return int64(v.GetValue()), nil
	case Parse_type_integer:
		if len(v.Slice) == 0 {
			return -1, nil
		}
		if len(v.Slice) > 8 {
			return 0, DataValueNotAccepted()
		}
		if len(v.Slice) > 1 {
			if v.Slice[0] == 0 && v.Slice[1] < 0x80 {
				return 0, DataValueNotAccepted()
			}
			if v.Slice[0] == 0xFF && v.Slice[1] >= 0x80 {
				return 0, DataValueNotAccepted()
			}
		}
		if len(v.Slice) == 1 && v.Slice[0] == 0xFF {
			return 0, DataValueNotAccepted()
		}
		var res int64
		if v.Slice[0] >= 0x80 {
			res = -1
		}
		for _, b := range v.Slice {
			res = (res << 8) | int64(b)
		}
		return res, nil
	default:
		return 0, DataTypeNotAccepted()
	}
}

func (v DbufVal) GetFloat64() (float64, error) {
	if v.GetType() != Parse_type_float {
		return 0, DataTypeNotAccepted()
	}
	if len(v.Slice) > 8 {
		return 0, DataValueNotAccepted()
	}
	var b [8]byte
	copy(b[:], v.Slice)
	switch len(v.Slice) {
	case 0:
		return math.NaN(), nil
	case 1:
		fallthrough
	case 2:
		return float64(float16.Frombits(binary.BigEndian.Uint16(b[:])).Float32()), nil
	case 3:
		fallthrough
	case 4:
		return float64(math.Float32frombits(binary.BigEndian.Uint32(b[:]))), nil
	default:
		return math.Float64frombits(binary.BigEndian.Uint64(b[:])), nil
	}
}

func (v DbufVal) ToDbufValString() DbufValString {
	return DbufValString{Type: v.Type, Value: v.Value, String: string(v.Slice)}
}
func (v DbufValSeq) ToString(registryNames map[uint32]string) string {
	switch v.Val.GetType() {
	case Parse_type_text:
		return "T:" + string(v.Val.Slice)
	case Parse_type_bytes:
		return "B:" + fmt.Sprint(v.Val.Slice)
	case Parse_type_integer:
		return "I:" + fmt.Sprint(v.Val.Slice)
	case Parse_type_uint:
		return "U:" + fmt.Sprint(v.Val.GetValue())
	case Parse_type_registry:
		if registryNames != nil {
			if name, exists := registryNames[v.Val.GetValue()]; exists {
				return "R:" + name
			}
		}
		return "R:" + fmt.Sprint(v.Val.GetValue())
	case Parse_type_float:
		return "F:" + fmt.Sprint(v.Val.Slice)
	case Parse_type_array:
		if v.Sequence == nil {
			ds := DecoderState{Data: v.Val.Slice}
			sq, err := ds.DecodeSequence()
			if err != nil {
				return "AER:" + err.Error()
			}
			sqq := []DbufValSeq{}
			for _, v := range sq {
				sqq = append(sqq, v.ToDbufSeq())
			}
			return DbufValSeq{Val: v.Val, Sequence: sqq}.ToString(registryNames)
		}
		x := ""
		for _, v := range v.Sequence {
			x += v.ToString(registryNames) + ","
		}
		return "A:[" + x + "]"
	case Parse_type_map:
		if v.Sequence == nil {
			ds := DecoderState{Data: v.Val.Slice}
			sq, err := ds.DecodeSequence()
			if err != nil {
				return "MER:" + err.Error()
			}
			sqq := []DbufValSeq{}
			for _, v := range sq {
				sqq = append(sqq, v.ToDbufSeq())
			}
			return DbufValSeq{Val: v.Val, Sequence: sqq}.ToString(registryNames)
		}
		x := ""
		for _, v := range v.Sequence {
			x += v.ToString(registryNames) + ","
		}
		return "M:{" + x + "}"
	default:
		return "Unknown type"
	}
}

var DebugRegistryNames map[uint32]string

func (v DbufValSeq) DebugString() string {
	return v.ToString(DebugRegistryNames)
}
func Uint(n uint32) DbufVal {
	return DbufVal{Type: Parse_type_uint, Value: n}
}
func Integer(b []byte) DbufVal {
	return DbufVal{Type: Parse_type_integer, Slice: b}
}
func Registry(n uint32) DbufVal {
	return DbufVal{Type: Parse_type_registry, Value: n}
}
func Float(b []byte) DbufVal {
	return DbufVal{Type: Parse_type_float, Slice: b}
}
func Text(s string) DbufVal {
	return DbufVal{Type: Parse_type_text, Slice: []byte(s)}
}
func Bytes(b []byte) DbufVal {
	return DbufVal{Type: Parse_type_bytes, Slice: b}
}
func Map() DbufVal {
	return DbufVal{Type: Parse_type_map}
}
func Array() DbufVal {
	return DbufVal{Type: Parse_type_array}
}
func MapSlice(b []byte) DbufVal {
	return DbufVal{Type: Parse_type_map, Slice: b}
}
func ArraySlice(b []byte) DbufVal {
	return DbufVal{Type: Parse_type_array, Slice: b}
}
func Uint64(val uint64) DbufVal {
	if val <= maxU32 {
		return Uint(uint32(val))
	}
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], val)
	i := 0
	for i < 7 && b[i] == 0 {
		i++
	}
	if b[i] >= 0x80 {
		var b9 [9]byte
		binary.BigEndian.PutUint64(b9[1:], val)
		return Integer(b9[i:])
	}
	return Integer(b[i:])
}

func Float64(val float64) DbufVal {
	if math.IsNaN(val) {
		return Float([]byte{})
	}
	v32 := float32(val)
	if float64(v32) == val {
		v16 := float16.Fromfloat32(v32)
		if v16.Float32() == v32 {
			var b [2]byte
			binary.BigEndian.PutUint16(b[:], v16.Bits())
			if b[1] == 0 {
				return Float(b[:1])
			}
			return Float(b[:])
		}
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], math.Float32bits(v32))
		if b[3] == 0 {
			return Float(b[:3])
		}
		return Float(b[:])
	}
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], math.Float64bits(val))
	i := 8
	for i > 5 && b[i-1] == 0 {
		i--
	}
	return Float(b[:i])
}
func Int64(val int64) DbufVal {
	if val >= 0 && uint64(val) <= maxU32 {
		return Uint(uint32(val))
	}
	if val == -1 {
		return Integer([]byte{})
	}
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], uint64(val))
	i := 0
	if val >= 0 {
		for i < 7 && b[i] == 0 {
			i++
		}
		if b[i] >= 0x80 {
			var b9 [9]byte
			binary.BigEndian.PutUint64(b9[1:], uint64(val))
			return Integer(b9[i:])
		}
		return Integer(b[i:])
	}
	for i < 7 && b[i] == 0xff {
		i++
	}
	if b[i] < 0x80 {
		i--
	}
	return Integer(b[i:])
}
