package protocol

import (
	"context"
	"crypto/rand"
	"fmt"

	"github.com/bintoca/dbuf-demo-go/basic"
)

type IdentityWrite struct {
	IdentityKeyId    basic.Optional[basic.DbufVal]
	IdentityKeyData  basic.Optional[basic.DbufVal]
	IdentityRecovery basic.Optional[basic.DbufVal]
}

var identityProps = map[basic.DbufValString]struct{}{basic.Registry(Registry_identity_key).ToDbufValString(): {}}

func ParseIdentityWrite(body basic.DbufVal) (IdentityWrite, error) {
	out := IdentityWrite{}
	switch body.GetType() {
	case basic.Parse_type_map:
		ds := basic.DecoderState{Data: body.Slice}
		sv, err := ds.DecodeSequence()
		if err != nil {
			return out, err
		}
		for i := 0; i < len(sv); i += 2 {
			v := sv[i]
			if _, exists := identityProps[v.ToDbufValString()]; !exists {
				return out, basic.DataError(v, basic.DataKeyNotAccepted())
			}
			if v.Equal(basic.Registry(Registry_identity_key)) {
				uk := sv[i+1]
				switch uk.GetType() {
				case basic.Parse_type_map:
					ds := basic.DecoderState{Data: uk.Slice}
					sv, err := ds.DecodeSequence()
					if err != nil {
						return out, basic.DataError(basic.Registry(Registry_identity_key), err)
					}
					if len(sv) != 2 {
						return out, basic.DataError(basic.Registry(Registry_identity_key), basic.DataValueNotAccepted())
					}
					out.IdentityKeyId = basic.Optional[basic.DbufVal]{HasValue: true, Value: sv[0]}
					ukk := sv[1]
					switch ukk.GetType() {
					case basic.Parse_type_array:
						ds := basic.DecoderState{Data: ukk.Slice}
						sv, err := ds.DecodeSequence()
						if err != nil {
							return out, basic.DataError(basic.Registry(Registry_identity_key), basic.DataError(sv[0], err))
						}
						if len(sv) != 2 {
							return out, basic.DataError(basic.Registry(Registry_identity_key), basic.DataError(sv[0], basic.DataValueNotAccepted()))
						}
						if sv[0].Equal(basic.Registry(Registry_ed25519)) {
							if sv[1].GetType() != basic.Parse_type_bytes || len(sv[1].Slice) != 32 {
								return out, basic.DataError(basic.Registry(Registry_identity_key), basic.DataError(sv[0], basic.DataValueNotAccepted()))
							}
						} else {
							return out, basic.DataError(basic.Registry(Registry_identity_key), basic.DataError(sv[0], basic.DataValueNotAccepted()))
						}
					default:
						return out, basic.DataError(basic.Registry(Registry_identity_key), basic.DataError(sv[0], basic.DataTypeNotAccepted()))
					}
					out.IdentityKeyData = basic.Optional[basic.DbufVal]{HasValue: true, Value: ukk}
				default:
					return out, basic.DataError(basic.Registry(Registry_identity_key), basic.DataTypeNotAccepted())
				}
			} else if v.Equal(basic.Registry(Registry_identity_recovery)) {
				out.IdentityRecovery = basic.Optional[basic.DbufVal]{HasValue: true, Value: sv[i+1]}
			} else {
				return out, basic.DataError(v, basic.DataKeyNotAccepted())
			}
		}
	default:
		return out, basic.DataTypeNotAccepted()
	}
	return out, nil
}
func CreateIdentity(rr *RequestResponse, ctx context.Context) error {
	body, err := GetSingleBody(rr.RequestReader, rr.RequestHeaderStoreProperties, SmallRequestBufferSize)
	if err != nil {
		rr.ResponseError = err
		return nil
	}
	identityWrite, err := ParseIdentityWrite(body)
	if err != nil {
		rr.ResponseError = basic.DataError(basic.Registry(Registry_body), err)
		return nil
	}
	if !identityWrite.IdentityKeyId.HasValue {
		rr.ResponseError = basic.DataError(basic.Registry(Registry_body), basic.DataError(basic.Registry(Registry_identity_key), basic.DataKeyMissing()))
		return nil
	}
	b := make([]byte, 16)
	_, err = rand.Read(b)
	if err != nil {
		return fmt.Errorf("Create identity id failed: %w", err)
	}
	id := basic.Bytes(b).ToDbufSeq()
	identityKeyStorageKey, err := basic.EncodeSequence(basic.Registry(Registry_identity_key).ToDbufSeq(), id, identityWrite.IdentityKeyId.Value.ToDbufSeq())
	if err != nil {
		return fmt.Errorf("Encode identity key failed: %w", err)
	}
	identityKeyStorageValue, err := basic.EncodeFullSeq(identityWrite.IdentityKeyData.Value.ToDbufSeq())
	if err != nil {
		return fmt.Errorf("Encode identity key storage value failed: %w", err)
	}
	tx, err := rr.HeaderStoreCache.Storage.NewTransaction(rr.RequestPathProperties.Authority, true, ctx)
	defer tx.Discard()
	if err != nil {
		return fmt.Errorf("New transaction failed: %w", err)
	}
	err = tx.Set(identityKeyStorageKey, identityKeyStorageValue, ctx)
	if err != nil {
		return fmt.Errorf("Set identity key failed: %w", err)
	}
	if identityWrite.IdentityRecovery.HasValue {
		identityRecoveryStorageKey, err := basic.EncodeSequence(basic.Registry(Registry_identity_recovery).ToDbufSeq(), id)
		if err != nil {
			return fmt.Errorf("Encode identity recovery key failed: %w", err)
		}
		identityRecoveryStorageValue, err := basic.EncodeFullSeq(identityWrite.IdentityRecovery.Value.ToDbufSeq())
		if err != nil {
			return fmt.Errorf("Create identity recovery storage value failed: %w", err)
		}
		err = tx.Set(identityRecoveryStorageKey, identityRecoveryStorageValue, ctx)
		if err != nil {
			return fmt.Errorf("Set identity recovery failed: %w", err)
		}
	}
	err = tx.Commit(ctx)
	if err != nil {
		return fmt.Errorf("Commit failed: %w", err)
	}
	rr.ResponseHeaders = ValueHeader(id)
	return nil
}
func CreateRequestForNewIdentity(authorityRef basic.DbufValSeq, keyId basic.DbufValSeq, keyData basic.DbufValSeq) Request {
	authorityRef.Sequence = append(authorityRef.Sequence, basic.Registry(Registry_identity).ToDbufSeq())
	identityKey := basic.DbufValSeq{Val: basic.Map(), Sequence: []basic.DbufValSeq{basic.Registry(Registry_identity_key).ToDbufSeq(), {Val: basic.Map(), Sequence: []basic.DbufValSeq{keyId, keyData}}}}
	return CreateRequest([]basic.DbufValSeq{basic.Registry(Registry_reference).ToDbufSeq(), authorityRef, basic.Registry(basic.Registry_value).ToDbufSeq(), identityKey}, nil)
}
func SetupNewIdentity(keyManager KeyManager, authorityRef basic.DbufValSeq, conn Connection, ctx context.Context) error {
	keyId, keyData, err := keyManager.CreateKey(authorityRef)
	if err != nil {
		return err
	}
	response, err := SendRequestCheckError(conn, CreateRequestForNewIdentity(authorityRef, keyId, keyData), ctx)
	if err != nil {
		return fmt.Errorf("Send request for new identity failed: %w", err)
	}
	body, err := GetSingleBody(response.Body, response.HeaderStoreProperties, SmallRequestBufferSize)
	if err != nil {
		return err
	}
	err = keyManager.InitIdentity(authorityRef, body.ToDbufSeq())
	if err != nil {
		return err
	}
	return nil
}
func GetKey(keyManager KeyManager, authorityRef basic.DbufValSeq, conn Connection, ctx context.Context) (identity basic.DbufValSeq, keyId basic.DbufValSeq, keyData basic.DbufValSeq, err error) {
	identity, keyId, keyData, err = keyManager.GetKey(authorityRef)
	if err != nil {
		if err == ErrKeyManagerKeyNotFound {
			err = SetupNewIdentity(keyManager, authorityRef, conn, ctx)
			if err != nil {
				err = fmt.Errorf("Setup new identity failed: %w", err)
				return
			}
		} else {
			return
		}
		return keyManager.GetKey(authorityRef)
	}
	return identity, keyId, keyData, nil
}
