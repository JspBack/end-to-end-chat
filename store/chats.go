package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/google/uuid"
	gocache "github.com/patrickmn/go-cache"

	"github.com/JspBack/end-to-end-chat/keys"
)

type ChatStore struct {
	db    *sql.DB
	cache *gocache.Cache
}

type ChatSummary struct {
	ID        uuid.UUID `json:"id"`
	CreatedAt time.Time `json:"created_at"`
}

func (t *ChatStore) Put(value string, secret keys.Key) (uuid.UUID, error) {
	id := uuid.New()
	if err := t.PutWithID(id, value, secret); err != nil {
		return uuid.Nil, err
	}
	return id, nil
}

func (t *ChatStore) PutWithID(id uuid.UUID, value string, secret keys.Key) error {
	encrypted, err := encrypt(secret, []byte(value))
	if err != nil {
		return fmt.Errorf("store: encrypt: %w", err)
	}
	now := time.Now().UTC().Format(time.RFC3339)
	q := "INSERT OR REPLACE INTO chats (id, value, created_at) VALUES (?, ?, ?)"
	if _, err = t.db.ExecContext(context.Background(), q, id.String(), encrypted, now); err != nil {
		return fmt.Errorf("store: put: %w", err)
	}
	t.cache.Delete(id.String())
	return nil
}

func (t *ChatStore) CacheStore(id uuid.UUID, decrypted string) {
	t.cache.Set(id.String(), decrypted, gocache.DefaultExpiration)
}

func (t *ChatStore) CacheLoad(id uuid.UUID) (string, bool) {
	v, ok := t.cache.Get(id.String())
	if !ok {
		return "", false
	}
	s, _ := v.(string)
	return s, true
}

func (t *ChatStore) Get(id uuid.UUID, secret keys.Key) (string, error) {
	var encrypted string
	q := "SELECT value FROM chats WHERE id = ?"
	if err := t.db.QueryRowContext(context.Background(), q, id.String()).Scan(&encrypted); err != nil {
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

func (t *ChatStore) Update(id uuid.UUID, value string, secret keys.Key) error {
	var exists bool
	q := "SELECT EXISTS(SELECT 1 FROM chats WHERE id = ?)"
	if err := t.db.QueryRowContext(context.Background(), q, id.String()).Scan(&exists); err != nil {
		return fmt.Errorf("store: update check: %w", err)
	}
	if !exists {
		return os.ErrNotExist
	}
	return t.PutWithID(id, value, secret)
}

func (t *ChatStore) List() ([]ChatSummary, error) {
	q := "SELECT id, created_at FROM chats ORDER BY created_at, id"
	rows, err := t.db.QueryContext(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("store: list: %w", err)
	}
	defer rows.Close()

	var out []ChatSummary
	for rows.Next() {
		var id string
		var createdAt string
		if err = rows.Scan(&id, &createdAt); err != nil {
			return nil, fmt.Errorf("store: scan: %w", err)
		}
		parsedID, idErr := uuid.Parse(id)
		if idErr != nil {
			return nil, fmt.Errorf("store: parse id: %w", idErr)
		}
		parsedTime, timeErr := time.Parse(time.RFC3339, createdAt)
		if timeErr != nil {
			return nil, fmt.Errorf("store: parse created_at: %w", timeErr)
		}
		out = append(out, ChatSummary{ID: parsedID, CreatedAt: parsedTime})
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: list rows: %w", err)
	}
	return out, nil
}

func (t *ChatStore) IndexSearch(id uuid.UUID, fromName, toName, searchText string) error {
	q := "INSERT OR REPLACE INTO chat_search (msg_id, from_name, to_name, search_text) VALUES (?, ?, ?, ?)"
	_, err := t.db.ExecContext(context.Background(), q, id, fromName, toName, searchText)
	if err != nil {
		return fmt.Errorf("store: index search: %w", err)
	}
	return nil
}

func (t *ChatStore) deleteSearchIndex(id uuid.UUID) error {
	_, err := t.db.ExecContext(context.Background(), "DELETE FROM chat_search WHERE msg_id = ?", id)
	if err != nil {
		return fmt.Errorf("store: delete search index: %w", err)
	}
	return nil
}

func (t *ChatStore) Search(query string, limit int) ([]string, error) {
	q := "SELECT msg_id FROM chat_search WHERE from_name LIKE ? OR to_name LIKE ? OR search_text LIKE ? ORDER BY msg_id"
	args := []any{"%" + query + "%", "%" + query + "%", "%" + query + "%"}
	if limit > 0 {
		q += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := t.db.QueryContext(context.Background(), q, args...)
	if err != nil {
		return nil, fmt.Errorf("store: search: %w", err)
	}
	defer rows.Close()

	var ids []string
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("store: search scan: %w", err)
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: search rows: %w", err)
	}
	return ids, nil
}

func (t *ChatStore) Delete(id uuid.UUID) error {
	q := "DELETE FROM chats WHERE id = ?"
	if _, err := t.db.ExecContext(context.Background(), q, id.String()); err != nil {
		return fmt.Errorf("store: delete: %w", err)
	}
	_ = t.deleteSearchIndex(id)
	t.cache.Delete(id.String())
	return nil
}
