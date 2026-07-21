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

type FileStore struct {
	db *sql.DB
}

type FileEntry struct {
	ID    string `json:"id"`
	MsgID string `json:"msg_id"`
}

func (f *FileStore) Put(secret, msgID string, data []byte) (string, error) {
	id := uuid.New().String()
	if err := f.PutWithID(secret, id, msgID, data); err != nil {
		return "", err
	}
	return id, nil
}

func (f *FileStore) PutWithID(secret, id, msgID string, data []byte) error {
	encrypted, err := encryptRaw(secret, data)
	if err != nil {
		return fmt.Errorf("store: encrypt file: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	q := "INSERT OR REPLACE INTO files (id, data, msg_id, created_at) VALUES (?, ?, ?, ?)"
	if _, err = f.db.ExecContext(context.Background(), q, id, encrypted, msgID, now); err != nil {
		return fmt.Errorf("store: file put: %w", err)
	}
	return nil
}

func (f *FileStore) Get(secret, id string) ([]byte, error) {
	var encrypted []byte
	q := "SELECT data FROM files WHERE id = ?"
	if err := f.db.QueryRowContext(context.Background(), q, id).Scan(&encrypted); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, os.ErrNotExist
		}
		return nil, fmt.Errorf("store: file get: %w", err)
	}
	plain, err := decryptRaw(secret, encrypted)
	if err != nil {
		return nil, fmt.Errorf("store: file decrypt: %w", err)
	}
	return plain, nil
}

func (f *FileStore) Delete(id string) error {
	q := "DELETE FROM files WHERE id = ?"
	if _, err := f.db.ExecContext(context.Background(), q, id); err != nil {
		return fmt.Errorf("store: file delete: %w", err)
	}
	return nil
}

func (f *FileStore) DeleteByMessage(msgID string) error {
	q := "DELETE FROM files WHERE msg_id = ?"
	if _, err := f.db.ExecContext(context.Background(), q, msgID); err != nil {
		return fmt.Errorf("store: file delete by message: %w", err)
	}
	return nil
}

func (f *FileStore) ListByMessage(msgID string) ([]FileEntry, error) {
	q := "SELECT id FROM files WHERE msg_id = ?"
	rows, err := f.db.QueryContext(context.Background(), q, msgID)
	if err != nil {
		return nil, fmt.Errorf("store: list files: %w", err)
	}
	defer rows.Close()

	var out []FileEntry
	for rows.Next() {
		var e FileEntry
		e.MsgID = msgID
		if err = rows.Scan(&e.ID); err != nil {
			return nil, fmt.Errorf("store: scan file: %w", err)
		}
		out = append(out, e)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list files rows: %w", err)
	}
	return out, nil
}
