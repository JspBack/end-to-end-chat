package message

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/JspBack/end-to-end-chat/store"
)

func Search(s *store.Store, secret, query string, limit int) ([]Message, error) {
	if query == "" {
		return nil, nil
	}
	q := strings.ToLower(query)

	summaries, err := s.Chats.List()
	if err != nil {
		return nil, fmt.Errorf("message: list: %w", err)
	}

	var out []Message
	for _, sum := range summaries {
		msg, e := Get(s, secret, sum.ID)
		if e != nil {
			continue
		}
		if !strings.Contains(strings.ToLower(msg.Content), q) &&
			!strings.Contains(strings.ToLower(msg.From), q) &&
			!strings.Contains(strings.ToLower(msg.To), q) {
			continue
		}
		out = append(out, *msg)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, nil
}

type Message struct {
	ID      string `json:"id,omitempty"`
	From    string `json:"from"`
	To      string `json:"to"`
	Content string `json:"content"`
	Time    string `json:"time"`
}

func NewMessage(from, to, content string) *Message {
	return &Message{From: from, To: to, Content: content, Time: time.Now().Format(time.RFC3339)}
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
	var id string
	if msg.ID != "" {
		id = msg.ID
		if err = s.Chats.PutWithID(id, string(plain), secret); err != nil {
			return "", fmt.Errorf("message: store put: %w", err)
		}
	} else {
		id, err = s.Chats.Put(string(plain), secret)
		if err != nil {
			return "", fmt.Errorf("message: store put: %w", err)
		}
		msg.ID = id
	}
	s.Chats.CacheStore(id, string(plain))
	return id, nil
}

func Get(s *store.Store, secret, id string) (*Message, error) {
	if cached, ok := s.Chats.CacheLoad(id); ok {
		var msg Message
		if err := json.Unmarshal([]byte(cached), &msg); err != nil {
			return nil, fmt.Errorf("message: unmarshal: %w", err)
		}
		return &msg, nil
	}
	plain, err := s.Chats.Get(id, secret)
	if err != nil {
		return nil, fmt.Errorf("message: store get: %w", err)
	}
	var msg Message
	if err = json.Unmarshal([]byte(plain), &msg); err != nil {
		return nil, fmt.Errorf("message: unmarshal: %w", err)
	}
	s.Chats.CacheStore(id, plain)
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
	s.Chats.CacheStore(id, string(plain))
	return nil
}

func Delete(s *store.Store, id string) error {
	if err := s.Chats.Delete(id); err != nil {
		return fmt.Errorf("message: store delete: %w", err)
	}
	return nil
}
