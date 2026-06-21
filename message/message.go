package message

import (
	"encoding/json"
	"fmt"

	"github.com/JspBack/end-to-end-chat/store"
)

type Message struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Content string `json:"content"`
}

func NewMessage(from, to, content string) *Message {
	return &Message{From: from, To: to, Content: content}
}

func (m *Message) Encode() ([]byte, error) {
	out, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("message: encode: %w", err)
	}
	return out, nil
}

func ToMessage(data []byte) (*Message, error) {
	var m Message
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("message: decode: %w", err)
	}
	return &m, nil
}

func Put(s *store.Store, secret string, msg *Message) (string, error) {
	plain, err := json.Marshal(msg)
	if err != nil {
		return "", fmt.Errorf("message: marshal: %w", err)
	}
	id, err := s.Chats.Put(string(plain), secret)
	if err != nil {
		return "", fmt.Errorf("message: store put: %w", err)
	}
	return id, nil
}

func Get(s *store.Store, secret, id string) (*Message, error) {
	plain, err := s.Chats.Get(id, secret)
	if err != nil {
		return nil, fmt.Errorf("message: store get: %w", err)
	}
	var msg Message
	if err = json.Unmarshal([]byte(plain), &msg); err != nil {
		return nil, fmt.Errorf("message: unmarshal: %w", err)
	}
	return &msg, nil
}

func Update(s *store.Store, secret, id string, msg *Message) error {
	plain, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("message: marshal: %w", err)
	}
	if err = s.Chats.Update(id, string(plain), secret); err != nil {
		return fmt.Errorf("message: store update: %w", err)
	}
	return nil
}

func Delete(s *store.Store, id string) error {
	if err := s.Chats.Delete(id); err != nil {
		return fmt.Errorf("message: store delete: %w", err)
	}
	return nil
}
