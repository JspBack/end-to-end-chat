package client

import (
	"context"
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

	if os.IsNotExist(err) {
		peer = &store.KnownPeer{PeerIP: peerIP, PubKey: pubKey, Status: store.PeerStatusPending}
		if err = c.Store.KnownPeers.Add(peer); err != nil {
			c.log.ErrorContext(r.Context(), "save pending peer", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		c.log.InfoContext(r.Context(), "new peer saved as pending", "pub_key", pubKey, "ip", peerIP)
		http.Error(w, "pending approval", http.StatusForbidden)
		return
	}

	switch peer.Status {
	case store.PeerStatusRejected:
		http.Error(w, "peer rejected", http.StatusForbidden)
		return
	case store.PeerStatusPending:
		http.Error(w, "pending approval", http.StatusForbidden)
		return
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

	c.log.InfoContext(r.Context(), "peer connected", "remote", conn.RemoteAddr(), "pub_key", pubKey)

	conn.SetReadLimit(c.MaxMessageSize)
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(c.Timeout))
	})

	sess := newSession(conn, c.Name, c.log, c.Timeout)
	if err = sess.performKeyExchange(r.Context()); err != nil {
		c.log.ErrorContext(r.Context(), "key exchange failed", "error", err)
		conn.Close()

		return
	}

	c.log.InfoContext(r.Context(), "session key established", "remote", conn.RemoteAddr(), "pub_key", pubKey)

	c.readLoop(r.Context(), sess)
}

func (c *Client) readLoop(ctx context.Context, sess *Session) {
	defer sess.Close()

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
			c.log.InfoContext(ctx, "peer disconnected", "remote", sess.conn.RemoteAddr(), "error", err)
			return
		}

		plain, err := sess.Decrypt(data)
		if err != nil {
			c.log.WarnContext(ctx, "decrypt failed", "error", err)
			continue
		}

		msg, err := message.ToMessage(plain)
		if err != nil {
			c.log.WarnContext(ctx, "decode message failed", "error", err)
			continue
		}

		sess.log.InfoContext(ctx, "received",
			"from", msg.From,
			"to", msg.To,
			"content", msg.Content,
		)
	}
}
