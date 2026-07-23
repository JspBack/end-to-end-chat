package message_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/message"
	"github.com/JspBack/end-to-end-chat/store"
)

var testSecret = func() keys.Key {
	k, _ := keys.FromHex("abcdef0102030405060708091011121314151617181920212223242526272829303132")
	return k
}()

func TestNewMessage(t *testing.T) {
	m := message.NewMessage("bob", "hello")
	if m.To != "bob" {
		t.Errorf("To = %q, want bob", m.To)
	}
	if m.Content != "hello" {
		t.Errorf("Content = %q, want hello", m.Content)
	}
	if m.Time.IsZero() {
		t.Error("Time is zero")
	}
}

func TestEncodeDecode(t *testing.T) {
	orig := message.NewMessage("bob", "hello")
	data, err := orig.Encode()
	if err != nil {
		t.Fatal("Encode:", err)
	}

	dec, err := message.ToMessage(data)
	if err != nil {
		t.Fatal("ToMessage:", err)
	}

	if dec.To != orig.To {
		t.Errorf("To = %q, want %q", dec.To, orig.To)
	}
	if dec.Content != orig.Content {
		t.Errorf("Content = %q, want %q", dec.Content, orig.Content)
	}
	if !dec.Time.Equal(orig.Time) {
		t.Errorf("Time = %v, want %v", dec.Time, orig.Time)
	}
}

func TestToMessageInvalid(t *testing.T) {
	_, err := message.ToMessage([]byte{0x00, 0x01})
	if err == nil {
		t.Error("expected error for invalid data")
	}
}

func TestNewMessageIDNotEmpty(t *testing.T) {
	m := message.NewMessage("bob", "hello")
	if m.ID == uuid.Nil {
		t.Error("NewMessage should set a non-nil ID")
	}
}

func TestEncodeDecodePreservesID(t *testing.T) {
	m := message.NewMessage("b", "c")
	customID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	m.ID = customID
	data, err := m.Encode()
	if err != nil {
		t.Fatal("Encode:", err)
	}

	dec, err := message.ToMessage(data)
	if err != nil {
		t.Fatal("ToMessage:", err)
	}
	if dec.ID != customID {
		t.Errorf("ID = %v, want %v", dec.ID, customID)
	}
}

func TestMessageEncodeDecodePartial(t *testing.T) {
	now := time.Now().UTC()
	m := &message.Message{To: "b", Content: "c", Time: now, CreatedAt: now, UpdatedAt: now}
	data, err := m.Encode()
	if err != nil {
		t.Fatal("Encode:", err)
	}
	dec, err := message.ToMessage(data)
	if err != nil {
		t.Fatal("ToMessage:", err)
	}
	if dec.To != "b" || dec.Content != "c" {
		t.Errorf("got To=%q Content=%q, want To=b Content=c", dec.To, dec.Content)
	}
}

func TestMessagePutGeneratesID(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_put_genid")
	s := store.New(dir)

	msg := message.NewMessage("bob", "hello")
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}
	if id == uuid.Nil {
		t.Fatal("expected non-empty id")
	}
	if msg.ID == uuid.Nil {
		t.Fatal("Put should have a non-nil msg.ID")
	}
	if msg.ID != id {
		t.Errorf("msg.ID = %q, want %q", msg.ID, id)
	}
}

func TestMessagePutReusesID(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_put_reuseid")
	s := store.New(dir)

	customID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	msg := message.NewMessage("bob", "custom id test")
	msg.ID = customID
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}
	if id != customID {
		t.Errorf("id = %q, want %q", id, customID)
	}

	got, err := message.Get(s, testSecret, customID)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "custom id test" {
		t.Errorf("Content = %q, want %q", got.Content, "custom id test")
	}
}

func TestMessagePutWithIDThenUpdate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_putid_upd")
	s := store.New(dir)

	fixedID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	msg := message.NewMessage("bob", "original")
	msg.ID = fixedID
	message.Put(s, testSecret, "alice", msg)

	msg.Content = "updated"
	err := message.Update(s, testSecret, fixedID, "alice", msg)
	if err != nil {
		t.Fatal("Update:", err)
	}

	got, err := message.Get(s, testSecret, fixedID)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "updated" {
		t.Errorf("Content = %q, want %q", got.Content, "updated")
	}
}

func TestEncodeDecodeEmpty(t *testing.T) {
	now := time.Now().UTC()
	m := &message.Message{Time: now, CreatedAt: now, UpdatedAt: now}
	data, err := m.Encode()
	if err != nil {
		t.Fatal("Encode:", err)
	}
	dec, err := message.ToMessage(data)
	if err != nil {
		t.Fatal("ToMessage:", err)
	}
	if dec.To != "" || dec.Content != "" {
		t.Error("empty message fields should be preserved")
	}
}

