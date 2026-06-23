package client

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
)

func (c *Client) registerAdminRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/peers", c.adminListPeers)
	mux.HandleFunc("PUT /admin/peers/{pubKey}/accept", c.adminAcceptPeer)
	mux.HandleFunc("PUT /admin/peers/{pubKey}/reject", c.adminRejectPeer)
	mux.HandleFunc("PUT /admin/peers/{pubKey}/pending", c.adminPendingPeer)
	mux.HandleFunc("GET /admin/sessions", c.adminListSessions)
	mux.HandleFunc("POST /api/peers/connect", c.apiConnectPeer)
	mux.HandleFunc("POST /api/messages/{pubKey}", c.apiSendMessage)
}

func (c *Client) adminListPeers(w http.ResponseWriter, _ *http.Request) {
	peers, err := c.Store.KnownPeers.List()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if peers == nil {
		peers = []store.KnownPeer{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(peers)
}

func (c *Client) adminUpdatePeerStatus(w http.ResponseWriter, r *http.Request, status string) {
	pubKey := r.PathValue("pubKey")
	if pubKey == "" {
		http.Error(w, "missing pubKey", http.StatusBadRequest)
		return
	}

	peer, err := c.Store.KnownPeers.GetByPubKey(pubKey)
	if err != nil {
		http.Error(w, "peer not found\n", http.StatusNotFound)
		return
	}

	peer.Status = status
	if err = c.Store.KnownPeers.Add(peer); err != nil {
		http.Error(w, err.Error()+"\n", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(peer)
}

func (c *Client) adminAcceptPeer(w http.ResponseWriter, r *http.Request) {
	pubKey := r.PathValue("pubKey")
	c.adminUpdatePeerStatus(w, r, store.PeerStatusAccepted)
	if v, loaded := c.sessions.Load(pubKey); loaded {
		sess, ok := v.(*Session)
		if ok {
			sess.SetStatus(store.PeerStatusAccepted)
			c.log.InfoContext(r.Context(), "peer session activated", "pub_key", pubKey, "peer_name", sess.PeerName())
		}
	}
}

func (c *Client) adminRejectPeer(w http.ResponseWriter, r *http.Request) {
	pubKey := r.PathValue("pubKey")
	c.adminUpdatePeerStatus(w, r, store.PeerStatusRejected)
	if v, loaded := c.sessions.LoadAndDelete(pubKey); loaded {
		sess, ok := v.(*Session)
		if ok {
			sess.Close()
			c.log.InfoContext(r.Context(), "peer session rejected and closed", "pub_key", pubKey)
		}
	}
}

func (c *Client) adminPendingPeer(w http.ResponseWriter, r *http.Request) {
	pubKey := r.PathValue("pubKey")
	c.adminUpdatePeerStatus(w, r, store.PeerStatusPending)
	if v, loaded := c.sessions.Load(pubKey); loaded {
		sess, ok := v.(*Session)
		if ok {
			sess.SetStatus(store.PeerStatusPending)
			c.log.InfoContext(r.Context(), "peer session moved to pending", "pub_key", pubKey)
		}
	}
}

func (c *Client) apiSendMessage(w http.ResponseWriter, r *http.Request) {
	pubKey := r.PathValue("pubKey")
	if pubKey == "" {
		http.Error(w, "missing pubKey\n", http.StatusBadRequest)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body\n", http.StatusBadRequest)
		return
	}

	v, ok := c.sessions.Load(pubKey)
	if !ok {
		http.Error(w, "peer not connected\n", http.StatusNotFound)
		return
	}
	sess, ok := v.(*Session)
	if !ok {
		http.Error(w, "invalid session\n", http.StatusInternalServerError)
		return
	}

	if sess.Status() != store.PeerStatusAccepted {
		c.log.WarnContext(r.Context(), "peer is not accepted yet — message not sent",
			"pub_key", pubKey, "peer_name", sess.PeerName())
		http.Error(w, "peer not accepted\n", http.StatusForbidden)
		return
	}

	msg := message.Message{From: c.Name, To: sess.PeerName(), Content: req.Content}

	if _, err := message.Put(c.Store, c.Keys.Private, &msg); err != nil {
		c.log.WarnContext(r.Context(), "store message failed", "error", err)
	}

	plain, err := json.Marshal(msg)
	if err != nil {
		http.Error(w, "encode error\n", http.StatusInternalServerError)
		return
	}

	if err = sess.Send(plain); err != nil {
		http.Error(w, "send failed\n", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "sent"})
}

func (c *Client) adminListSessions(w http.ResponseWriter, _ *http.Request) {
	type sessionInfo struct {
		PubKey string `json:"pub_key"`
		Status string `json:"status"`
		Name   string `json:"name"`
	}
	var out []sessionInfo
	c.sessions.Range(func(key, value interface{}) bool {
		sess, ok := value.(*Session)
		if !ok {
			return true
		}
		pk, ok := key.(string)
		if !ok {
			return true
		}
		out = append(out, sessionInfo{
			PubKey: pk,
			Status: sess.Status(),
			Name:   sess.PeerName(),
		})
		return true
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (c *Client) apiConnectPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Addr string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body\n", http.StatusBadRequest)
		return
	}
	if req.Addr == "" {
		http.Error(w, "missing addr\n", http.StatusBadRequest)
		return
	}

	go func() {
		if err := c.connectSession(context.Background(), req.Addr); err != nil {
			c.log.WarnContext(r.Context(), "connect peer", "addr", req.Addr, "error", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{"status": "connecting"})
}
