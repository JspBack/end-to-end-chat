package signal

import (
	"encoding/json"
	"fmt"
)

const (
	TypeMessage     = "message"
	TypeDelete      = "delete"
	TypeUpdate      = "update"
	TypeFileMeta    = "file_meta"
	TypeKeyExchange = "key_exchange"
)

type Signal struct {
	Type    string `json:"type"`
	From    string `json:"from,omitempty"`
	ID      string `json:"id,omitempty"`
	Content []byte `json:"content,omitempty"`
}

func Parse(data []byte) (*Signal, error) {
	var s Signal
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("signal: parse: %w", err)
	}
	return &s, nil
}

func New(typ, from, id string, content []byte) []byte {
	raw, _ := json.Marshal(Signal{Type: typ, From: from, ID: id, Content: content})
	return raw
}
