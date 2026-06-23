package client

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/message"
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

	peerAddr  string
	writeMode bool
}

func New(cfg config.Config, logger *slog.Logger) *Client {
	k := keys.Load(cfg.KeyFile)
	return &Client{
		Name:           cfg.ClientName,
		Keys:           k,
		Store:          store.New(k.Derive(), cfg.DB),
		listenPort:     cfg.Port,
		log:            logger,
		Timeout:        cfg.Timeout,
		RateLimit:      cfg.RateLimit,
		RateWindow:     cfg.RateWindow,
		MaxMessageSize: cfg.MaxMessageSize,
		pingPeriod:     cfg.PingWindow,
		peerAddr:       cfg.PeerAddr,
		writeMode:      cfg.WriteMode,
	}
}

func (c *Client) AddKnownPeer(peer *store.KnownPeer) error {
	if err := c.Store.KnownPeers.Add(peer); err != nil {
		return fmt.Errorf("client: add known peer: %w", err)
	}
	return nil
}

func (c *Client) GetKnownPeer(peerIP string) (*store.KnownPeer, error) {
	peer, err := c.Store.KnownPeers.Get(peerIP)
	if err != nil {
		return nil, fmt.Errorf("client: get known peer: %w", err)
	}
	return peer, nil
}

func (c *Client) RemoveKnownPeer(peerIP string) error {
	if err := c.Store.KnownPeers.Remove(peerIP); err != nil {
		return fmt.Errorf("client: remove known peer: %w", err)
	}
	return nil
}

func (c *Client) SetPeerStatus(pubKey, status string) (*store.KnownPeer, error) {
	peer, err := c.Store.KnownPeers.GetByPubKey(pubKey)
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
			sess.Close()
		}
		return true
	})

	if c.srv != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = c.srv.Shutdown(ctx)
	}
}

func (c *Client) broadcastMessage(content string) {
	msg := message.Message{From: c.Name, To: "all", Content: content}

	if _, err := message.Put(c.Store, c.Keys.Private, &msg); err != nil {
		c.log.Warn("store broadcast failed", "error", err)
	}

	plain, err := json.Marshal(msg)
	if err != nil {
		c.log.Warn("encode broadcast", "error", err)
		return
	}

	count := 0
	c.sessions.Range(func(key, value interface{}) bool {
		sess, ok := value.(*Session)
		if !ok {
			return true
		}
		if sendErr := sess.Send(plain); sendErr != nil {
			c.log.Warn("send to session", "pub_key", key, "error", sendErr)
		} else {
			count++
		}
		return true
	})

	c.log.Debug("broadcast", "message", content, "peers", count)
}

func (c *Client) stdinLoop(ctx context.Context) {
	scanner := bufio.NewScanner(os.Stdin)
	c.log.InfoContext(ctx, "write mode enabled — type messages to broadcast to connected peers")
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		c.broadcastMessage(line)
	}
}

func (c *Client) Listen() {
	addr := fmt.Sprintf(":%d", c.listenPort)
	mux := http.NewServeMux()
	mux.HandleFunc("/transport/", c.handleWS)
	c.registerAdminRoutes(mux)

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
	c.log.Info("listening", "addr", addr, "client", c.Name)
	if err := c.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		c.log.Error("listen", "err", err)
	}
}
