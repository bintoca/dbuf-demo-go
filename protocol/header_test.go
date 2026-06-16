package protocol

import (
	"context"
	"reflect"
	"testing"

	"github.com/bintoca/dbuf-demo-go/basic"
)

func TestGetHeaderStoreParams(t *testing.T) {
	t.Run("NoHeaderStoreKey", func(t *testing.T) {
		seq := []basic.DbufVal{basic.Uint(1), basic.Uint(2)}
		p, err := GetHeaderStoreParams(seq)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if p.IsPresent {
			t.Error("IsPresent should be false when key is missing")
		}
	})

	t.Run("LoadID", func(t *testing.T) {
		seq := []basic.DbufVal{
			basic.Registry(Registry_header_store),
			basic.Uint(500),
		}
		p, err := GetHeaderStoreParams(seq)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !p.IsLoad || p.LoadID != 500 {
			t.Errorf("Expected LoadID 500, got %+v", p)
		}
	})

	t.Run("StoreParams", func(t *testing.T) {
		// Array structure: [StoreID, Map]
		storeData, _ := basic.EncodeSequence(basic.Uint(1000).ToDbufSeq(), basic.DbufValSeq{Val: basic.Map(), Sequence: []basic.DbufValSeq{}})
		a := basic.Array()
		a.Slice = storeData
		seq := []basic.DbufVal{
			basic.Registry(Registry_header_store),
			a,
		}
		p, err := GetHeaderStoreParams(seq)
		if err != nil {
			t.Fatalf("Unexpected error: %v", err)
		}
		if !p.IsStore || p.StoreID != 1000 || len(p.StoreParams) != 2 {
			t.Errorf("Store params mismatch: %+v", p)
		}
	})
}

func TestHeaderStoreCache_StoreLoad(t *testing.T) {
	ctx := context.Background()
	tc := MakeHeaderStoreCache(nil, nil)

	item := &HeaderStoreCacheItem{
		StoreParams: []basic.DbufValString{
			basic.Registry(1).ToDbufValString(),
			basic.Text("headers").ToDbufValString(),
		},
	}

	// Test Store
	err := tc.Store(item, 42, ctx)
	if err != nil {
		t.Fatalf("Store failed: %v", err)
	}

	// Test Load
	loaded, err := tc.Load(42)
	if err != nil {
		t.Fatalf("Load failed: %v", err)
	}
	if !reflect.DeepEqual(loaded, item) {
		t.Error("Loaded item does not match stored item")
	}

	// Test MaxSize constraint
	tc.MaxSize = 10
	err = tc.Store(item, 43, ctx)
	if err == nil {
		t.Error("Expected error due to MaxSize limit, but got nil")
	}
}

func TestGetHeaderStoreProperties(t *testing.T) {
	ctx := context.Background()
	tc := MakeHeaderStoreCache(nil, nil)

	// 1. Setup a cached item with some "protected" headers
	// Protected headers sequence: [Registry(100) -> Uint(200)]
	protectedData, _ := basic.EncodeSequence(basic.Registry(100).ToDbufSeq(), basic.Uint(200).ToDbufSeq())
	ci := &HeaderStoreCacheItem{
		StoreParams: []basic.DbufValString{
			basic.Uint(1).ToDbufValString(), // StoreID
			basic.Bytes(protectedData).ToDbufValString(),
		},
	}
	tc.Store(ci, 1, ctx)

	// 2. Mock incoming header details that "loads" ID 1 and adds a local header
	headerDetails := []basic.DbufVal{
		basic.Registry(Registry_header_store), basic.Uint(1), // Load ID 1
		basic.Registry(300), basic.Uint(400), // Local header
	}

	props, err := GetHeaderStoreProperties(tc, basic.Map(), headerDetails, ctx)
	if err != nil {
		t.Fatalf("GetHeaderStoreProperties failed: %v", err)
	}

	// Verify merged headers
	if v, ok := props.Headers[basic.Registry(100).ToDbufValString()]; !ok || v.GetValue() != 200 {
		t.Errorf("Protected header (100) missing or incorrect: %v", v)
	}
	if v, ok := props.Headers[basic.Registry(300).ToDbufValString()]; !ok || v.GetValue() != 400 {
		t.Errorf("Local header (300) missing or incorrect: %v", v)
	}

	t.Run("ReferenceSuffixAppend", func(t *testing.T) {
		// Prefix from cache
		preData, _ := basic.EncodeSequence(basic.Registry(Registry_host).ToDbufSeq(), basic.Text("host.com").ToDbufSeq())
		// Suffix from header
		sufData, _ := basic.EncodeSequence(basic.Text("sub").ToDbufSeq())

		// Cached protected header with Registry_reference
		a := basic.Array()
		a.Slice = preData
		ciRef, _ := basic.EncodeSequence(basic.Registry(Registry_reference).ToDbufSeq(), a.ToDbufSeq())
		ci2 := &HeaderStoreCacheItem{
			StoreParams: []basic.DbufValString{
				basic.Uint(2).ToDbufValString(),
				basic.Text(string(ciRef)).ToDbufValString(),
			},
		}
		tc.Store(ci2, 2, ctx)

		a1 := basic.Array()
		a1.Slice = sufData
		details := []basic.DbufVal{
			basic.Registry(Registry_header_store), basic.Uint(2),
			basic.Registry(Registry_reference), a1,
		}

		p2, _ := GetHeaderStoreProperties(tc, basic.Map(), details, ctx)
		refHeader := p2.Headers[basic.Registry(Registry_reference).ToDbufValString()]

		// The merged slice should contain the concatenation of preData and sufData
		expectedSlice := append(preData, sufData...)
		if !reflect.DeepEqual(refHeader.Slice, expectedSlice) {
			t.Errorf("Reference slice was not correctly appended.\nGot: %x\nWant: %x", refHeader.Slice, expectedSlice)
		}
	})
}

