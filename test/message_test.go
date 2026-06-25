package test_test

import (
	"testing"

	"github.com/JspBack/end-to-end-chat/message"
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
