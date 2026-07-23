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

func TestOutboxPutAndGet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_putget")
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

	entries, err := s.Outbox.Get(targetKey, outboxSecret)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if string(entries[0].SignalContent) != string(rawSignal) {
		t.Errorf("signal content mismatch:\ngot:  %q\nwant: %q", entries[0].SignalContent, rawSignal)
	}
}

func TestOutboxGetWrongPeer(t *testing.T) {
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

func TestOutboxMultiplePutsSamePeer(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_multi")
	s := store.New(dir)

	var peerKey keys.Key
	copy(peerKey[:], []byte("peer-key-for-multi-put-test!!"))

	_, _ = s.Outbox.Put(peerKey, []byte(`{"type":"message","content":"first"}`), outboxSecret)
	_, _ = s.Outbox.Put(peerKey, []byte(`{"type":"message","content":"second"}`), outboxSecret)
	_, _ = s.Outbox.Put(peerKey, []byte(`{"type":"delete","id":"msg-1"}`), outboxSecret)

	entries, err := s.Outbox.Get(peerKey, outboxSecret)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
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

	entries, _ := s.Outbox.Get(peerKey, outboxSecret)
	if len(entries) != 0 {
		t.Errorf("expected 0 entries after delete, got %d", len(entries))
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

	id, _ := s.Outbox.Put(peerKey, []byte(`{"type":"message"}`), outboxSecret)

	err := s.Outbox.IncrementRetry(id)
	if err != nil {
		t.Fatal("IncrementRetry:", err)
	}

	entries, _ := s.Outbox.Get(peerKey, outboxSecret)
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].RetryCount != 1 {
		t.Errorf("expected RetryCount=1, got %d", entries[0].RetryCount)
	}

	_ = s.Outbox.IncrementRetry(id)
	entries, _ = s.Outbox.Get(peerKey, outboxSecret)
	if entries[0].RetryCount != 2 {
		t.Errorf("expected RetryCount=2, got %d", entries[0].RetryCount)
	}
}

func TestOutboxList(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_all")
	s := store.New(dir)

	var keyA, keyB keys.Key
	copy(keyA[:], []byte("peer-a-key-for-outbox-list!!!!"))
	copy(keyB[:], []byte("peer-b-key-for-outbox-list!!!!"))

	_, _ = s.Outbox.Put(keyA, []byte(`{"type":"message","content":"a1"}`), outboxSecret)
	_, _ = s.Outbox.Put(keyB, []byte(`{"type":"message","content":"b1"}`), outboxSecret)
	_, _ = s.Outbox.Put(keyA, []byte(`{"type":"message","content":"a2"}`), outboxSecret)

	entries, err := s.Outbox.List()
	if err != nil {
		t.Fatal("List:", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	peers := map[string]int{}
	for _, e := range entries {
		peers[e.TargetPubKey.String()]++
	}
	if peers[keyA.String()] != 2 {
		t.Errorf("expected 2 entries for peer-a, got %d", peers[keyA.String()])
	}
	if peers[keyB.String()] != 1 {
		t.Errorf("expected 1 entry for peer-b, got %d", peers[keyB.String()])
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

func TestOutboxEncryption(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_enc")
	s := store.New(dir)

	var wrongKey keys.Key
	copy(wrongKey[:], []byte("wrong-key-for-outbox-enc-test!"))

	var peerKey keys.Key
	copy(peerKey[:], []byte("peer-key-for-encryption-test"))

	rawSignal := []byte(`{"type":"message","content":"secret message"}`)
	_, err := s.Outbox.Put(peerKey, rawSignal, outboxSecret)
	if err != nil {
		t.Fatal("Put:", err)
	}

	_, err = s.Outbox.Get(peerKey, wrongKey)
	if err == nil {
		t.Error("expected error with wrong key")
	}

	entries, err := s.Outbox.Get(peerKey, outboxSecret)
	if err != nil {
		t.Fatal("Get with correct key:", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if string(entries[0].SignalContent) != string(rawSignal) {
		t.Errorf("got %q, want %q", entries[0].SignalContent, rawSignal)
	}
}

func TestOutboxOrderPreserved(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_order")
	s := store.New(dir)

	var peerKey keys.Key
	copy(peerKey[:], []byte("peer-key-for-order-test!!!!!"))

	msgs := []string{"first", "second", "third"}
	for _, m := range msgs {
		_, _ = s.Outbox.Put(peerKey, []byte(`{"content":"`+m+`"}`), outboxSecret)
	}

	entries, _ := s.Outbox.Get(peerKey, outboxSecret)
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}
