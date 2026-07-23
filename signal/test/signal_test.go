package signal_test

import (
	"testing"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/signal"
)

func keyFromStr(s string) keys.Key {
	var k keys.Key
	copy(k[:], []byte(s))
	return k
}

var aliceKey = keyFromStr("alice_has_a_very_secure_key_!")
var bobKey = keyFromStr("bob_has_a_very_secure_key_!!!")

func TestSignalDeleteType(t *testing.T) {
	if signal.Delete != "delete" {
		t.Errorf("TypeDelete = %q, want %q", signal.Delete, "delete")
	}
	if signal.Update != "update" {
		t.Errorf("TypeUpdate = %q, want %q", signal.Update, "update")
	}
}

func TestNewDeleteSignal(t *testing.T) {
	data, err := signal.New(signal.Delete, aliceKey, uuid.Nil, nil)
	if err != nil {
		t.Fatal("New:", err)
	}
	parsed, err := signal.Parse(data)
	if err != nil {
		t.Fatal("Parse:", err)
	}
	if parsed.Type != signal.Delete {
		t.Errorf("Type = %q, want %q", parsed.Type, signal.Delete)
	}
	if parsed.From != aliceKey {
		t.Errorf("From = %v, want %v", parsed.From, aliceKey)
	}
	if parsed.ID != uuid.Nil {
		t.Errorf("ID = %v, want %v", parsed.ID, uuid.Nil)
	}
}

func TestNewUpdateSignal(t *testing.T) {
	id := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	data, err := signal.New(signal.Update, aliceKey, id, []byte("new content"))
	if err != nil {
		t.Fatal("New:", err)
	}
	parsed, err := signal.Parse(data)
	if err != nil {
		t.Fatal("Parse:", err)
	}
	if parsed.Type != signal.Update {
		t.Errorf("Type = %q, want %q", parsed.Type, signal.Update)
	}
	if parsed.From != aliceKey {
		t.Errorf("From = %v, want %v", parsed.From, aliceKey)
	}
	if parsed.ID != id {
		t.Errorf("ID = %v, want %v", parsed.ID, id)
	}
	if string(parsed.Content) != "new content" {
		t.Errorf("Content = %q, want %q", string(parsed.Content), "new content")
	}
}

func TestParseInvalid(t *testing.T) {
	_, err := signal.Parse([]byte{0x01, 0x00, 0x01, 0x00})
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func TestParseEmpty(t *testing.T) {
	_, err := signal.Parse([]byte{0x01, 0x00, 0x00})
	if err == nil {
		t.Error("expected error for empty type")
	}
}

func TestParseNil(t *testing.T) {
	_, err := signal.Parse(nil)
	if err == nil {
		t.Error("expected error for nil input")
	}
}

func TestFileMetaConstant(t *testing.T) {
	if signal.FileMeta != "file_meta" {
		t.Errorf("FileMeta = %q, want %q", signal.FileMeta, "file_meta")
	}
}

func TestNewFileMetaSignal(t *testing.T) {
	id := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	meta := `{"name":"photo.jpg","size":12345,"mime":"image/jpeg"}`
	data, err := signal.New(signal.FileMeta, aliceKey, id, []byte(meta))
	if err != nil {
		t.Fatal("New:", err)
	}
	parsed, err := signal.Parse(data)
	if err != nil {
		t.Fatal("Parse:", err)
	}
	if parsed.Type != signal.FileMeta {
		t.Errorf("Type = %q, want %q", parsed.Type, signal.FileMeta)
	}
	if parsed.From != aliceKey {
		t.Errorf("From = %v, want %v", parsed.From, aliceKey)
	}
	if parsed.ID != id {
		t.Errorf("ID = %v, want %v", parsed.ID, id)
	}
	if string(parsed.Content) != meta {
		t.Errorf("Content = %q, want %q", string(parsed.Content), meta)
	}
}

func TestFileMetaRoundTrip(t *testing.T) {
	id := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	meta := `{"name":"doc.pdf","size":999,"mime":"application/pdf"}`
	data, err := signal.New(signal.FileMeta, bobKey, id, []byte(meta))
	if err != nil {
		t.Fatal("New:", err)
	}
	parsed, err := signal.Parse(data)
	if err != nil {
		t.Fatal("Parse:", err)
	}
	if parsed.Type != signal.FileMeta {
		t.Errorf("Type = %q, want %q", parsed.Type, signal.FileMeta)
	}
	if parsed.From != bobKey {
		t.Errorf("From = %v, want %v", parsed.From, bobKey)
	}
	if parsed.ID != id {
		t.Errorf("ID = %v, want %v", parsed.ID, id)
	}
	if string(parsed.Content) != meta {
		t.Errorf("Content = %q, want %q", string(parsed.Content), meta)
	}
}
