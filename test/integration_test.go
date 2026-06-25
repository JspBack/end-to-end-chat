package test_test

import (
	"os"
	"strings"
	"testing"

	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
)

func TestMessageStoreRoundTrip(t *testing.T) {
	dir := "test_msg_rt"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	msg := message.NewMessage("alice", "bob", "hello from alice")
	id, err := message.Put(s, "some-secret", msg)
	if err != nil {
		t.Fatal("message.Put:", err)
	}

	got, err := message.Get(s, "some-secret", id)
	if err != nil {
		t.Fatal("message.Get:", err)
	}

	if got.From != "alice" {
		t.Errorf("From = %q, want %q", got.From, "alice")
	}
	if got.To != "bob" {
		t.Errorf("To = %q, want %q", got.To, "bob")
	}
	if got.Content != "hello from alice" {
		t.Errorf("Content = %q, want %q", got.Content, "hello from alice")
	}
	if got.Time != msg.Time {
		t.Errorf("Time = %q, want %q", got.Time, msg.Time)
	}
}

func TestMessageStoreListIncludesTimestamp(t *testing.T) {
	dir := "test_msg_list"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	message.Put(s, "secret", message.NewMessage("a", "b", "first"))
	message.Put(s, "secret", message.NewMessage("b", "a", "second"))

	list, err := s.Chats.List()
	if err != nil {
		t.Fatal("List:", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(list))
	}

	for _, item := range list {
		if item.CreatedAt == "" {
			t.Errorf("message %q has no created_at", item.ID)
		}
	}
}

func TestMessageStoreGetWithTimestamp(t *testing.T) {
	dir := "test_msg_ts"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	msg := message.NewMessage("alice", "bob", "timed message")
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, "secret", id)
	if err != nil {
		t.Fatal("Get:", err)
	}

	if got.Time == "" {
		t.Error("message.Time should not be empty")
	}
}

func TestMessageUpdate(t *testing.T) {
	dir := "test_msg_update"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	msg := message.NewMessage("alice", "bob", "original")
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	updated := message.NewMessage("alice", "bob", "updated")
	err = message.Update(s, "secret", id, updated)
	if err != nil {
		t.Fatal("Update:", err)
	}

	got, err := message.Get(s, "secret", id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "updated" {
		t.Errorf("Content = %q, want %q", got.Content, "updated")
	}
}

func TestMessageDelete(t *testing.T) {
	dir := "test_msg_delete"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	msg := message.NewMessage("alice", "bob", "delete me")
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	err = message.Delete(s, id)
	if err != nil {
		t.Fatal("Delete:", err)
	}

	_, err = message.Get(s, "secret", id)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestMessageUpdateOverwrites(t *testing.T) {
	dir := "test_msg_updover"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	msg1 := message.NewMessage("a", "b", "first")
	id, _ := message.Put(s, "secret", msg1)

	msg2 := message.NewMessage("a", "b", "second")
	err := message.Update(s, "secret", id, msg2)
	if err != nil {
		t.Fatal("Update:", err)
	}

	got, err := message.Get(s, "secret", id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "second" {
		t.Errorf("Content = %q, want %q", got.Content, "second")
	}
}

func TestMessageUnicode(t *testing.T) {
	dir := "test_unicode"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	content := "Hello 世界 👋"
	msg := message.NewMessage("alice", "bob", content)
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, "secret", id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != content {
		t.Errorf("Content = %q, want %q", got.Content, content)
	}
}

func TestMessageEmptyContent(t *testing.T) {
	dir := "test_emptycontent"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	msg := message.NewMessage("alice", "bob", "")
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, "secret", id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "" {
		t.Errorf("Content = %q, want empty", got.Content)
	}
}

func TestMessageLargeContent(t *testing.T) {
	dir := "test_large"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	content := strings.Repeat("A", 10000)
	msg := message.NewMessage("alice", "bob", content)
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, "secret", id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(got.Content) != 10000 {
		t.Errorf("Content length = %d, want 10000", len(got.Content))
	}
}

func TestMessageSpecialChars(t *testing.T) {
	dir := "test_specialchars"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	content := "tab\tnewline\nquote\"backslash\\end"
	msg := message.NewMessage("a", "b", content)
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, "secret", id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != content {
		t.Errorf("Content = %q, want %q", got.Content, content)
	}
}
