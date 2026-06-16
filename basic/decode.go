package basic

type DecoderState struct {
	Data  []byte
	Index int
}

func (d *DecoderState) ReadExtraValue(l1 byte) (val uint32, missingExtraLength byte, extraLength byte) {
	if l1 < 4 {
		v := uint32(l1 & 3)
		if len(d.Data) <= d.Index {
			return v, 1, 1
		}
		v = v<<8 + uint32(d.Data[d.Index])
		d.Index++
		return v, 0, 1
	}
	if l1 < 6 {
		v := uint32(l1 & 1)
		if len(d.Data) <= d.Index {
			return v, 2, 2
		}
		v = v<<8 + uint32(d.Data[d.Index])
		d.Index++
		if len(d.Data) <= d.Index {
			return v, 1, 2
		}
		v = v<<8 + uint32(d.Data[d.Index])
		d.Index++
		return v, 0, 2
	}
	if l1 < 7 {
		v := uint32(0)
		if len(d.Data) <= d.Index {
			return v, 3, 3
		}
		v = v<<8 + uint32(d.Data[d.Index])
		d.Index++
		if len(d.Data) <= d.Index {
			return v, 2, 3
		}
		v = v<<8 + uint32(d.Data[d.Index])
		d.Index++
		if len(d.Data) <= d.Index {
			return v, 1, 3
		}
		v = v<<8 + uint32(d.Data[d.Index])
		d.Index++
		return v, 0, 3
	}
	v := uint32(0)
	if len(d.Data) <= d.Index {
		return v, 4, 4
	}
	v = v<<8 + uint32(d.Data[d.Index])
	d.Index++
	if len(d.Data) <= d.Index {
		return v, 3, 4
	}
	v = v<<8 + uint32(d.Data[d.Index])
	d.Index++
	if len(d.Data) <= d.Index {
		return v, 2, 4
	}
	v = v<<8 + uint32(d.Data[d.Index])
	d.Index++
	if len(d.Data) <= d.Index {
		return v, 1, 4
	}
	v = v<<8 + uint32(d.Data[d.Index])
	d.Index++
	return v, 0, 4
}
func (d *DecoderState) DecodeInitial(i *InitialDecode) {
	if i.MissingExtraLength > 0 {
		for {
			if len(d.Data) > d.Index && i.MissingExtraLength > 0 {
				i.Value = i.Value<<8 + uint32(d.Data[d.Index])
				d.Index++
				i.MissingExtraLength--
			} else {
				return
			}
		}
	}
	b := d.Data[d.Index]
	d.Index++
	i.Type = b >> 5
	i.Value = uint32(b & 31)
	if i.Value >= Single_byte_threshold {
		extraValue, missing, extraLength := d.ReadExtraValue(b & 7)
		i.MissingExtraLength = missing
		i.Value = extraValue
		i.ExtraLength = extraLength
	}
}
func (d *DecoderState) CheckCanonical(i *InitialDecode) byte {
	if i.MissingExtraLength > 0 {
		for {
			if len(d.Data) > d.Index && i.MissingExtraLength > 0 {
				i.Value = i.Value<<8 + uint32(d.Data[d.Index])
				d.Index++
				i.MissingExtraLength--
			} else {
				measuredLength := MeasureExtraValue(i.Value, Single_byte_threshold)
				if i.ExtraLength+1 != measuredLength {
					return CheckComplete_NonCanonical_InitialDecode
				}
				return CheckComplete_Done
			}
		}
	}
	b := d.Data[d.Index]
	d.Index++
	i.Type = b >> 5
	i.Value = uint32(b & 31)
	if i.Value >= Single_byte_threshold {
		extraValue, missing, length := d.ReadExtraValue(b & 7)
		i.Value = extraValue
		i.MissingExtraLength = missing
		i.ExtraLength = length
		if missing > 0 {
			return CheckComplete_Continue
		}
		measuredLength := MeasureExtraValue(i.Value, Single_byte_threshold)
		if i.ExtraLength+1 != measuredLength {
			return CheckComplete_NonCanonical_InitialDecode
		}
	}
	return CheckComplete_Done
}
func (r *DecoderState) ReadSlice(c uint32) []byte {
	remaining := uint32(len(r.Data) - r.Index)
	if remaining < c {
		b := r.Data[r.Index:]
		r.Index = len(r.Data)
		return b
	}
	newIndex := r.Index + int(c)
	b := r.Data[r.Index:newIndex]
	r.Index = newIndex
	return b
}
func (d *DecoderState) Decode() (InitialDecode, []byte, error) {
	if len(d.Data) <= d.Index {
		return InitialDecode{}, nil, IncompleteStream()
	}
	var v InitialDecode
	d.DecodeInitial(&v)
	if v.MissingExtraLength != 0 {
		return v, nil, IncompleteStream()
	}
	var parseBytes []byte
	if v.TypeHasSlice() {
		parseBytes = d.ReadSlice(v.Value)
		if uint32(len(parseBytes)) < v.Value {
			return v, nil, IncompleteStream()
		}
		v.Value = 0
	}
	return v, parseBytes, nil
}
func (d *DecoderState) DecodeSequence() ([]DbufVal, error) {
	seq := []DbufVal{}
	for d.Index < len(d.Data) {
		v, bytes, err := d.Decode()
		if err != nil {
			return seq, err
		}
		seq = append(seq, v.GetDbufVal(bytes))
	}
	return seq, nil
}
func ValidateKeysUnique(seq []DbufVal) DbufError {
	m := make(map[DbufValString]DbufVal)
	for i := 0; i < len(seq); i += 2 {
		sv := seq[i]
		svs := DbufValString{Type: sv.Type, Value: sv.Value, String: string(sv.Slice)}
		if _, exists := m[svs]; exists {
			return DataError(seq[i], DataKeyNotAccepted())
		} else {
			m[svs] = sv
		}
	}
	return nil
}

