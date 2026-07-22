package message_test

// import (
// 	"os"
// 	"path/filepath"
// 	"strings"
// 	"testing"

// 	"github.com/JspBack/end-to-end-chat/message"
// 	"github.com/JspBack/end-to-end-chat/store"
// 	"github.com/google/uuid"
// )

// func TestNewMessage(t *testing.T) {
// 	m := message.NewMessage("bob", "hello")
// 	if m.To != "bob" {
// 		t.Errorf("To = %q, want bob", m.To)
// 	}
// 	if m.Content != "hello" {
// 		t.Errorf("Content = %q, want hello", m.Content)
// 	}
// 	if m.Time == "" {
// 		t.Error("Time is empty")
// 	}
// }

// func TestEncodeDecode(t *testing.T) {
// 	orig := message.NewMessage("bob", "hello")
// 	data, err := orig.Encode()
// 	if err != nil {
// 		t.Fatal("Encode:", err)
// 	}

// 	dec, err := message.ToMessage(data)
// 	if err != nil {
// 		t.Fatal("ToMessage:", err)
// 	}

// 	if dec.To != orig.To {
// 		t.Errorf("To = %q, want %q", dec.To, orig.To)
// 	}
// 	if dec.Content != orig.Content {
// 		t.Errorf("Content = %q, want %q", dec.Content, orig.Content)
// 	}
// 	if dec.Time != orig.Time {
// 		t.Errorf("Time = %q, want %q", dec.Time, orig.Time)
// 	}
// }

// func TestToMessageInvalid(t *testing.T) {
// 	_, err := message.ToMessage([]byte(`{invalid`))
// 	if err == nil {
// 		t.Error("expected error for invalid JSON")
// 	}
// }

// func TestNewMessageIDNotEmpty(t *testing.T) {
// 	m := message.NewMessage("bob", "hello")
// 	if m.ID == uuid.Nil {
// 		t.Error("NewMessage should set a non-nil ID")
// 	}
// }

// func TestEncodeDecodePreservesID(t *testing.T) {
// 	m := message.NewMessage("b", "c")
// 	customID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
// 	m.ID = customID
// 	data, err := m.Encode()
// 	if err != nil {
// 		t.Fatal("Encode:", err)
// 	}

// 	dec, err := message.ToMessage(data)
// 	if err != nil {
// 		t.Fatal("ToMessage:", err)
// 	}
// 	if dec.ID != customID {
// 		t.Errorf("ID = %v, want %v", dec.ID, customID)
// 	}
// }

// func TestMessageIDOmitEmpty(t *testing.T) {
// 	m := &message.Message{To: "b", Content: "c"}
// 	data, _ := m.Encode()
// 	if !strings.Contains(string(data), `"id"`) {
// 		t.Error("JSON should contain 'id' field")
// 	}
// }

// func TestMessagePutGeneratesID(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_put_genid")
// 	s := store.New(dir)

// 	msg := message.NewMessage("bob", "hello")
// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}
// 	if id == "" {
// 		t.Fatal("expected non-empty id")
// 	}
// 	if msg.ID == uuid.Nil {
// 		t.Fatal("Put should have a non-nil msg.ID")
// 	}
// 	if msg.ID.String() != id {
// 		t.Errorf("msg.ID = %q, want %q", msg.ID.String(), id)
// 	}
// }

// func TestMessagePutReusesID(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_put_reuseid")
// 	s := store.New(dir)

// 	customID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
// 	msg := message.NewMessage("bob", "custom id test")
// 	msg.ID = customID
// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}
// 	if id != customID.String() {
// 		t.Errorf("id = %q, want %q", id, customID.String())
// 	}

