package store_test

import (
	"path/filepath"
	"testing"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/store"
)

var outboxSecret = func() keys.Key {
	k, _ := keys.FromHex("deadbeef0102030405060708091011121314151617181920212223242526272829303132")
	return k
}()

func TestOutboxPut(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_put")
	s := store.New(dir)

	var targetKey keys.Key
	copy(targetKey[:], []byte("peer-pubkey-for-outbox-testing!"))

	rawSignal := []byte(`{"type":"message","from":"alice","content":"hello"}`)
	id, err := s.Outbox.Put(targetKey, rawSignal, outboxSecret)
	if err != nil {
		t.Fatal("Put:", err)
	}
	if id == uuid.Nil {
		t.Fatal("expected non-empty id")
	}
}

func TestOutboxGetWrongPeerEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_wrongpeer")
	s := store.New(dir)

	var key1, key2 keys.Key
	copy(key1[:], []byte("peer-one-key-for-outbox-test!!"))
	copy(key2[:], []byte("peer-two-key-for-outbox-test!!"))

	rawSignal := []byte(`{"type":"message","content":"hello"}`)
	_, err := s.Outbox.Put(key1, rawSignal, outboxSecret)
	if err != nil {
		t.Fatal("Put:", err)
	}

	entries, err := s.Outbox.Get(key2, outboxSecret)
	if err != nil {
		t.Fatal("Get for wrong peer:", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for wrong peer, got %d", len(entries))
	}
}

func TestOutboxMultiplePuts(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_multi")
	s := store.New(dir)

	var peerKey keys.Key
	copy(peerKey[:], []byte("peer-key-for-multi-put-test!!"))

	id1, err := s.Outbox.Put(peerKey, []byte(`{"type":"message","content":"first"}`), outboxSecret)
	if err != nil {
		t.Fatal("Put 1:", err)
	}
	if id1 == uuid.Nil {
		t.Fatal("expected non-empty id for first put")
	}

	id2, err := s.Outbox.Put(peerKey, []byte(`{"type":"message","content":"second"}`), outboxSecret)
	if err != nil {
		t.Fatal("Put 2:", err)
	}
	if id2 == uuid.Nil {
		t.Fatal("expected non-empty id for second put")
	}

	id3, err := s.Outbox.Put(peerKey, []byte(`{"type":"delete","id":"msg-1"}`), outboxSecret)
	if err != nil {
		t.Fatal("Put 3:", err)
	}
	if id3 == uuid.Nil {
		t.Fatal("expected non-empty id for third put")
	}
}

func TestOutboxDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_delete")
	s := store.New(dir)

	var peerKey keys.Key
	copy(peerKey[:], []byte("peer-key-for-delete-test!!"))

	id, _ := s.Outbox.Put(peerKey, []byte(`{"type":"message"}`), outboxSecret)
	err := s.Outbox.Delete(id)
	if err != nil {
		t.Fatal("Delete:", err)
	}
}

func TestOutboxDeleteNotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_delnf")
	s := store.New(dir)

	err := s.Outbox.Delete(uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	if err != nil {
		t.Errorf("delete non-existent should not error, got %v", err)
	}
}

func TestOutboxIncrementRetry(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_retry")
	s := store.New(dir)

	var peerKey keys.Key
	copy(peerKey[:], []byte("peer-key-for-retry-test!!!!!"))

	id, err := s.Outbox.Put(peerKey, []byte(`{"type":"message"}`), outboxSecret)
	if err != nil {
		t.Fatal("Put:", err)
	}

	err = s.Outbox.IncrementRetry(id)
	if err != nil {
		t.Fatal("IncrementRetry:", err)
	}

	err = s.Outbox.IncrementRetry(id)
	if err != nil {
		t.Fatal("IncrementRetry 2:", err)
	}
}

func TestOutboxListEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_allempty")
	s := store.New(dir)

	entries, err := s.Outbox.List()
	if err != nil {
		t.Fatal("List:", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty list, got %d entries", len(entries))
	}
}

func TestOutboxPutDuplicateID(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_dup")
	s := store.New(dir)

	var peerKey keys.Key
	copy(peerKey[:], []byte("peer-key-for-dup-test!!!!!!"))

	id1, _ := s.Outbox.Put(peerKey, []byte(`{"type":"msg","content":"first"}`), outboxSecret)
	id2, _ := s.Outbox.Put(peerKey, []byte(`{"type":"msg","content":"second"}`), outboxSecret)

	if id1 == id2 {
		t.Error("two Puts should produce different IDs")
	}
}
