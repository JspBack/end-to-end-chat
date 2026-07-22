package signal_test

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
	data := signal.New(signal.TypeDelete, "alice", "msg-123", nil)
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
	data := signal.New(signal.TypeUpdate, "alice", "msg-456", []byte("new content"))
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
	if string(parsed.Content) != "new content" {
		t.Errorf("Content = %q, want %q", string(parsed.Content), "new content")
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

func TestTypeFileMetaConstant(t *testing.T) {
	if signal.TypeFileMeta != "file_meta" {
		t.Errorf("TypeFileMeta = %q, want %q", signal.TypeFileMeta, "file_meta")
	}
}

func TestNewFileMetaSignal(t *testing.T) {
	meta := `{"name":"photo.jpg","size":12345,"mime":"image/jpeg"}`
	data := signal.New(signal.TypeFileMeta, "alice", "file-uuid-1", []byte(meta))
	parsed, err := signal.Parse(data)
	if err != nil {
		t.Fatal("Parse:", err)
	}
	if parsed.Type != "file_meta" {
		t.Errorf("Type = %q, want %q", parsed.Type, "file_meta")
	}
	if parsed.From != "alice" {
		t.Errorf("From = %q, want %q", parsed.From, "alice")
	}
	if parsed.ID != "file-uuid-1" {
		t.Errorf("ID = %q, want %q", parsed.ID, "file-uuid-1")
	}
	if string(parsed.Content) != meta {
		t.Errorf("Content = %q, want %q", string(parsed.Content), meta)
	}
}

func TestTypeFileMetaNewRoundTrip(t *testing.T) {
	meta := `{"name":"doc.pdf","size":999,"mime":"application/pdf"}`
	data := signal.New(signal.TypeFileMeta, "bob", "abc-123", []byte(meta))
	parsed, err := signal.Parse(data)
	if err != nil {
		t.Fatal("Parse:", err)
	}
	if parsed.Type != signal.TypeFileMeta {
		t.Errorf("Type = %q, want %q", parsed.Type, signal.TypeFileMeta)
	}
	if parsed.From != "bob" {
		t.Errorf("From = %q, want %q", parsed.From, "bob")
	}
	if parsed.ID != "abc-123" {
		t.Errorf("ID = %q, want %q", parsed.ID, "abc-123")
	}
	if string(parsed.Content) != meta {
		t.Errorf("Content = %q, want %q", string(parsed.Content), meta)
	}
}
