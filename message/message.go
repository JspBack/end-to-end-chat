package message

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/store"
)

const timeFieldSize = 8

func u16Len(n int) uint16 {
	if n < 0 || n > math.MaxUint16 {
		panic("message: length exceeds uint16")
	}
	return uint16(n)
}

func u32Len(n int) uint32 {
	if n < 0 || n > math.MaxUint32 {
		panic("message: length exceeds uint32")
	}
	return uint32(n)
}

func u64FromInt64(n int64) uint64 {
	if n < 0 {
		panic("message: negative size")
	}
	return uint64(n)
}

func int64FromU64(n uint64) int64 {
	if n > math.MaxInt64 {
		panic("message: size exceeds int64")
	}
	return int64(n)
}

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
	FromPubKey  keys.Key     `json:"from_pub_key"`
	To          string       `json:"to"`
	Content     string       `json:"content"`
	Time        time.Time    `json:"time"`
	Attachments []Attachment `json:"attachments"`
}

func NewMessage(to, content string, attachments ...Attachment) *Message {
	return &Message{ID: uuid.New(), To: to, Content: content, Time: time.Now().UTC(), Attachments: attachments}
}

func attachmentEncodedSize(a Attachment) int {
	return 16 + 2 + len(a.Name) + 2 + len(a.MIMEType) + 8 + 4 + len(a.Data)
}

func encodeAttachment(buf []byte, off int, a Attachment) int {
	copy(buf[off:], a.ID[:])
	off += 16

	nameBytes := []byte(a.Name)
	binary.BigEndian.PutUint16(buf[off:], u16Len(len(nameBytes)))
	off += 2
	copy(buf[off:], nameBytes)
	off += len(nameBytes)

	mimeBytes := []byte(a.MIMEType)
	binary.BigEndian.PutUint16(buf[off:], u16Len(len(mimeBytes)))
	off += 2
	copy(buf[off:], mimeBytes)
	off += len(mimeBytes)

	binary.BigEndian.PutUint64(buf[off:], u64FromInt64(a.Size))
	off += 8

	binary.BigEndian.PutUint32(buf[off:], u32Len(len(a.Data)))
	off += 4
	copy(buf[off:], a.Data)
	off += len(a.Data)

	return off
}

func decodeAttachment(data []byte, off int) (Attachment, int, error) {
	var a Attachment
	if len(data) < off+16+2 {
		return a, 0, errors.New("message: buffer truncated at attachment id")
	}
	copy(a.ID[:], data[off:off+16])
	off += 16

	nameLen := int(binary.BigEndian.Uint16(data[off:]))
	off += 2
	if len(data) < off+nameLen+2 {
		return a, 0, errors.New("message: buffer truncated at attachment name")
	}
	a.Name = string(data[off : off+nameLen])
	off += nameLen

	mimeLen := int(binary.BigEndian.Uint16(data[off:]))
	off += 2
	if len(data) < off+mimeLen+8+4 {
		return a, 0, errors.New("message: buffer truncated at attachment mime")
	}
	a.MIMEType = string(data[off : off+mimeLen])
	off += mimeLen

	a.Size = int64FromU64(binary.BigEndian.Uint64(data[off:]))
	off += 8

	dataLen := int(binary.BigEndian.Uint32(data[off:]))
	off += 4
	if len(data) < off+dataLen {
		return a, 0, errors.New("message: buffer truncated at attachment data")
	}
	if dataLen > 0 {
		a.Data = make([]byte, dataLen)
		copy(a.Data, data[off:off+dataLen])
		off += dataLen
	}

	return a, off, nil
}

func (m *Message) Encode() ([]byte, error) {
	toBytes := []byte(m.To)
	contentBytes := []byte(m.Content)

	//nolint: mnd // size calculation for message encoding
	size := 16 + config.AesKeySize + 2 + len(toBytes) + 4 + len(contentBytes) + timeFieldSize + 2
	for _, a := range m.Attachments {
		size += attachmentEncodedSize(a)
	}

	buf := make([]byte, size)
	off := 0
	copy(buf[off:], m.ID[:])
	off += 16
	copy(buf[off:], m.FromPubKey[:])
	off += config.AesKeySize

	binary.BigEndian.PutUint16(buf[off:], u16Len(len(toBytes)))
	off += 2
	copy(buf[off:], toBytes)
	off += len(toBytes)

	binary.BigEndian.PutUint32(buf[off:], u32Len(len(contentBytes)))
	off += 4
	copy(buf[off:], contentBytes)
	off += len(contentBytes)

	binary.BigEndian.PutUint64(buf[off:], u64FromInt64(m.Time.UnixNano()))
	off += 8

	binary.BigEndian.PutUint16(buf[off:], u16Len(len(m.Attachments)))
	off += 2
	for _, a := range m.Attachments {
		off = encodeAttachment(buf, off, a)
	}

	return buf, nil
}

