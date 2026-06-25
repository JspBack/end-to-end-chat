package test_test

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/JspBack/end-to-end-chat/keys"
)

func TestKeysGenerateAndSign(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "test_key")
	k := keys.Load(keyFile)
	defer os.Remove(keyFile)

	if k.Public == "" {
		t.Error("Public key is empty")
	}
	if k.Private == "" {
		t.Error("Private key is empty")
	}

	pubBytes, err := hex.DecodeString(k.Public)
	if err != nil {
		t.Fatal("decode public:", err)
	}
	if len(pubBytes) != 32 {
		t.Errorf("public key length = %d, want 32", len(pubBytes))
	}
}

func TestSignAndVerify(t *testing.T) {
	dir := t.TempDir()
	k := keys.Load(filepath.Join(dir, "key_sv"))
	defer os.Remove(filepath.Join(dir, "key_sv"))

	msg := []byte("hello world")
	sig := k.Sign(msg)

	if !keys.Verify(k.Public, msg, sig) {
		t.Error("Verify failed for correct signature")
	}
}

func TestVerifyWrongMessage(t *testing.T) {
	dir := t.TempDir()
	k := keys.Load(filepath.Join(dir, "key_wm"))
	defer os.Remove(filepath.Join(dir, "key_wm"))

	sig := k.Sign([]byte("message"))
	if keys.Verify(k.Public, []byte("wrong message"), sig) {
		t.Error("Verify should fail for wrong message")
	}
}

func TestVerifyBadPubKey(t *testing.T) {
	if keys.Verify("nothex", []byte("msg"), []byte("sig")) {
		t.Error("Verify should return false for invalid hex key")
	}
	if keys.Verify("abcd", []byte("msg"), []byte("sig")) {
		t.Error("Verify should return false for short key")
	}
}

func TestDerive(t *testing.T) {
	dir := t.TempDir()
	k := keys.Load(filepath.Join(dir, "key_der"))
	defer os.Remove(filepath.Join(dir, "key_der"))

	d1 := k.Derive()
	d2 := k.Derive()
	if d1 != d2 {
		t.Error("Derive should be deterministic")
	}
	if len(d1) != 64 {
		t.Errorf("Derive length = %d, want 64 (SHA-256 hex)", len(d1))
	}
}

func TestKeyPersistence(t *testing.T) {
	dir := t.TempDir()
	keyFile := filepath.Join(dir, "persist_key")

	k1 := keys.Load(keyFile)
	k2 := keys.Load(keyFile)

	if k1.Public != k2.Public {
		t.Error("Public keys should match after reload")
	}
	if k1.Private != k2.Private {
		t.Error("Private keys should match after reload")
	}
	if k1.Derive() != k2.Derive() {
		t.Error("Derive should match after reload")
	}

	sig := k1.Sign([]byte("test"))
	if !keys.Verify(k2.Public, []byte("test"), sig) {
		t.Error("Signature from k1 should verify with k2's public key")
	}
}

func TestSignEmptyMessage(t *testing.T) {
	dir := t.TempDir()
	k := keys.Load(filepath.Join(dir, "key_emptymsgsig"))

	sig := k.Sign([]byte{})
	if !keys.Verify(k.Public, []byte{}, sig) {
		t.Error("verify should pass for empty message")
	}
}

func TestSignNilMessage(t *testing.T) {
	dir := t.TempDir()
	k := keys.Load(filepath.Join(dir, "key_nilmsgsig"))

	sig := k.Sign(nil)
	if !keys.Verify(k.Public, nil, sig) {
		t.Error("verify should pass for nil message")
	}
}

func TestVerifyNilSignature(t *testing.T) {
	dir := t.TempDir()
	k := keys.Load(filepath.Join(dir, "key_nilsig"))

	if keys.Verify(k.Public, []byte("msg"), nil) {
		t.Error("Verify should return false for nil signature")
	}
}

func TestVerifyEmptySignature(t *testing.T) {
	dir := t.TempDir()
	k := keys.Load(filepath.Join(dir, "key_emptysig"))

	if keys.Verify(k.Public, []byte("msg"), []byte{}) {
		t.Error("Verify should return false for empty signature")
	}
}

func TestVerifyTamperedSignature(t *testing.T) {
	dir := t.TempDir()
	k := keys.Load(filepath.Join(dir, "key_tampersig"))

	msg := []byte("hello")
	sig := k.Sign(msg)
	sig[0] ^= 0xFF

	if keys.Verify(k.Public, msg, sig) {
		t.Error("Verify should return false for tampered signature")
	}
}

func TestKeysDeterministicFromSeed(t *testing.T) {
	dir := t.TempDir()
	kf := filepath.Join(dir, "k_det")

	k1 := keys.Load(kf)

	pub1 := k1.Public
	priv1 := k1.Private

	k2 := keys.Load(kf)

	if k2.Public != pub1 {
		t.Error("public key should be deterministic from same seed file")
	}
	if k2.Private != priv1 {
		t.Error("private key (seed) should be deterministic from same seed file")
	}
}
