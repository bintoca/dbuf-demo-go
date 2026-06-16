package protocol

import (
	"io"

	"github.com/bintoca/dbuf-demo-go/basic"
)

const (
	MaxHeaderSize = 1 << 14
)

func ReadFull(r io.Reader, buf []byte) error {
	_, err := io.ReadFull(r, buf)
	return err
}

func ReadStreamGroup(r io.Reader) (streamGroup uint32, firstByte byte, useFirstByte bool, err error) {
	buf := [9]byte{}
	firstBuf := buf[:1]
	err = ReadFull(r, firstBuf)
	if err != nil {
		return 0, 0, false, basic.DataError(basic.Registry(Registry_stream_group), err)
	}
	d := basic.DecoderState{Data: firstBuf}
	a := basic.InitialDecode{}
	d.DecodeInitial(&a)
	switch a.Type {
	case basic.Parse_type_uint:
		if a.MissingExtraLength == 0 {
			return a.Value, 0, false, nil
		}
		b := buf[:a.MissingExtraLength]
		err = ReadFull(r, b)
		if err != nil {
			return 0, 0, false, basic.DataError(basic.Registry(Registry_stream_group), err)
		}
		d = basic.DecoderState{Data: b}
		d.DecodeInitial(&a)
		return a.Value, 0, false, nil
	default:
		return 0, buf[0], true, nil
	}
}

func ReadHeaderInitial(r io.Reader, firstByte byte, useFirstByte bool) (basic.InitialDecode, error) {
	buf := [4]byte{}
	firstBuf := buf[:1]
	a := basic.InitialDecode{}
	if useFirstByte {
		firstBuf[0] = firstByte
	} else {
		err := ReadFull(r, firstBuf)
		if err != nil {
			return a, err
		}
	}
	d := basic.DecoderState{Data: firstBuf}
	d.DecodeInitial(&a)
	if a.MissingExtraLength > 0 {
		d = basic.DecoderState{Data: buf[:a.MissingExtraLength]}
		err := ReadFull(r, d.Data)
		if err != nil {
			return a, basic.DataError(basic.Registry(Registry_header), err)
		}
		d.DecodeInitial(&a)
	}
	switch a.Type {
	case basic.Parse_type_array:
		fallthrough
	case basic.Parse_type_map:
		if a.Value > MaxHeaderSize {
			return a, basic.DataError(basic.Registry(Registry_header), basic.DataValueNotAccepted())
		}
		return a, nil
	default:
		return a, basic.DataError(basic.Registry(Registry_header), basic.DataTypeNotAccepted())
	}
}

type HeaderParams struct {
	Header        basic.DbufVal
	HeaderDetails []basic.DbufVal
}

func GetHeaderParams(r io.Reader, headerInitial basic.InitialDecode) (params HeaderParams, err error) {
	headerBuf := make([]byte, headerInitial.Value)
	err = ReadFull(r, headerBuf)
	if err != nil {
		return params, basic.DataError(basic.Registry(Registry_header), err)
	}
	if len(headerBuf) > 0 {
		ccs := basic.CheckCompleteState{
			Stack:     []basic.InitialDecode{headerInitial},
			UseLength: true,
			Length:    uint64(len(headerBuf))}
		if headerInitial.Type == basic.Parse_type_map {
			ccs.MapCounts = []uint32{0}
		}
		ccr := ccs.CheckComplete(headerBuf)
		if ccr == basic.CheckComplete_Continue {
			return params, basic.DataError(basic.Registry(Registry_header), basic.IncompleteStream())
		}
		if ccr != basic.CheckComplete_Done {
			return params, basic.DataError(basic.Registry(Registry_header), basic.DataValueNotAccepted())
		}
	}
	ds := basic.DecoderState{Data: headerBuf}
	ps, err := ds.DecodeSequence()
	if err != nil {
		return params, err
	}
	if headerInitial.Type == basic.Parse_type_map {
		if err := basic.ValidateKeysUnique(ps); err != nil {
			return params, basic.DataError(basic.Registry(Registry_header), err)
		}
	}
	params.Header = headerInitial.GetDbufVal(headerBuf)
	params.HeaderDetails = ps
	return params, nil
}

type BodyLengthProperties struct {
	HasValue            bool
	Value               basic.DbufVal
	HasLength           bool
	Length              uint64
	HasIndefiniteLength bool
}

func GetBodyLengthProperties(tp HeaderStoreProperties) (BodyLengthProperties, error) {
	out := BodyLengthProperties{}
	if p, exists := tp.Headers[basic.Registry(basic.Registry_value).ToDbufValString()]; exists {
		out.HasValue = true
		out.Value = p
	}
	bodyLengthKey := basic.Registry(Registry_body_length)
	if p, exists := tp.Headers[bodyLengthKey.ToDbufValString()]; exists {
		if out.HasValue {
			return out, basic.DataError(bodyLengthKey, basic.DataKeyNotAccepted())
		}
		switch p.GetType() {
		case basic.Parse_type_uint, basic.Parse_type_integer:
			u64, err := p.GetU64()
			if err != nil {
				return out, basic.DataError(bodyLengthKey, err)
			}
			out.HasLength = true
			out.Length = u64
		case basic.Parse_type_registry:
			if p.GetValue() != basic.Registry_describe_no_value {
				return out, basic.DataError(bodyLengthKey, basic.DataValueNotAccepted())
			}
			out.HasIndefiniteLength = true
		default:
			return out, basic.DataError(bodyLengthKey, basic.DataValueNotAccepted())
		}
	}
	return out, nil
}

