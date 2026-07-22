package keys_test

// import (
// 	"encoding/hex"
// 	"testing"

// 	"github.com/JspBack/end-to-end-chat/keys"
// )

// func TestAutoLoadAndSign(t *testing.T) {
// 	k := keys.AutoLoad()

// 	if k.Public == "" {
// 		t.Error("Public key is empty")
// 	}
// 	if k.Private == "" {
// 		t.Error("Private key is empty")
// 	}

// 	pubBytes, err := hex.DecodeString(k.Public)
// 	if err != nil {
// 		t.Fatal("decode public:", err)
// 	}
// 	if len(pubBytes) != 32 {
// 		t.Errorf("public key length = %d, want 32", len(pubBytes))
// 	}
// }

// func TestAutoLoadSignAndVerify(t *testing.T) {
// 	k := keys.AutoLoad()

// 	msg := []byte("hello world")
// 	sig := k.Sign(msg)

// 	if !keys.Verify(k.Public, msg, sig) {
// 		t.Error("Verify failed for correct signature")
// 	}
// }

// func TestAutoLoadVerifyWrongMessage(t *testing.T) {
// 	k := keys.AutoLoad()

// 	sig := k.Sign([]byte("message"))
// 	if keys.Verify(k.Public, []byte("wrong message"), sig) {
// 		t.Error("Verify should fail for wrong message")
// 	}
// }

// func TestAutoLoadVerifyBadPubKey(t *testing.T) {
// 	if keys.Verify("nothex", []byte("msg"), []byte("sig")) {
// 		t.Error("Verify should return false for invalid hex key")
// 	}
// 	if keys.Verify("abcd", []byte("msg"), []byte("sig")) {
// 		t.Error("Verify should return false for short key")
// 	}
// }

// func TestAutoLoadDerive(t *testing.T) {
// 	k := keys.AutoLoad()

// 	d1 := k.Derive()
// 	d2 := k.Derive()
// 	if d1 != d2 {
// 		t.Error("Derive should be deterministic")
// 	}
// 	if len(d1) != 16 {
// 		t.Errorf("Derive length = %d, want 16 (truncated SHA-256 hex)", len(d1))
// 	}
// }

// func TestAutoLoadKeyPersistence(t *testing.T) {
// 	k1 := keys.AutoLoad()
// 	k2 := keys.AutoLoad()

// 	if k1.Public != k2.Public {
// 		t.Error("Public keys should match after reload")
// 	}
// 	if k1.Private != k2.Private {
// 		t.Error("Private keys should match after reload")
// 	}
// 	if k1.Derive() != k2.Derive() {
// 		t.Error("Derive should match after reload")
// 	}

// 	sig := k1.Sign([]byte("test"))
// 	if !keys.Verify(k2.Public, []byte("test"), sig) {
// 		t.Error("Signature from k1 should verify with k2's public key")
// 	}
// }

// func TestAutoLoadSignEmptyMessage(t *testing.T) {
// 	k := keys.AutoLoad()

// 	sig := k.Sign([]byte{})
// 	if !keys.Verify(k.Public, []byte{}, sig) {
// 		t.Error("verify should pass for empty message")
// 	}
// }

// func TestAutoLoadSignNilMessage(t *testing.T) {
// 	k := keys.AutoLoad()

// 	sig := k.Sign(nil)
// 	if !keys.Verify(k.Public, nil, sig) {
// 		t.Error("verify should pass for nil message")
// 	}
// }

// func TestAutoLoadVerifyNilSignature(t *testing.T) {
// 	k := keys.AutoLoad()

// 	if keys.Verify(k.Public, []byte("msg"), nil) {
// 		t.Error("Verify should return false for nil signature")
// 	}
// }

// func TestAutoLoadVerifyEmptySignature(t *testing.T) {
// 	k := keys.AutoLoad()

// 	if keys.Verify(k.Public, []byte("msg"), []byte{}) {
// 		t.Error("Verify should return false for empty signature")
// 	}
// }

// func TestAutoLoadVerifyTamperedSignature(t *testing.T) {
// 	k := keys.AutoLoad()

// 	msg := []byte("hello")
// 	sig := k.Sign(msg)
// 	sig[0] ^= 0xFF

// 	if keys.Verify(k.Public, msg, sig) {
// 		t.Error("Verify should return false for tampered signature")
// 	}
// }
