package signal

import (
	"encoding/json"
	"fmt"
)

const (
	TypeMessage = "message"
	TypeDelete  = "delete"
	TypeUpdate  = "update"
)

type Signal struct {
	Type    string `json:"type"`
	From    string `json:"from,omitempty"`
	ID      string `json:"id,omitempty"`
	Content string `json:"content,omitempty"`
}

func Parse(data []byte) (*Signal, error) {
	var s Signal
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("signal: parse: %w", err)
	}
	return &s, nil
}

func New(typ, from, id, content string) []byte {
	raw, _ := json.Marshal(Signal{Type: typ, From: from, ID: id, Content: content})
	return raw
}
