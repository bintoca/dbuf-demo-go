package protocol

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/bintoca/dbuf-demo-go/basic"
	"github.com/bintoca/dbuf-demo-go/data"
)

type HeaderStoreParams struct {
	IsPresent   bool
	IsStore     bool
	IsLoad      bool
	LoadID      uint32
	StoreID     uint32
	StoreParams []basic.DbufVal
}

func GetHeaderStoreParams(seq []basic.DbufVal) (HeaderStoreParams, error) {
	p := HeaderStoreParams{}
	storeKeyIndex := basic.FindKey(seq, basic.Registry(Registry_header_store))
	if storeKeyIndex == -1 {
		return p, nil
	}
	p.IsPresent = true
	value := seq[storeKeyIndex+1]
	switch value.GetType() {
	case basic.Parse_type_uint:
		p.IsLoad = true
		p.LoadID = value.GetValue()
	case basic.Parse_type_array:
		ds := basic.DecoderState{Data: value.Slice}
		sv, err := ds.DecodeSequence()
		if err != nil {
			return p, HeaderStoreError(basic.ErrorWrap(basic.DataValueNotAccepted(), err))
		}
		p.StoreParams = sv
		if len(sv) < 2 {
			return p, HeaderStoreError(basic.DataValueNotAccepted())
		}
		switch sv[0].GetType() {
		case basic.Parse_type_uint:
			p.IsStore = true
			p.StoreID = sv[0].GetValue()
		default:
			return p, HeaderStoreError(basic.DataError(basic.Uint(0), basic.DataTypeNotAccepted()))
		}
		switch sv[1].GetType() {
		case basic.Parse_type_map:
		default:
			return p, HeaderStoreError(basic.DataError(basic.Uint(1), basic.DataTypeNotAccepted()))
		}
	default:
		return p, HeaderStoreError(basic.DataTypeNotAccepted())
	}
	return p, nil
}

type HeaderStoreProperties struct {
	Headers   map[basic.DbufValString]basic.DbufVal
	CacheItem *HeaderStoreCacheItem
}
type HeaderStoreCacheItem struct {
	StoreParams     []basic.DbufValString
	IsAuthenticated bool
}

func (i HeaderStoreCacheItem) ProtectedHeaders() basic.DbufValString {
	return i.StoreParams[1]
}
func (i HeaderStoreCacheItem) Authority() []basic.DbufVal {
	m := make(map[basic.DbufValString]basic.DbufVal)
	buf := []byte(i.ProtectedHeaders().String)
	d := basic.DecoderState{Data: buf}
	loadVals, err := d.DecodeSequence()
	if err != nil {
		return nil
	}
	for i, h := range loadVals {
		if i%2 == 0 {
			if h.Equal(basic.Registry(Registry_reference)) {
				m[h.ToDbufValString()] = loadVals[i+1]
			}
		}
	}
	pp, err := GetPathProperties(HeaderStoreProperties{Headers: m})
	if err != nil {
		return nil
	}
	return pp.Authority
}
func (i HeaderStoreCacheItem) Identity() basic.DbufValString {
	return i.StoreParams[2]
}
func (i HeaderStoreCacheItem) IdentityKey() basic.DbufValString {
	return i.StoreParams[3]
}

type HeaderStoreCache struct {
	Mutex                sync.Mutex
	Cache                map[uint32]*HeaderStoreCacheItem
	CurrentSize          int
	MaxSize              int
	Storage              data.Storage
	ExportKeyingMaterial func(label string, context []byte, length int) ([]byte, error)
}

