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
	"github.com/JspBack/end-to-end-chat/signal"
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
	go func() { errCh <- c.peerReadLoop(sess) }()
	go func() { c.peerWriteLoop(pctx, sess, lines); errCh <- nil }()

	select {
	case e := <-errCh:
		cancel()
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

	go func() { _ = c.peerReadLoop(sess) }()
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
}

func (c *Client) peerReadLoop(sess *Session) error {
	conn := sess.conn
	conn.SetReadLimit(c.MaxMessageSize)

	defer func() {
		c.sessions.Delete(sess.peerPubKey())
		_ = sess.closeConn()
	}()

	for {
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		plain, err := sess.decrypt(data)
		if err != nil {
			c.log.Warn("decrypt error", "error", err)
			continue
		}

		if !sess.msgLimiter.Allow() {
			c.log.Warn("rate limit exceeded, dropping message", "remote", sess.conn.RemoteAddr())
			continue
		}

		if sess.status() != store.PeerStatusAccepted {
			continue
		}

		if sig, parseErr := signal.Parse(plain); parseErr == nil {
			switch sig.Type {
			case signal.TypeDelete:
				if delErr := message.Delete(c.Store, sig.ID); delErr != nil {
					c.log.Warn("delete message failed", "error", delErr)
				}
				continue
			case signal.TypeUpdate:
				if updErr := c.applyRemoteUpdate(sig.ID, sig.Content); updErr != nil {
					c.log.Warn("update message failed", "error", updErr)
				}
				continue
			}
		}

		msg, err := message.ToMessage(plain)
		if err != nil {
			c.log.Warn("decode error", "error", err)
			continue
		}

		if _, err = message.Put(c.Store, c.Keys.Private, msg); err != nil {
			c.log.Warn("store message failed", "error", err)
		}

		if msg.To == c.Name {
			c.log.Debug("message received",
				"from", msg.From, "to", msg.To, "content", msg.Content)
		}
	}
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
			if err := c.sendMessage(sess, line); err != nil {
				c.log.WarnContext(ctx, "send failed", "peer", sess.peerName(), "error", err)
			}
		}
	}
}
