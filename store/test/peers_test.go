package store_test

// import (
// 	"net"
// 	"os"
// 	"path/filepath"
// 	"testing"

// 	"github.com/JspBack/end-to-end-chat/keys"
// 	"github.com/JspBack/end-to-end-chat/store"
// )

// func TestKnownPeersAddGet(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_peers_addget")
// 	s := store.New(dir)

// 	var pk keys.Key
// 	copy(pk[:], "abc123")

// 	peer := &store.KnownPeer{PubKey: pk, PeerIP: net.ParseIP("10.0.0.1"), Status: store.Pending}
// 	err := s.KnownPeers.Add(peer)
// 	if err != nil {
// 		t.Fatal("Add:", err)
// 	}

// 	got, err := s.KnownPeers.Get(pk)
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if got.PubKey != pk || !got.PeerIP.Equal(net.ParseIP("10.0.0.1")) || got.Status != store.Pending {
// 		t.Errorf("got %+v, want PubKey=%s PeerIP=10.0.0.1 Status=pending", got, pk.String())
// 	}
// }

// func TestKnownPeersGetNotFound(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_peers_notfound")
// 	s := store.New(dir)

// 	_, err := s.KnownPeers.Get("nonexistent")
// 	if !os.IsNotExist(err) {
// 		t.Errorf("expected ErrNotExist, got %v", err)
// 	}
// }

// func TestKnownPeersReplace(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_peers_replace")
// 	s := store.New(dir)

// 	var pk keys.Key
// 	copy(pk[:], "k1")

// 	s.KnownPeers.Add(&store.KnownPeer{PubKey: pk, PeerIP: net.ParseIP("10.0.0.1"), Status: store.Pending})
// 	s.KnownPeers.Add(&store.KnownPeer{PubKey: pk, PeerIP: net.ParseIP("10.0.0.2"), Status: store.Accepted})

// 	got, err := s.KnownPeers.Get(pk.String())
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if !got.PeerIP.Equal(net.ParseIP("10.0.0.2")) || got.Status != store.Accepted {
// 		t.Errorf("got %+v, want IP=10.0.0.2 Status=accepted", got)
// 	}
// }

// func TestKnownPeersList(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_peers_list")
// 	s := store.New(dir)

// 	var pk1, pk2 keys.Key
// 	copy(pk1[:], "k1")
// 	copy(pk2[:], "k2")

// 	s.KnownPeers.Add(&store.KnownPeer{PubKey: pk1, PeerIP: net.ParseIP("10.0.0.1"), Status: store.Pending})
// 	s.KnownPeers.Add(&store.KnownPeer{PubKey: pk2, PeerIP: net.ParseIP("10.0.0.2"), Status: store.Accepted})

// 	list, err := s.KnownPeers.List()
// 	if err != nil {
// 		t.Fatal("List:", err)
// 	}
// 	if len(list) != 2 {
// 		t.Fatalf("expected 2 peers, got %d", len(list))
// 	}
// }

// func TestKnownPeersListEmpty(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_peers_empty")
// 	s := store.New(dir)

// 	list, err := s.KnownPeers.List()
// 	if err != nil {
// 		t.Fatal("List:", err)
// 	}
// 	if len(list) != 0 {
// 		t.Errorf("expected empty list, got %d items", len(list))
// 	}
// }

// func TestKnownPeersRemove(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_peers_remove")
// 	s := store.New(dir)

// 	var pk keys.Key
// 	copy(pk[:], "k1")

// 	s.KnownPeers.Add(&store.KnownPeer{PubKey: pk, PeerIP: net.ParseIP("10.0.0.1"), Status: store.Pending})
// 	err := s.KnownPeers.Remove(pk.String())
// 	if err != nil {
// 		t.Fatal("Remove:", err)
// 	}

// 	_, err = s.KnownPeers.Get(pk.String())
// 	if !os.IsNotExist(err) {
// 		t.Errorf("expected ErrNotExist after remove, got %v", err)
// 	}
// }

// func TestKnownPeersRemoveTwice(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_peers_rm2")
// 	s := store.New(dir)

// 	var pk keys.Key
// 	copy(pk[:], "k1")

// 	s.KnownPeers.Add(&store.KnownPeer{PubKey: pk, PeerIP: net.ParseIP("10.0.0.1"), Status: store.Pending})
// 	s.KnownPeers.Remove(pk.String())
// 	err := s.KnownPeers.Remove(pk.String())
// 	if err != nil {
// 		t.Errorf("removing twice should not error, got %v", err)
// 	}
// }
