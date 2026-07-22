package client

import (
	"net"
	"net/http"
)

func (c *Client) localhostOnly(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		if err != nil {
			http.Error(w, "invalid remote address\n", http.StatusBadRequest)
			return
		}
		ip := net.ParseIP(host)
		if ip == nil || !ip.IsLoopback() {
			c.log.WarnContext(r.Context(), "admin request from non-localhost rejected",
				"remote", r.RemoteAddr, "path", r.URL.Path)
			http.Error(w, "forbidden\n", http.StatusForbidden)
			return
		}
		next(w, r)
	}
}

func (c *Client) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /admin/peers", c.localhostOnly(c.adminListPeers))
	mux.HandleFunc("PUT /admin/peers/{pubKey}/accept", c.localhostOnly(c.adminAcceptPeer))
	mux.HandleFunc("PUT /admin/peers/{pubKey}/reject", c.localhostOnly(c.adminRejectPeer))
	mux.HandleFunc("GET /admin/sessions", c.localhostOnly(c.adminListSessions))
	mux.HandleFunc("POST /api/peers/connect", c.localhostOnly(c.apiConnectPeer))
	mux.HandleFunc("POST /api/messages/{pubKey}", c.localhostOnly(c.apiSendMessage))
	mux.HandleFunc("GET /api/messages", c.localhostOnly(c.apiListMessages))
	mux.HandleFunc("GET /api/messages/{id}", c.localhostOnly(c.apiGetMessage))
	mux.HandleFunc("GET /api/messages/search", c.localhostOnly(c.apiSearchMessages))
	mux.HandleFunc("PUT /api/messages/{id}", c.localhostOnly(c.apiUpdateMessage))
	mux.HandleFunc("DELETE /api/messages/{id}", c.localhostOnly(c.apiDeleteMessage))
	mux.HandleFunc("GET /api/files/{id}", c.localhostOnly(c.apiGetFile))
	mux.HandleFunc("GET /admin/outbox", c.localhostOnly(c.adminListOutbox))
	mux.HandleFunc("DELETE /admin/outbox/{id}", c.localhostOnly(c.adminDeleteOutboxEntry))
	mux.HandleFunc("POST /admin/outbox/flush/{pubKey}", c.localhostOnly(c.adminFlushOutbox))
}
