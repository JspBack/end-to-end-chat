package client

import (
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/keys"
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
}

func New(cfg config.Config, logger *slog.Logger) *Client {
	return &Client{
		Name:           cfg.ClientName,
		Keys:           keys.Load(cfg.KeyFile),
		Store:          store.New(cfg.DB),
		listenPort:     cfg.Port,
		log:            logger,
		Timeout:        cfg.Timeout,
		RateLimit:      cfg.RateLimit,
		RateWindow:     cfg.RateWindow,
		MaxMessageSize: cfg.MaxMessageSize,
		pingPeriod:     cfg.PingWindow,
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

func (c *Client) Listen() {
	addr := fmt.Sprintf(":%d", c.listenPort)
	mux := http.NewServeMux()
	mux.HandleFunc("/transport/", c.handleWS)
	srv := &http.Server{
		Addr:              addr,
		Handler:           httprate.LimitByIP(c.RateLimit, c.RateWindow)(mux),
		ReadHeaderTimeout: c.Timeout,
		ReadTimeout:       c.Timeout,
		WriteTimeout:      c.Timeout,
		IdleTimeout:       c.Timeout,
	}
	c.log.Info("listening", "addr", addr, "client", c.Name)
	if err := srv.ListenAndServe(); err != nil {
		c.log.Error("listen", "err", err)
	}
}
