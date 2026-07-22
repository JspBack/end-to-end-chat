package store_test

import (
	"path/filepath"
	"testing"

	"github.com/JspBack/end-to-end-chat/store"
	"github.com/google/uuid"
)

func TestOutboxPutAndGetPending(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_putget")
	s := store.New(dir)

	rawSignal := []byte(`{"type":"message","from":"alice","content":"hello"}`)
	id, err := s.Outbox.Put("peer-pubkey", rawSignal, "secret")
	if err != nil {
		t.Fatal("Put:", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	entries, err := s.Outbox.GetPending("peer-pubkey", "secret")
	if err != nil {
		t.Fatal("GetPending:", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if string(entries[0].SignalContent) != string(rawSignal) {
		t.Errorf("signal content mismatch:\ngot:  %q\nwant: %q", entries[0].SignalContent, rawSignal)
	}
}

func TestOutboxGetPendingWrongPeer(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_wrongpeer")
	s := store.New(dir)

	rawSignal := []byte(`{"type":"message","content":"hello"}`)
	_, err := s.Outbox.Put("peer-1", rawSignal, "secret")
	if err != nil {
		t.Fatal("Put:", err)
	}

	entries, err := s.Outbox.GetPending("peer-2", "secret")
	if err != nil {
		t.Fatal("GetPending for wrong peer:", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected 0 entries for wrong peer, got %d", len(entries))
	}
}

func TestOutboxMultiplePutsSamePeer(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_multi")
	s := store.New(dir)

	_, _ = s.Outbox.Put("peer", []byte(`{"type":"message","content":"first"}`), "secret")
	_, _ = s.Outbox.Put("peer", []byte(`{"type":"message","content":"second"}`), "secret")
	_, _ = s.Outbox.Put("peer", []byte(`{"type":"delete","id":"msg-1"}`), "secret")

	entries, err := s.Outbox.GetPending("peer", "secret")
	if err != nil {
		t.Fatal("GetPending:", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}

func TestOutboxDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_delete")
	s := store.New(dir)

	id, _ := s.Outbox.Put("peer", []byte(`{"type":"message"}`), "secret")
	err := s.Outbox.Delete(uuid.MustParse(id))
	if err != nil {
		t.Fatal("Delete:", err)
	}

	entries, _ := s.Outbox.GetPending("peer", "secret")
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

	id, _ := s.Outbox.Put("peer", []byte(`{"type":"message"}`), "secret")

	err := s.Outbox.IncrementRetry(uuid.MustParse(id))
	if err != nil {
		t.Fatal("IncrementRetry:", err)
	}

	entries, _ := s.Outbox.GetPending("peer", "secret")
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].RetryCount != 1 {
		t.Errorf("expected RetryCount=1, got %d", entries[0].RetryCount)
	}

	_ = s.Outbox.IncrementRetry(uuid.MustParse(id))
	entries, _ = s.Outbox.GetPending("peer", "secret")
	if entries[0].RetryCount != 2 {
		t.Errorf("expected RetryCount=2, got %d", entries[0].RetryCount)
	}
}

func TestOutboxGetAllPending(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_all")
	s := store.New(dir)

	_, _ = s.Outbox.Put("peer-a", []byte(`{"type":"message","content":"a1"}`), "secret")
	_, _ = s.Outbox.Put("peer-b", []byte(`{"type":"message","content":"b1"}`), "secret")
	_, _ = s.Outbox.Put("peer-a", []byte(`{"type":"message","content":"a2"}`), "secret")

	entries, err := s.Outbox.GetAllPending()
	if err != nil {
		t.Fatal("GetAllPending:", err)
	}
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}

	peers := map[string]int{}
	for _, e := range entries {
		peers[e.TargetPubKey]++
	}
	if peers["peer-a"] != 2 {
		t.Errorf("expected 2 entries for peer-a, got %d", peers["peer-a"])
	}
	if peers["peer-b"] != 1 {
		t.Errorf("expected 1 entry for peer-b, got %d", peers["peer-b"])
	}
}

func TestOutboxGetAllPendingEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_allempty")
	s := store.New(dir)

	entries, err := s.Outbox.GetAllPending()
	if err != nil {
		t.Fatal("GetAllPending:", err)
	}
	if len(entries) != 0 {
		t.Errorf("expected empty list, got %d entries", len(entries))
	}
}

func TestOutboxEncryption(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_outbox_enc")
	s := store.New(dir)

	rawSignal := []byte(`{"type":"message","content":"secret message"}`)
	_, err := s.Outbox.Put("peer", rawSignal, "correct-key")
	if err != nil {
		t.Fatal("Put:", err)
	}

	_, err = s.Outbox.GetPending("peer", "wrong-key")
	if err == nil {
		t.Error("expected error with wrong key")
	}

	entries, err := s.Outbox.GetPending("peer", "correct-key")
	if err != nil {
		t.Fatal("GetPending with correct key:", err)
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

	msgs := []string{"first", "second", "third"}
	for _, m := range msgs {
		_, _ = s.Outbox.Put("peer", []byte(`{"content":"`+m+`"}`), "secret")
	}

	entries, _ := s.Outbox.GetPending("peer", "secret")
	if len(entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(entries))
	}
}
