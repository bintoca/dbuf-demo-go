package protocol

import (
	"bytes"
	"crypto/ed25519"
	"errors"
	"fmt"

	"github.com/bintoca/dbuf-demo-go/basic"
)

const (
	ExporterOutputPrefixSize = 32
	ExporterOutputSuffixSize = 16
	ExporterLabel            = "EXPORTER: DBUF demo transport authentication"
	SignatureInputPrefix     = "DBUF demo transport authentication"
)

func CreateExporterContext(protectedHeaders basic.DbufValSeq, identity basic.DbufValSeq, identityKey basic.DbufValSeq, keyData basic.DbufValSeq) ([]byte, error) {
	return basic.EncodeSequence(protectedHeaders, identity, identityKey, keyData)
}
func CreateSigningMessage(exporterContext []byte, exporterOutputPrefix []byte) []byte {
	b := make([]byte, 0, len(SignatureInputPrefix)+len(exporterContext)+len(exporterOutputPrefix))
	b = append(b, []byte(SignatureInputPrefix)...)
	b = append(b, exporterContext...)
	b = append(b, exporterOutputPrefix...)
	return b
}
func ValidateKeyParams(tc *HeaderStoreCache, ci *HeaderStoreCacheItem, keyData []byte) error {
	if len(ci.StoreParams) < 6 {
		return basic.DataError(basic.Registry(Registry_header_store), basic.DataError(basic.Uint(5), basic.DataKeyMissing()))
	}
	ds := basic.DecoderState{Data: keyData}
	keyDataSeq, err := ds.DecodeSequence()
	if err != nil {
		return fmt.Errorf("Decode keyData failed: %w", err)
	}
	exporterContext, err := CreateExporterContext(ci.ProtectedHeaders().ToDbufSeq(), ci.Identity().ToDbufSeq(), ci.IdentityKey().ToDbufSeq(), keyDataSeq[0].ToDbufSeq())
	if err != nil {
		return err
	}
	exporterOutput, err := tc.ExportKeyingMaterial(ExporterLabel, exporterContext, ExporterOutputPrefixSize+ExporterOutputSuffixSize)
	if err != nil {
		return err
	}
	exporterOutputPrefix := exporterOutput[:ExporterOutputPrefixSize]
	exporterOutputSuffix := exporterOutput[ExporterOutputPrefixSize:]
	exporterOutputCompare := []byte(ci.StoreParams[4].String)
	if !bytes.Equal(exporterOutputSuffix, exporterOutputCompare) {
		return HeaderStoreError(basic.DataError(basic.Uint(4), basic.DataValueNotAccepted()))
	}
	signingMessage := CreateSigningMessage(exporterContext, exporterOutputPrefix)
	switch keyDataSeq[0].GetType() {
	case basic.Parse_type_array:
		ds := basic.DecoderState{Data: keyDataSeq[0].Slice}
		keyArray, err := ds.DecodeSequence()
		if err != nil {
			return fmt.Errorf("Decode keyArray failed: %w", err)
		}
		switch keyArray[0].GetType() {
		case basic.Parse_type_registry:
			switch keyArray[0].GetValue() {
			case Registry_ed25519:
				signature := []byte(ci.StoreParams[5].String)
				publicKey := keyArray[1].Slice
				if !ed25519.Verify(publicKey, signingMessage, signature) {
					return HeaderStoreError(basic.DataError(basic.Uint(5), basic.DataValueNotAccepted()))
				}
				ci.IsAuthenticated = true
			default:
				return fmt.Errorf("Key type value not supported: %d", keyArray[0].GetValue())
			}
		default:
			return fmt.Errorf("Key type not supported: %d", keyArray[0].GetType())
		}
	default:
		return fmt.Errorf("Key data type not supported: %d", keyDataSeq[0].GetType())
	}
	return nil
}

var ErrKeyManagerKeyNotFound = errors.New("Key not found")

type KeyManager interface {
	CreateKey(authorityRef basic.DbufValSeq) (keyId basic.DbufValSeq, keyData basic.DbufValSeq, err error)
	InitIdentity(authorityRef basic.DbufValSeq, identity basic.DbufValSeq) error
	GetKey(authorityRef basic.DbufValSeq) (identity basic.DbufValSeq, keyId basic.DbufValSeq, keyData basic.DbufValSeq, err error)
	GetSignature(authorityRef basic.DbufValSeq, message []byte) (signature basic.DbufValSeq, err error)
}
