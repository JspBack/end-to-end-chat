package main

import (
	"bufio"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdh"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"os/signal"
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

type Message struct {
	From    string `json:"from"`
	To      string `json:"to"`
	Content string `json:"content"`
}

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	addr := flag.String("addr", "localhost:8080", "server address")
	pubKey := flag.String("pubkey", "test", "public key for transport path")
	name := flag.String("name", "peer", "client name to identify to the server")
	flag.Parse()

	if err := run(logger, *addr, *pubKey, *name); err != nil {
		logger.Error("fatal", "error", err)
		os.Exit(1)
	}
}

const maxMessageSize = 65536

func run(logger *slog.Logger, addr, pubKey, name string) error {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/transport/" + pubKey}

	c, _, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer c.Close()

	logger.Info("connected", "addr", u.String())

	sessionKey, serverName, err := handshake(c, name)
	if err != nil {
		return fmt.Errorf("handshake: %w", err)
	}

	logger.Info("session key established", "server_name", serverName)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-sig:
		case <-done:
		}
		c.Close()
	}()

	go readLoop(logger, c, sessionKey)
	go writeLoop(logger, c, sessionKey, name, serverName)

	<-sig
	logger.Info("shutting down")

	return nil
}

func handshake(c *websocket.Conn, name string) ([]byte, string, error) {
	curve := ecdh.X25519()
	ourKey, err := curve.GenerateKey(rand.Reader)
	if err != nil {
		return nil, "", fmt.Errorf("generate key: %w", err)
	}

	ourPub := hex.EncodeToString(ourKey.PublicKey().Bytes())
	msg, _ := json.Marshal(keyExchangeMsg{
		Type:       "key_exchange",
		PubKey:     ourPub,
		ClientName: name,
	})

	if err = c.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, "", fmt.Errorf("set write deadline: %w", err)
	}
	if err = c.WriteMessage(websocket.TextMessage, msg); err != nil {
		return nil, "", fmt.Errorf("send pubkey: %w", err)
	}

	if err = c.SetReadDeadline(time.Now().Add(10 * time.Second)); err != nil {
		return nil, "", fmt.Errorf("set deadline: %w", err)
	}

	var raw []byte
	_, raw, err = c.ReadMessage()
	if err != nil {
		return nil, "", fmt.Errorf("read server key: %w", err)
	}

	if err = c.SetReadDeadline(time.Time{}); err != nil {
		return nil, "", fmt.Errorf("clear deadline: %w", err)
	}

	var reply keyExchangeMsg
	if err = json.Unmarshal(raw, &reply); err != nil {
		return nil, "", fmt.Errorf("unmarshal server key: %w", err)
	}
	if reply.Type != "key_exchange" {
		return nil, "", fmt.Errorf("expected key_exchange, got %s", reply.Type)
	}

	peerKeyBytes, err := hex.DecodeString(reply.PubKey)
	if err != nil {
		return nil, "", fmt.Errorf("decode server pubkey: %w", err)
	}

	peerKey, err := curve.NewPublicKey(peerKeyBytes)
	if err != nil {
		return nil, "", fmt.Errorf("parse server pubkey: %w", err)
	}

	shared, err := ourKey.ECDH(peerKey)
	if err != nil {
		return nil, "", fmt.Errorf("derive shared secret: %w", err)
	}

	hash := sha256.Sum256(shared)

	return hash[:], reply.ClientName, nil
}

func encrypt(sessionKey, plain []byte) ([]byte, error) {
	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("nonce: %w", err)
	}

	ciphertext := gcm.Seal(nil, nonce, plain, nil)

	em := encryptedMsg{
		Type:   "message",
		Nonce:  base64.StdEncoding.EncodeToString(nonce),
		Cipher: base64.StdEncoding.EncodeToString(ciphertext),
	}

	out, err := json.Marshal(em)
	if err != nil {
		return nil, fmt.Errorf("marshal encrypted: %w", err)
	}

	return out, nil
}

func decrypt(sessionKey, data []byte) ([]byte, error) {
	var em encryptedMsg
	if err := json.Unmarshal(data, &em); err != nil {
		return nil, fmt.Errorf("unmarshal: %w", err)
	}

	if em.Type != "message" {
		return nil, fmt.Errorf("expected message, got %s", em.Type)
	}

	nonce, err := base64.StdEncoding.DecodeString(em.Nonce)
	if err != nil {
		return nil, fmt.Errorf("decode nonce: %w", err)
	}

	ciphertext, err := base64.StdEncoding.DecodeString(em.Cipher)
	if err != nil {
		return nil, fmt.Errorf("decode cipher: %w", err)
	}

	block, err := aes.NewCipher(sessionKey)
	if err != nil {
		return nil, fmt.Errorf("new cipher: %w", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("new gcm: %w", err)
	}

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decrypt: %w", err)
	}

	return plain, nil
}

func readLoop(logger *slog.Logger, c *websocket.Conn, sessionKey []byte) {
	c.SetReadLimit(maxMessageSize)

	for {
		if err := c.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			logger.Warn("set read deadline", "error", err)
			return
		}

		_, data, err := c.ReadMessage()
		if err != nil {
			logger.Info("read error", "error", err)
			return
		}

		plain, err := decrypt(sessionKey, data)
		if err != nil {
			logger.Warn("decrypt error", "error", err)
			continue
		}

		var msg Message
		if err = json.Unmarshal(plain, &msg); err != nil {
			logger.Warn("decode error", "error", err)
			continue
		}

		fmt.Printf("> from=%s to=%s content=%s\n", msg.From, msg.To, msg.Content)
	}
}

func writeLoop(logger *slog.Logger, c *websocket.Conn, sessionKey []byte, name, serverName string) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		msg := Message{From: name, To: serverName, Content: scanner.Text()}
		plain, err := json.Marshal(msg)
		if err != nil {
			logger.Warn("encode error", "error", err)
			continue
		}

		enc, err := encrypt(sessionKey, plain)
		if err != nil {
			logger.Warn("encrypt error", "error", err)
			continue
		}

		if err = c.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
			logger.Warn("set write deadline", "error", err)
			return
		}
		if err = c.WriteMessage(websocket.TextMessage, enc); err != nil {
			logger.Info("write error", "error", err)
			return
		}
	}
}