func (i HeaderStoreCacheItem) GetSize() int {
	return len(i.ProtectedHeaders().String) + 32
}
func MakeHeaderStoreCache(store data.Storage, exportKeyingMaterial func(label string, context []byte, length int) ([]byte, error)) *HeaderStoreCache {
	return &HeaderStoreCache{Cache: make(map[uint32]*HeaderStoreCacheItem), CurrentSize: 0, MaxSize: HeaderStoreCacheMaxSize, Storage: store, ExportKeyingMaterial: exportKeyingMaterial}
}
func HeaderStoreError(err error) error {
	return basic.DataError(basic.Registry(Registry_header), basic.DataError(basic.Registry(Registry_header_store), err))
}
func (c *HeaderStoreCache) Store(ci *HeaderStoreCacheItem, id uint32, ctx context.Context) error {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.Cache == nil {
		c.Cache = make(map[uint32]*HeaderStoreCacheItem)
	}
	newSize := c.CurrentSize + ci.GetSize()
	if i, exists := c.Cache[id]; exists {
		newSize -= i.GetSize()
	}
	if newSize > c.MaxSize {
		return HeaderStoreError(basic.DataError(basic.Uint(1), basic.DataValueNotAccepted()))
	}
	if len(ci.StoreParams) > 2 {
		if len(ci.StoreParams) < 4 {
			return HeaderStoreError(basic.DataError(basic.Uint(3), basic.DataKeyMissing()))
		}
		identityKeyStorageKey, err := basic.EncodeSequence(basic.Registry(Registry_identity_key).ToDbufSeq(), ci.Identity().ToDbufSeq(), ci.IdentityKey().ToDbufSeq())
		if err != nil {
			return err
		}
		tx, err := c.Storage.NewTransaction(ci.Authority(), false, ctx)
		defer tx.Discard()
		if err != nil {
			return HeaderStoreError(basic.DataError(basic.Uint(1), err))
		}
		keyData, err := tx.Get(identityKeyStorageKey, ctx)
		if err != nil {
			if err == data.ErrKeyNotFound {
				return HeaderStoreError(basic.DataError(basic.Uint(3), basic.DataValueNotAccepted()))
			}
			return err
		}
		err = ValidateKeyParams(c, ci, keyData)
		if err != nil {
			return err
		}
	}
	c.CurrentSize = newSize
	c.Cache[id] = ci
	return nil
}
func (c *HeaderStoreCache) Load(id uint32) (*HeaderStoreCacheItem, error) {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	if c.Cache == nil {
		c.Cache = make(map[uint32]*HeaderStoreCacheItem)
	}
	if tc, exists := c.Cache[id]; exists {
		return tc, nil
	}
	return nil, HeaderStoreError(basic.DataValueNotAccepted())
}

func GetHeaderStoreProperties(headerStoreCache *HeaderStoreCache, header basic.DbufVal, headerDetails []basic.DbufVal, ctx context.Context) (HeaderStoreProperties, error) {
	out := HeaderStoreProperties{}
	tp, err := GetHeaderStoreParams(headerDetails)
	if err != nil {
		return out, err
	}
	m := make(map[basic.DbufValString]basic.DbufVal)
	refIndex := -1
	for i, h := range headerDetails {
		if i%2 == 0 {
			m[h.ToDbufValString()] = headerDetails[i+1]
			if h.Equal(basic.Registry(Registry_reference)) {
				refIndex = i + 1
			}
		}
	}
	if tp.IsStore {
		kp := []basic.DbufValString{}
		for _, h := range tp.StoreParams {
			kp = append(kp, h.ToDbufValString())
		}
		out.CacheItem = &HeaderStoreCacheItem{StoreParams: kp}
		err := headerStoreCache.Store(out.CacheItem, tp.StoreID, ctx)
		if err != nil {
			return out, err
		}
	}
	if tp.IsLoad {
		ci, err := headerStoreCache.Load(tp.LoadID)
		if err != nil {
			return out, err
		}
		out.CacheItem = ci
	}
	if out.CacheItem != nil {
		buf := []byte(out.CacheItem.ProtectedHeaders().String)
		d := basic.DecoderState{Data: buf}
		loadVals, err := d.DecodeSequence()
		if err != nil {
			return out, fmt.Errorf("Decode protected headers failed: %w", err)
		}
		for i, h := range loadVals {
			if i%2 == 0 {
				if h.Equal(basic.Registry(Registry_reference)) && refIndex != -1 {
					prefix := loadVals[i+1]
					suffix := headerDetails[refIndex]
					if suffix.GetType() != basic.Parse_type_array {
						return out, basic.DataError(basic.Registry(Registry_header), basic.DataError(h, basic.DataTypeNotAccepted()))
					}
					m[h.ToDbufValString()] = basic.DbufVal{Type: prefix.Type, Value: prefix.Value, Slice: append(prefix.Slice, suffix.Slice...)}
				} else {
					if _, exists := m[h.ToDbufValString()]; exists {
						return out, basic.DataError(basic.Registry(Registry_header), basic.DataError(h, basic.DataKeyNotAccepted()))
					}
					m[h.ToDbufValString()] = loadVals[i+1]
				}
			}
		}
	}
	out.Headers = m
	return out, nil
}

