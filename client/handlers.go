package client

import (
	"context"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/store"
)

func isValidPubKey(s string) bool {
	if len(s) != config.PubKeyLen {
		return false
	}
	b, err := hex.DecodeString(s)
	return err == nil && len(b) == 32
}

func (c *Client) handleWS(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/transport/")

	if !isValidPubKey(key) {
		http.Error(w, "invalid public key\n", http.StatusBadRequest)
		return
	}

	pubKey, err := keys.FromHex(key)
	if err != nil {
		http.Error(w, "invalid public key\n", http.StatusBadRequest)
		return
	}

	if pubKey == c.Keys.Public {
		http.Error(w, "cannot connect to self\n", http.StatusForbidden)
		return
	}

	peerIP, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		http.Error(w, "invalid remote address", http.StatusBadRequest)
		return
	}

	_, err = c.Store.KnownPeers.Get(pubKey)
	if err != nil && !os.IsNotExist(err) {
		c.log.ErrorContext(r.Context(), "lookup peer", "error", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	var status store.PeerStatus
	if os.IsNotExist(err) {
		status = store.Pending
		peer := &store.KnownPeer{PeerIP: net.ParseIP(peerIP), PubKey: pubKey, Status: status}
		if err = c.Store.KnownPeers.Add(peer); err != nil {
			c.log.ErrorContext(r.Context(), "save pending peer", "error", err)
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		c.log.DebugContext(r.Context(), "new peer saved as pending", "pub_key", pubKey, "ip", peerIP)
	} else {
		status = store.NilStatus
	}

	if status == store.Rejected {
		c.log.DebugContext(r.Context(), "rejected peer attempted reconnect", "pub_key", pubKey)
		http.Error(w, "forbidden\n", http.StatusForbidden)
		return
	}

	if _, loaded := c.sessions.Load(pubKey); loaded {
		c.log.DebugContext(r.Context(), "session already active, rejecting connection", "pub_key", pubKey)
		http.Error(w, "session already active\n", http.StatusConflict)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin:       func(_ *http.Request) bool { return true },
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

func (c *Client) closeExistingSession(pubKey keys.Key, conn *websocket.Conn) {
	if old, loaded := c.sessions.LoadAndDelete(pubKey); loaded {
		if oldSess, ok := old.(*Session); ok {
			c.log.Debug("replacing existing session", "pub_key", pubKey, "remote", conn.RemoteAddr())
			if p, err := c.Store.KnownPeers.Get(pubKey); err == nil {
				p.LastSeen = time.Now().UTC()
				_ = c.Store.KnownPeers.Add(p)
			}
			_ = oldSess.closeConn()
		}
	}
}

func (c *Client) startSession(ctx context.Context, conn *websocket.Conn, pubKey keys.Key, status store.PeerStatus) {
	conn.SetReadLimit(c.MaxMessageSize)
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(c.Timeout))
	})

	sess := newSession(conn, c.getName(), c.Keys, c.log, c.Timeout, c.RateLimit, c.RateWindow)
	sess.setStatus(status)

	if err := sess.handshake(ctx, false); err != nil {
		c.log.ErrorContext(ctx, "handshake failed", "error", err)
		conn.Close()
		return
	}

	c.sessions.Store(pubKey, sess)
	if p, err := c.Store.KnownPeers.Get(pubKey); err == nil {
		p.Name = sess.peerName()
		_ = c.Store.KnownPeers.Add(p)
	}
	c.flushOutbox(pubKey)

	if status == store.Pending {
		c.requestPeerInfo(sess)
	}

	c.log.DebugContext(ctx, "session ready",
		"remote", conn.RemoteAddr(), "pub_key", pubKey,
		"peer_name", sess.peerName(), "status", status,
	)

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go c.pingLoop(ctx, sess, cancel)

	_ = c.readLoop(ctx, sess)
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

func (c *Client) readLoop(ctx context.Context, sess *Session) error {
	pubKey := sess.peerPubKey()
	defer func() {
		c.sessions.Delete(pubKey)
		if p, err := c.Store.KnownPeers.Get(pubKey); err == nil {
			p.LastSeen = time.Now().UTC()
			_ = c.Store.KnownPeers.Add(p)
		}
		_ = sess.closeConn()
	}()

	sess.conn.SetReadLimit(c.MaxMessageSize)

	for {
		if err := sess.conn.SetReadDeadline(time.Now().Add(sess.timeout)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		_, data, err := sess.conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		plain, err := sess.decrypt(data)
		if err != nil {
			c.log.WarnContext(ctx, "decrypt failed", "error", err)
			continue
		}

		c.handleDecrypted(plain, sess, pubKey)
	}
}
