package client

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"

	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
	"github.com/google/uuid"
)

var errNotOwner = errors.New("not the message owner")

const (
	statusConnected    = "connected"
	statusDisconnected = "disconnected"
	statusDeleted      = "deleted"
	statusUpdated      = "updated"
	statusFlushed      = "flushed"
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
	for i := range peers {
		if _, ok := c.sessions.Load(peers[i].PubKey); ok {
			peers[i].Online = true
		}
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

func (c *Client) adminDeleteSession(w http.ResponseWriter, r *http.Request) {
	pubKey := r.PathValue("pubKey")
	if pubKey == "" {
		http.Error(w, "missing pubKey\n", http.StatusBadRequest)
		return
	}
	v, loaded := c.sessions.LoadAndDelete(pubKey)
	if !loaded {
		http.Error(w, "session not found\n", http.StatusNotFound)
		return
	}
	if sess, ok := v.(*Session); ok {
		_ = sess.closeConn()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResponse{Status: statusDisconnected})
}

func parseMessageForm(r *http.Request, maxSize int64) (string, []message.Attachment, error) {
	if err := r.ParseMultipartForm(config.MultipartMemBuf); err != nil {
		return "", nil, errors.New("invalid form")
	}
	content := r.FormValue("content")
	var atts []message.Attachment
	var totalSize int64
	for _, fh := range r.MultipartForm.File["files"] {
		if totalSize+fh.Size > maxSize {
			return "", nil, fmt.Errorf("attachments exceed max size %d", maxSize)
		}
		totalSize += fh.Size
		f, fe := fh.Open()
		if fe != nil {
			return "", nil, errors.New("read file")
		}
		data, fe := io.ReadAll(io.LimitReader(f, maxSize))
		f.Close()
		if fe != nil {
			return "", nil, errors.New("read file")
		}
		atts = append(atts, message.Attachment{
			Name:     fh.Filename,
			MIMEType: fh.Header.Get("Content-Type"),
			Size:     int64(len(data)),
			Data:     data,
		})
	}
	return content, atts, nil
}

func (c *Client) sendAttachments(ctx context.Context, pubKey string, attachments []message.Attachment, msgID string) {
	v, loaded := c.sessions.Load(pubKey)
	if !loaded {
		return
	}
	sess, ok := v.(*Session)
	if !ok || sess.status() != store.PeerStatusAccepted {
		return
	}
	c.log.DebugContext(ctx, "api send files", "count", len(attachments), "msg_id", msgID)
	for _, att := range attachments {
		fileReader := io.LimitReader(bytes.NewReader(att.Data), att.Size)
		if sendErr := c.sendFile(sess, att.ID, att.Name, att.MIMEType, att.Size, fileReader); sendErr != nil {
			c.log.WarnContext(ctx, "send file failed", "id", att.ID, "error", sendErr)
		}
	}
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

	content, attachments, formErr := parseMessageForm(r, c.MaxMessageSize)
	if formErr != nil {
		http.Error(w, formErr.Error()+"\n", http.StatusBadRequest)
		return
	}
	defer func() { _ = r.MultipartForm.RemoveAll() }()

	peer, err := c.Store.KnownPeers.Get(pubKey)
	if err != nil {
		http.Error(w, "unknown peer\n", http.StatusNotFound)
		return
	}
	if peer.Status == store.PeerStatusRejected {
		http.Error(w, "peer rejected\n", http.StatusForbidden)
		return
	}

	sessRaw, sessLoaded := c.sessions.Load(pubKey)
	peerName := pubKey[:16]
	if sessLoaded {
		if sess, ok := sessRaw.(*Session); ok {
			peerName = sess.peerName()
			if sess.status() == store.PeerStatusRejected {
				http.Error(w, "peer rejected\n", http.StatusForbidden)
				return
			}
		}
	}

	msg := message.NewMessage(c.Name, peerName, content)
	queued := !sessLoaded
	if sessLoaded {
		if sess, ok := sessRaw.(*Session); ok {
			queued = sess.status() != store.PeerStatusAccepted
		}
	}

	haveFiles := len(attachments) > 0
	if haveFiles {
		msg.ID = uuid.New().String()
		for i := range attachments {
			attachments[i].ID = uuid.New().String()
		}
		msg.Attachments = make([]message.Attachment, len(attachments))
		for i, a := range attachments {
			msg.Attachments[i] = message.Attachment{
				ID: a.ID, Name: a.Name, MIMEType: a.MIMEType, Size: a.Size,
			}
		}
	}

	if sendErr := c.sendMessageToPeer(pubKey, msg); sendErr != nil {
		http.Error(w, "send failed\n", http.StatusInternalServerError)
		return
	}

	if haveFiles {
		c.sendAttachments(r.Context(), pubKey, attachments, msg.ID)
		if storeErr := message.StoreAttachments(c.Store, c.Keys.Private, msg.ID, attachments); storeErr != nil {
			c.log.WarnContext(r.Context(), "store attachments failed", "error", storeErr)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	status := "sent"
	if queued {
		status = "queued"
	}
	_ = json.NewEncoder(w).Encode(statusResponse{Status: status})
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
	_ = json.NewEncoder(w).Encode(statusResponse{Status: statusConnected})
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

func (c *Client) getOwnedMessage(id string) (*message.Message, error) {
	msg, err := message.Get(c.Store, c.Keys.Private, id)
	if err != nil {
		return nil, fmt.Errorf("get message: %w", err)
	}
	if msg.From != c.Name {
		return nil, errNotOwner
	}
	return msg, nil
}

func (c *Client) apiUpdateMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id\n", http.StatusBadRequest)
		return
	}

	msg, err := c.getOwnedMessage(id)
	if errors.Is(err, errNotOwner) {
		http.Error(w, "cannot edit another user's message\n", http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusNotFound)
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
	_ = json.NewEncoder(w).Encode(statusResponse{Status: statusUpdated})
}

func (c *Client) apiDeleteMessage(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id\n", http.StatusBadRequest)
		return
	}

	_, err := c.getOwnedMessage(id)
	if errors.Is(err, errNotOwner) {
		http.Error(w, "cannot delete another user's message\n", http.StatusForbidden)
		return
	}
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusNotFound)
		return
	}

	if delErr := message.Delete(c.Store, c.Keys.Private, id); delErr != nil {
		http.Error(w, delErr.Error()+"\n", http.StatusInternalServerError)
		return
	}

	c.broadcastDelete(id)

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResponse{Status: statusDeleted})
}

func (c *Client) apiGetFile(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id\n", http.StatusBadRequest)
		return
	}

	raw, err := c.Store.Files.Get(c.Keys.Private, id)
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Header().Set("Content-Length", strconv.Itoa(len(raw)))
	_, _ = w.Write(raw)
}

func (c *Client) adminListOutbox(w http.ResponseWriter, _ *http.Request) {
	entries, err := c.Store.Outbox.GetAllPending()
	if err != nil {
		http.Error(w, err.Error()+"\n", http.StatusInternalServerError)
		return
	}
	if entries == nil {
		entries = []store.OutboxEntry{}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(entries)
}

func (c *Client) adminDeleteOutboxEntry(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "missing id\n", http.StatusBadRequest)
		return
	}
	if err := c.Store.Outbox.Delete(id); err != nil {
		http.Error(w, err.Error()+"\n", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResponse{Status: statusDeleted})
}

func (c *Client) adminFlushOutbox(w http.ResponseWriter, r *http.Request) {
	pubKey := r.PathValue("pubKey")
	if pubKey == "" {
		http.Error(w, "missing pubKey\n", http.StatusBadRequest)
		return
	}
	c.flushOutbox(pubKey)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(statusResponse{Status: statusFlushed})
}
