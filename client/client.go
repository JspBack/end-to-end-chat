package client

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/signal"
	"github.com/JspBack/end-to-end-chat/store"
	"github.com/go-chi/httprate"
)

type fileMeta struct {
	Name string `json:"name"`
	Size int64  `json:"size"`
	MIME string `json:"mime"`
}

type pendingFile struct {
	meta     fileMeta
	received int64
	buf      bytes.Buffer
}

type Client struct {
	Name           string
	Keys           *keys.Keys
	Store          *store.Store
	listenPort     int
	log            *slog.Logger
	Timeout        time.Duration
	RateLimit      int
	RateWindow     time.Duration
	MaxMessageSize int64

	sessions   sync.Map // pubKey -> *Session
	pingPeriod time.Duration
	srv        *http.Server
	tlsConfig  *tls.Config
	certFile   string
	keyFileTLS string

	peerAddr  string
	writeMode bool

	pendingMu    sync.Mutex
	pendingFiles map[string]*pendingFile
}

func loadTLSConfig(cfg config.Config, logger *slog.Logger) *tls.Config {
	if !cfg.UseTLS() {
		return nil
	}
	cert, err := tls.LoadX509KeyPair(cfg.CertFile, cfg.KeyFileTLS)
	if err != nil {
		logger.Error("load TLS cert", "error", err)
		os.Exit(1)
	}
	return &tls.Config{Certificates: []tls.Certificate{cert}}
}

func New(cfg config.Config, logger *slog.Logger) *Client {
	k := keys.AutoLoad()
	return &Client{
		Name:           cfg.ClientName,
		Keys:           k,
		Store:          store.New(k.Derive()),
		listenPort:     cfg.Port,
		log:            logger,
		Timeout:        cfg.Timeout,
		RateLimit:      cfg.RateLimit,
		RateWindow:     cfg.RateWindow,
		MaxMessageSize: cfg.MaxMessageSize,
		pingPeriod:     cfg.PingWindow,
		tlsConfig:      loadTLSConfig(cfg, logger),
		certFile:       cfg.CertFile,
		keyFileTLS:     cfg.KeyFileTLS,
		peerAddr:       cfg.PeerAddr,
		writeMode:      cfg.WriteMode,
		pendingFiles:   make(map[string]*pendingFile),
	}
}

func (c *Client) UseTLS() bool {
	return c.tlsConfig != nil
}

func (c *Client) Shutdown() {
	c.sessions.Range(func(_, value interface{}) bool {
		if sess, ok := value.(*Session); ok {
			_ = sess.closeConn()
		}
		return true
	})

	if c.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.srv.Shutdown(ctx)
	}
}

