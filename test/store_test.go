package test_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/JspBack/end-to-end-chat/store"
)

func dbPath(dir string) string {
	exe, err := os.Executable()
	if err != nil {
		panic(err)
	}
	return filepath.Join(filepath.Dir(exe), dir+".db")
}

func TestStorePutGet(t *testing.T) {
	dir := "test_putget"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	id, err := s.Chats.Put("hello world", "secret")
	if err != nil {
		t.Fatal("Put:", err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	val, err := s.Chats.Get(id, "secret")
	if err != nil {
		t.Fatal("Get:", err)
	}
	if val != "hello world" {
		t.Errorf("got %q, want %q", val, "hello world")
	}
}

func TestStoreGetWrongSecret(t *testing.T) {
	dir := "test_wrongsecret"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	id, err := s.Chats.Put("secret message", "correct")
	if err != nil {
		t.Fatal("Put:", err)
	}

	_, err = s.Chats.Get(id, "wrong")
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestStoreGetNotFound(t *testing.T) {
	dir := "test_notfound"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	_, err := s.Chats.Get("nonexistent-id", "secret")
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

func TestStoreUpdate(t *testing.T) {
	dir := "test_update"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	id, err := s.Chats.Put("old value", "secret")
	if err != nil {
		t.Fatal("Put:", err)
	}

	err = s.Chats.Update(id, "new value", "secret")
	if err != nil {
		t.Fatal("Update:", err)
	}

	val, err := s.Chats.Get(id, "secret")
	if err != nil {
		t.Fatal("Get after update:", err)
	}
	if val != "new value" {
		t.Errorf("got %q, want %q", val, "new value")
	}
}

func TestStoreUpdateNotFound(t *testing.T) {
	dir := "test_updatenf"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	err := s.Chats.Update("nonexistent", "value", "secret")
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

func TestStoreList(t *testing.T) {
	dir := "test_list"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	id1, _ := s.Chats.Put("msg1", "secret")
	id2, _ := s.Chats.Put("msg2", "secret")
	id3, _ := s.Chats.Put("msg3", "secret")

	list, err := s.Chats.List()
	if err != nil {
		t.Fatal("List:", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 items, got %d", len(list))
	}

	ids := map[string]bool{list[0].ID: true, list[1].ID: true, list[2].ID: true}
	if !ids[id1] || !ids[id2] || !ids[id3] {
		t.Error("List missing some IDs")
	}

	for _, item := range list {
		if item.CreatedAt == "" {
			t.Errorf("item %q has empty created_at", item.ID)
		}
	}
}

func TestStoreDelete(t *testing.T) {
	dir := "test_delete"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	id, _ := s.Chats.Put("to delete", "secret")
	err := s.Chats.Delete(id)
	if err != nil {
		t.Fatal("Delete:", err)
	}

	_, err = s.Chats.Get(id, "secret")
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist after delete, got %v", err)
	}
}

func TestStorePutWithID(t *testing.T) {
	dir := "test_putwithid"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	err := s.Chats.PutWithID("custom-id", "custom value", "secret")
	if err != nil {
		t.Fatal("PutWithID:", err)
	}

	val, err := s.Chats.Get("custom-id", "secret")
	if err != nil {
		t.Fatal("Get:", err)
	}
	if val != "custom value" {
		t.Errorf("got %q, want %q", val, "custom value")
	}
}

func TestStoreReplaceWithPutWithID(t *testing.T) {
	dir := "test_replace"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	s.Chats.PutWithID("same-id", "first", "secret")
	s.Chats.PutWithID("same-id", "second", "secret")

	val, err := s.Chats.Get("same-id", "secret")
	if err != nil {
		t.Fatal("Get:", err)
	}
	if val != "second" {
		t.Errorf("got %q, want %q", val, "second")
	}
}

func TestStoreListEmpty(t *testing.T) {
	dir := "test_listempty"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	list, err := s.Chats.List()
	if err != nil {
		t.Fatal("List:", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestStoreDeleteNotFound(t *testing.T) {
	dir := "test_delnotfound"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	err := s.Chats.Delete("nonexistent")
	if err != nil {
		t.Errorf("delete non-existent should not error, got %v", err)
	}
}

func TestStoreMultiplePutsListOrder(t *testing.T) {
	dir := "test_listorder"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	s.Chats.Put("first", "sec")
	s.Chats.Put("second", "sec")
	s.Chats.Put("third", "sec")

	list, err := s.Chats.List()
	if err != nil {
		t.Fatal("List:", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3, got %d", len(list))
	}
}

func TestStoreEncryptDifferentKeys(t *testing.T) {
	dir := "test_encdiff"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	id, err := s.Chats.Put("secret data", "key1")
	if err != nil {
		t.Fatal("Put with key1:", err)
	}

	val, err := s.Chats.Get(id, "key1")
	if err != nil {
		t.Fatal("Get with key1:", err)
	}
	if val != "secret data" {
		t.Errorf("got %q, want %q", val, "secret data")
	}

	_, err = s.Chats.Get(id, "key2")
	if err == nil {
		t.Error("expected error with wrong key")
	}
}

func TestStoreLongValue(t *testing.T) {
	dir := "test_longval"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	val := strings.Repeat("x", 100000)

	id, err := s.Chats.Put(val, "secret")
	if err != nil {
		t.Fatal("Put long value:", err)
	}

	got, err := s.Chats.Get(id, "secret")
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(got) != 100000 {
		t.Errorf("length = %d, want 100000", len(got))
	}
}

func TestStoreRepeatedPutSameContent(t *testing.T) {
	dir := "test_repeat"
	s := store.New(dir)
	defer os.Remove(dbPath(dir))

	id1, _ := s.Chats.Put("same", "secret")
	id2, _ := s.Chats.Put("same", "secret")

	if id1 == id2 {
		t.Error("two Puts should produce different IDs")
	}

	v1, _ := s.Chats.Get(id1, "secret")
	v2, _ := s.Chats.Get(id2, "secret")
	if v1 != v2 {
		t.Errorf("values differ: %q vs %q", v1, v2)
	}
}
