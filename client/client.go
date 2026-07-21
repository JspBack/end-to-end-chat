package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
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
	}
}

func (c *Client) UseTLS() bool {
	return c.tlsConfig != nil
}

func (c *Client) AddKnownPeer(peer *store.KnownPeer) error {
	if err := c.Store.KnownPeers.Add(peer); err != nil {
		return fmt.Errorf("client: add known peer: %w", err)
	}
	return nil
}

func (c *Client) GetKnownPeer(pubKey string) (*store.KnownPeer, error) {
	peer, err := c.Store.KnownPeers.Get(pubKey)
	if err != nil {
		return nil, fmt.Errorf("client: get known peer: %w", err)
	}
	return peer, nil
}

func (c *Client) RemoveKnownPeer(pubKey string) error {
	if err := c.Store.KnownPeers.Remove(pubKey); err != nil {
		return fmt.Errorf("client: remove known peer: %w", err)
	}
	return nil
}

func (c *Client) SetPeerStatus(pubKey, status string) (*store.KnownPeer, error) {
	peer, err := c.Store.KnownPeers.Get(pubKey)
	if err != nil {
		return nil, fmt.Errorf("client: get peer: %w", err)
	}
	peer.Status = status
	if err = c.Store.KnownPeers.Add(peer); err != nil {
		return nil, fmt.Errorf("client: set peer status: %w", err)
	}
	return peer, nil
}

func (c *Client) ListKnownPeers() ([]store.KnownPeer, error) {
	peers, err := c.Store.KnownPeers.List()
	if err != nil {
		return nil, fmt.Errorf("client: list known peers: %w", err)
	}
	return peers, nil
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

func (c *Client) sendFile(sess *Session, id string, data []byte) error {
	return sess.send(signal.New(signal.TypeFile, c.Keys.Public, id, data))
}

func (c *Client) sendToAll(content string) {
	now := time.Now().Format(time.RFC3339)
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

func (c *Client) handleSignal(sig *signal.Signal, _ *Session, pubKey string) {
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
				"from", msg.From, "to", msg.To, "content", msg.Content)
		}

	case signal.TypeFile:
		if sig.From != pubKey {
			c.log.Warn("file signal pubkey mismatch")
			return
		}
		if sig.ID == "" || len(sig.Content) == 0 {
			c.log.Warn("file signal missing id or data")
			return
		}
		att := message.Attachment{ID: sig.ID, Data: sig.Content}
		if storeErr := message.StoreAttachments(c.Store, c.Keys.Private, "", []message.Attachment{att}); storeErr != nil {
			c.log.Warn("store file failed", "error", storeErr)
		}

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

func (c *Client) verifyAndDelete(sig *signal.Signal, pubKey string) error {
	msg, err := message.Get(c.Store, c.Keys.Private, sig.ID)
	if err != nil {
		return fmt.Errorf("get for delete: %w", err)
	}
	if msg.FromPubKey == "" || pubKey != msg.FromPubKey {
		return errors.New("sender pubkey does not match message author")
	}
	if err = message.Delete(c.Store, c.Keys.Private, sig.ID); err != nil {
		return fmt.Errorf("verify delete: %w", err)
	}
	return nil
}

func (c *Client) verifyAndUpdate(sig *signal.Signal, pubKey string) error {
	msg, err := message.Get(c.Store, c.Keys.Private, sig.ID)
	if err != nil {
		return fmt.Errorf("get for update: %w", err)
	}
	if msg.FromPubKey == "" || pubKey != msg.FromPubKey {
		return errors.New("sender pubkey does not match message author")
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
