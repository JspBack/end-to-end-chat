package client

import (
	"bufio"
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
	"github.com/gorilla/websocket"
)

type disconnectedError struct {
	err error
}

func (e *disconnectedError) Error() string { return e.err.Error() }
func (e *disconnectedError) Unwrap() error { return e.err }

func (c *Client) ConnectToPeer(ctx context.Context) error {
	lines := make(chan string, 100)
	go readStdin(lines)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		err := c.connectAndChat(ctx, lines)
		if err != nil {
			var disconn *disconnectedError
			if errors.As(err, &disconn) {
				c.log.WarnContext(ctx, "disconnected, reconnecting in 2s...", "error", disconn.err)
				select {
				case <-ctx.Done():
					return nil
				case <-time.After(2 * time.Second):
				}
			} else {
				c.log.WarnContext(ctx, "connection failed, not retrying", "error", err)
				return nil
			}
		}
	}
}

func readStdin(lines chan<- string) {
	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
}

func (c *Client) dialAndHandshake(ctx context.Context, addr string) (*Session, error) {
	scheme := "ws"
	if c.UseTLS() {
		scheme = "wss"
	}
	u := url.URL{Scheme: scheme, Host: addr, Path: "/transport/" + c.Keys.Public}

	dialer := websocket.DefaultDialer
	if c.UseTLS() {
		dialer.TLSClientConfig = &tls.Config{InsecureSkipVerify: false}
	}
	conn, resp, err := dialer.Dial(u.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("dial: %w", err)
	}
	resp.Body.Close()

	sess := newSession(conn, c.Name, c.Keys, c.log, c.Timeout, c.RateLimit, c.RateWindow)
	if err = sess.handshake(ctx, true); err != nil {
		conn.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}

	conn.SetPingHandler(func(data string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(5*time.Second))
	})
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	c.storeSession(sess)
	return sess, nil
}

func (c *Client) connectAndChat(ctx context.Context, lines <-chan string) error {
	sess, err := c.dialAndHandshake(ctx, c.peerAddr)
	if err != nil {
		return err
	}

	c.log.DebugContext(ctx, "session ready", "peer_name", sess.peerName())

	pctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)
	go func() { errCh <- c.readLoop(pctx, sess) }()
	go func() { c.peerWriteLoop(pctx, sess, lines); errCh <- nil }()

	select {
	case e := <-errCh:
		return &disconnectedError{err: e}
	case <-ctx.Done():
		return nil
	}
}

func (c *Client) connectSession(ctx context.Context, addr string) error {
	sess, err := c.dialAndHandshake(ctx, addr)
	if err != nil {
		return err
	}

	_ = c.Store.KnownPeers.Add(&store.KnownPeer{
		PeerIP: addr,
		PubKey: sess.peerPubKey(),
		Status: store.PeerStatusAccepted,
	})

	go func() { _ = c.readLoop(ctx, sess) }()
	return nil
}

func (c *Client) storeSession(sess *Session) {
	pubKey := sess.peerPubKey()
	sess.setStatus(store.PeerStatusAccepted)
	if old, loaded := c.sessions.LoadAndDelete(pubKey); loaded {
		if oldSess, ok := old.(*Session); ok {
			_ = oldSess.closeConn()
		}
	}
	c.sessions.Store(pubKey, sess)
	c.flushOutbox(pubKey)
}

func (c *Client) peerWriteLoop(ctx context.Context, sess *Session, lines <-chan string) {
	for {
		select {
		case <-ctx.Done():
			return
		case line, ok := <-lines:
			if !ok {
				return
			}
			msg := message.NewMessage(c.Name, sess.peerName(), line)
			if err := c.sendMessage(sess, msg); err != nil {
				c.log.WarnContext(ctx, "send failed", "peer", sess.peerName(), "error", err)
			}
		}
	}
}
