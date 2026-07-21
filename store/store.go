package store

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha512"
	"database/sql"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	// sqLite driver.
	_ "modernc.org/sqlite"
)

type Store struct {
	Chats      *ChatStore
	KnownPeers *KnownPeerStore
	Files      *FileStore
}

func New(dir string) *Store {
	exe, err := os.Executable()
	if err != nil {
		panic(fmt.Errorf("store: get executable path: %w", err))
	}
	dbPath := filepath.Join(filepath.Dir(exe), dir+".db")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		panic(fmt.Errorf("store: open database: %w", err))
	}
	db.SetMaxOpenConns(1)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, q := range []string{
		"CREATE TABLE IF NOT EXISTS chats (id TEXT PRIMARY KEY, value TEXT, created_at TEXT)",
		"CREATE TABLE IF NOT EXISTS known_peers (pub_key TEXT PRIMARY KEY, peer_ip TEXT, status TEXT)",
		"CREATE TABLE IF NOT EXISTS files (id TEXT PRIMARY KEY, data BLOB, msg_id TEXT, created_at TEXT)",
	} {
		if _, err = db.ExecContext(ctx, q); err != nil {
			panic(fmt.Errorf("store: create table: %w", err))
		}
	}

	return &Store{
		Chats:      &ChatStore{db: db},
		KnownPeers: &KnownPeerStore{db: db},
		Files:      &FileStore{db: db},
	}
}

func aesKey(secret string) []byte {
	h := sha512.Sum512([]byte(secret))
	return h[:32]
}

func encryptRaw(secret string, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey(secret))
	if err != nil {
		return nil, fmt.Errorf("store: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("store: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("store: random nonce: %w", err)
	}
	return gcm.Seal(nonce, nonce, plain, nil), nil
}

func decryptRaw(secret string, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(aesKey(secret))
	if err != nil {
		return nil, fmt.Errorf("store: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("store: new gcm: %w", err)
	}
	n := gcm.NonceSize()
	if len(data) < n {
		return nil, errors.New("store: invalid ciphertext")
	}
	plain, err := gcm.Open(nil, data[:n], data[n:], nil)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	return plain, nil
}

func encrypt(secret string, plain []byte) (string, error) {
	raw, err := encryptRaw(secret, plain)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func decrypt(secret, data string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("store: decode base64: %w", err)
	}
	return decryptRaw(secret, raw)
}
