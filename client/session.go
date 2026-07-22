package client

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/signal"
	"github.com/JspBack/end-to-end-chat/store"
)

type keyExchangeData struct {
	PubKey       keys.Key `json:"pub_key"`
	StaticPubKey keys.Key `json:"static_pub_key"`
	ClientName   string   `json:"client_name"`
	Signature    string   `json:"signature"`
}

type peerInfo struct {
	name         string
	ephemeralKey keys.Key
	staticPubKey keys.Key
}

type cipherState struct {
	key   []byte
	nonce uint64
}

type Session struct {
	mu         sync.RWMutex
	conn       *websocket.Conn
	ourName    string
	ourStatic  keys.Key
	ourKeys    *keys.Keys
	log        *slog.Logger
	sendr      cipherState
	recv       cipherState
	peer       peerInfo
	timeout    time.Duration
	stat       store.PeerStatus
	msgLimiter *msgLimiter
}

func newSession(
	conn *websocket.Conn, ourName string, ourKeys *keys.Keys,
	log *slog.Logger, timeout time.Duration, rateLimit int, rateWindow time.Duration,
) *Session {
	return &Session{
		conn:       conn,
		ourName:    ourName,
		ourStatic:  ourKeys.Public,
		ourKeys:    ourKeys,
		log:        log,
		timeout:    timeout,
		msgLimiter: newMsgLimiter(rateLimit, rateWindow),
	}
}

func (s *Session) doSend(ourEphemeral keys.Key, sig []byte) error {
	data := keyExchangeData{
		PubKey:       ourEphemeral,
		StaticPubKey: s.ourStatic,
		ClientName:   s.ourName,
		Signature:    base64.StdEncoding.EncodeToString(sig),
	}
	inner, _ := json.Marshal(data)
	payload := signal.New(signal.KeyExchange, s.ourStatic, uuid.Nil, inner)
	_ = s.conn.SetWriteDeadline(time.Now().Add(s.timeout))
	if err := s.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
		return fmt.Errorf("session: write: %w", err)
	}
	return nil
}

func (s *Session) doRecv() (keyExchangeData, error) {
	_ = s.conn.SetReadDeadline(time.Now().Add(s.timeout))
	_, raw, readErr := s.conn.ReadMessage()
	_ = s.conn.SetReadDeadline(time.Time{})
	if readErr != nil {
		return keyExchangeData{}, fmt.Errorf("session: read peer key: %w", readErr)
	}
	sig, err := signal.Parse(raw)
	if err != nil {
		return keyExchangeData{}, fmt.Errorf("session: parse signal: %w", err)
	}
	if sig.Type != signal.KeyExchange {
		return keyExchangeData{}, fmt.Errorf("session: expected key_exchange, got %s", sig.Type)
	}
	var data keyExchangeData
	if err = json.Unmarshal(sig.Content, &data); err != nil {
		return keyExchangeData{}, fmt.Errorf("session: unmarshal key exchange data: %w", err)
	}

	if sig.From != keys.NilKey {
		data.StaticPubKey = sig.From
	}

	return data, nil
}

func (s *Session) verifyPeer(peer keyExchangeData) error {
	var zero keys.Key
	if peer.StaticPubKey == zero {
		return errors.New("session: peer sent no static public key")
	}
	if peer.StaticPubKey == s.ourStatic {
		return errors.New("session: cannot connect to self")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(peer.Signature)
	if err != nil {
		return fmt.Errorf("session: decode peer signature: %w", err)
	}
	if !keys.Verify(peer.StaticPubKey, peer.PubKey[:], sigBytes) {
		return errors.New("session: peer signature verification failed — possible MitM")
	}
	return nil
}

func (s *Session) deriveKeys(shared []byte, initiator bool, peer keyExchangeData) {
	sendKey := sha256.Sum256(append([]byte("session:send"), shared...))
	recvKey := sha256.Sum256(append([]byte("session:recv"), shared...))

	s.mu.Lock()
	if initiator {
		s.sendr.key, s.recv.key = sendKey[:], recvKey[:]
	} else {
		s.sendr.key, s.recv.key = recvKey[:], sendKey[:]
	}
	s.sendr.nonce, s.recv.nonce = 0, 0
	s.peer = peerInfo{
		name:         peer.ClientName,
		ephemeralKey: peer.PubKey,
		staticPubKey: peer.StaticPubKey,
	}
	s.mu.Unlock()
}

func (s *Session) handshake(ctx context.Context, initiator bool) error {
	curve := ecdh.X25519()
	ourKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("session: generate key: %w", err)
	}

	var ourEphemeral keys.Key
	copy(ourEphemeral[:], ourKey.PublicKey().Bytes())
	sig := s.ourKeys.Sign(ourEphemeral[:])

	var peer keyExchangeData
	if initiator {
		if err = s.doSend(ourEphemeral, sig); err != nil {
			return err
		}
		if peer, err = s.doRecv(); err != nil {
			return err
		}
	} else {
		if peer, err = s.doRecv(); err != nil {
			return err
		}
		if err = s.doSend(ourEphemeral, sig); err != nil {
			return err
		}
	}

	if err = s.verifyPeer(peer); err != nil {
		return err
	}

	peerKey, err := curve.NewPublicKey(peer.PubKey[:])
	if err != nil {
		return fmt.Errorf("session: parse peer key: %w", err)
	}
	shared, err := ourKey.ECDH(peerKey)
	if err != nil {
		return fmt.Errorf("session: derive shared secret: %w", err)
	}

	s.deriveKeys(shared, initiator, peer)

	s.log.DebugContext(ctx, "handshake done",
		"remote", s.conn.RemoteAddr(),
		"peer_name", s.peerName(),
		"peer_static_pub", peer.StaticPubKey.String(),
	)
	return nil
}

