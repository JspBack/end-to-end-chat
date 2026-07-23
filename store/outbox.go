package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/signal"
)

type OutboxSignal struct {
	ID            uuid.UUID   `json:"id"`
	TargetPubKey  keys.Key    `json:"target_pub_key"`
	SignalType    signal.Type `json:"signal_type"`
	SignalFrom    keys.Key    `json:"signal_from"`
	SignalID      uuid.UUID   `json:"signal_id"`
	SignalContent []byte      `json:"signal_content"`
	CreatedAt     time.Time   `json:"created_at,omitzero"`
	RetryCount    int         `json:"retry_count"`
}

type OutboxStore struct {
	db *sql.DB
}

func (o *OutboxStore) Put(targetPubKey keys.Key, rawSignalBytes []byte, secret keys.Key) (uuid.UUID, error) {
	enc, err := encryptRaw(secret, rawSignalBytes)
	if err != nil {
		return uuid.Nil, fmt.Errorf("store: encrypt outbox: %w", err)
	}
	var meta signal.Signal
	_ = json.Unmarshal(rawSignalBytes, &meta)
	id := uuid.New()
	now := time.Now().UTC().Format(time.RFC3339)
	q := `INSERT INTO outbox (id, target_pub_key, signal_type, signal_from, signal_id, signal_content, created_at)
	      VALUES (?, ?, ?, ?, ?, ?, ?)`
	if _, err = o.db.ExecContext(context.Background(), q, id.String(), targetPubKey.String(),
		string(meta.Type), meta.From.String(), meta.ID.String(), enc, now); err != nil {
		return uuid.Nil, fmt.Errorf("store: outbox put: %w", err)
	}
	return id, nil
}

func (o *OutboxStore) Get(targetPubKey, secret keys.Key) ([]OutboxSignal, error) {
	q := `SELECT id, target_pub_key, signal_type, signal_from, signal_id, signal_content, created_at, retry_count
	      FROM outbox WHERE target_pub_key = ? ORDER BY created_at`
	rows, err := o.db.QueryContext(context.Background(), q, targetPubKey.String())
	if err != nil {
		return nil, fmt.Errorf("store: outbox get: %w", err)
	}
	defer rows.Close()
	var out []OutboxSignal
	for rows.Next() {
		var e OutboxSignal
		var enc []byte
		var createdAt string
		if err = rows.Scan(&e.ID, &e.TargetPubKey, &e.SignalType, &e.SignalFrom, &e.SignalID, &enc, &createdAt, &e.RetryCount); err != nil {
			return nil, fmt.Errorf("store: outbox scan: %w", err)
		}
		parsedTime, timeErr := time.Parse(time.RFC3339, createdAt)
		if timeErr != nil {
			return nil, fmt.Errorf("store: outbox parse created_at: %w", timeErr)
		}
		e.CreatedAt = parsedTime
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

func (o *OutboxStore) List() ([]OutboxSignal, error) {
	q := `SELECT id, target_pub_key, signal_type, signal_from, signal_id, created_at, retry_count FROM outbox ORDER BY created_at`
	rows, err := o.db.QueryContext(context.Background(), q)
	if err != nil {
		return nil, fmt.Errorf("store: outbox list: %w", err)
	}
	defer rows.Close()
	var out []OutboxSignal
	for rows.Next() {
		var e OutboxSignal
		var createdAt string
		if err = rows.Scan(&e.ID, &e.TargetPubKey, &e.SignalType, &e.SignalFrom, &e.SignalID, &createdAt, &e.RetryCount); err != nil {
			return nil, fmt.Errorf("store: outbox scan: %w", err)
		}
		parsedTime, timeErr := time.Parse(time.RFC3339, createdAt)
		if timeErr != nil {
			return nil, fmt.Errorf("store: outbox parse created_at: %w", timeErr)
		}
		e.CreatedAt = parsedTime
		out = append(out, e)
	}
	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("store: outbox rows: %w", err)
	}
	return out, nil
}

func (o *OutboxStore) Delete(id uuid.UUID) error {
	q := "DELETE FROM outbox WHERE id = ?"
	if _, err := o.db.ExecContext(context.Background(), q, id.String()); err != nil {
		return fmt.Errorf("store: outbox delete: %w", err)
	}
	return nil
}

func (o *OutboxStore) IncrementRetry(id uuid.UUID) error {
	q := "UPDATE outbox SET retry_count = retry_count + 1 WHERE id = ?"
	if _, err := o.db.ExecContext(context.Background(), q, id.String()); err != nil {
		return fmt.Errorf("store: outbox increment retry: %w", err)
	}
	return nil
}
