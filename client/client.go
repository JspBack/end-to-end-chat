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

	"github.com/go-chi/httprate"
	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/signal"
	"github.com/JspBack/end-to-end-chat/store"
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

type Info struct {
	Name       string    `json:"name"`
	ProfilePic uuid.UUID `json:"profile_pic"`
}

type Client struct {
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
	pendingFiles map[uuid.UUID]*pendingFile
}

type infoResponseData struct {
	Name       string `json:"name"`
	ProfilePic []byte `json:"profile_pic,omitempty"`
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
	c := &Client{
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
		pendingFiles:   make(map[uuid.UUID]*pendingFile),
	}
	if cfg.ClientName != "" {
		_ = c.Store.Profile.Update(cfg.ClientName, uuid.Nil)
	}
	return c
}

func (c *Client) Info() Info {
	prof, err := c.Store.Profile.Get()
	if err != nil {
		return Info{}
	}
	return Info{Name: prof.Name, ProfilePic: prof.ProfilePic}
}

func (c *Client) setName(s string) {
	_ = c.Store.Profile.Update(s, c.Info().ProfilePic)
}

func (c *Client) getName() string {
	return c.Info().Name
}

func (c *Client) UseTLS() bool {
	return c.tlsConfig != nil
}

func (c *Client) Shutdown() {
	c.sessions.Range(func(_, value any) bool {
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
	if _, err := message.Put(c.Store, c.Keys.Private, c.getName(), msg); err != nil {
		c.log.Warn("store message failed", "error", err)
	}

	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	return sess.send(signal.New(signal.Message, c.Keys.Public, uuid.Nil, payload))
}

func (c *Client) sendFile(sess *Session, id uuid.UUID, name, mimeType string, size int64, data io.Reader) error {
	meta := fileMeta{Name: name, Size: size, MIME: mimeType}
	metaRaw, _ := json.Marshal(meta)
	c.log.Debug("send file start", "id", id.String(), "name", name, "size", size)
	if err := sess.send(signal.New(signal.FileMeta, c.Keys.Public, id, metaRaw)); err != nil {
		return fmt.Errorf("send file meta: %w", err)
	}
	buf := make([]byte, config.FileChunkSize)
	var sent int64
	for {
		n, err := data.Read(buf)
		if n > 0 {
			idStr := id.String()
			plain := make([]byte, config.FileIDLen+n)
			copy(plain, []byte(idStr))
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

func (c *Client) queueSignal(targetPubKey keys.Key, sig []byte) error {
	_, err := c.Store.Outbox.Put(targetPubKey, sig, c.Keys.Private)
	if err != nil {
		return fmt.Errorf("queue signal: %w", err)
	}
	return nil
}

func (c *Client) sendMessageToPeer(targetPubKey keys.Key, msg *message.Message) error {
	msg.FromPubKey = c.Keys.Public
	if _, err := message.Put(c.Store, c.Keys.Private, c.getName(), msg); err != nil {
		c.log.Warn("store message failed", "error", err)
	}
	payload, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	sig := signal.New(signal.Message, c.Keys.Public, uuid.Nil, payload)

	v, ok := c.sessions.Load(targetPubKey)
	if !ok {
		return c.queueSignal(targetPubKey, sig)
	}
	sess, ok := v.(*Session)
	if !ok || sess.status() != store.Accepted {
		return c.queueSignal(targetPubKey, sig)
	}
	return sess.send(sig)
}

func (c *Client) flushOutbox(pubKey keys.Key) {
	entries, err := c.Store.Outbox.Get(pubKey, c.Keys.Private)
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
	if !ok || sess.status() != store.Accepted {
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
	now := time.Now().UTC()
	c.sessions.Range(func(_, value any) bool {
		if sess, ok := value.(*Session); ok {
			msg := message.NewMessage(sess.peerName(), content)
			msg.Time = now
			if err := c.sendMessage(sess, msg); err != nil {
				c.log.Warn("send to session", "peer", sess.peerName(), "error", err)
			}
		}
		return true
	})
}

func (c *Client) broadcastDelete(id uuid.UUID) {
	payload := signal.New(signal.Delete, c.Keys.Public, id, nil)
	c.sessions.Range(func(_, value any) bool {
		if sess, ok := value.(*Session); ok {
			if err := sess.send(payload); err != nil {
				c.log.Warn("broadcast delete", "peer", sess.peerName(), "error", err)
			}
		}
		return true
	})
}

func (c *Client) handleSignal(sig *signal.Signal, pubKey keys.Key) {
	switch sig.Type {
	case signal.Message:
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
		fromName := c.displayNameForPubKey(pubKey)
		if _, putErr := message.Put(c.Store, c.Keys.Private, fromName, msg); putErr != nil {
			c.log.Warn("store message failed", "error", putErr)
		}
		if msg.To == c.getName() {
			c.log.Debug("message received",
				"from", fromName, "to", msg.To)
		}

	case signal.FileMeta:
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
		c.log.Debug("recv file meta", "id", sig.ID.String(), "name", meta.Name, "size", meta.Size)

	case signal.Delete:
		if sig.From != pubKey {
			c.log.Warn("delete signal pubkey mismatch")
			return
		}
		if delErr := c.verifyAndDelete(sig, pubKey); delErr != nil {
			c.log.Warn("delete message failed", "error", delErr)
		}

	case signal.Update:
		if sig.From != pubKey {
			c.log.Warn("update signal pubkey mismatch")
			return
		}
		if updErr := c.verifyAndUpdate(sig, pubKey); updErr != nil {
			c.log.Warn("update message failed", "error", updErr)
		}

	case signal.InfoRequest:
		c.handleInfoRequest(pubKey)

	case signal.InfoResponse:
		c.handleInfoResponse(pubKey, sig.Content)

	case signal.InfoChanged:
		c.handleInfoChanged(pubKey)

	case signal.PeerAccepted:
		c.handlePeerAccepted(pubKey)

	case signal.KeyExchange:
		c.log.Warn("unexpected signal type")
	}
}

func (c *Client) handleInfoRequest(pubKey keys.Key) {
	info := c.Info()
	resp := infoResponseData{Name: info.Name}
	if info.ProfilePic != uuid.Nil {
		data, err := c.Store.Files.Get(c.Keys.Private, info.ProfilePic)
		if err == nil {
			resp.ProfilePic = data
		}
	}
	payload, _ := json.Marshal(resp)
	sig := signal.New(signal.InfoResponse, c.Keys.Public, uuid.Nil, payload)
	v, ok := c.sessions.Load(pubKey)
	if !ok {
		return
	}
	sess, ok := v.(*Session)
	if !ok {
		return
	}
	_ = sess.send(sig)
}

func (c *Client) handleInfoResponse(pubKey keys.Key, content []byte) {
	var resp infoResponseData
	if err := json.Unmarshal(content, &resp); err != nil {
		c.log.Warn("decode info response", "error", err)
		return
	}
	if resp.Name == "" && len(resp.ProfilePic) == 0 {
		return
	}

	peer, err := c.Store.KnownPeers.Get(pubKey)
	if err != nil {
		return
	}

	var newID uuid.UUID
	if len(resp.ProfilePic) > 0 {
		newID = uuid.New()
		if err = c.Store.Files.PutWithID(c.Keys.Private, newID, "", resp.ProfilePic); err != nil {
			c.log.Warn("store profile pic", "error", err)
			return
		}
		_ = c.Store.Files.Delete(peer.ProfilePic)
	}
	if resp.Name != "" {
		peer.Name = resp.Name
	}
	if newID != uuid.Nil {
		peer.ProfilePic = newID
	}
	_ = c.Store.KnownPeers.Add(peer)
}

func (c *Client) handleInfoChanged(pubKey keys.Key) {
	v, ok := c.sessions.Load(pubKey)
	if !ok {
		return
	}
	sess, ok := v.(*Session)
	if !ok {
		return
	}
	c.requestPeerInfo(sess)
}

func (c *Client) handlePeerAccepted(pubKey keys.Key) {
	peer, err := c.Store.KnownPeers.Get(pubKey)
	if err != nil {
		return
	}
	peer.Status = store.Accepted
	_ = c.Store.KnownPeers.Add(peer)

	if v, loaded := c.sessions.Load(pubKey); loaded {
		if sess, ok := v.(*Session); ok {
			sess.setStatus(store.Accepted)
		}
	}

	c.log.Debug("peer accepted our connection", "pub_key", pubKey)
}

func (c *Client) requestPeerInfo(sess *Session) {
	sig := signal.New(signal.InfoRequest, c.Keys.Public, uuid.Nil, nil)
	_ = sess.send(sig)
}

func (c *Client) broadcastInfoChanged() {
	payload := signal.New(signal.InfoChanged, c.Keys.Public, uuid.Nil, nil)
	c.sessions.Range(func(_, value any) bool {
		if sess, ok := value.(*Session); ok {
			if err := sess.send(payload); err != nil {
				_ = c.queueSignal(sess.peerPubKey(), payload)
			}
		}
		return true
	})
}

func (c *Client) handleFileChunk(plain []byte) {
	if len(plain) < config.FileIDLen {
		c.log.Warn("file chunk too short")
		return
	}
	id, err := uuid.Parse(string(plain[:config.FileIDLen]))
	if err != nil {
		c.log.Warn("file chunk invalid id", "error", err)
		return
	}
	data := plain[config.FileIDLen:]

	c.pendingMu.Lock()
	pf, ok := c.pendingFiles[id]
	c.pendingMu.Unlock()

	if !ok {
		c.log.Warn("file chunk for unknown transfer", "id", id.String())
		return
	}

	pf.buf.Write(data)
	pf.received += int64(len(data))
	c.log.Debug("recv file chunk", "id", id, "chunk_bytes", len(data), "received", pf.received, "total", pf.meta.Size)

	if pf.received >= pf.meta.Size {
		c.finalizeFile(id, pf)
	}
}

func (c *Client) finalizeFile(id uuid.UUID, pf *pendingFile) {
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
	c.log.Debug("recv file done", "id", id.String(), "name", pf.meta.Name, "size", pf.meta.Size)
}

func (c *Client) handleDecrypted(plain []byte, sess *Session, pubKey keys.Key) {
	if len(plain) == 0 {
		return
	}
	if plain[0] == '{' {
		sig, err := signal.Parse(plain)
		if err != nil {
			c.log.Warn("decode envelope failed", "error", err)
			return
		}

		if sess.status() != store.Accepted {
			switch sig.Type {
			case signal.InfoRequest, signal.InfoResponse, signal.InfoChanged, signal.PeerAccepted:
				c.handleSignal(sig, pubKey)
			case signal.Message, signal.Delete, signal.Update, signal.FileMeta, signal.KeyExchange:
				c.log.Warn("unexpected signal type")
			}
			return
		}

		c.log.Debug("recv signal", "len", len(plain))
		if !sess.msgLimiter.Allow() {
			c.log.Warn("rate limit exceeded, dropping message")
			return
		}
		c.handleSignal(sig, pubKey)
	} else {
		c.log.Debug("recv file chunk raw", "len", len(plain))
		c.handleFileChunk(plain)
	}
}

func (c *Client) displayNameForPubKey(pubKey keys.Key) string {
	if peer, err := c.Store.KnownPeers.Get(pubKey); err == nil {
		n := peer.Nickname
		if n == "" {
			n = peer.Name
		}
		if n != "" {
			return n
		}
	}
	return pubKey.String()[:16]
}

func (c *Client) verifyAuthor(id uuid.UUID, pubKey keys.Key) (*message.Message, error) {
	msg, err := message.Get(c.Store, c.Keys.Private, id)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	if msg.FromPubKey == keys.NilKey || pubKey != msg.FromPubKey {
		return nil, errors.New("sender pubkey does not match message author")
	}
	return msg, nil
}

func (c *Client) verifyAndDelete(sig *signal.Signal, pubKey keys.Key) error {
	if _, err := c.verifyAuthor(sig.ID, pubKey); err != nil {
		return err
	}
	if err := message.Delete(c.Store, c.Keys.Private, sig.ID); err != nil {
		return fmt.Errorf("verify delete: %w", err)
	}
	return nil
}

func (c *Client) verifyAndUpdate(sig *signal.Signal, pubKey keys.Key) error {
	msg, err := c.verifyAuthor(sig.ID, pubKey)
	if err != nil {
		return err
	}
	msg.Content = string(sig.Content)
	if err = message.Update(c.Store, c.Keys.Private, sig.ID, c.getName(), msg); err != nil {
		return fmt.Errorf("verify update: %w", err)
	}
	return nil
}

func (c *Client) broadcastUpdate(id uuid.UUID, content string) {
	payload := signal.New(signal.Update, c.Keys.Public, id, []byte(content))
	c.sessions.Range(func(_, value any) bool {
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
	c.log.Info("listening", "addr", addr, "client", c.getName(), "tls", c.UseTLS())
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
