package store_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/JspBack/end-to-end-chat/store"
)

func TestKnownPeersAddGet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_addget")
	s := store.New(dir)

	peer := &store.KnownPeer{PubKey: "abc123", PeerIP: "10.0.0.1", Status: store.PeerStatusPending}
	err := s.KnownPeers.Add(peer)
	if err != nil {
		t.Fatal("Add:", err)
	}

	got, err := s.KnownPeers.Get("abc123")
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.PubKey != "abc123" || got.PeerIP != "10.0.0.1" || got.Status != store.PeerStatusPending {
		t.Errorf("got %+v, want PubKey=abc123 PeerIP=10.0.0.1 Status=pending", got)
	}
}

func TestKnownPeersGetNotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_notfound")
	s := store.New(dir)

	_, err := s.KnownPeers.Get("nonexistent")
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

func TestKnownPeersReplace(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_replace")
	s := store.New(dir)

	s.KnownPeers.Add(&store.KnownPeer{PubKey: "k1", PeerIP: "10.0.0.1", Status: store.PeerStatusPending})
	s.KnownPeers.Add(&store.KnownPeer{PubKey: "k1", PeerIP: "10.0.0.2", Status: store.PeerStatusAccepted})

	got, err := s.KnownPeers.Get("k1")
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.PeerIP != "10.0.0.2" || got.Status != store.PeerStatusAccepted {
		t.Errorf("got %+v, want IP=10.0.0.2 Status=accepted", got)
	}
}

func TestKnownPeersList(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_list")
	s := store.New(dir)

	s.KnownPeers.Add(&store.KnownPeer{PubKey: "k1", PeerIP: "10.0.0.1", Status: store.PeerStatusPending})
	s.KnownPeers.Add(&store.KnownPeer{PubKey: "k2", PeerIP: "10.0.0.2", Status: store.PeerStatusAccepted})

	list, err := s.KnownPeers.List()
	if err != nil {
		t.Fatal("List:", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 peers, got %d", len(list))
	}
}

func TestKnownPeersListEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_empty")
	s := store.New(dir)

	list, err := s.KnownPeers.List()
	if err != nil {
		t.Fatal("List:", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestKnownPeersRemove(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_remove")
	s := store.New(dir)

	s.KnownPeers.Add(&store.KnownPeer{PubKey: "k1", PeerIP: "10.0.0.1", Status: store.PeerStatusPending})
	err := s.KnownPeers.Remove("k1")
	if err != nil {
		t.Fatal("Remove:", err)
	}

	_, err = s.KnownPeers.Get("k1")
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist after remove, got %v", err)
	}
}

func TestKnownPeersRemoveTwice(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_rm2")
	s := store.New(dir)

	s.KnownPeers.Add(&store.KnownPeer{PubKey: "k1", PeerIP: "10.0.0.1", Status: store.PeerStatusPending})
	s.KnownPeers.Remove("k1")
	err := s.KnownPeers.Remove("k1")
	if err != nil {
		t.Errorf("removing twice should not error, got %v", err)
	}
}
