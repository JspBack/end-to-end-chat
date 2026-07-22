package signal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"math"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/config"
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
	From    keys.Key  `json:"from"`
	ID      uuid.UUID `json:"id"`
	Content []byte    `json:"content"`
}

const Marker = 0x01
const headerPrefixLen = 3

func u16Vrf(n int) (uint16, error) {
	if n < 0 || n > math.MaxUint16 {
		return 0, errors.New("signal: length exceeds uint16")
	}
	return uint16(n), nil
}

func u32Vrf(n int) (uint32, error) {
	if n < 0 || n > math.MaxUint32 {
		return 0, errors.New("signal: length exceeds uint32")
	}
	return uint32(n), nil
}

func New(typ Type, from keys.Key, id uuid.UUID, content []byte) ([]byte, error) {
	typeBytes := []byte(typ)
	buf := make([]byte, 1+2+len(typeBytes)+config.AesKeySize+16+4+len(content))
	off := 0
	buf[off] = Marker
	off++
	tpbt, err := u16Vrf(len(typeBytes))
	if err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint16(buf[off:], tpbt)
	off += 2
	copy(buf[off:], typeBytes)
	off += len(typeBytes)
	copy(buf[off:], from[:])
	off += config.AesKeySize
	copy(buf[off:], id[:])
	off += 16
	ctn, err := u32Vrf(len(content))
	if err != nil {
		return nil, err
	}
	binary.BigEndian.PutUint32(buf[off:], ctn)
	off += 4
	copy(buf[off:], content)
	return buf, nil
}

func Parse(data []byte) (*Signal, error) {
	if len(data) < headerPrefixLen {
		return nil, errors.New("signal: buffer too short for header")
	}
	off := 0
	if data[off] != Marker {
		return nil, errors.New("signal: missing marker byte")
	}
	off++
	typeLen := int(binary.BigEndian.Uint16(data[off:]))
	off += 2
	if len(data) < off+typeLen+config.AesKeySize+16+4 {
		return nil, errors.New("signal: buffer too short for header")
	}
	typ := Type(data[off : off+typeLen])
	off += typeLen
	var from keys.Key
	copy(from[:], data[off:off+config.AesKeySize])
	off += config.AesKeySize
	var id uuid.UUID
	copy(id[:], data[off:off+16])
	off += 16
	contentLen := int(binary.BigEndian.Uint32(data[off:]))
	off += 4
	if len(data)-off < contentLen {
		return nil, fmt.Errorf("signal: content length mismatch: want %d, have %d", contentLen, len(data)-off)
	}
	return &Signal{Type: typ, From: from, ID: id, Content: data[off : off+contentLen]}, nil
}
