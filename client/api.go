package client

import (
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strconv"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
)

type statusResponse struct {
	Status string `json:"status"`
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

	peer, err := c.Store.KnownPeers.Get(pubKey)
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

func (c *Client) updateSessionStatus(pubKey, status string) {
	if status == store.PeerStatusRejected {
		if v, loaded := c.sessions.LoadAndDelete(pubKey); loaded {
			if sess, ok := v.(*Session); ok {
				_ = sess.closeConn()
			}
		}
		return
	}
	if v, loaded := c.sessions.Load(pubKey); loaded {
		if sess, ok := v.(*Session); ok {
			sess.setStatus(status)
		}
	}
}

func (c *Client) adminAcceptPeer(w http.ResponseWriter, r *http.Request) {
	pubKey := r.PathValue("pubKey")
	c.adminUpdatePeerStatus(w, r, store.PeerStatusAccepted)
	c.updateSessionStatus(pubKey, store.PeerStatusAccepted)
	c.log.InfoContext(r.Context(), "peer accepted", "pub_key", pubKey)
}

func (c *Client) adminRejectPeer(w http.ResponseWriter, r *http.Request) {
	pubKey := r.PathValue("pubKey")
	c.adminUpdatePeerStatus(w, r, store.PeerStatusRejected)
	c.updateSessionStatus(pubKey, store.PeerStatusRejected)
	c.log.InfoContext(r.Context(), "peer rejected and closed", "pub_key", pubKey)
}

func (c *Client) adminListSessions(w http.ResponseWriter, _ *http.Request) {
	type sessionInfo struct {
		PubKey string `json:"pub_key"`
		Status string `json:"status"`
		Name   string `json:"name"`
	}
	out := make([]sessionInfo, 0)
	c.sessions.Range(func(key, value interface{}) bool {
		sess, ok := value.(*Session)
		if !ok {
			return true
		}
		pk, ok := key.(string)
		if !ok {
			return true
		}
		out = append(out, sessionInfo{PubKey: pk, Status: sess.status(), Name: sess.peerName()})
		return true
	})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(out)
}

func (c *Client) apiSendMessage(w http.ResponseWriter, r *http.Request) {
	pubKey := r.PathValue("pubKey")
	if pubKey == "" {
		http.Error(w, "missing pubKey\n", http.StatusBadRequest)
		return
	}

	if pubKey == c.Keys.Public {
		http.Error(w, "cannot message self\n", http.StatusBadRequest)
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

	if sess.status() != store.PeerStatusAccepted {
		c.log.WarnContext(r.Context(), "peer not accepted — message not sent",
			"pub_key", pubKey, "peer_name", sess.peerName())
		http.Error(w, "peer not accepted\n", http.StatusForbidden)
		return
	}

	if err := c.sendMessage(sess, req.Content); err != nil {
		http.Error(w, "send failed\n", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResponse{Status: "sent"})
}

func validateAddr(addr string) (string, error) {
	if addr == "" || addr == ":" {
		return "", errors.New("invalid address")
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host = addr
		port = strconv.Itoa(config.DefaultPort)
	}
	if port == "" {
		port = strconv.Itoa(config.DefaultPort)
	}
	return net.JoinHostPort(host, port), nil
}

func (c *Client) apiConnectPeer(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Addr string `json:"addr"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid body\n", http.StatusBadRequest)
		return
	}

	addr, err := validateAddr(req.Addr)
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusBadRequest)
		return
	}

	if err = c.connectSession(r.Context(), addr); err != nil {
		c.log.WarnContext(r.Context(), "connect peer", "addr", addr, "error", err)
		http.Error(w, err.Error()+"\n", http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResponse{Status: "connected"})
}

func (c *Client) apiListMessages(w http.ResponseWriter, _ *http.Request) {
	summaries, err := c.Store.Chats.List()
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusInternalServerError)
		return
	}
	if summaries == nil {
		summaries = []store.ChatSummary{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(summaries)
}

func (c *Client) apiGetMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id\n", http.StatusBadRequest)
		return
	}

	msg, err := message.Get(c.Store, c.Keys.Private, id)
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(msg)
}

func (c *Client) apiSearchMessages(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	if q == "" {
		http.Error(w, "missing query parameter 'q'\n", http.StatusBadRequest)
		return
	}

	results, err := message.Search(c.Store, c.Keys.Private, q, 50)
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusInternalServerError)
		return
	}
	if results == nil {
		results = []message.Message{}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(results)
}

func (c *Client) apiUpdateMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id\n", http.StatusBadRequest)
		return
	}

	msg, err := message.Get(c.Store, c.Keys.Private, id)
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusNotFound)
		return
	}

	if msg.From != c.Name {
		http.Error(w, "cannot edit another user's message\n", http.StatusForbidden)
		return
	}

	var req struct {
		Content string `json:"content"`
	}
	if decodeErr := json.NewDecoder(r.Body).Decode(&req); decodeErr != nil {
		http.Error(w, "invalid body\n", http.StatusBadRequest)
		return
	}
	if req.Content == "" {
		http.Error(w, "content required\n", http.StatusBadRequest)
		return
	}

	msg.Content = req.Content
	if updateErr := message.Update(c.Store, c.Keys.Private, id, msg); updateErr != nil {
		http.Error(w, updateErr.Error()+"\n", http.StatusInternalServerError)
		return
	}

	c.broadcastUpdate(id, req.Content)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResponse{Status: "updated"})
}

func (c *Client) apiDeleteMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id\n", http.StatusBadRequest)
		return
	}

	msg, err := message.Get(c.Store, c.Keys.Private, id)
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusNotFound)
		return
	}

	if msg.From != c.Name {
		http.Error(w, "cannot delete another user's message\n", http.StatusForbidden)
		return
	}

	if delErr := message.Delete(c.Store, id); delErr != nil {
		http.Error(w, delErr.Error()+"\n", http.StatusInternalServerError)
		return
	}

	c.broadcastDelete(id)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResponse{Status: "deleted"})
}