type ContentProperties struct {
	HasBody   bool
	Body      []basic.DbufVal
	HasFooter bool
	Footer    basic.DbufVal
}

func ReadCheckValue(r io.Reader, ccs basic.CheckCompleteState, maxSize int) (basic.DbufVal, bool, error) {
	buf4 := [4]byte{}
	buf1 := buf4[:1]
	var cc_err byte
	a := basic.InitialDecode{}
	err := ReadFull(r, buf1)
	if err != nil {
		return a.GetDbufVal(nil), false, err
	}
	cc_err = ccs.CheckComplete(buf1)
	if cc_err > basic.CheckComplete_Continue {
		return a.GetDbufVal(nil), false, basic.DataValueNotAccepted()
	}

	d := basic.DecoderState{Data: buf1}
	d.DecodeInitial(&a)
	if a.MissingExtraLength > 0 {
		ccs.ReadState.Initial = a
		bm := buf4[:a.MissingExtraLength]
		err := ReadFull(r, bm)
		if err != nil {
			return a.GetDbufVal(nil), false, err
		}
		cc_err = ccs.CheckComplete(bm)
		if cc_err > basic.CheckComplete_Continue {
			return a.GetDbufVal(nil), false, basic.DataValueNotAccepted()
		}
		d = basic.DecoderState{Data: bm}
		d.DecodeInitial(&a)
	}
	var buf []byte
	if a.TypeHasSlice() {
		if ccs.ReadState.TotalBytesRead()+int64(a.Value) > int64(maxSize) {
			return a.GetDbufVal(nil), false, basic.DataValueNotAccepted()
		}
		buf = make([]byte, a.Value)
		err := ReadFull(r, buf)
		if err != nil {
			return a.GetDbufVal(nil), false, err
		}
		cc_err = ccs.CheckComplete(buf)
		if cc_err > basic.CheckComplete_Continue {
			return a.GetDbufVal(nil), false, basic.DataValueNotAccepted()
		}
	}
	if ccs.ReadState.TotalBytesRead() > int64(maxSize) {
		return a.GetDbufVal(nil), false, basic.DataValueNotAccepted()
	}
	return a.GetDbufVal(buf), cc_err == basic.CheckComplete_Done, nil
}

func GetContent(r io.Reader, clp BodyLengthProperties, maxBufferSize int) (ContentProperties, error) {
	out := ContentProperties{}
	out.HasBody = clp.HasIndefiniteLength || clp.HasLength || clp.HasValue
	out.HasFooter = clp.HasIndefiniteLength || clp.HasLength

	if clp.HasValue {
		out.Body = []basic.DbufVal{clp.Value}
	} else if out.HasBody && maxBufferSize > 0 {
		if clp.HasLength && clp.Length > uint64(maxBufferSize) {
			return out, basic.DataError(basic.Registry(Registry_header), basic.DataError(basic.Registry(Registry_body_length), basic.DataValueNotAccepted()))
		}
		ccs := basic.CheckCompleteState{
			UseLength:    clp.HasLength,
			Length:       clp.Length,
			UseEndMarker: clp.HasIndefiniteLength,
		}
		for {
			v, done, err := ReadCheckValue(r, ccs, maxBufferSize)
			if err != nil {
				return out, basic.DataError(basic.Registry(Registry_body), err)
			}
			isEndMarker := v.Equal(basic.Registry(basic.Registry_end_marker))
			if clp.HasLength || !isEndMarker {
				out.Body = append(out.Body, v)
			}
			if done {
				break
			}
		}
		v, _, err := ReadCheckValue(r, basic.CheckCompleteState{
			UseLength: true,
			Length:    MaxHeaderSize,
		}, MaxHeaderSize)
		if err != nil {
			return out, basic.DataError(basic.Registry(Registry_footer), err)
		}
		out.Footer = v
	}
	return out, nil
}
func (c ContentProperties) GetSingleBody() (basic.DbufVal, error) {
	if len(c.Body) > 1 {
		return basic.DbufVal{}, basic.DataError(basic.Registry(Registry_body), basic.DataError(basic.Uint(1), basic.DataValueNotAccepted()))
	}
	if len(c.Body) == 0 {
		return basic.DbufVal{}, basic.DataError(basic.Registry(Registry_body), basic.DataKeyMissing())
	}
	return c.Body[0], nil
}
func GetSingleBody(r io.Reader, tp HeaderStoreProperties, maxBufferSize int) (basic.DbufVal, error) {
	clp, err := GetBodyLengthProperties(tp)
	if err != nil {
		return basic.DbufVal{}, err
	}
	c, err := GetContent(r, clp, maxBufferSize)
	if err != nil {
		return basic.DbufVal{}, err
	}
	return c.GetSingleBody()
}
