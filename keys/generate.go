package keys

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Keys struct {
	Public  string
	Private string

	sign ed25519.PrivateKey
}

func generateRandomSeed() []byte {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		panic(fmt.Errorf("keys: generate random seed: %w", err))
	}
	return seed
}

func keysDir() string {
	exe, err := os.Executable()
	if err != nil {
		panic(fmt.Errorf("keys: get executable path: %w", err))
	}
	return filepath.Dir(exe)
}

func findExistingKey(dir string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(e.Name(), ".") && strings.HasSuffix(e.Name(), "_key") {
			return filepath.Join(dir, e.Name())
		}
	}
	return ""
}

func AutoLoad() *Keys {
	dir := keysDir()

	if path := findExistingKey(dir); path != "" {
		seed, err := os.ReadFile(path)
		if err == nil && len(seed) == ed25519.SeedSize {
			priv := ed25519.NewKeyFromSeed(seed)
			pub, _ := priv.Public().(ed25519.PublicKey)
			return &Keys{
				Private: hex.EncodeToString(seed),
				Public:  hex.EncodeToString(pub),
				sign:    priv,
			}
		}
	}

	seed := generateRandomSeed()
	priv := ed25519.NewKeyFromSeed(seed)
	pub, _ := priv.Public().(ed25519.PublicKey)

	k := &Keys{
		Private: hex.EncodeToString(seed),
		Public:  hex.EncodeToString(pub),
		sign:    priv,
	}

	keyFile := filepath.Join(dir, "."+k.Derive()+"_key")
	if err := os.WriteFile(keyFile, seed, 0600); err != nil {
		panic(fmt.Errorf("keys: write key file: %w", err))
	}

	return k
}

func (k *Keys) Sign(msg []byte) []byte {
	return ed25519.Sign(k.sign, msg)
}

func Verify(pubKeyHex string, msg, sig []byte) bool {
	pub, err := hex.DecodeString(pubKeyHex)
	if err != nil || len(pub) != ed25519.PublicKeySize {
		return false
	}
	return ed25519.Verify(pub, msg, sig)
}

func (k *Keys) Derive() string {
	h := sha256.Sum256([]byte(k.Private))
	return hex.EncodeToString(h[:8])
}
