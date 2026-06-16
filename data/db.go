package data

import (
	"context"

	"github.com/bintoca/dbuf-demo-go/basic"
	"github.com/dgraph-io/badger/v4"
)

type BadgerStorage struct {
	kv        *badger.DB
	SetLogger func(key []byte, value []byte)
	Authority []basic.DbufVal
}

func NewBadgerMemoryStorage(authority []basic.DbufVal) (BadgerStorage, error) {
	opts := badger.DefaultOptions("").WithInMemory(true)
	db, err := badger.Open(opts)
	if err != nil {
		return BadgerStorage{}, err
	}
	return BadgerStorage{kv: db, Authority: authority}, nil
}

func NewBadgerStorage(path string, authority []basic.DbufVal) (BadgerStorage, error) {
	opts := badger.DefaultOptions(path)
	db, err := badger.Open(opts)
	if err != nil {
		return BadgerStorage{}, err
	}
	return BadgerStorage{kv: db, Authority: authority}, nil
}

func (d *BadgerStorage) Close() error {
	return d.kv.Close()
}

func (b BadgerStorage) NewTransaction(authority []basic.DbufVal, write bool, ctx context.Context) (Transaction, error) {
	if !basic.SliceEqual(authority, b.Authority) {
		return nil, basic.DataValueNotAccepted()
	}
	return BadgerTransaction{tx: b.kv.NewTransaction(write), SetLogger: b.SetLogger}, nil
}

type BadgerTransaction struct {
	tx        *badger.Txn
	SetLogger func(key []byte, value []byte)
}

func (b BadgerTransaction) Get(key []byte, ctx context.Context) ([]byte, error) {
	item, err := b.tx.Get(key)
	if err != nil {
		if err == badger.ErrKeyNotFound {
			return nil, ErrKeyNotFound
		}
		return nil, err
	}
	return item.ValueCopy(nil)
}
func (b BadgerTransaction) Set(key, value []byte, ctx context.Context) error {
	if b.SetLogger != nil {
		b.SetLogger(key, value)
	}
	return b.tx.Set(key, value)
}
func (b BadgerTransaction) Commit(ctx context.Context) error {
	return b.tx.Commit()
}
func (b BadgerTransaction) Discard() {
	b.tx.Discard()
}