func TestMessageStoreRoundTrip(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_msg_rt")
	s := store.New(dir)

	msg := message.NewMessage("bob", "hello from alice")
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("message.Put:", err)
	}

	got, err := message.Get(s, testSecret, id)
	if err != nil {
		t.Fatal("message.Get:", err)
	}

	if got.To != "bob" {
		t.Errorf("To = %q, want %q", got.To, "bob")
	}
	if got.Content != "hello from alice" {
		t.Errorf("Content = %q, want %q", got.Content, "hello from alice")
	}
	if !got.Time.Equal(msg.Time) {
		t.Errorf("Time = %v, want %v", got.Time, msg.Time)
	}
}

func TestMessageStoreListIncludesTimestamp(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_msg_list")
	s := store.New(dir)

	message.Put(s, testSecret, "a", message.NewMessage("b", "first"))
	message.Put(s, testSecret, "b", message.NewMessage("a", "second"))

	list, err := s.Chats.List()
	if err != nil {
		t.Fatal("List:", err)
	}

	if len(list) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(list))
	}

	for _, item := range list {
		if item.CreatedAt.IsZero() {
			t.Errorf("message %q has zero created_at", item.ID)
		}
	}
}

func TestMessageStoreGetWithTimestamp(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_msg_ts")
	s := store.New(dir)

	msg := message.NewMessage("bob", "timed message")
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, testSecret, id)
	if err != nil {
		t.Fatal("Get:", err)
	}

	if got.Time.IsZero() {
		t.Error("message.Time should not be zero")
	}
}

func TestMessageUpdate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_msg_update")
	s := store.New(dir)

	msg := message.NewMessage("bob", "original")
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	updated := message.NewMessage("bob", "updated")
	err = message.Update(s, testSecret, id, "alice", updated)
	if err != nil {
		t.Fatal("Update:", err)
	}

	got, err := message.Get(s, testSecret, id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "updated" {
		t.Errorf("Content = %q, want %q", got.Content, "updated")
	}
}

func TestMessageDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_msg_delete")
	s := store.New(dir)

	msg := message.NewMessage("bob", "delete me")
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	err = message.Delete(s, testSecret, id)
	if err != nil {
		t.Fatal("Delete:", err)
	}

	_, err = message.Get(s, testSecret, id)
	if err == nil {
		t.Error("expected error after delete")
	}
}

func TestMessageUpdateOverwrites(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_msg_updover")
	s := store.New(dir)

	msg1 := message.NewMessage("b", "first")
	id, _ := message.Put(s, testSecret, "a", msg1)

	msg2 := message.NewMessage("b", "second")
	err := message.Update(s, testSecret, id, "a", msg2)
	if err != nil {
		t.Fatal("Update:", err)
	}

	got, err := message.Get(s, testSecret, id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "second" {
		t.Errorf("Content = %q, want %q", got.Content, "second")
	}
}

func TestMessageUnicode(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_unicode")
	s := store.New(dir)

	content := "Hello 世界 👋"
	msg := message.NewMessage("bob", content)
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, testSecret, id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != content {
		t.Errorf("Content = %q, want %q", got.Content, content)
	}
}

func TestMessageEmptyContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_emptycontent")
	s := store.New(dir)

	msg := message.NewMessage("bob", "")
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, testSecret, id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != "" {
		t.Errorf("Content = %q, want empty", got.Content)
	}
}

func TestMessageLargeContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_large")
	s := store.New(dir)

	content := strings.Repeat("A", 10000)
	msg := message.NewMessage("bob", content)
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, testSecret, id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(got.Content) != 10000 {
		t.Errorf("Content length = %d, want 10000", len(got.Content))
	}
}

func TestSearchBasic(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_search_basic")
	s := store.New(dir)

	message.Put(s, testSecret, "alice", message.NewMessage("bob", "hello world"))
	message.Put(s, testSecret, "bob", message.NewMessage("alice", "how are you"))
	message.Put(s, testSecret, "alice", message.NewMessage("bob", "world of go"))

	results, err := message.Search(s, testSecret, "world", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 results for 'world', got %d", len(results))
	}
}

func TestSearchNoMatch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_search_nomatch")
	s := store.New(dir)

	message.Put(s, testSecret, "alice", message.NewMessage("bob", "hello"))

	results, err := message.Search(s, testSecret, "zzzz", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestSearchLimit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_search_limit")
	s := store.New(dir)

	message.Put(s, testSecret, "a", message.NewMessage("b", "match one"))
	message.Put(s, testSecret, "c", message.NewMessage("d", "match two"))

	results, err := message.Search(s, testSecret, "match", 1)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 1 {
		t.Errorf("expected 1 result with limit 1, got %d", len(results))
	}
}

