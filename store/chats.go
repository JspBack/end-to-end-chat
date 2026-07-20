package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
)

type ChatStore struct {
	db *sql.DB
}

type ChatSummary struct {
	ID        string `json:"id"`
	CreatedAt string `json:"created_at"`
}

func (t *ChatStore) Put(value, secret string) (string, error) {
	id := uuid.New().String()
	if err := t.PutWithID(id, value, secret); err != nil {
		return "", err
	}
	return id, nil
}

func (t *ChatStore) PutWithID(id, value, secret string) error {
	encrypted, err := encrypt(secret, []byte(value))
	if err != nil {
		return fmt.Errorf("store: encrypt: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	q := "INSERT OR REPLACE INTO chats (id, value, created_at) VALUES (?, ?, ?)"
	if _, err = t.db.ExecContext(context.Background(), q, id, encrypted, now); err != nil {
		return fmt.Errorf("store: put: %w", err)
	}
	return nil
}

func (t *ChatStore) Get(id, secret string) (string, error) {
	var encrypted string
	q := "SELECT value FROM chats WHERE id = ?"
	if err := t.db.QueryRowContext(context.Background(), q, id).Scan(&encrypted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", os.ErrNotExist
		}
		return "", fmt.Errorf("store: get: %w", err)
	}
	plain, err := decrypt(secret, encrypted)
	if err != nil {
		return "", fmt.Errorf("store: decrypt: %w", err)
	}
	return string(plain), nil
}

func (t *ChatStore) Update(id, value, secret string) error {
	var exists bool
	q := "SELECT EXISTS(SELECT 1 FROM chats WHERE id = ?)"
	if err := t.db.QueryRowContext(context.Background(), q, id).Scan(&exists); err != nil {
		return fmt.Errorf("store: update check: %w", err)
	}
	if !exists {
		return os.ErrNotExist
	}
	return t.PutWithID(id, value, secret)
}

func (t *ChatStore) List() ([]ChatSummary, error) {
	q := "SELECT id, COALESCE(created_at, '') FROM chats ORDER BY created_at, id"
	rows, err := t.db.QueryContext(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("store: list: %w", err)
	}
	defer rows.Close()

	var out []ChatSummary
	for rows.Next() {
		var s ChatSummary
		if err = rows.Scan(&s.ID, &s.CreatedAt); err != nil {
			return nil, fmt.Errorf("store: scan: %w", err)
		}
		out = append(out, s)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list rows: %w", err)
	}
	return out, nil
}

func (t *ChatStore) Delete(id string) error {
	q := "DELETE FROM chats WHERE id = ?"
	if _, err := t.db.ExecContext(context.Background(), q, id); err != nil {
		return fmt.Errorf("store: delete: %w", err)
	}
	return nil
}
