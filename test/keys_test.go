package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JspBack/end-to-end-chat/keys"
)

func TestLoadGeneratesKeysOnFirstCall(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key")

	k := keys.Load(keyFile)
	if k.Public == "" {
		t.Error("Public key is empty")
	}
	if k.Private == "" {
		t.Error("Private key is empty")
	}
}

func TestLoadWritesKeyFile(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key")

	keys.Load(keyFile)
	if _, err := os.Stat(keyFile); os.IsNotExist(err) {
		t.Fatal("key file not created by Load")
	}
}

func TestLoadReturnsSamePrivateOnSubsequentCalls(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key")

	first := keys.Load(keyFile)
	second := keys.Load(keyFile)
	if first.Private != second.Private {
		t.Fatal("Private key changed between Load calls")
	}
}

func TestLoadReturnsDifferentPublicEachCall(t *testing.T) {
	keyFile := filepath.Join(t.TempDir(), "key")

	first := keys.Load(keyFile)
	second := keys.Load(keyFile)
	if first.Public == second.Public {
		t.Fatal("Public key should be different each Load")
	}
}