func TestSearchCaseInsensitive(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_search_case")
	s := store.New(dir)

	message.Put(s, testSecret, "Alice", message.NewMessage("Bob", "HELLO World"))

	results, err := message.Search(s, testSecret, "hello", 10)
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
	dir := filepath.Join(t.TempDir(), "test_search_from")
	s := store.New(dir)

	message.Put(s, testSecret, "alice", message.NewMessage("bob", "hi"))
	message.Put(s, testSecret, "charlie", message.NewMessage("bob", "hello"))

	results, err := message.Search(s, testSecret, "hi", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Content != "hi" {
		t.Errorf("Content = %q, want %q", results[0].Content, "hi")
	}
}

func TestSearchEmptyQuery(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_search_emptyq")
	s := store.New(dir)

	results, err := message.Search(s, testSecret, "", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if results != nil {
		t.Errorf("expected nil for empty query, got %v", results)
	}
}

func TestAttachmentStoreAndRetrieve(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_attachment")
	s := store.New(dir)

	att := message.Attachment{
		Name:     "test.txt",
		MIMEType: "text/plain",
		Data:     []byte("hello world"),
	}
	msg := message.NewMessage("bob", "file", att)
	msgID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	msg.ID = msgID

	if err := message.StoreAttachments(s, testSecret, msg.ID.String(), msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}

	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, testSecret, id)
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
	if got.Attachments[0].ID == uuid.Nil {
		t.Fatal("attachment ID should not be nil")
	}

	raw, err := s.Files.Get(testSecret, got.Attachments[0].ID)
	if err != nil {
		t.Fatal("Files.Get:", err)
	}
	if string(raw) != "hello world" {
		t.Errorf("file content = %q, want %q", string(raw), "hello world")
	}
}

func TestSearchAttachmentName(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_search_attname")
	s := store.New(dir)

	att := message.Attachment{
		Name: "report.pdf",
		Data: []byte("pdf data"),
	}
	msg := message.NewMessage("bob", "here it is", att)
	msg.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	var err error
	if err = message.StoreAttachments(s, testSecret, msg.ID.String(), msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}
	if _, err = message.Put(s, testSecret, "alice", msg); err != nil {
		t.Fatal("Put:", err)
	}

	results, err := message.Search(s, testSecret, "report", 10)
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
	dir := filepath.Join(t.TempDir(), "test_delete_files")
	s := store.New(dir)

	att := message.Attachment{
		Name: "photo.jpg",
		Data: []byte("image data"),
	}
	msg := message.NewMessage("bob", "pic", att)
	msg.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	if err := message.StoreAttachments(s, testSecret, msg.ID.String(), msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}
	id, err := message.Put(s, testSecret, "alice", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	fileID := msg.Attachments[0].ID
	raw, err := s.Files.Get(testSecret, fileID)
	if err != nil {
		t.Fatalf("Files.Get before delete: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("expected non-empty file data")
	}

	if err = message.Delete(s, testSecret, id); err != nil {
		t.Fatal("Delete:", err)
	}

	if _, err = s.Files.Get(testSecret, fileID); !os.IsNotExist(err) {
		t.Errorf("expected file to be deleted, got err=%v", err)
	}
}

func TestMultipleAttachments(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_multi_att")
	s := store.New(dir)

	att1 := message.Attachment{
		Name: "doc.txt",
		Data: []byte("text content"),
	}
	att2 := message.Attachment{
		Name: "img.png",
		Data: []byte("png data"),
	}
	msg := message.NewMessage("bob", "two files", att1, att2)
	msg.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	if err := message.StoreAttachments(s, testSecret, msg.ID.String(), msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}
	var putErr error
	if _, putErr = message.Put(s, testSecret, "alice", msg); putErr != nil {
		t.Fatal("Put:", putErr)
	}

	got, err := message.Get(s, testSecret, msg.ID)
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
		raw, err = s.Files.Get(testSecret, a.ID)
		if err != nil {
			t.Fatalf("Files.Get(%q): %v", a.ID.String(), err)
		}
		if len(raw) == 0 {
			t.Errorf("empty file data for %q", a.ID.String())
		}
	}
}

func TestStoreAttachmentsEmptyData(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_empty_attdata")
	s := store.New(dir)

	att := message.Attachment{Name: "empty.bin"}
	msg := message.NewMessage("bob", "empty att", att)
	msg.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
	if err := message.StoreAttachments(s, testSecret, msg.ID.String(), msg.Attachments); err != nil {
		t.Fatal("StoreAttachments:", err)
	}
	if _, err := message.Put(s, testSecret, "alice", msg); err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, testSecret, msg.ID)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(got.Attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(got.Attachments))
	}
	if got.Attachments[0].Name != "empty.bin" {
		t.Errorf("Name = %q", got.Attachments[0].Name)
	}
	if got.Attachments[0].ID != uuid.Nil {
		t.Errorf("expected nil ID for empty-data attachment, got %v", got.Attachments[0].ID)
	}
}

func TestSpecialChars(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_specialchars")
	s := store.New(dir)

	content := "tab\tnewline\nquote\"backslash\\end"
	msg := message.NewMessage("b", content)
	id, err := message.Put(s, testSecret, "a", msg)
	if err != nil {
		t.Fatal("Put:", err)
	}

	got, err := message.Get(s, testSecret, id)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got.Content != content {
		t.Errorf("Content = %q, want %q", got.Content, content)
	}
}
