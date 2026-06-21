package client

import (
	"context"
	"net"
	"net/http"
	"os"
	"strings"

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
		ReadBufferSize:    1024,
		WriteBufferSize:   1024,
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		c.log.ErrorContext(r.Context(), "websocket upgrade failed", "error", err)
		return
	}

	c.log.InfoContext(r.Context(), "peer connected", "remote", conn.RemoteAddr(), "pub_key", pubKey)

	c.readLoop(r.Context(), conn)
}

func (c *Client) readLoop(ctx context.Context, conn *websocket.Conn) {
	defer conn.Close()

	for {
		var msg []byte
		_, msg, err := conn.ReadMessage()
		if err != nil {
			c.log.InfoContext(ctx, "peer disconnected", "remote", conn.RemoteAddr(), "error", err)

			return
		}

		c.log.InfoContext(ctx, "received", "message", string(msg))
	}
}

func (c *Client) handleAdminPeer(w http.ResponseWriter, r *http.Request) {
	pubKey := strings.TrimPrefix(r.URL.Path, "/admin/peer/")
	pubKey = strings.TrimSuffix(pubKey, "/approve")
	pubKey = strings.TrimSuffix(pubKey, "/reject")
	if pubKey == "" {
		http.Error(w, "missing public key", http.StatusBadRequest)
		return
	}

	var status string
	switch {
	case strings.HasSuffix(r.URL.Path, "/approve"):
		status = store.PeerStatusAccepted
	case strings.HasSuffix(r.URL.Path, "/reject"):
		status = store.PeerStatusRejected
	default:
		http.Error(w, "use /admin/peer/<pub_key>/approve or /reject", http.StatusBadRequest)
		return
	}

	peer, err := c.SetPeerStatus(pubKey, status)
	if err != nil {
		c.log.ErrorContext(r.Context(), "set peer status", "error", err)
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	c.log.InfoContext(r.Context(), "peer status updated", "pub_key", pubKey, "status", status)
	if _, werr := w.Write([]byte("peer " + peer.PeerIP + " " + status + "\n")); werr != nil {
		c.log.ErrorContext(r.Context(), "write response", "error", werr)
	}
}
