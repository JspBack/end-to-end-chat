package signal

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"
)

const (
	TypeMessage      = "message"
	TypeDelete       = "delete"
	TypeUpdate       = "update"
	TypeFileMeta     = "file_meta"
	TypeKeyExchange  = "key_exchange"
	TypeInfoRequest  = "info_request"
	TypeInfoResponse = "info_response"
	TypeInfoChanged  = "info_changed"
	TypePeerAccepted = "peer_accepted"
)

type Signal struct {
	Type    string    `json:"type"`
	From    string    `json:"from,omitempty"`
	ID      uuid.UUID `json:"id"`
	Content []byte    `json:"content,omitempty"`
}

func Parse(data []byte) (*Signal, error) {
	raw := struct {
		Type    string          `json:"type"`
		From    string          `json:"from,omitempty"`
		ID      json.RawMessage `json:"id,omitempty"`
		Content []byte          `json:"content,omitempty"`
	}{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("signal: parse: %w", err)
	}
	var id uuid.UUID
	if len(raw.ID) > 0 {
		var s string
		if err := json.Unmarshal(raw.ID, &s); err == nil {
			id, _ = uuid.Parse(s)
		}
	}
	return &Signal{Type: raw.Type, From: raw.From, ID: id, Content: raw.Content}, nil
}

func New(typ, from string, id uuid.UUID, content []byte) []byte {
	raw, _ := json.Marshal(Signal{Type: typ, From: from, ID: id, Content: content})
	return raw
}