func (c *Client) sendMessage(sess *Session, msg *message.Message) error {
	msg.FromPubKey = c.Keys.Public
	if _, err := message.Put(c.Store, c.Keys.Private, msg); err != nil {
		c.log.Warn("store message failed", "error", err)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	return sess.send(signal.New(signal.TypeMessage, c.Keys.Public, "", payload))
}

func (c *Client) sendFile(sess *Session, id, name, mimeType string, size int64, data io.Reader) error {
	meta := fileMeta{Name: name, Size: size, MIME: mimeType}
	metaRaw, _ := json.Marshal(meta)
	c.log.Debug("send file start", "id", id, "name", name, "size", size)
	if err := sess.send(signal.New(signal.TypeFileMeta, c.Keys.Public, id, metaRaw)); err != nil {
		return fmt.Errorf("send file meta: %w", err)
	}
	buf := make([]byte, config.FileChunkSize)
	var sent int64
	for {
		n, err := data.Read(buf)
		if n > 0 {
			plain := make([]byte, config.FileIDLen+n)
			copy(plain, []byte(id))
			copy(plain[config.FileIDLen:], buf[:n])
			if sendErr := sess.send(plain); sendErr != nil {
				if qerr := c.queueSignal(sess.peerPubKey(), plain); qerr != nil {
					return fmt.Errorf("queue file chunk failed: %w", qerr)
				}
				c.log.Warn("send file chunk failed, queued for retry", "id", id, "chunk_bytes", n, "error", sendErr)
			}
			sent += int64(n)
			c.log.Debug("send file chunk", "id", id, "chunk_bytes", n, "sent", sent, "total", size)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read file data: %w", err)
		}
	}
	c.log.Debug("send file done", "id", id, "name", name, "total_sent", sent)
	return nil
}

func (c *Client) queueSignal(targetPubKey string, sig []byte) error {
	_, err := c.Store.Outbox.Put(targetPubKey, sig, c.Keys.Private)
	if err != nil {
		return fmt.Errorf("queue signal: %w", err)
	}
	return nil
}

func (c *Client) sendMessageToPeer(targetPubKey string, msg *message.Message) error {
	msg.FromPubKey = c.Keys.Public
	if _, err := message.Put(c.Store, c.Keys.Private, msg); err != nil {
		c.log.Warn("store message failed", "error", err)
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	sig := signal.New(signal.TypeMessage, c.Keys.Public, "", payload)

	v, ok := c.sessions.Load(targetPubKey)
	if !ok {
		return c.queueSignal(targetPubKey, sig)
	}
	sess, ok := v.(*Session)
	if !ok || sess.status() != store.PeerStatusAccepted {
		return c.queueSignal(targetPubKey, sig)
	}
	return sess.send(sig)
}

func (c *Client) flushOutbox(pubKey string) {
	entries, err := c.Store.Outbox.GetPending(pubKey, c.Keys.Private)
	if err != nil {
		c.log.Warn("flush outbox: get pending", "error", err)
		return
	}
	if len(entries) == 0 {
		return
	}
	v, ok := c.sessions.Load(pubKey)
	if !ok {
		return
	}
	sess, ok := v.(*Session)
	if !ok || sess.status() != store.PeerStatusAccepted {
		return
	}

	for _, entry := range entries {
		if sendErr := sess.send(entry.SignalContent); sendErr != nil {
			_ = c.Store.Outbox.IncrementRetry(entry.ID)
			return
		}
		if delErr := c.Store.Outbox.Delete(entry.ID); delErr != nil {
			c.log.Warn("flush outbox: delete delivered entry", "entry_id", entry.ID, "error", delErr)
		}
	}
}

func (c *Client) sendToAll(content string) {
	now := time.Now().UTC().Format(time.RFC3339)
	c.sessions.Range(func(_, value interface{}) bool {
		if sess, ok := value.(*Session); ok {
			msg := message.NewMessage(c.Name, sess.peerName(), content)
			msg.Time = now
			if err := c.sendMessage(sess, msg); err != nil {
				c.log.Warn("send to session", "peer", sess.peerName(), "error", err)
			}
		}
		return true
	})
}

func (c *Client) broadcastDelete(id string) {
	payload := signal.New(signal.TypeDelete, c.Keys.Public, id, nil)
	c.sessions.Range(func(_, value interface{}) bool {
		if sess, ok := value.(*Session); ok {
			if err := sess.send(payload); err != nil {
				c.log.Warn("broadcast delete", "peer", sess.peerName(), "error", err)
			}
		}
		return true
	})
}

func (c *Client) handleSignal(sig *signal.Signal, pubKey string) {
	switch sig.Type {
	case signal.TypeMessage:
		if sig.From != pubKey {
			c.log.Warn("message signal pubkey mismatch")
			return
		}
		msg, msgErr := message.ToMessage(sig.Content)
		if msgErr != nil {
			c.log.Warn("decode message failed", "error", msgErr)
			return
		}
		msg.FromPubKey = pubKey
		if _, putErr := message.Put(c.Store, c.Keys.Private, msg); putErr != nil {
			c.log.Warn("store message failed", "error", putErr)
		}
		if msg.To == c.Name {
			c.log.Debug("message received",
				"from", msg.From, "to", msg.To)
		}

	case signal.TypeFileMeta:
		if sig.From != pubKey {
			c.log.Warn("file meta pubkey mismatch")
			return
		}
		var meta fileMeta
		if err := json.Unmarshal(sig.Content, &meta); err != nil {
			c.log.Warn("decode file meta", "error", err)
			return
		}
		c.pendingMu.Lock()
		c.pendingFiles[sig.ID] = &pendingFile{meta: meta}
		c.pendingMu.Unlock()
		c.log.Debug("recv file meta", "id", sig.ID, "name", meta.Name, "size", meta.Size)

	case signal.TypeDelete:
		if sig.From != pubKey {
			c.log.Warn("delete signal pubkey mismatch")
			return
		}
		if delErr := c.verifyAndDelete(sig, pubKey); delErr != nil {
			c.log.Warn("delete message failed", "error", delErr)
		}

	case signal.TypeUpdate:
		if sig.From != pubKey {
			c.log.Warn("update signal pubkey mismatch")
			return
		}
		if updErr := c.verifyAndUpdate(sig, pubKey); updErr != nil {
			c.log.Warn("update message failed", "error", updErr)
		}
	}
}

func (c *Client) handleFileChunk(plain []byte) {
	if len(plain) < config.FileIDLen {
		c.log.Warn("file chunk too short")
		return
	}
	id := string(plain[:config.FileIDLen])
	data := plain[config.FileIDLen:]

	c.pendingMu.Lock()
	pf, ok := c.pendingFiles[id]
	c.pendingMu.Unlock()

	if !ok {
		c.log.Warn("file chunk for unknown transfer", "id", id)
		return
	}

	pf.buf.Write(data)
	pf.received += int64(len(data))
	c.log.Debug("recv file chunk", "id", id, "chunk_bytes", len(data), "received", pf.received, "total", pf.meta.Size)

	if pf.received >= pf.meta.Size {
		c.finalizeFile(id, pf)
	}
}

func (c *Client) finalizeFile(id string, pf *pendingFile) {
	defer func() {
		c.pendingMu.Lock()
		delete(c.pendingFiles, id)
		c.pendingMu.Unlock()
	}()

	att := message.Attachment{
		ID:       id,
		Name:     pf.meta.Name,
		MIMEType: pf.meta.MIME,
		Size:     pf.meta.Size,
		Data:     pf.buf.Bytes(),
	}
	if err := message.StoreAttachments(c.Store, c.Keys.Private, "", []message.Attachment{att}); err != nil {
		c.log.Warn("store file", "error", err)
		return
	}
	c.log.Debug("recv file done", "id", id, "name", pf.meta.Name, "size", pf.meta.Size)
}

func (c *Client) handleDecrypted(plain []byte, sess *Session, pubKey string) {
	if sess.status() != store.PeerStatusAccepted {
		return
	}
	if len(plain) == 0 {
		return
	}
	if plain[0] == '{' {
		c.log.Debug("recv signal", "len", len(plain))
		if !sess.msgLimiter.Allow() {
			c.log.Warn("rate limit exceeded, dropping message")
			return
		}
		sig, err := signal.Parse(plain)
		if err != nil {
			c.log.Warn("decode envelope failed", "error", err)
			return
		}
		c.handleSignal(sig, pubKey)
	} else {
		c.log.Debug("recv file chunk raw", "len", len(plain))
		c.handleFileChunk(plain)
	}
}

func (c *Client) verifyAuthor(id, pubKey string) (*message.Message, error) {
	msg, err := message.Get(c.Store, c.Keys.Private, id)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	if msg.FromPubKey == "" || pubKey != msg.FromPubKey {
		return nil, errors.New("sender pubkey does not match message author")
	}
	return msg, nil
}

func (c *Client) verifyAndDelete(sig *signal.Signal, pubKey string) error {
	if _, err := c.verifyAuthor(sig.ID, pubKey); err != nil {
		return err
	}
	if err := message.Delete(c.Store, c.Keys.Private, sig.ID); err != nil {
		return fmt.Errorf("verify delete: %w", err)
	}
	return nil
}

func (c *Client) verifyAndUpdate(sig *signal.Signal, pubKey string) error {
	msg, err := c.verifyAuthor(sig.ID, pubKey)
	if err != nil {
		return err
	}
	msg.Content = string(sig.Content)
	if err = message.Update(c.Store, c.Keys.Private, sig.ID, msg); err != nil {
		return fmt.Errorf("verify update: %w", err)
	}
	return nil
}

func (c *Client) broadcastUpdate(id, content string) {
	payload := signal.New(signal.TypeUpdate, c.Keys.Public, id, []byte(content))
	c.sessions.Range(func(_, value interface{}) bool {
		if sess, ok := value.(*Session); ok {
			if err := sess.send(payload); err != nil {
				c.log.Warn("broadcast delete", "peer", sess.peerName(), "error", err)
			}
		}
		return true
	})
}

func (c *Client) stdinLoop(ctx context.Context) {
	scanner := bufio.NewScanner(os.Stdin)
	c.log.InfoContext(ctx, "write mode enabled — type messages")
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		c.sendToAll(line)
	}
}

func (c *Client) Listen() {
	addr := fmt.Sprintf(":%d", c.listenPort)
	mux := http.NewServeMux()
	mux.HandleFunc("/transport/", c.handleWS)
	c.registerRoutes(mux)

	if c.writeMode && c.peerAddr == "" {
		go c.stdinLoop(context.Background())
	}

	c.srv = &http.Server{
		Addr:              addr,
		Handler:           httprate.LimitByIP(c.RateLimit, c.RateWindow)(mux),
		ReadHeaderTimeout: c.Timeout,
		ReadTimeout:       c.Timeout,
		WriteTimeout:      c.Timeout,
		IdleTimeout:       c.Timeout,
	}
	c.log.Info("listening", "addr", addr, "client", c.Name, "tls", c.UseTLS())
	var serveErr error
	if c.UseTLS() {
		serveErr = c.srv.ListenAndServeTLS(c.certFile, c.keyFileTLS)
	} else {
		serveErr = c.srv.ListenAndServe()
	}
	if serveErr != nil && serveErr != http.ErrServerClosed {
		c.log.Error("listen", "err", serveErr)
	}
}
