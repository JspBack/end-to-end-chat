package keys

import (
	"crypto/ed25519"
	"crypto/rand"
	"crypto/sha256"
	"database/sql/driver"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const minQuotedHexLen = 2

//nolint:recvcheck // Unmarshal/Scan must be pointer receivers to satisfy
type (
	Key        [32]byte
	PubKey     = Key
	PrivateKey = Key
)

type Keys struct {
	Public  Key
	Private Key
	sign    ed25519.PrivateKey
}

var NilKey Key

func (p Key) String() string {
	return hex.EncodeToString(p[:])
}
func (p Key) MarshalJSON() ([]byte, error) {
	return []byte(`"` + p.String() + `"`), nil
}

func FromHex(s string) (Key, error) {
	if s == "" {
		return NilKey, errors.New("keys: empty key")
	}
	var k Key
	buf, err := hex.DecodeString(s)
	if err != nil {
		return k, fmt.Errorf("keys: decode hex: %w", err)
	}
	if len(buf) != len(k) {
		return k, fmt.Errorf("keys: unexpected key length %d", len(buf))
	}
	copy(k[:], buf)
	return k, nil
}
func (p *Key) UnmarshalJSON(b []byte) error {
	s, err := parseHexQuoted(b)
	if err != nil {
		return err
	}
	k, err := FromHex(s)
	if err != nil {
		return err
	}
	*p = k
	return nil
}
func (p Key) MarshalBinary() ([]byte, error) {
	return p[:], nil
}
func (p *Key) UnmarshalBinary(b []byte) error {
	if len(b) != len(p) {
		return fmt.Errorf("keys: unexpected binary length %d", len(b))
	}
	copy(p[:], b)
	return nil
}
func (p Key) Value() (driver.Value, error) {
	return p.String(), nil
}
func (p *Key) Scan(src any) error {
	var s string
	switch v := src.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("keys: cannot scan type %T", src)
	}
	k, err := FromHex(s)
	if err != nil {
		return err
	}
	*p = k
	return nil
}
func parseHexQuoted(b []byte) (string, error) {
	if len(b) < minQuotedHexLen {
		return "", errors.New("keys: too short for quoted hex")
	}
	if b[0] != '"' || b[len(b)-1] != '"' {
		return "", errors.New("keys: not a quoted string")
	}
	return string(b[1 : len(b)-1]), nil
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
			pubBytes, _ := priv.Public().(ed25519.PublicKey)
			var pub PubKey
			copy(pub[:], pubBytes)
			var privKey PrivateKey
			copy(privKey[:], seed)
			return &Keys{
				Private: privKey,
				Public:  pub,
				sign:    priv,
			}
		}
	}
	seed := generateRandomSeed()
	priv := ed25519.NewKeyFromSeed(seed)
	pubBytes, _ := priv.Public().(ed25519.PublicKey)
	var pub PubKey
	copy(pub[:], pubBytes)
	var privKey PrivateKey
	copy(privKey[:], seed)
	k := &Keys{
		Private: privKey,
		Public:  pub,
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
func Verify(pubKey PubKey, msg, sig []byte) bool {
	return ed25519.Verify(pubKey[:], msg, sig)
}
func (k *Keys) Derive() string {
	h := sha256.Sum256(k.Private[:])
	return hex.EncodeToString(h[:8])
}
