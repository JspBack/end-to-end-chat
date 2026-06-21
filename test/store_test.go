package store_test

import (
	"os"
	"testing"

	"github.com/JspBack/end-to-end-chat/store"
)

const testSecret = "test-secret-key-for-testing"

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "store-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())
	s := store.New(f.Name())
	t.Cleanup(func() { os.Remove(f.Name()) })
	return s
}

func TestNewCreatesFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "store-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())

	_ = store.New(f.Name())

	if _, err = os.Stat(f.Name()); os.IsNotExist(err) {
		t.Error("New did not create the file")
	}
}

func TestPutGet(t *testing.T) {
	s := newTestStore(t)

	id, err := s.Chats.Put("hello", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	got, err := s.Chats.Get(id, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestGetMissingKey(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Chats.Get("nonexistent", testSecret)
	if !os.IsNotExist(err) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestUpdateExisting(t *testing.T) {
	s := newTestStore(t)

	id, err := s.Chats.Put("first", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Chats.Update(id, "second", testSecret); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Chats.Get(id, testSecret)
	if got != "second" {
		t.Fatalf("got %q, want %q", got, "second")
	}
}

func TestUpdateMissing(t *testing.T) {
	s := newTestStore(t)
	err := s.Chats.Update("nonexistent", "value", testSecret)
	if !os.IsNotExist(err) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)

	id, err := s.Chats.Put("value", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Chats.Delete(id); err != nil {
		t.Fatal(err)
	}

	_, err = s.Chats.Get(id, testSecret)
	if !os.IsNotExist(err) {
		t.Fatal("expected ErrNotExist after delete")
	}
}

func TestKnownPeers(t *testing.T) {
	s := newTestStore(t)

	peer := &store.KnownPeer{
		PeerIP: "192.168.1.10",
		PubKey: "abc123",
		Status: "online",
	}

	if err := s.KnownPeers.Add(peer); err != nil {
		t.Fatal(err)
	}

	got, err := s.KnownPeers.Get("192.168.1.10")
	if err != nil {
		t.Fatal(err)
	}
	if got.PeerIP != "192.168.1.10" || got.PubKey != "abc123" || got.Status != "online" {
		t.Fatalf("got %+v, want {192.168.1.10 abc123 online}", got)
	}

	if err = s.KnownPeers.Remove("192.168.1.10"); err != nil {
		t.Fatal(err)
	}

	_, err = s.KnownPeers.Get("192.168.1.10")
	if !os.IsNotExist(err) {
		t.Fatal("expected ErrNotExist after remove")
	}
}

func TestKnownPeersList(t *testing.T) {
	s := newTestStore(t)

	peers := []*store.KnownPeer{
		{PeerIP: "10.0.0.1", PubKey: "key1", Status: "online"},
		{PeerIP: "10.0.0.2", PubKey: "key2", Status: "offline"},
	}
	for _, p := range peers {
		if err := s.KnownPeers.Add(p); err != nil {
			t.Fatal(err)
		}
	}

	for _, expected := range peers {
		got, err := s.KnownPeers.Get(expected.PeerIP)
		if err != nil {
			t.Fatal(err)
		}
		if got.PeerIP != expected.PeerIP || got.PubKey != expected.PubKey || got.Status != expected.Status {
			t.Fatalf("got %+v, want %+v", got, expected)
		}
	}
}