// 	got, err := message.Get(s, "secret", customID)
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if got.Content != "custom id test" {
// 		t.Errorf("Content = %q, want %q", got.Content, "custom id test")
// 	}
// }

// func TestMessagePutWithIDThenUpdate(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_putid_upd")
// 	s := store.New(dir)

// 	fixedID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
// 	msg := message.NewMessage("bob", "original")
// 	msg.ID = fixedID
// 	message.Put(s, "secret", "alice", msg)

// 	msg.Content = "updated"
// 	err := message.Update(s, "secret", fixedID, "alice", msg)
// 	if err != nil {
// 		t.Fatal("Update:", err)
// 	}

// 	got, err := message.Get(s, "secret", fixedID)
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if got.Content != "updated" {
// 		t.Errorf("Content = %q, want %q", got.Content, "updated")
// 	}
// }

// func TestEncodeDecodeEmpty(t *testing.T) {
// 	m := &message.Message{}
// 	data, err := m.Encode()
// 	if err != nil {
// 		t.Fatal("Encode:", err)
// 	}
// 	dec, err := message.ToMessage(data)
// 	if err != nil {
// 		t.Fatal("ToMessage:", err)
// 	}
// 	if dec.To != "" || dec.Content != "" || dec.Time != "" {
// 		t.Error("empty message fields should be preserved")
// 	}
// }

// func TestMessageStoreRoundTrip(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_msg_rt")
// 	s := store.New(dir)

// 	msg := message.NewMessage("bob", "hello from alice")
// 	id, err := message.Put(s, "some-secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("message.Put:", err)
// 	}

// 	got, err := message.Get(s, "some-secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("message.Get:", err)
// 	}

// 	if got.To != "bob" {
// 		t.Errorf("To = %q, want %q", got.To, "bob")
// 	}
// 	if got.Content != "hello from alice" {
// 		t.Errorf("Content = %q, want %q", got.Content, "hello from alice")
// 	}
// 	if got.Time != msg.Time {
// 		t.Errorf("Time = %q, want %q", got.Time, msg.Time)
// 	}
// }

// func TestMessageStoreListIncludesTimestamp(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_msg_list")
// 	s := store.New(dir)

// 	message.Put(s, "secret", "a", message.NewMessage("b", "first"))
// 	message.Put(s, "secret", "b", message.NewMessage("a", "second"))

// 	list, err := s.Chats.List()
// 	if err != nil {
// 		t.Fatal("List:", err)
// 	}

// 	if len(list) != 2 {
// 		t.Fatalf("expected 2 messages, got %d", len(list))
// 	}

// 	for _, item := range list {
// 		if item.CreatedAt == "" {
// 			t.Errorf("message %q has no created_at", item.ID)
// 		}
// 	}
// }

// func TestMessageStoreGetWithTimestamp(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_msg_ts")
// 	s := store.New(dir)

// 	msg := message.NewMessage("bob", "timed message")
// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	got, err := message.Get(s, "secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}

// 	if got.Time == "" {
// 		t.Error("message.Time should not be empty")
// 	}
// }

// func TestMessageUpdate(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_msg_update")
// 	s := store.New(dir)

// 	msg := message.NewMessage("bob", "original")
// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	updated := message.NewMessage("bob", "updated")
// 	err = message.Update(s, "secret", uuid.MustParse(id), "alice", updated)
// 	if err != nil {
// 		t.Fatal("Update:", err)
// 	}

// 	got, err := message.Get(s, "secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if got.Content != "updated" {
// 		t.Errorf("Content = %q, want %q", got.Content, "updated")
// 	}
// }

// func TestMessageDelete(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_msg_delete")
// 	s := store.New(dir)

// 	msg := message.NewMessage("bob", "delete me")
// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	err = message.Delete(s, "secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("Delete:", err)
// 	}

// 	_, err = message.Get(s, "secret", uuid.MustParse(id))
// 	if err == nil {
// 		t.Error("expected error after delete")
// 	}
// }

// func TestMessageUpdateOverwrites(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_msg_updover")
// 	s := store.New(dir)

// 	msg1 := message.NewMessage("b", "first")
// 	id, _ := message.Put(s, "secret", "a", msg1)

// 	msg2 := message.NewMessage("b", "second")
// 	err := message.Update(s, "secret", uuid.MustParse(id), "a", msg2)
// 	if err != nil {
// 		t.Fatal("Update:", err)
// 	}

// 	got, err := message.Get(s, "secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if got.Content != "second" {
// 		t.Errorf("Content = %q, want %q", got.Content, "second")
// 	}
// }

// func TestMessageUnicode(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_unicode")
// 	s := store.New(dir)

// 	content := "Hello 世界 👋"
// 	msg := message.NewMessage("bob", content)
// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	got, err := message.Get(s, "secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if got.Content != content {
// 		t.Errorf("Content = %q, want %q", got.Content, content)
// 	}
// }

// func TestMessageEmptyContent(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_emptycontent")
// 	s := store.New(dir)

// 	msg := message.NewMessage("bob", "")
// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	got, err := message.Get(s, "secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if got.Content != "" {
// 		t.Errorf("Content = %q, want empty", got.Content)
// 	}
// }

// func TestMessageLargeContent(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_large")
// 	s := store.New(dir)

// 	content := strings.Repeat("A", 10000)
// 	msg := message.NewMessage("bob", content)
// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	got, err := message.Get(s, "secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if len(got.Content) != 10000 {
// 		t.Errorf("Content length = %d, want 10000", len(got.Content))
// 	}
// }

// func TestSearchBasic(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_search_basic")
// 	s := store.New(dir)

// 	message.Put(s, "secret", "alice", message.NewMessage("bob", "hello world"))
// 	message.Put(s, "secret", "bob", message.NewMessage("alice", "how are you"))
// 	message.Put(s, "secret", "alice", message.NewMessage("bob", "world of go"))

// 	results, err := message.Search(s, "secret", "world", 10)
// 	if err != nil {
// 		t.Fatal("Search:", err)
// 	}
// 	if len(results) != 2 {
// 		t.Fatalf("expected 2 results for 'world', got %d", len(results))
// 	}
// }

// func TestSearchNoMatch(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_search_nomatch")
// 	s := store.New(dir)

// 	message.Put(s, "secret", "alice", message.NewMessage("bob", "hello"))

// 	results, err := message.Search(s, "secret", "zzzz", 10)
// 	if err != nil {
// 		t.Fatal("Search:", err)
// 	}
// 	if len(results) != 0 {
// 		t.Errorf("expected 0 results, got %d", len(results))
// 	}
// }

// func TestSearchLimit(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_search_limit")
// 	s := store.New(dir)

// 	message.Put(s, "secret", "a", message.NewMessage("b", "match one"))
// 	message.Put(s, "secret", "c", message.NewMessage("d", "match two"))

// 	results, err := message.Search(s, "secret", "match", 1)
// 	if err != nil {
// 		t.Fatal("Search:", err)
// 	}
// 	if len(results) != 1 {
// 		t.Errorf("expected 1 result with limit 1, got %d", len(results))
// 	}
// }

// func TestSearchCaseInsensitive(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_search_case")
// 	s := store.New(dir)

// 	message.Put(s, "secret", "Alice", message.NewMessage("Bob", "HELLO World"))

// 	results, err := message.Search(s, "secret", "hello", 10)
// 	if err != nil {
// 		t.Fatal("Search:", err)
// 	}
// 	if len(results) != 1 {
// 		t.Fatalf("expected 1 result, got %d", len(results))
// 	}
// 	if results[0].Content != "HELLO World" {
// 		t.Errorf("Content = %q, want %q", results[0].Content, "HELLO World")
// 	}
// }

// func TestSearchMatchesFrom(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_search_from")
// 	s := store.New(dir)

// 	message.Put(s, "secret", "alice", message.NewMessage("bob", "hi"))
// 	message.Put(s, "secret", "charlie", message.NewMessage("bob", "hello"))

// 	results, err := message.Search(s, "secret", "hi", 10)
// 	if err != nil {
// 		t.Fatal("Search:", err)
// 	}
// 	if len(results) != 1 {
// 		t.Fatalf("expected 1 result, got %d", len(results))
// 	}
// 	if results[0].Content != "hi" {
// 		t.Errorf("Content = %q, want %q", results[0].Content, "hi")
// 	}
// }

// func TestSearchEmptyQuery(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_search_emptyq")
// 	s := store.New(dir)

// 	results, err := message.Search(s, "secret", "", 10)
// 	if err != nil {
// 		t.Fatal("Search:", err)
// 	}
// 	if results != nil {
// 		t.Errorf("expected nil for empty query, got %v", results)
// 	}
// }

// func TestAttachmentStoreAndRetrieve(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_attachment")
// 	s := store.New(dir)

// 	att := message.Attachment{
// 		Name:     "test.txt",
// 		MIMEType: "text/plain",
// 		Data:     []byte("hello world"),
// 	}
// 	msg := message.NewMessage("bob", "file", att)
// 	msgID := uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
// 	msg.ID = msgID

// 	if err := message.StoreAttachments(s, "secret", msg.ID.String(), msg.Attachments); err != nil {
// 		t.Fatal("StoreAttachments:", err)
// 	}

// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	got, err := message.Get(s, "secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if len(got.Attachments) != 1 {
// 		t.Fatalf("expected 1 attachment, got %d", len(got.Attachments))
// 	}
// 	if got.Attachments[0].Name != "test.txt" {
// 		t.Errorf("Name = %q, want %q", got.Attachments[0].Name, "test.txt")
// 	}
// 	if len(got.Attachments[0].Data) != 0 {
// 		t.Errorf("Data should be nil after storage, got %v", got.Attachments[0].Data)
// 	}
// 	if got.Attachments[0].ID == uuid.Nil {
// 		t.Fatal("attachment ID should not be nil")
// 	}

// 	raw, err := s.Files.Get("secret", got.Attachments[0].ID)
// 	if err != nil {
// 		t.Fatal("Files.Get:", err)
// 	}
// 	if string(raw) != "hello world" {
// 		t.Errorf("file content = %q, want %q", string(raw), "hello world")
// 	}
// }

// func TestSearchAttachmentName(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_search_attname")
// 	s := store.New(dir)

// 	att := message.Attachment{
// 		Name: "report.pdf",
// 		Data: []byte("pdf data"),
// 	}
// 	msg := message.NewMessage("bob", "here it is", att)
// 	msg.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
// 	var err error
// 	if err = message.StoreAttachments(s, "secret", msg.ID.String(), msg.Attachments); err != nil {
// 		t.Fatal("StoreAttachments:", err)
// 	}
// 	if _, err = message.Put(s, "secret", "alice", msg); err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	results, err := message.Search(s, "secret", "report", 10)
// 	if err != nil {
// 		t.Fatal("Search:", err)
// 	}
// 	if len(results) != 1 {
// 		t.Fatalf("expected 1 result for 'report', got %d", len(results))
// 	}
// 	if results[0].Content != "here it is" {
// 		t.Errorf("Content = %q, want %q", results[0].Content, "here it is")
// 	}
// }

// func TestMessageDeleteRemovesFiles(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_delete_files")
// 	s := store.New(dir)

// 	att := message.Attachment{
// 		Name: "photo.jpg",
// 		Data: []byte("image data"),
// 	}
// 	msg := message.NewMessage("bob", "pic", att)
// 	msg.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
// 	if err := message.StoreAttachments(s, "secret", msg.ID.String(), msg.Attachments); err != nil {
// 		t.Fatal("StoreAttachments:", err)
// 	}
// 	id, err := message.Put(s, "secret", "alice", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	fileID := msg.Attachments[0].ID
// 	raw, err := s.Files.Get("secret", fileID)
// 	if err != nil {
// 		t.Fatalf("Files.Get before delete: %v", err)
// 	}
// 	if len(raw) == 0 {
// 		t.Fatal("expected non-empty file data")
// 	}

// 	if err = message.Delete(s, "secret", uuid.MustParse(id)); err != nil {
// 		t.Fatal("Delete:", err)
// 	}

// 	if _, err = s.Files.Get("secret", fileID); !os.IsNotExist(err) {
// 		t.Errorf("expected file to be deleted, got err=%v", err)
// 	}
// }

// func TestMultipleAttachments(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_multi_att")
// 	s := store.New(dir)

// 	att1 := message.Attachment{
// 		Name: "doc.txt",
// 		Data: []byte("text content"),
// 	}
// 	att2 := message.Attachment{
// 		Name: "img.png",
// 		Data: []byte("png data"),
// 	}
// 	msg := message.NewMessage("bob", "two files", att1, att2)
// 	msg.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
// 	if err := message.StoreAttachments(s, "secret", msg.ID.String(), msg.Attachments); err != nil {
// 		t.Fatal("StoreAttachments:", err)
// 	}
// 	var putErr error
// 	if _, putErr = message.Put(s, "secret", "alice", msg); putErr != nil {
// 		t.Fatal("Put:", putErr)
// 	}

// 	got, err := message.Get(s, "secret", msg.ID)
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if len(got.Attachments) != 2 {
// 		t.Fatalf("expected 2 attachments, got %d", len(got.Attachments))
// 	}
// 	if got.Attachments[0].Name != "doc.txt" || got.Attachments[1].Name != "img.png" {
// 		t.Errorf("attachment names mismatch")
// 	}
// 	for _, a := range got.Attachments {
// 		var raw []byte
// 		raw, err = s.Files.Get("secret", a.ID)
// 		if err != nil {
// 			t.Fatalf("Files.Get(%q): %v", a.ID.String(), err)
// 		}
// 		if len(raw) == 0 {
// 			t.Errorf("empty file data for %q", a.ID.String())
// 		}
// 	}
// }

// func TestStoreAttachmentsEmptyData(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_empty_attdata")
// 	s := store.New(dir)

// 	att := message.Attachment{Name: "empty.bin"}
// 	msg := message.NewMessage("bob", "empty att", att)
// 	msg.ID = uuid.MustParse("6ba7b810-9dad-11d1-80b4-00c04fd430c8")
// 	if err := message.StoreAttachments(s, "secret", msg.ID.String(), msg.Attachments); err != nil {
// 		t.Fatal("StoreAttachments:", err)
// 	}
// 	if _, err := message.Put(s, "secret", "alice", msg); err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	got, err := message.Get(s, "secret", msg.ID)
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if len(got.Attachments) != 1 {
// 		t.Fatalf("expected 1 attachment, got %d", len(got.Attachments))
// 	}
// 	if got.Attachments[0].Name != "empty.bin" {
// 		t.Errorf("Name = %q", got.Attachments[0].Name)
// 	}
// 	if got.Attachments[0].ID != uuid.Nil {
// 		t.Errorf("expected nil ID for empty-data attachment, got %v", got.Attachments[0].ID)
// 	}
// }

// func TestSpecialChars(t *testing.T) {
// 	dir := filepath.Join(t.TempDir(), "test_specialchars")
// 	s := store.New(dir)

// 	content := "tab\tnewline\nquote\"backslash\\end"
// 	msg := message.NewMessage("b", content)
// 	id, err := message.Put(s, "secret", "a", msg)
// 	if err != nil {
// 		t.Fatal("Put:", err)
// 	}

// 	got, err := message.Get(s, "secret", uuid.MustParse(id))
// 	if err != nil {
// 		t.Fatal("Get:", err)
// 	}
// 	if got.Content != content {
// 		t.Errorf("Content = %q, want %q", got.Content, content)
// 	}
// }
