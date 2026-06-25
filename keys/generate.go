package keys

import (
	"crypto/ed25519"
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

	sign ed25519.PrivateKey
}

func generateRandomSeed() []byte {
	seed := make([]byte, ed25519.SeedSize)
	if _, err := rand.Read(seed); err != nil {
		panic(fmt.Errorf("keys: generate random seed: %w", err))
	}
	return seed
}

func Load(keyFile string) *Keys {
	exe, err := os.Executable()
	if err != nil {
		panic(fmt.Errorf("keys: get executable path: %w", err))
	}
	keyFile = filepath.Join(filepath.Dir(exe), keyFile)

	seed, err := os.ReadFile(keyFile)
	if err != nil {
		if err = os.MkdirAll(filepath.Dir(keyFile), 0755); err != nil {
			panic(fmt.Errorf("keys: create directory: %w", err))
		}
		seed = generateRandomSeed()
		if err = os.WriteFile(keyFile, seed, 0600); err != nil {
			panic(fmt.Errorf("keys: write key file: %w", err))
		}
	}
	if len(seed) != ed25519.SeedSize {
		panic(fmt.Errorf("keys: seed file has unexpected length %d (want %d)", len(seed), ed25519.SeedSize))
	}

	priv := ed25519.NewKeyFromSeed(seed)
	pub, _ := priv.Public().(ed25519.PublicKey)

	return &Keys{
		Private: hex.EncodeToString(seed),
		Public:  hex.EncodeToString(pub),
		sign:    priv,
	}
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
	return hex.EncodeToString(h[:])
}
