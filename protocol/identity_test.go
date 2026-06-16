package protocol

import (
	"testing"

	"github.com/bintoca/dbuf-demo-go/basic"
)

func TestParseIdentityWrite(t *testing.T) {
	t.Run("ValidIdentityKey", func(t *testing.T) {
		keyId := basic.Uint(1)
		keyDataArr := []basic.DbufValSeq{
			basic.Registry(Registry_ed25519).ToDbufSeq(),
			basic.Bytes(make([]byte, 32)).ToDbufSeq(),
		}
		keyDataSlice, _ := basic.EncodeSequence(keyDataArr...)
		keyData := basic.Array()
		keyData.Slice = keyDataSlice

		innerMapSeq := []basic.DbufValSeq{keyId.ToDbufSeq(), keyData.ToDbufSeq()}
		innerMapSlice, _ := basic.EncodeSequence(innerMapSeq...)
		innerMap := basic.Map()
		innerMap.Slice = innerMapSlice

		bodySeq := []basic.DbufValSeq{basic.Registry(Registry_identity_key).ToDbufSeq(), innerMap.ToDbufSeq()}
		bodySlice, _ := basic.EncodeSequence(bodySeq...)
		body := basic.Map()
		body.Slice = bodySlice

		got, err := ParseIdentityWrite(body)
		if err != nil {
			t.Fatalf("ParseIdentityWrite failed: %v", err)
		}

		if !got.IdentityKeyId.HasValue || !got.IdentityKeyId.Value.Equal(keyId) {
			t.Errorf("IdentityKeyId mismatch: got %v, want %v", got.IdentityKeyId.Value, keyId)
		}
		if !got.IdentityKeyData.HasValue || !got.IdentityKeyData.Value.Equal(keyData) {
			t.Errorf("IdentityKeyData mismatch")
		}
	})

	// Note: This test highlights that Registry_identity_recovery is currently
	// missing from the identityProps map in identity.go, which causes it to be rejected.
	t.Run("WithRecoveryFailsDueToMissingProp", func(t *testing.T) {
		recovery := basic.Text("recovery-token")
		bodySeq := []basic.DbufValSeq{
			basic.Registry(Registry_identity_recovery).ToDbufSeq(),
			recovery.ToDbufSeq(),
		}
		bodySlice, _ := basic.EncodeSequence(bodySeq...)
		body := basic.Map()
		body.Slice = bodySlice

		_, err := ParseIdentityWrite(body)
		if err == nil {
			t.Error("Expected error for identity_recovery as it's not in identityProps map")
		}
	})

	t.Run("InvalidKey", func(t *testing.T) {
		bodySeq := []basic.DbufValSeq{basic.Registry(999).ToDbufSeq(), basic.Uint(1).ToDbufSeq()}
		bodySlice, _ := basic.EncodeSequence(bodySeq...)
		body := basic.Map()
		body.Slice = bodySlice

		_, err := ParseIdentityWrite(body)
		if err == nil {
			t.Error("Expected error for unknown registry key")
		}
	})
}
