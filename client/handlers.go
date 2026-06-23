package client

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
	"github.com/gorilla/websocket"
)

func (c *Client) handleWS(w http.ResponseWriter, r *http.Request) {
	pubKey := strings.TrimPrefix(r.URL.Path, "/transport/")
	if pubKey == "" {
		http.Error(w, "missing public key", http.StatusBadRequest)
		return
	}

	peerIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "invalid remote address", http.StatusBadRequest)
		return
	}

	peer, err := c.Store.KnownPeers.GetByPubKey(pubKey)
	if err != nil && !os.IsNotExist(err) {
		c.log.ErrorContext(r.Context(), "lookup peer", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var status string
	if os.IsNotExist(err) {
		status = store.PeerStatusPending
		peer = &store.KnownPeer{PeerIP: peerIP, PubKey: pubKey, Status: status}
		if err = c.Store.KnownPeers.Add(peer); err != nil {
			c.log.ErrorContext(r.Context(), "save pending peer", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		c.log.DebugContext(r.Context(), "new peer saved as pending", "pub_key", pubKey, "ip", peerIP)
	} else {
		status = peer.Status
	}

	var upgrader = websocket.Upgrader{
		CheckOrigin: func(_ *http.Request) bool {
			return true
		},
		EnableCompression: true,
		HandshakeTimeout:  c.Timeout,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		c.log.ErrorContext(r.Context(), "websocket upgrade failed", "error", err)
		return
	}

	c.closeExistingSession(pubKey, conn)

	c.log.DebugContext(r.Context(), "peer connected",
		"remote", conn.RemoteAddr(), "pub_key", pubKey, "status", status)

	c.startSession(r.Context(), conn, pubKey, status)
}

func (c *Client) closeExistingSession(pubKey string, conn *websocket.Conn) {
	if old, loaded := c.sessions.LoadAndDelete(pubKey); loaded {
		if oldSess, ok := old.(*Session); ok {
			c.log.Debug("replacing existing session", "pub_key", pubKey, "remote", conn.RemoteAddr())
			oldSess.Close()
		}
	}
}

func (c *Client) startSession(ctx context.Context, conn *websocket.Conn, pubKey, status string) {
	conn.SetReadLimit(c.MaxMessageSize)
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(c.Timeout))
	})

	sess := newSession(conn, c.Name, c.Keys.Public, c.log, c.Timeout)
	sess.SetStatus(status)

	if err := sess.performKeyExchange(ctx); err != nil {
		c.log.ErrorContext(ctx, "key exchange failed", "error", err)
		conn.Close()
		return
	}

	c.sessions.Store(pubKey, sess)

	c.log.DebugContext(ctx, "session key established",
		"remote", conn.RemoteAddr(), "pub_key", pubKey,
		"peer_name", sess.PeerName(), "status", status,
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go c.pingLoop(ctx, sess, cancel)

	c.readLoop(ctx, sess, pubKey)
}

func (c *Client) pingLoop(ctx context.Context, sess *Session, cancel context.CancelFunc) {
	ticker := time.NewTicker(c.pingPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if err := sess.conn.WriteControl(websocket.PingMessage, nil, time.Now().Add(5*time.Second)); err != nil {
				c.log.WarnContext(ctx, "ping failed, closing session",
					"remote", sess.conn.RemoteAddr(), "error", err)
				cancel()
				return
			}
		}
	}
}

func (c *Client) readLoop(ctx context.Context, sess *Session, pubKey string) {
	defer func() {
		c.sessions.Delete(pubKey)
		sess.Close()
	}()

	done := make(chan struct{})
	defer close(done)

	go func() {
		select {
		case <-ctx.Done():
			sess.conn.Close()
		case <-done:
		}
	}()

	for {
		if err := sess.conn.SetReadDeadline(time.Now().Add(sess.timeout)); err != nil {
			c.log.WarnContext(ctx, "set read deadline failed", "error", err)
			return
		}

		_, data, err := sess.conn.ReadMessage()
		if err != nil {
			c.log.DebugContext(ctx, "peer disconnected",
				"remote", sess.conn.RemoteAddr(), "error", err)
			return
		}

		plain, err := sess.Decrypt(data)
		if err != nil {
			c.log.WarnContext(ctx, "decrypt failed", "error", err)
			continue
		}

		if sess.Status() != store.PeerStatusAccepted {
			continue
		}

		msg, err := message.ToMessage(plain)
		if err != nil {
			c.log.WarnContext(ctx, "decode message failed", "error", err)
			continue
		}

		if _, err = message.Put(c.Store, c.Keys.Private, msg); err != nil {
			c.log.WarnContext(ctx, "store message failed", "error", err)
		}

		fmt.Printf("> from=%s to=%s content=%s\n", msg.From, msg.To, msg.Content)
	}
}