type PathProperties struct {
	Authority       []basic.DbufVal
	SubPath         []basic.DbufVal
	HasOperation    bool
	OperationId     basic.DbufVal
	OperationParams []basic.DbufVal
}

func GetPathProperties(tp HeaderStoreProperties) (PathProperties, error) {
	out := PathProperties{}
	refKey := basic.Registry(Registry_reference)
	if p, exists := tp.Headers[refKey.ToDbufValString()]; exists {
		switch p.GetType() {
		case basic.Parse_type_array:
			ds := basic.DecoderState{Data: p.Slice}
			sv, err := ds.DecodeSequence()
			if err != nil {
				return out, basic.DataError(refKey, err)
			}
			if len(sv) < 1 {
				return out, basic.DataError(refKey, basic.DataValueNotAccepted())
			}
			if !sv[0].Equal(basic.Registry(Registry_host)) {
				return out, basic.DataError(refKey, basic.DataValueNotAccepted())
			}
			if len(sv) < 2 {
				return out, basic.DataError(refKey, basic.DataValueNotAccepted())
			}
			markerIndex := -1
			for i, h := range sv {
				if h.Equal(basic.Registry(Registry_authority_marker)) {
					markerIndex = i
					break
				}
			}
			if markerIndex == -1 {
				out.Authority = sv
			} else {
				out.Authority = sv[:markerIndex]
				out.SubPath = sv[markerIndex+1:]
			}
		default:
			return out, basic.DataError(refKey, basic.DataTypeNotAccepted())
		}
	} else {
		return out, basic.DataError(refKey, basic.DataKeyMissing())
	}
	opKey := basic.Registry(Registry_operation)
	if op, exists := tp.Headers[opKey.ToDbufValString()]; exists {
		out.HasOperation = true
		switch op.GetType() {
		case basic.Parse_type_registry:
			out.OperationId = op
		case basic.Parse_type_array:
			ds := basic.DecoderState{Data: op.Slice}
			sv, err := ds.DecodeSequence()
			if err != nil {
				return out, basic.DataError(opKey, err)
			}
			if len(sv) < 1 {
				return out, basic.DataError(opKey, basic.DataValueNotAccepted())
			}
			opId := sv[0]
			switch opId.GetType() {
			case basic.Parse_type_registry:
				out.OperationId = opId
			default:
				return out, basic.DataError(opKey, basic.DataError(basic.Uint(0), basic.DataTypeNotAccepted()))
			}
			out.OperationParams = sv[1:]
		default:
			return out, basic.DataError(opKey, basic.DataTypeNotAccepted())
		}
	}
	return out, nil
}
func CreateHeaderStoreData(storeId uint64, headerSequence []basic.DbufValSeq) basic.DbufValSeq {
	return basic.DbufValSeq{Val: basic.Array(), Sequence: []basic.DbufValSeq{
		basic.Uint64(storeId).ToDbufSeq(),
		{Val: basic.Map(), Sequence: headerSequence},
	}}
}
func CreateHeaderStoreDataWithIdentity(storeId uint64, protectedHeaders basic.DbufValSeq, identity basic.DbufValSeq, identityKey basic.DbufValSeq, exporterContextSuffix basic.DbufValSeq, signature basic.DbufValSeq) basic.DbufValSeq {
	return basic.DbufValSeq{Val: basic.Array(), Sequence: []basic.DbufValSeq{
		basic.Uint64(storeId).ToDbufSeq(),
		protectedHeaders,
		identity,
		identityKey,
		exporterContextSuffix,
		signature,
	}}
}
func GetAuthorityRef(protectedHeaders basic.DbufValSeq) (basic.DbufValSeq, error) {
	for i := 0; i < len(protectedHeaders.Sequence); i++ {
		if i%2 == 0 {
			sv := protectedHeaders.Sequence[i]
			if sv.Val.Equal(basic.Registry(Registry_reference)) && i+1 < len(protectedHeaders.Sequence) {
				return protectedHeaders.Sequence[i+1], nil
			}
		}
	}
	return basic.DbufValSeq{}, errors.New("AuthorityRef not found")
}
func CreateHeaderStoreHeader(keyManager KeyManager, storeId uint64, protectedHeaders basic.DbufValSeq, conn Connection, ctx context.Context) (h basic.DbufValSeq, err error) {
	authorityRef, err := GetAuthorityRef(protectedHeaders)
	if err != nil {
		err = fmt.Errorf("Get authority ref failed: %w", err)
		return
	}
	identity, keyId, keyData, err := GetKey(keyManager, authorityRef, conn, ctx)
	if err != nil {
		err = fmt.Errorf("Get key failed: %w", err)
		return
	}
	exporterContext, err := CreateExporterContext(protectedHeaders, identity, keyId, keyData)
	if err != nil {
		err = fmt.Errorf("Create exporter context failed: %w", err)
		return
	}
	exporterOutput, err := conn.HeaderStoreCache().ExportKeyingMaterial(ExporterLabel, exporterContext, ExporterOutputPrefixSize+ExporterOutputSuffixSize)
	if err != nil {
		err = fmt.Errorf("Export keying material failed: %w", err)
		return
	}
	exporterOutputPrefix := exporterOutput[:ExporterOutputPrefixSize]
	exporterOutputSuffix := basic.Bytes(exporterOutput[ExporterOutputPrefixSize:]).ToDbufSeq()
	signature, err := keyManager.GetSignature(authorityRef, CreateSigningMessage(exporterContext, exporterOutputPrefix))
	if err != nil {
		err = fmt.Errorf("Get signature failed: %w", err)
		return
	}
	h = CreateHeaderStoreDataWithIdentity(storeId, protectedHeaders, identity, keyId, exporterOutputSuffix, signature)
	return
}

