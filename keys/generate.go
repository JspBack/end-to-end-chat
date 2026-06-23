package keys

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
)

type Keys struct {
	Public  string
	Private string
}

func generate() []byte {
	key := make([]byte, 32)
	rand.Read(key)
	return key
}

func Load(keyFile string) *Keys {
	exe, err := os.Executable()
	if err != nil {
		panic(fmt.Errorf("keys: get executable path: %w", err))
	}
	keyFile = filepath.Join(filepath.Dir(exe), keyFile)

	b, err := os.ReadFile(keyFile)
	if err != nil {
		if err = os.MkdirAll(filepath.Dir(keyFile), 0755); err != nil {
			panic(fmt.Errorf("keys: create directory: %w", err))
		}
		b = generate()
		if err = os.WriteFile(keyFile, b, 0600); err != nil {
			panic(fmt.Errorf("keys: write key file: %w", err))
		}
	}
	return &Keys{
		Public:  hex.EncodeToString(generate()),
		Private: hex.EncodeToString(b),
	}
}

func (k *Keys) Derive() string {
	h := sha256.Sum256([]byte(k.Private))
	return hex.EncodeToString(h[:])
}
