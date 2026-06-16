package protocol

import (
	"bytes"
	"testing"

	"github.com/bintoca/dbuf-demo-go/basic"
)

func TestReadStreamGroup(t *testing.T) {
	t.Run("SimpleUint", func(t *testing.T) {
		data, _ := basic.EncodeFull(basic.Uint(5))
		r := bytes.NewReader(data)
		sg, _, useFirst, err := ReadStreamGroup(r)
		if err != nil || sg != 5 || useFirst {
			t.Errorf("ReadStreamGroup failed: sg=%d, useFirst=%v, err=%v", sg, useFirst, err)
		}
	})

	t.Run("MultiByteUint", func(t *testing.T) {
		data, _ := basic.EncodeFull(basic.Uint(258))
		r := bytes.NewReader(data)
		sg, _, _, err := ReadStreamGroup(r)
		if err != nil || sg != 258 {
			t.Errorf("ReadStreamGroup failed: sg=%d, err=%v", sg, err)
		}
	})

	t.Run("DefaultBehavior", func(t *testing.T) {
		// Type Text (4) - should trigger default case
		r := bytes.NewReader([]byte{0x81, 'A'})
		sg, first, useFirst, err := ReadStreamGroup(r)
		if err != nil || sg != 0 || !useFirst || first != 0x81 {
			t.Errorf("ReadStreamGroup failed: sg=%d, first=%x, useFirst=%v, err=%v", sg, first, useFirst, err)
		}
	})
}

func TestReadHeaderInitial(t *testing.T) {
	t.Run("ValidArrayHeader", func(t *testing.T) {
		a := basic.Array()
		a.Slice = make([]byte, 10)
		data, _ := basic.EncodeFull(a)
		r := bytes.NewReader(data)
		ini, err := ReadHeaderInitial(r, 0, false)
		if err != nil || ini.Type != basic.Parse_type_array || ini.Value != 10 {
			t.Errorf("ReadHeaderInitial failed: type=%d, val=%d, err=%v", ini.Type, ini.Value, err)
		}
	})

	t.Run("UseFirstByte", func(t *testing.T) {
		firstByte := byte(0xA5)
		r := bytes.NewReader([]byte{})
		ini, err := ReadHeaderInitial(r, firstByte, true)
		if err != nil || ini.Type != basic.Parse_type_map || ini.Value != 5 {
			t.Errorf("ReadHeaderInitial failed with firstByte: type=%d, val=%d, err=%v", ini.Type, ini.Value, err)
		}
	})

	t.Run("InvalidType", func(t *testing.T) {
		data, _ := basic.EncodeFull(basic.Uint(10))
		r := bytes.NewReader(data)
		_, err := ReadHeaderInitial(r, 0, false)
		if err == nil {
			t.Error("Expected error for non-map/array header type")
		}
	})
}

func TestGetHeaderParams(t *testing.T) {
	t.Run("SuccessMap", func(t *testing.T) {
		key := basic.Registry(10)
		val := basic.Uint(20)
		content, _ := basic.EncodeSequence(key.ToDbufSeq(), val.ToDbufSeq())

		headerInitial := basic.InitialDecode{
			Type:  basic.Parse_type_map,
			Value: uint32(len(content)),
		}
		r := bytes.NewReader(content)
		params, err := GetHeaderParams(r, headerInitial)
		if err != nil {
			t.Fatalf("GetHeaderParams failed: %v", err)
		}
		if len(params.HeaderDetails) != 2 {
			t.Errorf("Expected 2 details, got %d", len(params.HeaderDetails))
		}
	})

	t.Run("IncompleteStream", func(t *testing.T) {
		headerInitial := basic.InitialDecode{
			Type:  basic.Parse_type_map,
			Value: 10,
		}
		r := bytes.NewReader([]byte{1, 2, 3}) // Only 3 bytes provided
		_, err := GetHeaderParams(r, headerInitial)
		if err == nil {
			t.Error("Expected error for incomplete stream")
		}
	})
}

