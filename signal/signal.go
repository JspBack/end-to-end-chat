package signal

import (
	"encoding/json"
	"fmt"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/keys"
)

type Type string

const (
	Message      Type = "message"
	Delete       Type = "delete"
	Update       Type = "update"
	FileMeta     Type = "file_meta"
	KeyExchange  Type = "key_exchange"
	InfoRequest  Type = "info_request"
	InfoResponse Type = "info_response"
	InfoChanged  Type = "info_changed"
	PeerAccepted Type = "peer_accepted"
)

type Signal struct {
	Type    Type      `json:"type"`
	From    keys.Key  `json:"from,omitempty"`
	ID      uuid.UUID `json:"id"`
	Content []byte    `json:"content,omitempty"`
}

func Parse(data []byte) (*Signal, error) {
	raw := struct {
		Type    Type            `json:"type"`
		From    keys.Key        `json:"from,omitempty"`
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

func New(typ Type, from keys.Key, id uuid.UUID, content []byte) []byte {
	raw, _ := json.Marshal(Signal{Type: typ, From: from, ID: id, Content: content})
	return raw
}