func TestGetPathProperties(t *testing.T) {
	// Helper to build a reference array
	buildRef := func(host string, path []string, hasMarker bool) basic.DbufVal {
		seq := []basic.DbufValSeq{basic.Registry(Registry_host).ToDbufSeq(), basic.Text(host).ToDbufSeq()}
		if hasMarker {
			seq = append(seq, basic.Registry(Registry_authority_marker).ToDbufSeq())
		}
		for _, p := range path {
			seq = append(seq, basic.Text(p).ToDbufSeq())
		}
		data, _ := basic.EncodeSequence(seq...)
		a := basic.Array()
		a.Slice = data
		return a
	}

	t.Run("AuthorityAndSubPath", func(t *testing.T) {
		ref := buildRef("example.com", []string{"v1", "users"}, true)
		tp := HeaderStoreProperties{
			Headers: map[basic.DbufValString]basic.DbufVal{
				basic.Registry(Registry_reference).ToDbufValString(): ref,
			},
		}

		pp, err := GetPathProperties(tp)
		if err != nil {
			t.Fatalf("GetPathProperties failed: %v", err)
		}

		if len(pp.Authority) != 2 || !pp.Authority[1].Equal(basic.Text("example.com")) {
			t.Errorf("Authority mismatch: %v", pp.Authority)
		}
		if len(pp.SubPath) != 2 || !pp.SubPath[0].Equal(basic.Text("v1")) {
			t.Errorf("SubPath mismatch: %v", pp.SubPath)
		}
	})

	t.Run("OperationParsing", func(t *testing.T) {
		opData, _ := basic.EncodeSequence(basic.Registry(50).ToDbufSeq(), basic.Uint(1).ToDbufSeq())
		a := basic.Array()
		a.Slice = opData
		tp := HeaderStoreProperties{
			Headers: map[basic.DbufValString]basic.DbufVal{
				basic.Registry(Registry_reference).ToDbufValString(): buildRef("localhost", nil, false),
				basic.Registry(Registry_operation).ToDbufValString(): a,
			},
		}

		pp, err := GetPathProperties(tp)
		if err != nil {
			t.Fatal(err)
		}
		if !pp.HasOperation || pp.OperationId.GetValue() != 50 || len(pp.OperationParams) != 1 {
			t.Errorf("Operation properties mismatch: %+v", pp)
		}
	})
}

func TestGetAuthorityRef(t *testing.T) {
	h := basic.DbufValSeq{Val: basic.Map(), Sequence: []basic.DbufValSeq{
		basic.Registry(Registry_reference).ToDbufSeq(), basic.Text("my-ref").ToDbufSeq(),
	}}
	got, err := GetAuthorityRef(h)
	if err != nil || !got.Val.Equal(basic.Text("my-ref")) {
		t.Errorf("GetAuthorityRef failed: got %v, err %v", got, err)
	}
}