func TestGetBodyLengthProperties(t *testing.T) {
	t.Run("HasValue", func(t *testing.T) {
		tp := HeaderStoreProperties{
			Headers: map[basic.DbufValString]basic.DbufVal{
				basic.Registry(basic.Registry_value).ToDbufValString(): basic.Uint(100),
			},
		}
		props, err := GetBodyLengthProperties(tp)
		if err != nil || !props.HasValue || props.Value.GetValue() != 100 {
			t.Errorf("GetBodyLengthProperties failed: %+v, err=%v", props, err)
		}
	})

	t.Run("HasLength", func(t *testing.T) {
		tp := HeaderStoreProperties{
			Headers: map[basic.DbufValString]basic.DbufVal{
				basic.Registry(Registry_body_length).ToDbufValString(): basic.Uint(500),
			},
		}
		props, err := GetBodyLengthProperties(tp)
		if err != nil || !props.HasLength || props.Length != 500 {
			t.Errorf("GetBodyLengthProperties failed: %+v, err=%v", props, err)
		}
	})

	t.Run("IndefiniteLength", func(t *testing.T) {
		tp := HeaderStoreProperties{
			Headers: map[basic.DbufValString]basic.DbufVal{
				basic.Registry(Registry_body_length).ToDbufValString(): basic.Registry(basic.Registry_describe_no_value),
			},
		}
		props, err := GetBodyLengthProperties(tp)
		if err != nil || !props.HasIndefiniteLength {
			t.Errorf("GetBodyLengthProperties failed: %+v, err=%v", props, err)
		}
	})
}

func TestGetContent(t *testing.T) {
	t.Run("IndefiniteLengthBody", func(t *testing.T) {
		// Body contains two Uint and then an end marker
		v1 := basic.Uint(1)
		v2 := basic.Uint(2)
		marker := basic.Registry(basic.Registry_end_marker)
		footer := basic.Uint(99)

		data, _ := basic.EncodeSequence(v1.ToDbufSeq(), v2.ToDbufSeq(), marker.ToDbufSeq(), footer.ToDbufSeq())
		r := bytes.NewReader(data)

		clp := BodyLengthProperties{HasIndefiniteLength: true}
		content, err := GetContent(r, clp, 1024)
		if err != nil {
			t.Fatalf("GetContent failed: %v", err)
		}

		if len(content.Body) != 2 {
			t.Errorf("Expected 2 body items, got %d", len(content.Body))
		}
		if !content.Footer.Equal(footer) {
			t.Errorf("Footer mismatch: got %v, want %v", content.Footer, footer)
		}
	})

	t.Run("LengthLimitedBody", func(t *testing.T) {
		v1 := basic.Uint(10)
		footer := basic.Uint(20)

		bodyData, _ := basic.EncodeFull(v1)
		footerData, _ := basic.EncodeFull(footer)

		r := bytes.NewReader(append(bodyData, footerData...))
		clp := BodyLengthProperties{
			HasLength: true,
			Length:    uint64(len(bodyData)),
		}

		content, err := GetContent(r, clp, 1024)
		if err != nil {
			t.Fatalf("GetContent failed: %v", err)
		}

		if len(content.Body) != 1 || content.Body[0].GetValue() != 10 {
			t.Errorf("Body content mismatch")
		}
		if !content.Footer.Equal(footer) {
			t.Errorf("Footer mismatch")
		}
	})
}

func TestGetSingleBodyFlow(t *testing.T) {
	tp := HeaderStoreProperties{
		Headers: map[basic.DbufValString]basic.DbufVal{
			basic.Registry(basic.Registry_value).ToDbufValString(): basic.Uint(42),
		},
	}

	// Case: HasValue in headers, r can be empty
	r := bytes.NewReader([]byte{})
	val, err := GetSingleBody(r, tp, 1024)
	if err != nil || val.GetValue() != 42 {
		t.Errorf("GetSingleBody (from header) failed: %v", err)
	}

	// Case: Multiple items in body (should fail GetSingleBody)
	c := ContentProperties{
		HasBody: true,
		Body:    []basic.DbufVal{basic.Uint(1), basic.Uint(2)},
	}
	_, err = c.GetSingleBody()
	if err == nil {
		t.Error("GetSingleBody should fail when multiple body items exist")
	}
}
