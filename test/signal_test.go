package test_test

import (
	"encoding/json"
	"testing"

	"github.com/JspBack/end-to-end-chat/signal"
)

func TestSignalDeleteType(t *testing.T) {
	if signal.TypeDelete != "delete" {
		t.Errorf("TypeDelete = %q, want %q", signal.TypeDelete, "delete")
	}
	if signal.TypeUpdate != "update" {
		t.Errorf("TypeUpdate = %q, want %q", signal.TypeUpdate, "update")
	}
}

func TestNewDeleteSignal(t *testing.T) {
	data := signal.New(signal.TypeDelete, "alice", "msg-123", "")
	var s struct {
		Type string `json:"type"`
		From string `json:"from"`
		ID   string `json:"id"`
	}
	if err := json.Unmarshal(data, &s); err != nil {
		t.Fatal("unmarshal:", err)
	}
	if s.Type != "delete" {
		t.Errorf("Type = %q, want %q", s.Type, "delete")
	}
	if s.From != "alice" {
		t.Errorf("From = %q, want %q", s.From, "alice")
	}
	if s.ID != "msg-123" {
		t.Errorf("ID = %q, want %q", s.ID, "msg-123")
	}
}

func TestNewUpdateSignal(t *testing.T) {
	data := signal.New(signal.TypeUpdate, "alice", "msg-456", "new content")
	parsed, err := signal.Parse(data)
	if err != nil {
		t.Fatal("Parse:", err)
	}
	if parsed.Type != "update" {
		t.Errorf("Type = %q, want %q", parsed.Type, "update")
	}
	if parsed.From != "alice" {
		t.Errorf("From = %q, want %q", parsed.From, "alice")
	}
	if parsed.ID != "msg-456" {
		t.Errorf("ID = %q, want %q", parsed.ID, "msg-456")
	}
	if parsed.Content != "new content" {
		t.Errorf("Content = %q, want %q", parsed.Content, "new content")
	}
}

func TestParseInvalid(t *testing.T) {
	_, err := signal.Parse([]byte(`{invalid`))
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestParseEmpty(t *testing.T) {
	s, err := signal.Parse([]byte(`{}`))
	if err != nil {
		t.Fatal("Parse:", err)
	}
	if s.Type != "" {
		t.Errorf("Type = %q, want empty", s.Type)
	}
}

func TestParseNil(t *testing.T) {
	_, err := signal.Parse(nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
}
