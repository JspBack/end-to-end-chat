package message

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/store"
)

func StoreAttachments(s *store.Store, secret keys.Key, msgID string, attachments []Attachment) error {
	for i := range attachments {
		if len(attachments[i].Data) == 0 {
			continue
		}
		id := attachments[i].ID
		if id == uuid.Nil {
			id = uuid.New()
		}
		if err := s.Files.PutWithID(secret, id, msgID, attachments[i].Data); err != nil {
			return fmt.Errorf("message: store attachment: %w", err)
		}
		attachments[i].ID = id
		attachments[i].Size = int64(len(attachments[i].Data))
		attachments[i].Data = nil
	}
	return nil
}

func Search(s *store.Store, secret keys.Key, query string, limit int) ([]Message, error) {
	if query == "" {
		return nil, nil
	}

	ids, err := s.Chats.Search(query, limit)
	if err != nil {
		return nil, fmt.Errorf("message: search index: %w", err)
	}

	var out []Message
	for _, id := range ids {
		msgID, parseErr := uuid.Parse(id)
		if parseErr != nil {
			continue
		}
		msg, getErr := Get(s, secret, msgID)
		if getErr != nil {
			continue
		}
		out = append(out, *msg)
	}
	return out, nil
}

type Attachment struct {
	ID       uuid.UUID `json:"id"`
	Name     string    `json:"name"`
	MIMEType string    `json:"mime_type"`
	Size     int64     `json:"size"`
	Data     []byte    `json:"data,omitempty"`
}

type Message struct {
	ID          uuid.UUID    `json:"id"`
	FromPubKey  keys.Key     `json:"from_pub_key,omitempty"`
	To          string       `json:"to"`
	Content     string       `json:"content"`
	Time        time.Time    `json:"time"`
	Attachments []Attachment `json:"attachments,omitempty"`
}

func NewMessage(to, content string, attachments ...Attachment) *Message {
	return &Message{ID: uuid.New(), To: to, Content: content, Time: time.Now().UTC(), Attachments: attachments}
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

func searchText(msg *Message) string {
	var b strings.Builder
	b.WriteString(msg.Content)
	for _, a := range msg.Attachments {
		if a.Name != "" {
			b.WriteString(" ")
			b.WriteString(a.Name)
		}
	}
	return b.String()
}

func Put(s *store.Store, secret keys.Key, fromName string, msg *Message) (uuid.UUID, error) {
	plain, err := json.Marshal(msg)
	if err != nil {
		return uuid.Nil, fmt.Errorf("message: marshal: %w", err)
	}
	var id uuid.UUID
	if msg.ID != uuid.Nil {
		id = msg.ID
		if err = s.Chats.PutWithID(msg.ID, string(plain), secret); err != nil {
			return uuid.Nil, fmt.Errorf("message: store put: %w", err)
		}
	} else {
		id, err = s.Chats.Put(string(plain), secret)
		if err != nil {
			return uuid.Nil, fmt.Errorf("message: store put: %w", err)
		}
		msg.ID = id
	}
	_ = s.Chats.IndexSearch(id, fromName, msg.To, searchText(msg))
	s.Chats.CacheStore(id, string(plain))
	return id, nil
}

func Get(s *store.Store, secret keys.Key, id uuid.UUID) (*Message, error) {
	if cached, ok := s.Chats.CacheLoad(id); ok {
		var msg Message
		if err := json.Unmarshal([]byte(cached), &msg); err != nil {
			return nil, fmt.Errorf("message: unmarshal: %w", err)
		}
		msg.ID = id
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
	msg.ID = id
	s.Chats.CacheStore(id, plain)
	return &msg, nil
}

func Update(s *store.Store, secret keys.Key, id uuid.UUID, fromName string, msg *Message) error {
	plain, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("message: marshal: %w", err)
	}
	if err = s.Chats.Update(id, string(plain), secret); err != nil {
		return fmt.Errorf("message: store update: %w", err)
	}
	_ = s.Chats.IndexSearch(id, fromName, msg.To, searchText(msg))
	s.Chats.CacheStore(id, string(plain))
	return nil
}

func Delete(s *store.Store, secret keys.Key, id uuid.UUID) error {
	if cached, ok := s.Chats.CacheLoad(id); ok {
		var msg Message
		if err := json.Unmarshal([]byte(cached), &msg); err == nil {
			for _, a := range msg.Attachments {
				_ = s.Files.Delete(a.ID)
			}
		}
	} else if plain, getErr := s.Chats.Get(id, secret); getErr == nil {
		var msg Message
		if err := json.Unmarshal([]byte(plain), &msg); err == nil {
			for _, a := range msg.Attachments {
				_ = s.Files.Delete(a.ID)
			}
		}
	}
	if err := s.Chats.Delete(id); err != nil {
		return fmt.Errorf("message: store delete: %w", err)
	}
	return nil
}
