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
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/signal"
	"github.com/gorilla/websocket"
)

type keyExchangeData struct {
	PubKey       string `json:"pub_key"`
	StaticPubKey string `json:"static_pub_key"`
	ClientName   string `json:"client_name"`
	Signature    string `json:"signature"`
}

type Session struct {
	mu               sync.RWMutex
	conn             *websocket.Conn
	ourName          string
	ourStaticPubKey  string
	ourKeys          *keys.Keys
	peerNameStr      string
	log              *slog.Logger
	sendKey          []byte
	recvKey          []byte
	sendNonce        uint64
	recvNonce        uint64
	peerEphemeralKey string
	peerStaticPubKey string
	timeout          time.Duration
	statusStr        string
	msgLimiter       *msgLimiter
}

func newSession(
	conn *websocket.Conn, ourName string, ourKeys *keys.Keys,
	log *slog.Logger, timeout time.Duration, rateLimit int, rateWindow time.Duration,
) *Session {
	return &Session{
		conn:            conn,
		ourName:         ourName,
		ourStaticPubKey: ourKeys.Public,
		ourKeys:         ourKeys,
		log:             log,
		timeout:         timeout,
		msgLimiter:      newMsgLimiter(rateLimit, rateWindow),
	}
}

func (s *Session) doSend(ourEphemeralHex string, sig []byte) error {
	data := keyExchangeData{
		PubKey:       ourEphemeralHex,
		StaticPubKey: s.ourStaticPubKey,
		ClientName:   s.ourName,
		Signature:    base64.StdEncoding.EncodeToString(sig),
	}
	inner, _ := json.Marshal(data)
	payload := signal.New(signal.TypeKeyExchange, s.ourStaticPubKey, "", inner)
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
	if sig.Type != signal.TypeKeyExchange {
		return keyExchangeData{}, fmt.Errorf("session: expected key_exchange, got %s", sig.Type)
	}
	var data keyExchangeData
	if err = json.Unmarshal(sig.Content, &data); err != nil {
		return keyExchangeData{}, fmt.Errorf("session: unmarshal key exchange data: %w", err)
	}
	if sig.From != "" {
		data.StaticPubKey = sig.From
	}
	return data, nil
}

func (s *Session) verifyPeer(peer keyExchangeData) error {
	if peer.StaticPubKey == "" {
		return errors.New("session: peer sent no static public key")
	}
	if peer.StaticPubKey == s.ourStaticPubKey {
		return errors.New("session: cannot connect to self")
	}
	sigBytes, err := base64.StdEncoding.DecodeString(peer.Signature)
	if err != nil {
		return fmt.Errorf("session: decode peer signature: %w", err)
	}
	if !keys.Verify(peer.StaticPubKey, []byte(peer.PubKey), sigBytes) {
		return errors.New("session: peer signature verification failed — possible MitM")
	}
	return nil
}

func (s *Session) deriveKeys(shared []byte, initiator bool, peer keyExchangeData) {
	sendKey := sha256.Sum256(append([]byte("session:send"), shared...))
	recvKey := sha256.Sum256(append([]byte("session:recv"), shared...))

	s.mu.Lock()
	if initiator {
		s.sendKey, s.recvKey = sendKey[:], recvKey[:]
	} else {
		s.sendKey, s.recvKey = recvKey[:], sendKey[:]
	}
	s.sendNonce, s.recvNonce = 0, 0
	s.peerEphemeralKey, s.peerStaticPubKey, s.peerNameStr = peer.PubKey, peer.StaticPubKey, peer.ClientName
	s.mu.Unlock()
}

func (s *Session) handshake(ctx context.Context, initiator bool) error {
	curve := ecdh.X25519()
	ourKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return fmt.Errorf("session: generate key: %w", err)
	}

	ourEphemeralHex := hex.EncodeToString(ourKey.PublicKey().Bytes())
	sig := s.ourKeys.Sign([]byte(ourEphemeralHex))

	var peer keyExchangeData
	if initiator {
		if err = s.doSend(ourEphemeralHex, sig); err != nil {
			return err
		}
		if peer, err = s.doRecv(); err != nil {
			return err
		}
	} else {
		if peer, err = s.doRecv(); err != nil {
			return err
		}
		if err = s.doSend(ourEphemeralHex, sig); err != nil {
			return err
		}
	}

	if err = s.verifyPeer(peer); err != nil {
		return err
	}

	peerKeyBytes, err := hex.DecodeString(peer.PubKey)
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

	s.deriveKeys(shared, initiator, peer)

	s.log.DebugContext(ctx, "handshake done",
		"remote", s.conn.RemoteAddr(),
		"peer_name", s.peerName(),
		"peer_static_pub", peer.StaticPubKey,
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
	if s.sendKey == nil {
		s.mu.Unlock()
		return nil, errors.New("session: no key established")
	}
	key := s.sendKey
	nonce := make([]byte, config.NonceSize)
	binary.BigEndian.PutUint64(nonce[:8], s.sendNonce)
	s.sendNonce++
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
	if s.recvKey == nil {
		s.mu.Unlock()
		return nil, errors.New("session: no key established")
	}
	key := s.recvKey
	expectedNonce := make([]byte, config.NonceSize)
	binary.BigEndian.PutUint64(expectedNonce[:8], s.recvNonce)
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
	s.recvNonce++
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

func (s *Session) peerPubKey() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if s.peerStaticPubKey != "" {
		return s.peerStaticPubKey
	}
	return s.peerEphemeralKey
}

func (s *Session) peerName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.peerNameStr
}

func (s *Session) status() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.statusStr
}

func (s *Session) setStatus(status string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.statusStr = status
}

func (s *Session) closeConn() error {
	s.mu.Lock()
	for i := range s.sendKey {
		s.sendKey[i] = 0
	}
	s.sendKey = nil
	for i := range s.recvKey {
		s.recvKey[i] = 0
	}
	s.recvKey = nil
	s.mu.Unlock()

	if err := s.conn.Close(); err != nil {
		return fmt.Errorf("session: close: %w", err)
	}
	return nil
}
