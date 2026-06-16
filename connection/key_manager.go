package connection

import (
	"crypto/ed25519"
	"errors"
	"log/slog"

	"github.com/bintoca/dbuf-demo-go/basic"
	dbp "github.com/bintoca/dbuf-demo-go/protocol"
)

type KeyInit struct {
	HasIdentity   bool
	AuthorityRef  basic.DbufValSeq
	Identity      basic.DbufValSeq
	KeyId         basic.DbufValSeq
	PublicKeyData basic.DbufValSeq
	PrivateKey    ed25519.PrivateKey
}
type TestKeyManager struct {
	Keys           map[string]KeyInit
	DataChangeFunc func(TestKeyManager)
}

func MakeTestKeyManager() *TestKeyManager {
	return &TestKeyManager{Keys: make(map[string]KeyInit)}
}

func (m *TestKeyManager) CreateKey(authorityRef basic.DbufValSeq) (keyId basic.DbufValSeq, publicKeyData basic.DbufValSeq, err error) {
	b, err := basic.EncodeFullSeq(authorityRef)
	if err != nil {
		return
	}
	slog.Info("TestKeyManager CreateKey", "authorityRef", authorityRef.DebugString())
	authorityRefString := string(b)
	if _, exists := m.Keys[authorityRefString]; exists {
		err = errors.New("key exists for authority ref")
		return
	}
	ki := KeyInit{AuthorityRef: authorityRef}
	keyId = basic.Uint(0).ToDbufSeq()
	pub, pri, err := ed25519.GenerateKey(nil)
	if err != nil {
		return
	}
	ki.PrivateKey = pri
	publicKeyData = basic.DbufValSeq{Val: basic.Array(), Sequence: []basic.DbufValSeq{{Val: basic.Registry(dbp.Registry_ed25519)}, {Val: basic.Bytes(pub)}}}
	ki.KeyId = keyId
	ki.PublicKeyData = publicKeyData
	ki.PrivateKey = pri
	m.Keys[authorityRefString] = ki
	if m.DataChangeFunc != nil {
		m.DataChangeFunc(*m)
	}
	return
}
func (m *TestKeyManager) GetSignature(authorityRef basic.DbufValSeq, message []byte) (signature basic.DbufValSeq, err error) {
	b, err := basic.EncodeFullSeq(authorityRef)
	if err != nil {
		return
	}
	slog.Info("TestKeyManager GetSignature", "authorityRef", authorityRef.DebugString())
	authorityRefString := string(b)
	if ki, exists := m.Keys[authorityRefString]; exists {
		signature = basic.DbufValSeq{Val: basic.Bytes(ed25519.Sign(ki.PrivateKey, message))}
		return
	}
	err = dbp.ErrKeyManagerKeyNotFound
	return
}
func (m *TestKeyManager) GetKey(authorityRef basic.DbufValSeq) (identity basic.DbufValSeq, keyId basic.DbufValSeq, keyData basic.DbufValSeq, err error) {
	b, err := basic.EncodeFullSeq(authorityRef)
	if err != nil {
		return
	}
	authorityRefString := string(b)
	if ki, exists := m.Keys[authorityRefString]; exists {
		if !ki.HasIdentity {
			err = errors.New("identity not set")
			return
		}
		identity = ki.Identity
		keyId = ki.KeyId
		keyData = ki.PublicKeyData
		return
	}
	err = dbp.ErrKeyManagerKeyNotFound
	return
}
func (m *TestKeyManager) InitIdentity(authorityRef basic.DbufValSeq, identity basic.DbufValSeq) error {
	b, err := basic.EncodeFullSeq(authorityRef)
	if err != nil {
		return err
	}
	slog.Info("TestKeyManager InitIdentity", "authorityRef", authorityRef.DebugString())
	authorityRefString := string(b)
	if ki, exists := m.Keys[authorityRefString]; exists {
		if ki.HasIdentity {
			return errors.New("identity already exists")
		}
		ki.HasIdentity = true
		ki.Identity = identity
		m.Keys[authorityRefString] = ki
		if m.DataChangeFunc != nil {
			m.DataChangeFunc(*m)
		}
		return nil
	}
	return dbp.ErrKeyManagerKeyNotFound
}
