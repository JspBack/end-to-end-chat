package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
)

type OutboxEntry struct {
	ID           string `json:"id"`
	TargetPubKey string `json:"target_pub_key"`
	SignalType   string `json:"signal_type"`
	CreatedAt    string `json:"created_at"`
	RetryCount   int    `json:"retry_count"`
}

type OutboxStore struct {
	db *sql.DB
}

func NewOutboxStore(db *sql.DB) *OutboxStore {
	return &OutboxStore{db: db}
}

type rawSignalMeta struct {
	Type string `json:"type"`
	From string `json:"from,omitempty"`
	ID   string `json:"id,omitempty"`
}

func (o *OutboxStore) Put(targetPubKey string, rawSignalBytes []byte, secret string) (string, error) {
	enc, err := encryptRaw(secret, rawSignalBytes)
	if err != nil {
		return "", fmt.Errorf("store: encrypt outbox: %w", err)
	}
	var meta rawSignalMeta
	_ = json.Unmarshal(rawSignalBytes, &meta)

	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	q := `INSERT INTO outbox (id, target_pub_key, signal_type, signal_from, signal_id, signal_content, created_at)
	      VALUES (?, ?, ?, ?, ?, ?, ?)`
	if _, err = o.db.ExecContext(context.Background(), q, id, targetPubKey, meta.Type, meta.From, meta.ID, enc, now); err != nil {
		return "", fmt.Errorf("store: outbox put: %w", err)
	}
	return id, nil
}

func (o *OutboxStore) GetPending(targetPubKey, secret string) ([]OutboxSignal, error) {
	q := `SELECT id, signal_type, signal_from, signal_id, signal_content, retry_count
	      FROM outbox WHERE target_pub_key = ? ORDER BY created_at`
	rows, err := o.db.QueryContext(context.Background(), q, targetPubKey)
	if err != nil {
		return nil, fmt.Errorf("store: outbox get pending: %w", err)
	}
	defer rows.Close()

	var out []OutboxSignal
	for rows.Next() {
		var e OutboxSignal
		var enc []byte
		if err = rows.Scan(&e.ID, &e.SignalType, &e.SignalFrom, &e.SignalID, &enc, &e.RetryCount); err != nil {
			return nil, fmt.Errorf("store: outbox scan: %w", err)
		}
		plain, decErr := decryptRaw(secret, enc)
		if decErr != nil {
			return nil, fmt.Errorf("store: outbox decrypt: %w", decErr)
		}
		e.SignalContent = plain
		out = append(out, e)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: outbox rows: %w", err)
	}
	return out, nil
}

func (o *OutboxStore) GetAllPending() ([]OutboxEntry, error) {
	q := `SELECT id, target_pub_key, signal_type, created_at, retry_count FROM outbox ORDER BY created_at`
	rows, err := o.db.QueryContext(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("store: outbox list all: %w", err)
	}
	defer rows.Close()

	var out []OutboxEntry
	for rows.Next() {
		var e OutboxEntry
		if err = rows.Scan(&e.ID, &e.TargetPubKey, &e.SignalType, &e.CreatedAt, &e.RetryCount); err != nil {
			return nil, fmt.Errorf("store: outbox scan: %w", err)
		}
		out = append(out, e)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: outbox rows: %w", err)
	}
	return out, nil
}

func (o *OutboxStore) Delete(id string) error {
	q := "DELETE FROM outbox WHERE id = ?"
	if _, err := o.db.ExecContext(context.Background(), q, id); err != nil {
		return fmt.Errorf("store: outbox delete: %w", err)
	}
	return nil
}

func (o *OutboxStore) IncrementRetry(id string) error {
	q := "UPDATE outbox SET retry_count = retry_count + 1 WHERE id = ?"
	if _, err := o.db.ExecContext(context.Background(), q, id); err != nil {
		return fmt.Errorf("store: outbox increment retry: %w", err)
	}
	return nil
}

type OutboxSignal struct {
	ID            string
	SignalType    string
	SignalFrom    string
	SignalID      string
	SignalContent []byte
	RetryCount    int
}
