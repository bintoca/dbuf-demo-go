package data

import (
	"context"
	"errors"

	"github.com/bintoca/dbuf-demo-go/basic"
)

var (
	ErrKeyNotFound = errors.New("Key not found")
)

type Storage interface {
	NewTransaction(authority []basic.DbufVal, write bool, ctx context.Context) (Transaction, error)
}
type Transaction interface {
	Get(key []byte, ctx context.Context) ([]byte, error)
	Set(key, value []byte, ctx context.Context) error
	Commit(ctx context.Context) error
	Discard()
}
