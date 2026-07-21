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

	err = message.Delete(s, "secret", id)
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

func TestSearchBasic(t *testing.T) {
	dir := "test_search_basic"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	message.Put(s, "secret", message.NewMessage("alice", "bob", "hello world"))
	message.Put(s, "secret", message.NewMessage("bob", "alice", "how are you"))
	message.Put(s, "secret", message.NewMessage("alice", "bob", "world of go"))

	results, err := message.Search(s, "secret", "world", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'world', got %d", len(results))
	}
}

func TestSearchNoMatch(t *testing.T) {
	dir := "test_search_nomatch"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	message.Put(s, "secret", message.NewMessage("alice", "bob", "hello"))

	results, err := message.Search(s, "secret", "zzzz", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchLimit(t *testing.T) {
	dir := "test_search_limit"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	message.Put(s, "secret", message.NewMessage("a", "b", "match one"))
	message.Put(s, "secret", message.NewMessage("c", "d", "match two"))

	results, err := message.Search(s, "secret", "match", 1)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result with limit 1, got %d", len(results))
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	dir := "test_search_case"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	message.Put(s, "secret", message.NewMessage("Alice", "Bob", "HELLO World"))

	results, err := message.Search(s, "secret", "hello", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "HELLO World" {
		t.Errorf("Content = %q, want %q", results[0].Content, "HELLO World")
	}
}

func TestSearchMatchesFrom(t *testing.T) {
	dir := "test_search_from"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	message.Put(s, "secret", message.NewMessage("alice", "bob", "hi"))
	message.Put(s, "secret", message.NewMessage("charlie", "bob", "hello"))

	results, err := message.Search(s, "secret", "alice", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].From != "alice" {
		t.Errorf("From = %q, want %q", results[0].From, "alice")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	dir := "test_search_emptyq"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	results, err := message.Search(s, "secret", "", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if results != nil {
		t.Errorf("expected nil for empty query, got %v", results)
	}
}

func TestAttachmentStoreAndRetrieve(t *testing.T) {
	dir := "test_attachment"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	att := message.Attachment{
		Name:     "test.txt",
		MIMEType: "text/plain",
		Data:     []byte("hello world"),
	}
	msg := message.NewMessage("alice", "bob", "file", att)
	msg.ID = "att-test-msg-id"

	if err := message.StoreAttachments(s, "secret", msg.ID, msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}

	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, "secret", id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(got.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(got.Attachments))
	}
	if got.Attachments[0].Name != "test.txt" {
		t.Errorf("Name = %q, want %q", got.Attachments[0].Name, "test.txt")
	}
	if len(got.Attachments[0].Data) != 0 {
		t.Errorf("Data should be nil after storage, got %v", got.Attachments[0].Data)
	}
	if got.Attachments[0].ID == "" {
		t.Fatal("attachment ID should not be empty")
	}

	raw, err := s.Files.Get("secret", got.Attachments[0].ID)
	if err != nil {
		t.Fatal("Files.Get:", err)
	}
	if string(raw) != "hello world" {
		t.Errorf("file content = %q, want %q", string(raw), "hello world")
	}
}

func TestSearchAttachmentName(t *testing.T) {
	dir := "test_search_attname"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	att := message.Attachment{
		Name: "report.pdf",
		Data: []byte("pdf data"),
	}
	msg := message.NewMessage("alice", "bob", "here it is", att)
	msg.ID = "search-att-msg"
	var err error
	if err = message.StoreAttachments(s, "secret", msg.ID, msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}
	if _, err = message.Put(s, "secret", msg); err != nil {
		t.Fatal("Put:", err)
	}

	results, err := message.Search(s, "secret", "report", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result for 'report', got %d", len(results))
	}
	if results[0].Content != "here it is" {
		t.Errorf("Content = %q, want %q", results[0].Content, "here it is")
	}
}

func TestMessageDeleteRemovesFiles(t *testing.T) {
	dir := "test_delete_files"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	att := message.Attachment{
		Name: "photo.jpg",
		Data: []byte("image data"),
	}
	msg := message.NewMessage("alice", "bob", "pic", att)
	msg.ID = "delete-files-msg"
	if err := message.StoreAttachments(s, "secret", msg.ID, msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}
	id, err := message.Put(s, "secret", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	files, err := s.Files.ListByMessage(id)
	if err != nil {
		t.Fatal("ListByMessage:", err)
	}
	if len(files) != 1 {
		t.Fatalf("expected 1 file, got %d", len(files))
	}

	if err = message.Delete(s, "secret", id); err != nil {
		t.Fatal("Delete:", err)
	}

	files, err = s.Files.ListByMessage(id)
	if err != nil {
		t.Fatal("ListByMessage after delete:", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files after delete, got %d", len(files))
	}
}

func TestMultipleAttachments(t *testing.T) {
	dir := "test_multi_att"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	att1 := message.Attachment{
		Name: "doc.txt",
		Data: []byte("text content"),
	}
	att2 := message.Attachment{
		Name: "img.png",
		Data: []byte("png data"),
	}
	msg := message.NewMessage("alice", "bob", "two files", att1, att2)
	msg.ID = "multi-att-msg"
	if err := message.StoreAttachments(s, "secret", msg.ID, msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}
	var putErr error
	if _, putErr = message.Put(s, "secret", msg); putErr != nil {
		t.Fatal("Put:", putErr)
	}

	got, err := message.Get(s, "secret", msg.ID)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(got.Attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(got.Attachments))
	}
	if got.Attachments[0].Name != "doc.txt" || got.Attachments[1].Name != "img.png" {
		t.Errorf("attachment names mismatch")
	}
	for _, a := range got.Attachments {
		var raw []byte
		raw, err = s.Files.Get("secret", a.ID)
		if err != nil {
			t.Fatalf("Files.Get(%q): %v", a.ID, err)
		}
		if len(raw) == 0 {
			t.Errorf("empty file data for %q", a.ID)
		}
	}
}

func TestStoreAttachmentsEmptyData(t *testing.T) {
	dir := "test_empty_attdata"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	att := message.Attachment{Name: "empty.bin"}
	msg := message.NewMessage("alice", "bob", "empty att", att)
	msg.ID = "empty-att-msg"
	if err := message.StoreAttachments(s, "secret", msg.ID, msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}
	if _, err := message.Put(s, "secret", msg); err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, "secret", msg.ID)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(got.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(got.Attachments))
	}
	if got.Attachments[0].Name != "empty.bin" {
		t.Errorf("Name = %q", got.Attachments[0].Name)
	}
	files, err := s.Files.ListByMessage(msg.ID)
	if err != nil {
		t.Fatal("ListByMessage:", err)
	}
	if len(files) != 0 {
		t.Errorf("expected 0 files for empty data, got %d", len(files))
	}
}

func TestSpecialChars(t *testing.T) {
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