func ToMessage(data []byte) (*Message, error) {
	m := &Message{}
	off := 0

	if len(data) < 16+config.AesKeySize+2 {
		return nil, errors.New("message: buffer too short for header")
	}
	copy(m.ID[:], data[off:off+16])
	off += 16
	copy(m.FromPubKey[:], data[off:off+config.AesKeySize])
	off += config.AesKeySize

	dLen := int(binary.BigEndian.Uint16(data[off:]))
	off += 2
	if len(data) < off+dLen+4 {
		return nil, errors.New("message: buffer truncated at to")
	}
	m.To = string(data[off : off+dLen])
	off += dLen

	contentLen := int(binary.BigEndian.Uint32(data[off:]))
	off += 4
	if len(data) < off+contentLen+8+2 {
		return nil, errors.New("message: buffer truncated at content")
	}
	m.Content = string(data[off : off+contentLen])
	off += contentLen

	nanos := int64FromU64(binary.BigEndian.Uint64(data[off:]))
	off += 8
	m.Time = time.Unix(0, nanos).UTC()

	attCount := int(binary.BigEndian.Uint16(data[off:]))
	off += 2

	m.Attachments = make([]Attachment, 0, attCount)
	for range attCount {
		a, newOff, err := decodeAttachment(data, off)
		if err != nil {
			return nil, err
		}
		off = newOff
		m.Attachments = append(m.Attachments, a)
	}

	return m, nil
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

func indexAndCache(s *store.Store, id uuid.UUID, fromName string, msg *Message, plain []byte) {
	_ = s.Chats.IndexSearch(id, fromName, msg.To, searchText(msg))
	s.Chats.CacheStore(id, string(plain))
}

func loadPlain(s *store.Store, secret keys.Key, id uuid.UUID) (string, error) {
	if cached, ok := s.Chats.CacheLoad(id); ok {
		return cached, nil
	}
	plain, err := s.Chats.Get(id, secret)
	if err != nil {
		return "", fmt.Errorf("message: store get: %w", err)
	}
	s.Chats.CacheStore(id, plain)
	return plain, nil
}

func Put(s *store.Store, secret keys.Key, fromName string, msg *Message) (uuid.UUID, error) {
	plain, err := msg.Encode()
	if err != nil {
		return uuid.Nil, fmt.Errorf("message: encode: %w", err)
	}
	id := msg.ID
	if id == uuid.Nil {
		id = uuid.New()
		msg.ID = id
	}
	if err = s.Chats.PutWithID(id, string(plain), secret); err != nil {
		return uuid.Nil, fmt.Errorf("message: store put: %w", err)
	}
	indexAndCache(s, id, fromName, msg, plain)
	return id, nil
}

func Get(s *store.Store, secret keys.Key, id uuid.UUID) (*Message, error) {
	plain, err := loadPlain(s, secret, id)
	if err != nil {
		return nil, err
	}
	msg, err := ToMessage([]byte(plain))
	if err != nil {
		return nil, fmt.Errorf("message: decode: %w", err)
	}
	msg.ID = id
	return msg, nil
}

func Update(s *store.Store, secret keys.Key, id uuid.UUID, fromName string, msg *Message) error {
	plain, err := msg.Encode()
	if err != nil {
		return fmt.Errorf("message: encode: %w", err)
	}
	if err = s.Chats.Update(id, string(plain), secret); err != nil {
		return fmt.Errorf("message: store update: %w", err)
	}
	indexAndCache(s, id, fromName, msg, plain)
	return nil
}

func Delete(s *store.Store, secret keys.Key, id uuid.UUID) error {
	if plain, err := loadPlain(s, secret, id); err == nil {
		if msg, decErr := ToMessage([]byte(plain)); decErr == nil {
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
