package client

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"time"

	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
	"github.com/gorilla/websocket"
)

// disconnectedError is returned after a successful connection is lost.
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

func (c *Client) connectAndChat(ctx context.Context, lines <-chan string) error {
	u := url.URL{Scheme: "ws", Host: c.peerAddr, Path: "/transport/" + c.Keys.Public}

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	resp.Body.Close()

	c.log.DebugContext(ctx, "connected to peer", "addr", u.String())

	sess := newSession(conn, c.Name, c.Keys.Public, c.log, c.Timeout)
	if err = sess.performInitiatorKeyExchange(ctx); err != nil {
		conn.Close()
		return fmt.Errorf("handshake: %w", err)
	}

	c.log.DebugContext(ctx, "session key established", "peer_name", sess.PeerName())

	conn.SetPingHandler(func(appData string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	})

	c.storeSession(sess)

	pctx, cancel := context.WithCancel(ctx)
	defer cancel()

	errCh := make(chan error, 2)

	go func() {
		errCh <- c.peerReadLoop(sess)
	}()
	go func() {
		errCh <- c.peerWriteLoop(pctx, sess, lines)
	}()

	select {
	case e := <-errCh:
		cancel()
		return &disconnectedError{err: e}
	case <-ctx.Done():
		cancel()
		return nil
	}
}

func (c *Client) connectSession(ctx context.Context, addr string) error {
	u := url.URL{Scheme: "ws", Host: addr, Path: "/transport/" + c.Keys.Public}

	conn, resp, err := websocket.DefaultDialer.Dial(u.String(), nil)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	resp.Body.Close()

	sess := newSession(conn, c.Name, c.Keys.Public, c.log, c.Timeout)
	if err = sess.performInitiatorKeyExchange(ctx); err != nil {
		conn.Close()
		return fmt.Errorf("handshake: %w", err)
	}

	conn.SetPingHandler(func(appData string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return conn.WriteControl(websocket.PongMessage, []byte(appData), time.Now().Add(5*time.Second))
	})
	conn.SetPongHandler(func(string) error {
		_ = conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})

	c.storeSession(sess)

	_ = c.Store.KnownPeers.Add(&store.KnownPeer{
		PeerIP: addr,
		PubKey: sess.PeerPubKey(),
		Status: store.PeerStatusAccepted,
	})

	go func() { _ = c.peerReadLoop(sess) }()

	return nil
}

func (c *Client) storeSession(sess *Session) {
	pubKey := sess.PeerPubKey()
	sess.SetStatus(store.PeerStatusAccepted)
	if old, loaded := c.sessions.LoadAndDelete(pubKey); loaded {
		if oldSess, ok := old.(*Session); ok {
			oldSess.Close()
		}
	}
	c.sessions.Store(pubKey, sess)
}

func (c *Client) peerReadLoop(sess *Session) error {
	conn := sess.conn
	conn.SetReadLimit(c.MaxMessageSize)

	defer func() {
		c.sessions.Delete(sess.PeerPubKey())
		sess.Close()
	}()

	for {
		if err := conn.SetReadDeadline(time.Now().Add(60 * time.Second)); err != nil {
			return fmt.Errorf("set read deadline: %w", err)
		}

		_, data, err := conn.ReadMessage()
		if err != nil {
			return fmt.Errorf("read: %w", err)
		}

		plain, err := sess.Decrypt(data)
		if err != nil {
			c.log.Warn("decrypt error", "error", err)
			continue
		}

		if sess.Status() != store.PeerStatusAccepted {
			continue
		}

		var msg message.Message
		if err = json.Unmarshal(plain, &msg); err != nil {
			c.log.Warn("decode error", "error", err)
			continue
		}

		if _, err = message.Put(c.Store, c.Keys.Private, &msg); err != nil {
			c.log.Warn("store message failed", "error", err)
		}

		if msg.To == c.Name {
			fmt.Printf("> from=%s to=%s content=%s\n", msg.From, msg.To, msg.Content)
		}
	}
}

func (c *Client) peerWriteLoop(ctx context.Context, sess *Session, lines <-chan string) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case line, ok := <-lines:
			if !ok {
				return nil
			}
			msg := message.Message{From: c.Name, To: sess.PeerName(), Content: line}

			if _, err := message.Put(c.Store, c.Keys.Private, &msg); err != nil {
				c.log.WarnContext(ctx, "store message failed", "error", err)
			}

			plain, err := json.Marshal(msg)
			if err != nil {
				c.log.WarnContext(ctx, "encode error", "error", err)
				continue
			}

			enc, err := sess.Encrypt(plain)
			if err != nil {
				c.log.WarnContext(ctx, "encrypt error", "error", err)
				continue
			}

			if err = sess.conn.SetWriteDeadline(time.Now().Add(10 * time.Second)); err != nil {
				return fmt.Errorf("set write deadline: %w", err)
			}
			if err = sess.conn.WriteMessage(websocket.TextMessage, enc); err != nil {
				return fmt.Errorf("write: %w", err)
			}
		}
	}
}