func FindKey(seq []DbufVal, val DbufVal) (keyIndex int) {
	for i := 0; i < len(seq); i++ {
		if i%2 == 0 {
			sv := seq[i]
			if sv.Equal(val) && i+1 < len(seq) {
				return i
			}
		}
	}
	return -1
}

type ReadState struct {
	State             DecoderState
	PreviousBytesRead int64
	Initial           InitialDecode
}

func (r *ReadState) TotalBytesRead() int64 {
	return r.PreviousBytesRead + int64(r.State.Index)
}
func (r *ReadState) SetBuffer(buf []byte) {
	r.PreviousBytesRead += int64(len(r.State.Data))
	r.State.Data = buf
	r.State.Index = 0
}

type CheckCompleteState struct {
	ReadState    ReadState
	Stack        []InitialDecode
	MapCounts    []uint32
	Length       uint64
	UseLength    bool
	UseEndMarker bool
	SliceMode    bool
}

const (
	CheckComplete_Done                                    = 0
	CheckComplete_Continue                                = 1
	CheckComplete_TotalLengthExceeded_InitialDecode       = 2
	CheckComplete_TotalLengthExceeded_Slice               = 3
	CheckComplete_NestedLengthExceeded_TopOfStackComplete = 4
	CheckComplete_NestedLengthExceeded_AppendToStack      = 5
	CheckComplete_NestedLengthExceeded_InitialDecode      = 6
	CheckComplete_MapCountOdd                             = 7
	CheckComplete_NonCanonical_InitialDecode              = 8
	CheckComplete_Invalid_UTF8                            = 9
)

