package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
)

type ChatStore struct {
	db *sql.DB
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
	q := "INSERT OR REPLACE INTO chats (id, value) VALUES (?, ?)"
	if _, err = t.db.ExecContext(context.Background(), q, id, encrypted); err != nil {
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

func (t *ChatStore) List() ([]string, error) {
	q := "SELECT id FROM chats ORDER BY rowid"
	rows, err := t.db.QueryContext(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("store: list: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list rows: %w", err)
	}
	return ids, nil
}

func (t *ChatStore) Delete(id string) error {
	q := "DELETE FROM chats WHERE id = ?"
	if _, err := t.db.ExecContext(context.Background(), q, id); err != nil {
		return fmt.Errorf("store: delete: %w", err)
	}
	return nil
}
