package protocol

import (
	"bytes"
	"crypto/ed25519"
	"testing"

	"github.com/bintoca/dbuf-demo-go/basic"
)

func TestCreateSigningMessage(t *testing.T) {
	eCtx := []byte("ctx")
	ePrefix := []byte("prefix")
	msg := CreateSigningMessage(eCtx, ePrefix)

	expected := append([]byte(SignatureInputPrefix), eCtx...)
	expected = append(expected, ePrefix...)

	if !bytes.Equal(msg, expected) {
		t.Errorf("got %x, want %x", msg, expected)
	}
}

func TestValidateKeyParams(t *testing.T) {
	pub, priv, _ := ed25519.GenerateKey(nil)

	// Setup inputs
	ph := basic.Text("ph")
	id := basic.Text("id")
	ik := basic.Text("ik")

	// keyDataSeq[0] must be an array [Registry_ed25519, bytes]
	keyArray := basic.DbufValSeq{
		Val: basic.Array(),
		Sequence: []basic.DbufValSeq{
			{Val: basic.Registry(Registry_ed25519)},
			{Val: basic.Bytes(pub)},
		},
	}
	keyArrayBytes, _ := basic.EncodeFullSeq(keyArray)

	// Exporter Logic Simulation
	exporterContext, _ := CreateExporterContext(ph.ToDbufSeq(), id.ToDbufSeq(), ik.ToDbufSeq(), keyArray)

	mockExporterOutput := make([]byte, ExporterOutputPrefixSize+ExporterOutputSuffixSize)
	for i := range mockExporterOutput {
		mockExporterOutput[i] = byte(i)
	}

	prefix := mockExporterOutput[:ExporterOutputPrefixSize]
	suffix := mockExporterOutput[ExporterOutputPrefixSize:]

	// Signing Message Construction
	signingMsg := CreateSigningMessage(exporterContext, prefix)
	sig := ed25519.Sign(priv, signingMsg)

	// Setup Cache and Item
	tc := &HeaderStoreCache{
		ExportKeyingMaterial: func(label string, context []byte, length int) ([]byte, error) {
			if label != ExporterLabel {
				t.Errorf("wrong label: %s", label)
			}
			return mockExporterOutput, nil
		},
	}

	ci := &HeaderStoreCacheItem{
		StoreParams: []basic.DbufValString{
			basic.Uint(0).ToDbufValString(), // Index 0: ID
			ph.ToDbufValString(),
			id.ToDbufValString(),
			ik.ToDbufValString(),
			basic.Bytes(suffix).ToDbufValString(),
			basic.Bytes(sig).ToDbufValString(),
		},
	}

	t.Run("Success", func(t *testing.T) {
		ci.IsAuthenticated = false
		err := ValidateKeyParams(tc, ci, keyArrayBytes)
		if err != nil {
			t.Fatalf("ValidateKeyParams failed: %v", err)
		}
		if !ci.IsAuthenticated {
			t.Error("expected IsAuthenticated to be true")
		}
	})

	t.Run("Failure_SignatureMismatch", func(t *testing.T) {
		ci.IsAuthenticated = false
		badSig := make([]byte, len(sig))
		copy(badSig, sig)
		badSig[0] ^= 0xFF
		ci.StoreParams[5] = basic.Bytes(badSig).ToDbufValString()
		err := ValidateKeyParams(tc, ci, keyArrayBytes)
		if err == nil {
			t.Error("expected error for wrong signature")
		}
		if ci.IsAuthenticated {
			t.Error("expected IsAuthenticated to be false")
		}
	})

	t.Run("Failure_SuffixMismatch", func(t *testing.T) {
		ci.IsAuthenticated = false
		ci.StoreParams[5] = basic.Bytes(sig).ToDbufValString() // reset sig
		ci.StoreParams[4] = basic.Bytes([]byte("wrong suffix")).ToDbufValString()
		err := ValidateKeyParams(tc, ci, keyArrayBytes)
		if err == nil {
			t.Error("expected error for suffix mismatch")
		}
	})
}
