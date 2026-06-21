package client

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type keyExchangeMsg struct {
	Type       string `json:"type"`
	PubKey     string `json:"pub_key"`
	ClientName string `json:"client_name"`
}

type encryptedMsg struct {
	Type   string `json:"type"`
	Nonce  string `json:"nonce"`
	Cipher string `json:"cipher"`
}

type Session struct {
	mu         sync.RWMutex
	conn       *websocket.Conn
	ourName    string
	peerName   string
	log        *slog.Logger
	sessionKey []byte
	peerPubKey string
	timeout    time.Duration
}

func newSession(conn *websocket.Conn, ourName string, log *slog.Logger, timeout time.Duration) *Session {
	return &Session{
		conn:    conn,
		ourName: ourName,
		log:     log,
		timeout: timeout,
	}
}

func (s *Session) performKeyExchange(ctx context.Context) error {
	curve := ecdh.X25519()
	ourKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("session: generate key: %w", err)
	}

	if err = s.conn.SetReadDeadline(time.Now().Add(s.timeout)); err != nil {
		return fmt.Errorf("session: set read deadline: %w", err)
	}

	var raw []byte
	_, raw, err = s.conn.ReadMessage()
	if err != nil {
		return fmt.Errorf("session: read peer key: %w", err)
	}

	if err = s.conn.SetReadDeadline(time.Time{}); err != nil {
		return fmt.Errorf("session: clear read deadline: %w", err)
	}

	var msg keyExchangeMsg
	if err = json.Unmarshal(raw, &msg); err != nil {
		return fmt.Errorf("session: unmarshal key exchange: %w", err)
	}
	if msg.Type != "key_exchange" {
		return fmt.Errorf("session: expected key_exchange, got %s", msg.Type)
	}

	peerKeyBytes, err := hex.DecodeString(msg.PubKey)
	if err != nil {
		return fmt.Errorf("session: decode peer key: %w", err)
	}

	peerKey, err := curve.NewPublicKey(peerKeyBytes)
	if err != nil {
		return fmt.Errorf("session: parse peer key: %w", err)
	}

	shared, err := ourKey.ECDH(peerKey)
	if err != nil {
		return fmt.Errorf("session: derive shared secret: %w", err)
	}

	hash := sha256.Sum256(shared)

	s.mu.Lock()
	s.sessionKey = hash[:]
	s.peerPubKey = msg.PubKey
	s.peerName = msg.ClientName
	s.mu.Unlock()

	ourPubKey := hex.EncodeToString(ourKey.PublicKey().Bytes())
	resp, err := json.Marshal(keyExchangeMsg{
		Type:       "key_exchange",
		PubKey:     ourPubKey,
		ClientName: s.ourName,
	})
	if err != nil {
		return fmt.Errorf("session: marshal response: %w", err)
	}

	if err = s.conn.SetWriteDeadline(time.Now().Add(s.timeout)); err != nil {
		return fmt.Errorf("session: set write deadline: %w", err)
	}
	if err = s.conn.WriteMessage(websocket.TextMessage, resp); err != nil {
		return fmt.Errorf("session: send our key: %w", err)
	}
	if err = s.conn.SetWriteDeadline(time.Time{}); err != nil {
		return fmt.Errorf("session: clear write deadline: %w", err)
	}

	s.log.InfoContext(ctx, "session key established",
		"remote", s.conn.RemoteAddr(),
		"peer_name", s.peerName,
		"peer_pub_key", s.peerPubKey,
	)

	return nil
}

func (s *Session) Encrypt(plain []byte) ([]byte, error) {
	s.mu.RLock()
	key := s.sessionKey
	s.mu.RUnlock()

	if key == nil {
		return nil, errors.New("session: no key established")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("session: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("session: new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("session: nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plain, nil)

	msg := encryptedMsg{
		Type:   "message",
		Nonce:  base64.StdEncoding.EncodeToString(nonce),
		Cipher: base64.StdEncoding.EncodeToString(ciphertext),
	}

	out, err := json.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("session: marshal encrypted: %w", err)
	}

	return out, nil
}

func (s *Session) Decrypt(data []byte) ([]byte, error) {
	s.mu.RLock()
	key := s.sessionKey
	s.mu.RUnlock()

	if key == nil {
		return nil, errors.New("session: no key established")
	}

	var msg encryptedMsg
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, fmt.Errorf("session: unmarshal: %w", err)
	}

	if msg.Type != "message" {
		return nil, fmt.Errorf("session: expected message, got %s", msg.Type)
	}

	nonce, err := base64.StdEncoding.DecodeString(msg.Nonce)
	if err != nil {
		return nil, fmt.Errorf("session: decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(msg.Cipher)
	if err != nil {
		return nil, fmt.Errorf("session: decode cipher: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("session: new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("session: new gcm: %w", err)
	}

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("session: decrypt: %w", err)
	}

	return plain, nil
}

func (s *Session) PeerPubKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerPubKey
}

func (s *Session) Close() error {
	s.mu.Lock()
	for i := range s.sessionKey {
		s.sessionKey[i] = 0
	}
	s.sessionKey = nil
	s.mu.Unlock()

	if err := s.conn.Close(); err != nil {
		return fmt.Errorf("session: close: %w", err)
	}

	return nil
}