type HeaderStoreState struct {
	StoreIds    map[string]uint64
	NextStoreId uint64
	KeyManager  KeyManager
	Connection  Connection
	Mutex       sync.Mutex
}

func MakeHeaderStoreState(keyManager KeyManager, connection Connection) *HeaderStoreState {
	return &HeaderStoreState{KeyManager: keyManager, Connection: connection, StoreIds: make(map[string]uint64)}
}

func (s *HeaderStoreState) GetHeaderStoreHeader(protectedHeaders basic.DbufValSeq, ctx context.Context) (h basic.DbufValSeq, err error) {
	s.Mutex.Lock()
	defer s.Mutex.Unlock()
	b, err := basic.EncodeFullSeq(protectedHeaders)
	if err != nil {
		err = fmt.Errorf("Encode protected headers failed: %w", err)
		return
	}
	if storeId, exists := s.StoreIds[string(b)]; exists {
		return basic.Uint64(storeId).ToDbufSeq(), nil
	}
	storeId := s.NextStoreId
	h, err = CreateHeaderStoreHeader(s.KeyManager, storeId, protectedHeaders, s.Connection, ctx)
	if err != nil {
		err = fmt.Errorf("Create header store header failed: %w", err)
		return
	}
	s.StoreIds[string(b)] = storeId
	s.NextStoreId++
	return
}
