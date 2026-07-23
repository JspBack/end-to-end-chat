package store_test

import (
	"net"
	"os"
	"path/filepath"
	"testing"

	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/store"
)

func TestKnownPeersAdd(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_add")
	s := store.New(dir)

	var pk keys.Key
	copy(pk[:], []byte("abcdefghijklmnopqrstuvwxyz123456"))

	peer := &store.KnownPeer{PubKey: pk, PeerIP: net.ParseIP("10.0.0.1"), Status: store.Pending}
	err := s.KnownPeers.Add(peer)
	if err != nil {
		t.Fatal("Add:", err)
	}
}

func TestKnownPeersGetNotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_notfound")
	s := store.New(dir)

	var nonexistent keys.Key
	copy(nonexistent[:], []byte("nonexistent_key_12345678901234"))

	_, err := s.KnownPeers.Get(nonexistent)
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

func TestKnownPeersAddThenReplace(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_replace")
	s := store.New(dir)

	var pk keys.Key
	copy(pk[:], []byte("abcdefghijklmnopqrstuvwxyz123456"))

	err := s.KnownPeers.Add(&store.KnownPeer{PubKey: pk, PeerIP: net.ParseIP("10.0.0.1"), Status: store.Pending})
	if err != nil {
		t.Fatal("Add first:", err)
	}

	err = s.KnownPeers.Add(&store.KnownPeer{PubKey: pk, PeerIP: net.ParseIP("10.0.0.2"), Status: store.Accepted})
	if err != nil {
		t.Fatal("Add second:", err)
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

func TestKnownPeersAddMultiple(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_peers_multi")
	s := store.New(dir)

	var pk1, pk2 keys.Key
	copy(pk1[:], []byte("peer_key_one_12345678901234567"))
	copy(pk2[:], []byte("peer_key_two_12345678901234567"))

	err := s.KnownPeers.Add(&store.KnownPeer{PubKey: pk1, PeerIP: net.ParseIP("10.0.0.1"), Status: store.Pending})
	if err != nil {
		t.Fatal("Add pk1:", err)
	}
	err = s.KnownPeers.Add(&store.KnownPeer{PubKey: pk2, PeerIP: net.ParseIP("10.0.0.2"), Status: store.Accepted})
	if err != nil {
		t.Fatal("Add pk2:", err)
	}
}
