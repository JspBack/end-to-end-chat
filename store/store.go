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

	"github.com/google/uuid"
	// sqLite driver.
	_ "github.com/mattn/go-sqlite3"
)

type Store struct {
	db *sql.DB
}

func New(dbName string) *Store {
	if err := os.MkdirAll(filepath.Dir(dbName), 0755); err != nil {
		panic(fmt.Errorf("store: create directory: %w", err))
	}
	db, err := sql.Open("sqlite3", dbName)
	if err != nil {
		panic(fmt.Errorf("store: open database: %w", err))
	}
	if _, err = db.ExecContext(context.Background(), "CREATE TABLE IF NOT EXISTS chats (id TEXT PRIMARY KEY, value TEXT)"); err != nil {
		panic(fmt.Errorf("store: create table: %w", err))
	}
	return &Store{db: db}
}

func aesKey(secret string) []byte {
	h := sha512.Sum512([]byte(secret))
	return h[:32]
}

func encrypt(secret string, plain []byte) (string, error) {
	block, err := aes.NewCipher(aesKey(secret))
	if err != nil {
		return "", fmt.Errorf("store: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("store: new gcm: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", fmt.Errorf("store: random nonce: %w", err)
	}
	return base64.StdEncoding.EncodeToString(gcm.Seal(nonce, nonce, plain, nil)), nil
}

func decrypt(secret, data string) ([]byte, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		return nil, fmt.Errorf("store: decode base64: %w", err)
	}
	block, err := aes.NewCipher(aesKey(secret))
	if err != nil {
		return nil, fmt.Errorf("store: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("store: new gcm: %w", err)
	}
	n := gcm.NonceSize()
	if len(ciphertext) < n {
		return nil, errors.New("store: invalid ciphertext")
	}
	plain, err := gcm.Open(nil, ciphertext[:n], ciphertext[n:], nil)
	if err != nil {
		return nil, fmt.Errorf("store: open: %w", err)
	}
	return plain, nil
}

func (s *Store) Put(value, secret string) (string, error) {
	id := uuid.New().String()
	encrypted, err := encrypt(secret, []byte(value))
	if err != nil {
		return "", fmt.Errorf("store: put encrypt: %w", err)
	}
	if _, err = s.db.ExecContext(context.Background(), "INSERT OR REPLACE INTO chats (id, value) VALUES (?, ?)", id, encrypted); err != nil {
		return "", fmt.Errorf("store: put: %w", err)
	}
	return id, nil
}

func (s *Store) Get(id, secret string) (string, error) {
	var encrypted string
	if err := s.db.QueryRowContext(context.Background(), "SELECT value FROM chats WHERE id = ?", id).Scan(&encrypted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("store: get: %w", err)
	}
	plain, err := decrypt(secret, encrypted)
	if err != nil {
		return "", fmt.Errorf("store: get decrypt: %w", err)
	}
	return string(plain), nil
}

func (s *Store) Update(id, value, secret string) error {
	var exists bool
	if err := s.db.QueryRowContext(context.Background(), "SELECT EXISTS(SELECT 1 FROM chats WHERE id = ?)", id).Scan(&exists); err != nil {
		return fmt.Errorf("store: update check: %w", err)
	}
	if !exists {
		return os.ErrNotExist
	}
	encrypted, err := encrypt(secret, []byte(value))
	if err != nil {
		return fmt.Errorf("store: update encrypt: %w", err)
	}
	if _, err = s.db.ExecContext(context.Background(), "INSERT OR REPLACE INTO chats (id, value) VALUES (?, ?)", id, encrypted); err != nil {
		return fmt.Errorf("store: update: %w", err)
	}
	return nil
}

func (s *Store) Delete(id string) error {
	if _, err := s.db.ExecContext(context.Background(), "DELETE FROM chats WHERE id = ?", id); err != nil {
		return fmt.Errorf("store: delete: %w", err)
	}
	return nil
}