func (s *CheckCompleteState) CheckComplete(buf []byte) byte {
	s.ReadState.SetBuffer(buf)
	for {
		if !s.SliceMode {
			if len(s.ReadState.State.Data) <= s.ReadState.State.Index {
				return CheckComplete_Continue
			}
			startIndex := s.ReadState.State.Index
			cc := s.ReadState.State.CheckCanonical(&s.ReadState.Initial)
			if cc > CheckComplete_Continue {
				return cc
			}
			bytesAdvanced := uint32(s.ReadState.State.Index - startIndex)
			if s.UseLength && s.Length < uint64(bytesAdvanced) {
				return CheckComplete_TotalLengthExceeded_InitialDecode
			}
			s.Length -= uint64(bytesAdvanced)
			if len(s.Stack) > 0 && s.Stack[len(s.Stack)-1].Value < bytesAdvanced {
				return CheckComplete_NestedLengthExceeded_InitialDecode
			}
			ini := s.ReadState.Initial
			if len(s.Stack) > 0 && s.Stack[len(s.Stack)-1].Type == Parse_type_map && ini.MissingExtraLength == 0 && !ini.TypeHasSlice() {
				s.MapCounts[len(s.MapCounts)-1]++
			}
			for i := len(s.Stack) - 1; i >= 0; i-- {
				s.Stack[i].Value -= bytesAdvanced
				if s.Stack[i].Value == 0 {
					if ini.TypeHasSlice() {
						return CheckComplete_NestedLengthExceeded_TopOfStackComplete
					}
					if s.Stack[i].Type == Parse_type_map {
						if s.MapCounts[len(s.MapCounts)-1]%2 == 1 {
							return CheckComplete_MapCountOdd
						}
						s.MapCounts = s.MapCounts[:len(s.MapCounts)-1]
					}
					s.Stack = s.Stack[:i]
					if i > 0 && s.Stack[i-1].Type == Parse_type_map {
						s.MapCounts[len(s.MapCounts)-1]++
					}
				}
			}
			if ini.MissingExtraLength > 0 {
				return CheckComplete_Continue
			}
			if ini.TypeHasSlice() {
				if len(s.Stack) > 0 && s.Stack[len(s.Stack)-1].Value < ini.Value {
					return CheckComplete_NestedLengthExceeded_AppendToStack
				}
				if ini.Type == Parse_type_map {
					s.MapCounts = append(s.MapCounts, 0)
				}
				s.Stack = append(s.Stack, ini)
				s.SliceMode = !ini.TypeHasNesting()
			} else if len(s.Stack) == 0 && ((s.UseEndMarker && ini.IsEndMarker()) || (s.UseLength && s.Length == 0)) {
				return CheckComplete_Done
			}
		} else {
			top := s.Stack[len(s.Stack)-1]
			topLength := top.Value
			remainingLength := uint32(len(s.ReadState.State.Data) - s.ReadState.State.Index)
			bytesAdvanced := min(topLength, remainingLength)
			if bytesAdvanced > 0 {
				if top.Type == Parse_type_text {
					for i := s.ReadState.State.Index; i < s.ReadState.State.Index+int(bytesAdvanced); i++ {
						if s.ReadState.State.Data[i] >= 248 {
							return CheckComplete_Invalid_UTF8
						}
					}
				}
			}
			s.ReadState.State.Index += int(bytesAdvanced)
			if s.UseLength && s.Length < uint64(bytesAdvanced) {
				return CheckComplete_TotalLengthExceeded_Slice
			}
			s.Length -= uint64(bytesAdvanced)
			for i := len(s.Stack) - 1; i >= 0; i-- {
				s.Stack[i].Value -= bytesAdvanced
				if s.Stack[i].Value == 0 {
					if s.Stack[i].Type == Parse_type_map {
						if s.MapCounts[len(s.MapCounts)-1]%2 == 1 {
							return CheckComplete_MapCountOdd
						}
						s.MapCounts = s.MapCounts[:len(s.MapCounts)-1]
					}
					s.Stack = s.Stack[:i]
					if i > 0 && s.Stack[i-1].Type == Parse_type_map {
						s.MapCounts[len(s.MapCounts)-1]++
					}
				}
			}
			if len(s.Stack) == 0 && s.UseLength && s.Length == 0 {
				return CheckComplete_Done
			}
			if topLength > remainingLength {
				return CheckComplete_Continue
			}
			s.SliceMode = false
		}
	}
}
