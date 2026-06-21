package store_test

import (
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/JspBack/end-to-end-chat/client"
	"github.com/JspBack/end-to-end-chat/config"
	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
)

func newTestClient(t *testing.T) *client.Client {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "client-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())

	c := client.New(config.Config{
		ClientName: "test",
		KeyFile:    filepath.Join(t.TempDir(), "key"),
	}, slog.Default())
	t.Cleanup(func() { os.Remove(f.Name()) })
	return c
}

func TestClientPutGet(t *testing.T) {
	c := newTestClient(t)
	msg := &message.Message{
		To:      "alice",
		Content: "hello",
	}

	id, err := message.Put(c.Store, c.Keys.Private, msg)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	got, err := message.Get(c.Store, c.Keys.Private, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.To != "alice" || got.Content != "hello" {
		t.Fatalf("got %+v, want {alice hello}", got)
	}
}

func TestClientGetMissingKey(t *testing.T) {
	c := newTestClient(t)
	_, err := message.Get(c.Store, c.Keys.Private, "nonexistent")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestClientUpdateExisting(t *testing.T) {
	c := newTestClient(t)
	msg := &message.Message{To: "bob", Content: "hi"}

	id, err := message.Put(c.Store, c.Keys.Private, msg)
	if err != nil {
		t.Fatal(err)
	}
	msg.Content = "updated"
	if err = message.Update(c.Store, c.Keys.Private, id, msg); err != nil {
		t.Fatal(err)
	}

	got, err := message.Get(c.Store, c.Keys.Private, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.Content != "updated" {
		t.Fatalf("got %q, want %q", got.Content, "updated")
	}
}

func TestClientUpdateMissing(t *testing.T) {
	c := newTestClient(t)
	err := message.Update(c.Store, c.Keys.Private, "nonexistent", &message.Message{})
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestClientDelete(t *testing.T) {
	c := newTestClient(t)
	msg := &message.Message{To: "carol", Content: "bye"}

	id, err := message.Put(c.Store, c.Keys.Private, msg)
	if err != nil {
		t.Fatal(err)
	}
	if err = message.Delete(c.Store, id); err != nil {
		t.Fatal(err)
	}

	_, err = message.Get(c.Store, c.Keys.Private, id)
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expected ErrNotExist after delete")
	}
}

func TestClientEncryptionRoundTrip(t *testing.T) {
	c := newTestClient(t)

	msg := &message.Message{To: "dave", Content: "secret msg"}
	id, err := message.Put(c.Store, c.Keys.Private, msg)
	if err != nil {
		t.Fatal(err)
	}

	raw, err := c.Store.Chats.Get(id, c.Keys.Private)
	if err != nil {
		t.Fatal(err)
	}
	var expected = `{"from":"","to":"dave","content":"secret msg"}`
	if raw != expected {
		t.Fatalf("got %q, want %q", raw, expected)
	}

	got, err := message.Get(c.Store, c.Keys.Private, id)
	if err != nil {
		t.Fatal(err)
	}
	if got.To != "dave" || got.Content != "secret msg" {
		t.Fatalf("got %+v, want {dave secret msg}", got)
	}
}

func TestClientKnownPeers(t *testing.T) {
	c := newTestClient(t)

	peer := &store.KnownPeer{
		PeerIP: "10.0.0.5",
		PubKey: "pubkey123",
		Status: "online",
	}

	if err := c.AddKnownPeer(peer); err != nil {
		t.Fatal(err)
	}

	got, err := c.GetKnownPeer("10.0.0.5")
	if err != nil {
		t.Fatal(err)
	}
	if got.PeerIP != "10.0.0.5" || got.PubKey != "pubkey123" || got.Status != "online" {
		t.Fatalf("got %+v", got)
	}

	if err = c.RemoveKnownPeer("10.0.0.5"); err != nil {
		t.Fatal(err)
	}

	_, err = c.GetKnownPeer("10.0.0.5")
	if !errors.Is(err, os.ErrNotExist) {
		t.Fatal("expected ErrNotExist after remove")
	}
}
