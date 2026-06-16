package basic

import (
	"fmt"
)

type EncoderState struct {
	Data  []byte
	Index int
}

func (d *EncoderState) WriteValue(b byte, l uint32) DbufError {
	if l == 0 {
		if len(d.Data) <= d.Index {
			return IncompleteStream()
		}
		d.Data[d.Index] = b
		d.Index++
		return nil
	}
	if l < 1<<10 {
		if len(d.Data) <= d.Index+1 {
			return IncompleteStream()
		}
		l1 := byte(l>>8) + b
		d.Data[d.Index] = l1
		d.Index++
		d.Data[d.Index] = byte(l & 255)
		d.Index++
		return nil
	}
	if l < 1<<17 {
		if len(d.Data) <= d.Index+2 {
			return IncompleteStream()
		}
		l1 := byte(l>>16) + b + 4
		d.Data[d.Index] = l1
		d.Index++
		d.Data[d.Index] = byte((l >> 8) & 255)
		d.Index++
		d.Data[d.Index] = byte(l & 255)
		d.Index++
		return nil
	}
	if l < 1<<24 {
		if len(d.Data) <= d.Index+3 {
			return IncompleteStream()
		}
		d.Data[d.Index] = b + 6
		d.Index++
		d.Data[d.Index] = byte((l >> 16) & 255)
		d.Index++
		d.Data[d.Index] = byte((l >> 8) & 255)
		d.Index++
		d.Data[d.Index] = byte(l & 255)
		d.Index++
		return nil
	}
	if len(d.Data) <= d.Index+4 {
		return IncompleteStream()
	}
	d.Data[d.Index] = b + 7
	d.Index++
	d.Data[d.Index] = byte((l >> 24) & 255)
	d.Index++
	d.Data[d.Index] = byte((l >> 16) & 255)
	d.Index++
	d.Data[d.Index] = byte((l >> 8) & 255)
	d.Index++
	d.Data[d.Index] = byte(l)
	d.Index++
	return nil
}
func MeasureExtraValue(l uint32, singleByteThreshold byte) byte {
	if l < uint32(singleByteThreshold) {
		return 1
	}
	if l < 1<<10 {
		return 2
	}
	if l < 1<<17 {
		return 3
	}
	if l < 1<<24 {
		return 4
	}
	return 5
}
func (d *EncoderState) WriteInitial(parseType byte, parseValue uint32) error {
	b := parseType << 5
	if parseValue >= Single_byte_threshold {
		b += Single_byte_threshold
	} else {
		b += byte(parseValue)
		parseValue = 0
	}
	err := d.WriteValue(b, parseValue)
	if err != nil {
		return err
	}
	return nil
}
func (d *EncoderState) WriteBytes(b []byte) error {
	l := len(b)
	if len(d.Data) < d.Index+l {
		return IncompleteStream()
	}
	copy(d.Data[d.Index:d.Index+l], b)
	d.Index += l
	return nil
}
func (d *EncoderState) EncodeSeq(v DbufValSeq) error {
	parseType := byte(v.Val.GetType())
	parseValue := v.Val.GetValue()
	parseBytes := v.Val.Slice
	byteLength := len(v.Val.Slice)
	if byteLength != 0 {
		if byteLength > maxU32 {
			return fmt.Errorf("u32 length overflow (%d)", byteLength)
		}
		parseValue = uint32(byteLength)
	}
	if v.Sequence != nil {
		sequenceLength, err := MeasureValue(v, false)
		if err != nil {
			return err
		}
		if sequenceLength > maxU32 {
			return fmt.Errorf("u32 length overflow (%d)", sequenceLength)
		}
		parseValue = uint32(sequenceLength)
	}
	err := d.WriteInitial(parseType, parseValue)
	if err != nil {
		return err
	}
	if parseBytes != nil {
		err := d.WriteBytes([]byte(parseBytes))
		if err != nil {
			return err
		}
	}
	if v.Sequence != nil {
		for _, sv := range v.Sequence {
			err := d.EncodeSeq(sv)
			if err != nil {
				return err
			}
		}
	}
	return nil
}
func (d *EncoderState) Encode(v DbufVal) error {
	return d.EncodeSeq(v.ToDbufSeq())
}
func Measure(l uint32, singleByteThreshold byte, isString bool) uint64 {
	size := MeasureExtraValue(l, singleByteThreshold)
	if isString {
		return uint64(size) + uint64(l)
	}
	return uint64(size)
}
func MeasureValue(v DbufValSeq, nested bool) (uint64, error) {
	if v.Sequence == nil {
		extraValue := v.Val.GetValue()
		if v.Val.Slice != nil {
			if len(v.Val.Slice) > maxU32 {
				return 0, fmt.Errorf("u32 length overflow (%d)", len(v.Val.Slice))
			}
			extraValue = uint32(len(v.Val.Slice))
		}
		return Measure(extraValue, Single_byte_threshold, v.Val.Slice != nil), nil
	}
	l := uint64(0)
	for _, sv := range v.Sequence {
		size, err := MeasureValue(sv, true)
		if err != nil {
			return 0, err
		}
		l += size
	}
	if nested {
		if l > maxU32 {
			return 0, fmt.Errorf("u32 length overflow (%d)", l)
		}
		return Measure(uint32(l), Single_byte_threshold, true), nil
	}
	return l, nil
}
func EncodeFullSeq(val DbufValSeq) ([]byte, error) {
	size, err := MeasureValue(val, true)
	if err != nil {
		return nil, err
	}
	es := EncoderState{Data: make([]byte, size)}
	err = es.EncodeSeq(val)
	if err != nil {
		return nil, err
	}
	return es.Data, nil
}
func EncodeSequence(val ...DbufValSeq) ([]byte, error) {
	v := DbufValSeq{Sequence: val}
	size, err := MeasureValue(v, false)
	if err != nil {
		return nil, err
	}
	es := EncoderState{Data: make([]byte, size)}
	for _, sv := range v.Sequence {
		err := es.EncodeSeq(sv)
		if err != nil {
			return nil, err
		}
	}
	return es.Data, nil
}
func EncodeFull(val DbufVal) ([]byte, error) {
	return EncodeFullSeq(val.ToDbufSeq())
}
