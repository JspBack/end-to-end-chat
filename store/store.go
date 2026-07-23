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

	gocache "github.com/patrickmn/go-cache"
	// sqLite driver.
	_ "modernc.org/sqlite"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/keys"
)

type Store struct {
	Chats      *ChatStore
	KnownPeers *KnownPeerStore
	Files      *FileStore
	Outbox     *OutboxStore
	Profile    *ProfileStore
}

func New(dir string) *Store {
	if !filepath.IsAbs(dir) {
		exe, err := os.Executable()
		if err != nil {
			panic(fmt.Errorf("store: get executable path: %w", err))
		}
		dir = filepath.Join(filepath.Dir(exe), dir)
	}
	dbPath := dir + ".db"

	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		panic(fmt.Errorf("store: open database: %w", err))
	}
	db.SetMaxOpenConns(8)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	for _, q := range []string{
		"CREATE TABLE IF NOT EXISTS chats (id TEXT PRIMARY KEY, value TEXT, created_at TEXT)",
		"CREATE TABLE IF NOT EXISTS known_peers (pub_key TEXT PRIMARY KEY, peer_ip TEXT, " +
			"name TEXT, nickname TEXT, status TEXT, last_seen TEXT, profile_pic TEXT)",
		"CREATE TABLE IF NOT EXISTS files (id TEXT PRIMARY KEY, data BLOB, msg_id TEXT, created_at TEXT)",
		"CREATE TABLE IF NOT EXISTS chat_search (msg_id TEXT PRIMARY KEY, from_name TEXT, to_name TEXT, search_text TEXT)",
		`CREATE TABLE IF NOT EXISTS outbox (
			id TEXT PRIMARY KEY,
			target_pub_key TEXT,
			signal_type TEXT,
			signal_from TEXT,
			signal_id TEXT,
			signal_content BLOB,
			created_at TEXT,
			retry_count INTEGER DEFAULT 0
		)`,
		"CREATE TABLE IF NOT EXISTS profile (name TEXT NOT NULL DEFAULT '', profile_pic TEXT NOT NULL DEFAULT '')",

		"CREATE INDEX IF NOT EXISTS idx_chats_created_at ON chats (created_at, id)",
		"CREATE INDEX IF NOT EXISTS idx_files_msg_id ON files (msg_id)",
		"CREATE INDEX IF NOT EXISTS idx_outbox_target ON outbox (target_pub_key)",
	} {
		if _, err = db.ExecContext(ctx, q); err != nil {
			panic(fmt.Errorf("store: create table: %w", err))
		}
	}

	return &Store{
		Chats:      &ChatStore{db: db, cache: gocache.New(config.CacheDefaultExp, config.CacheCleanUpInterval)},
		KnownPeers: &KnownPeerStore{db: db},
		Files:      &FileStore{db: db},
		Outbox:     &OutboxStore{db: db},
		Profile:    &ProfileStore{db: db},
	}
}

func aesKey(secret keys.Key) []byte {
	h := sha512.Sum512(secret[:])
	return h[:config.AesKeySize]
}

func encryptRaw(secret keys.Key, plain []byte) ([]byte, error) {
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

func decryptRaw(secret keys.Key, data []byte) ([]byte, error) {
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

func encrypt(secret keys.Key, plain []byte) (string, error) {
	raw, err := encryptRaw(secret, plain)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(raw), nil
}

func decrypt(secret keys.Key, data string) ([]byte, error) {
	raw, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("store: decode base64: %w", err)
	}
	return decryptRaw(secret, raw)
}
