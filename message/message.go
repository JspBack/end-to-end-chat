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

const (
	idSize        = 16
	u16Size       = 2
	u32Size       = 4
	u64Size       = 8
	timeFieldSize = 8
)

type unsignedWire interface {
	~uint16 | ~uint32 | ~uint64
}

//nolint:ireturn // T is a generic type parameter resolved at each call site, not a real interface return
func toWireUint[T unsignedWire](n int64) T {
	if n < 0 {
		panic(fmt.Sprintf("message: negative value %d cannot be encoded as unsigned", n))
	}
	var maxV T
	maxV--
	if uint64(n) > uint64(maxV) {
		panic(fmt.Sprintf("message: value %d overflows %T", n, maxV))
	}
	return T(n)
}

func toInt64(n uint64) int64 {
	if n > math.MaxInt64 {
		panic(fmt.Sprintf("message: value %d overflows int64", n))
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
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	Attachments []Attachment `json:"attachments"`
}

func NewMessage(to, content string, attachments ...Attachment) *Message {
	now := time.Now().UTC()
	return &Message{
		ID: uuid.New(), To: to, Content: content, Time: now,
		CreatedAt: now, UpdatedAt: now, Attachments: attachments,
	}
}

func attachmentEncodedSize(a Attachment) int {
	return idSize + u16Size + len(a.Name) + u16Size + len(a.MIMEType) + u64Size + u32Size + len(a.Data)
}

func encodeAttachment(buf []byte, off int, a Attachment) int {
	copy(buf[off:], a.ID[:])
	off += idSize

	nameBytes := []byte(a.Name)
	binary.BigEndian.PutUint16(buf[off:], toWireUint[uint16](int64(len(nameBytes))))
	off += u16Size
	copy(buf[off:], nameBytes)
	off += len(nameBytes)

	mimeBytes := []byte(a.MIMEType)
	binary.BigEndian.PutUint16(buf[off:], toWireUint[uint16](int64(len(mimeBytes))))
	off += u16Size
	copy(buf[off:], mimeBytes)
	off += len(mimeBytes)

	binary.BigEndian.PutUint64(buf[off:], toWireUint[uint64](a.Size))
	off += u64Size

	binary.BigEndian.PutUint32(buf[off:], toWireUint[uint32](int64(len(a.Data))))
	off += u32Size
	copy(buf[off:], a.Data)
	off += len(a.Data)

	return off
}

func decodeAttachment(data []byte, off int) (Attachment, int, error) {
	var a Attachment
	if len(data) < off+idSize+u16Size {
		return a, 0, errors.New("message: buffer truncated at attachment id")
	}
	copy(a.ID[:], data[off:off+idSize])
	off += idSize

	nameLen := int(binary.BigEndian.Uint16(data[off:]))
	off += u16Size
	if len(data) < off+nameLen+u16Size {
		return a, 0, errors.New("message: buffer truncated at attachment name")
	}
	a.Name = string(data[off : off+nameLen])
	off += nameLen

	mimeLen := int(binary.BigEndian.Uint16(data[off:]))
	off += u16Size
	if len(data) < off+mimeLen+u64Size+u32Size {
		return a, 0, errors.New("message: buffer truncated at attachment mime")
	}
	a.MIMEType = string(data[off : off+mimeLen])
	off += mimeLen

	a.Size = toInt64(binary.BigEndian.Uint64(data[off:]))
	off += u64Size

	dataLen := int(binary.BigEndian.Uint32(data[off:]))
	off += u32Size
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

	size := idSize + // m.ID
		config.AesKeySize + // m.FromPubKey
		u16Size + len(toBytes) + // m.To
		u32Size + len(contentBytes) + // m.Content
		timeFieldSize + // m.Time
		timeFieldSize + // m.CreatedAt
		timeFieldSize + // m.UpdatedAt
		u16Size // attachment count
	for _, a := range m.Attachments {
		size += attachmentEncodedSize(a)
	}

	buf := make([]byte, size)
	off := 0
	copy(buf[off:], m.ID[:])
	off += idSize
	copy(buf[off:], m.FromPubKey[:])
	off += config.AesKeySize

	binary.BigEndian.PutUint16(buf[off:], toWireUint[uint16](int64(len(toBytes))))
	off += u16Size
	copy(buf[off:], toBytes)
	off += len(toBytes)

	binary.BigEndian.PutUint32(buf[off:], toWireUint[uint32](int64(len(contentBytes))))
	off += u32Size
	copy(buf[off:], contentBytes)
	off += len(contentBytes)

	binary.BigEndian.PutUint64(buf[off:], toWireUint[uint64](m.Time.UnixNano()))
	off += timeFieldSize

	binary.BigEndian.PutUint64(buf[off:], toWireUint[uint64](m.CreatedAt.UnixNano()))
	off += timeFieldSize

	binary.BigEndian.PutUint64(buf[off:], toWireUint[uint64](m.UpdatedAt.UnixNano()))
	off += timeFieldSize

	binary.BigEndian.PutUint16(buf[off:], toWireUint[uint16](int64(len(m.Attachments))))
	off += u16Size
	for _, a := range m.Attachments {
		off = encodeAttachment(buf, off, a)
	}

	return buf, nil
}

func ToMessage(data []byte) (*Message, error) {
	m := &Message{}
	off := 0

	if len(data) < idSize+config.AesKeySize+u16Size {
		return nil, errors.New("message: buffer too short for header")
	}
	copy(m.ID[:], data[off:off+idSize])
	off += idSize
	copy(m.FromPubKey[:], data[off:off+config.AesKeySize])
	off += config.AesKeySize

	dLen := int(binary.BigEndian.Uint16(data[off:]))
	off += u16Size
	if len(data) < off+dLen+u32Size {
		return nil, errors.New("message: buffer truncated at to")
	}
	m.To = string(data[off : off+dLen])
	off += dLen

	contentLen := int(binary.BigEndian.Uint32(data[off:]))
	off += u32Size
	if len(data) < off+contentLen+3*timeFieldSize+u16Size {
		return nil, errors.New("message: buffer truncated at content")
	}
	m.Content = string(data[off : off+contentLen])
	off += contentLen

	nanos := toInt64(binary.BigEndian.Uint64(data[off:]))
	off += timeFieldSize
	m.Time = time.Unix(0, nanos).UTC()

	createdNanos := toInt64(binary.BigEndian.Uint64(data[off:]))
	off += timeFieldSize
	m.CreatedAt = time.Unix(0, createdNanos).UTC()

	updatedNanos := toInt64(binary.BigEndian.Uint64(data[off:]))
	off += timeFieldSize
	m.UpdatedAt = time.Unix(0, updatedNanos).UTC()

	attCount := int(binary.BigEndian.Uint16(data[off:]))
	off += u16Size

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
	if existing, err := Get(s, secret, id); err == nil {
		msg.CreatedAt = existing.CreatedAt
	}
	msg.UpdatedAt = time.Now().UTC()

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