func aesGCM(key []byte) (cipher.AEAD, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("session: new cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("session: new gcm: %w", err)
	}
	return gcm, nil
}

func (s *Session) encrypt(plain []byte) ([]byte, error) {
	s.mu.Lock()
	if s.sendr.key == nil {
		s.mu.Unlock()
		return nil, errors.New("session: no key established")
	}
	key := s.sendr.key
	nonce := make([]byte, config.NonceSize)
	binary.BigEndian.PutUint64(nonce[:8], s.sendr.nonce)
	s.sendr.nonce++
	s.mu.Unlock()

	gcm, err := aesGCM(key)
	if err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plain, nil)
	out := make([]byte, config.NonceSize+len(ciphertext))
	copy(out[:config.NonceSize], nonce)
	copy(out[config.NonceSize:], ciphertext)
	return out, nil
}

func (s *Session) decrypt(data []byte) ([]byte, error) {
	if len(data) < config.NonceSize {
		return nil, errors.New("session: data too short")
	}

	s.mu.Lock()
	if s.recv.key == nil {
		s.mu.Unlock()
		return nil, errors.New("session: no key established")
	}
	key := s.recv.key
	expectedNonce := make([]byte, config.NonceSize)
	binary.BigEndian.PutUint64(expectedNonce[:8], s.recv.nonce)
	s.mu.Unlock()

	nonce := data[:config.NonceSize]
	ciphertext := data[config.NonceSize:]

	if binary.BigEndian.Uint64(nonce[:8]) != binary.BigEndian.Uint64(expectedNonce[:8]) {
		return nil, errors.New("session: nonce mismatch — possible replay or out-of-order delivery")
	}

	gcm, err := aesGCM(key)
	if err != nil {
		return nil, err
	}

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("session: decrypt: %w", err)
	}

	s.mu.Lock()
	s.recv.nonce++
	s.mu.Unlock()
	return plain, nil
}

func (s *Session) send(plain []byte) error {
	enc, err := s.encrypt(plain)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.conn == nil {
		return errors.New("session: connection closed")
	}
	if err = s.conn.SetWriteDeadline(time.Now().Add(s.timeout)); err != nil {
		return fmt.Errorf("session: set write deadline: %w", err)
	}
	if err = s.conn.WriteMessage(websocket.BinaryMessage, enc); err != nil {
		return fmt.Errorf("session: write: %w", err)
	}
	return nil
}

func (s *Session) peerPubKey() keys.Key {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var zero keys.Key
	if s.peer.staticPubKey != zero {
		return s.peer.staticPubKey
	}
	return s.peer.ephemeralKey
}

func (s *Session) peerName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peer.name
}

func (s *Session) status() store.PeerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.stat
}

func (s *Session) setStatus(status store.PeerStatus) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.stat = status
}

func (s *Session) closeConn() error {
	s.mu.Lock()
	for i := range s.sendr.key {
		s.sendr.key[i] = 0
	}
	s.sendr.key = nil
	for i := range s.recv.key {
		s.recv.key[i] = 0
	}
	s.recv.key = nil
	s.mu.Unlock()

	if err := s.conn.Close(); err != nil {
		return fmt.Errorf("session: close: %w", err)
	}
	return nil
}
