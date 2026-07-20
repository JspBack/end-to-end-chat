package test_test

import (
	"os"
	"strings"
	"testing"

	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
)

func TestNewMessage(t *testing.T) {
	m := message.NewMessage("alice", "bob", "hello")
	if m.From != "alice" {
		t.Errorf("From = %q, want alice", m.From)
	}
	if m.To != "bob" {
		t.Errorf("To = %q, want bob", m.To)
	}
	if m.Content != "hello" {
		t.Errorf("Content = %q, want hello", m.Content)
	}
	if m.Time == "" {
		t.Error("Time is empty")
	}
}

func TestEncodeDecode(t *testing.T) {
	orig := message.NewMessage("alice", "bob", "hello")
	data, err := orig.Encode()
	if err != nil {
		t.Fatal("Encode:", err)
	}

	dec, err := message.ToMessage(data)
	if err != nil {
		t.Fatal("ToMessage:", err)
	}

	if dec.From != orig.From {
		t.Errorf("From = %q, want %q", dec.From, orig.From)
	}
	if dec.To != orig.To {
		t.Errorf("To = %q, want %q", dec.To, orig.To)
	}
	if dec.Content != orig.Content {
		t.Errorf("Content = %q, want %q", dec.Content, orig.Content)
	}
	if dec.Time != orig.Time {
		t.Errorf("Time = %q, want %q", dec.Time, orig.Time)
	}
}

func TestToMessageInvalid(t *testing.T) {
	_, err := message.ToMessage([]byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestNewMessageIDEmpty(t *testing.T) {
	m := message.NewMessage("alice", "bob", "hello")
	if m.ID != "" {
		t.Errorf("ID = %q, want empty", m.ID)
	}
}

func TestEncodeDecodePreservesID(t *testing.T) {
	m := message.NewMessage("a", "b", "c")
	m.ID = "my-custom-id"
	data, err := m.Encode()
	if err != nil {
		t.Fatal("Encode:", err)
	}

	dec, err := message.ToMessage(data)
	if err != nil {
		t.Fatal("ToMessage:", err)
	}
	if dec.ID != "my-custom-id" {
		t.Errorf("ID = %q, want %q", dec.ID, "my-custom-id")
	}
}

func TestMessageIDOmitEmpty(t *testing.T) {
	m := &message.Message{From: "a", To: "b", Content: "c"}
	data, _ := m.Encode()
	if idStr := `"id"`; strings.Contains(string(data), idStr) {
		t.Error("JSON should not contain 'id' when empty")
	}
}

func TestMessagePutGeneratesID(t *testing.T) {
	dir := "test_put_genid"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	msg := message.NewMessage("alice", "bob", "hello")
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}
	if msg.ID == "" {
		t.Fatal("Put should set msg.ID when empty")
	}
	if msg.ID != id {
		t.Errorf("msg.ID = %q, want %q", msg.ID, id)
	}
}

func TestMessagePutReusesID(t *testing.T) {
	dir := "test_put_reuseid"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	msg := message.NewMessage("alice", "bob", "custom id test")
	msg.ID = "pre-set-id"
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}
	if id != "pre-set-id" {
		t.Errorf("id = %q, want %q", id, "pre-set-id")
	}

	got, err := message.Get(s, "secret", "pre-set-id")
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "custom id test" {
		t.Errorf("Content = %q, want %q", got.Content, "custom id test")
	}
}

func TestMessagePutWithIDThenUpdate(t *testing.T) {
	dir := "test_putid_upd"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	msg := message.NewMessage("alice", "bob", "original")
	msg.ID = "fixed-id"
	message.Put(s, "secret", msg)

	msg.Content = "updated"
	err := message.Update(s, "secret", "fixed-id", msg)
	if err != nil {
		t.Fatal("Update:", err)
	}

	got, err := message.Get(s, "secret", "fixed-id")
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "updated" {
		t.Errorf("Content = %q, want %q", got.Content, "updated")
	}
}

func TestEncodeDecodeEmpty(t *testing.T) {
	m := &message.Message{}
	data, err := m.Encode()
	if err != nil {
		t.Fatal("Encode:", err)
	}
	dec, err := message.ToMessage(data)
	if err != nil {
		t.Fatal("ToMessage:", err)
	}
	if dec.From != "" || dec.To != "" || dec.Content != "" || dec.Time != "" {
		t.Error("empty message fields should be preserved")
	}
}
