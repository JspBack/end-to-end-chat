package store_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/JspBack/end-to-end-chat/keys"
	"github.com/JspBack/end-to-end-chat/store"
)

var testSecret = func() keys.Key {
	k, _ := keys.FromHex("abcdef0102030405060708091011121314151617181920212223242526272829303132")
	return k
}()

func TestStorePutWithIDAndGet(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_putget")
	s := store.New(dir)

	id := uuid.New()
	err := s.Chats.PutWithID(id, "hello world", testSecret)
	if err != nil {
		t.Fatal("PutWithID:", err)
	}

	val, err := s.Chats.Get(id, testSecret)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if val != "hello world" {
		t.Errorf("got %q, want %q", val, "hello world")
	}
}

func TestStoreGetWrongSecret(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_wrongsecret")
	s := store.New(dir)

	var wrongKey keys.Key
	copy(wrongKey[:], []byte("this is the wrong key 32 bytes!"))

	id := uuid.New()
	err := s.Chats.PutWithID(id, "secret message", testSecret)
	if err != nil {
		t.Fatal("PutWithID:", err)
	}

	_, err = s.Chats.Get(id, wrongKey)
	if err == nil {
		t.Error("expected error for wrong secret")
	}
}

func TestStoreGetNotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_notfound")
	s := store.New(dir)

	_, err := s.Chats.Get(uuid.MustParse("00000000-0000-0000-0000-000000000001"), testSecret)
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

func TestStoreUpdate(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_update")
	s := store.New(dir)

	id := uuid.New()
	err := s.Chats.PutWithID(id, "old value", testSecret)
	if err != nil {
		t.Fatal("PutWithID:", err)
	}

	err = s.Chats.Update(id, "new value", testSecret)
	if err != nil {
		t.Fatal("Update:", err)
	}

	val, err := s.Chats.Get(id, testSecret)
	if err != nil {
		t.Fatal("Get after update:", err)
	}
	if val != "new value" {
		t.Errorf("got %q, want %q", val, "new value")
	}
}

func TestStoreUpdateNotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_updatenf")
	s := store.New(dir)

	err := s.Chats.Update(uuid.MustParse("00000000-0000-0000-0000-000000000001"), "value", testSecret)
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}

func TestStoreList(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_list")
	s := store.New(dir)

	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	_ = s.Chats.PutWithID(id1, "msg1", testSecret)
	_ = s.Chats.PutWithID(id2, "msg2", testSecret)
	_ = s.Chats.PutWithID(id3, "msg3", testSecret)

	list, err := s.Chats.List()
	if err != nil {
		t.Fatal("List:", err)
	}

	if len(list) != 3 {
		t.Fatalf("expected 3 items, got %d", len(list))
	}

	ids := map[uuid.UUID]bool{list[0].ID: true, list[1].ID: true, list[2].ID: true}
	if !ids[id1] || !ids[id2] || !ids[id3] {
		t.Error("List missing some IDs")
	}

	for _, item := range list {
		if item.CreatedAt.IsZero() {
			t.Errorf("item %q has zero created_at", item.ID)
		}
	}
}

func TestStoreDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_delete")
	s := store.New(dir)

	id := uuid.New()
	_ = s.Chats.PutWithID(id, "to delete", testSecret)
	err := s.Chats.Delete(id)
	if err != nil {
		t.Fatal("Delete:", err)
	}

	_, err = s.Chats.Get(id, testSecret)
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist after delete, got %v", err)
	}
}

func TestStorePutWithIDCustom(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_putwithid")
	s := store.New(dir)

	customUUID := uuid.MustParse("00000000-0000-0000-0000-000000000002")
	err := s.Chats.PutWithID(customUUID, "custom value", testSecret)
	if err != nil {
		t.Fatal("PutWithID:", err)
	}

	val, err := s.Chats.Get(customUUID, testSecret)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if val != "custom value" {
		t.Errorf("got %q, want %q", val, "custom value")
	}
}

func TestStoreReplaceWithPutWithID(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_replace")
	s := store.New(dir)

	sameUUID := uuid.MustParse("00000000-0000-0000-0000-000000000003")
	_ = s.Chats.PutWithID(sameUUID, "first", testSecret)
	_ = s.Chats.PutWithID(sameUUID, "second", testSecret)

	val, err := s.Chats.Get(sameUUID, testSecret)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if val != "second" {
		t.Errorf("got %q, want %q", val, "second")
	}
}

func TestStoreListEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_listempty")
	s := store.New(dir)

	list, err := s.Chats.List()
	if err != nil {
		t.Fatal("List:", err)
	}
	if len(list) != 0 {
		t.Errorf("expected empty list, got %d items", len(list))
	}
}

func TestStoreDeleteNotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_delnotfound")
	s := store.New(dir)

	err := s.Chats.Delete(uuid.MustParse("00000000-0000-0000-0000-000000000001"))
	if err != nil {
		t.Errorf("delete non-existent should not error, got %v", err)
	}
}

func TestStoreMultiplePutsListOrder(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_listorder")
	s := store.New(dir)

	id1 := uuid.New()
	id2 := uuid.New()
	id3 := uuid.New()
	_ = s.Chats.PutWithID(id1, "first", testSecret)
	_ = s.Chats.PutWithID(id2, "second", testSecret)
	_ = s.Chats.PutWithID(id3, "third", testSecret)

	list, err := s.Chats.List()
	if err != nil {
		t.Fatal("List:", err)
	}
	if len(list) != 3 {
		t.Fatalf("expected 3, got %d", len(list))
	}
}

func TestStoreEncryptDifferentKeys(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_encdiff")
	s := store.New(dir)

	var wrongKey keys.Key
	copy(wrongKey[:], []byte("wrong key for encryption test 32b"))

	id := uuid.New()
	err := s.Chats.PutWithID(id, "secret data", testSecret)
	if err != nil {
		t.Fatal("PutWithID with key1:", err)
	}

	val, err := s.Chats.Get(id, testSecret)
	if err != nil {
		t.Fatal("Get with correct key:", err)
	}
	if val != "secret data" {
		t.Errorf("got %q, want %q", val, "secret data")
	}

	_, err = s.Chats.Get(id, wrongKey)
	if err == nil {
		t.Error("expected error with wrong key")
	}
}

func TestStoreLongValue(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_longval")
	s := store.New(dir)

	val := strings.Repeat("x", 100000)
	id := uuid.New()

	err := s.Chats.PutWithID(id, val, testSecret)
	if err != nil {
		t.Fatal("PutWithID long value:", err)
	}

	got, err := s.Chats.Get(id, testSecret)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if len(got) != 100000 {
		t.Errorf("length = %d, want 100000", len(got))
	}
}

func TestStoreRepeatedPutSameContent(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_repeat")
	s := store.New(dir)

	id1 := uuid.New()
	id2 := uuid.New()
	_ = s.Chats.PutWithID(id1, "same", testSecret)
	_ = s.Chats.PutWithID(id2, "same", testSecret)

	if id1 == id2 {
		t.Error("two Puts should produce different IDs")
	}

	v1, _ := s.Chats.Get(id1, testSecret)
	v2, _ := s.Chats.Get(id2, testSecret)
	if v1 != v2 {
		t.Errorf("values differ: %q vs %q", v1, v2)
	}
}

func TestStoreIndexSearch(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_idx_search")
	s := store.New(dir)

	s.Chats.IndexSearch(uuid.MustParse("00000000-0000-0000-0000-000000000010"), "alice", "bob", "hello world")
	s.Chats.IndexSearch(uuid.MustParse("00000000-0000-0000-0000-000000000011"), "charlie", "dave", "golang programming")
	s.Chats.IndexSearch(uuid.MustParse("00000000-0000-0000-0000-000000000012"), "alice", "bob", "hello again")

	ids, err := s.Chats.Search("hello", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 results for 'hello', got %d", len(ids))
	}

	ids, err = s.Chats.Search("alice", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(ids) != 2 {
		t.Errorf("expected 2 results for 'alice', got %d", len(ids))
	}

	ids, err = s.Chats.Search("zzzz", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 results for 'zzzz', got %d", len(ids))
	}
}

func TestStoreIndexSearchLimit(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_idx_search_lim")
	s := store.New(dir)

	s.Chats.IndexSearch(uuid.MustParse("00000000-0000-0000-0000-000000000020"), "a", "b", "match this")
	s.Chats.IndexSearch(uuid.MustParse("00000000-0000-0000-0000-000000000021"), "c", "d", "match that")
	s.Chats.IndexSearch(uuid.MustParse("00000000-0000-0000-0000-000000000022"), "e", "f", "match too")

	ids, err := s.Chats.Search("match", 2)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(ids) > 2 {
		t.Errorf("expected at most 2 results with limit 2, got %d", len(ids))
	}
}

func TestStoreIndexDelete(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_idx_del")
	s := store.New(dir)

	delID := uuid.MustParse("00000000-0000-0000-0000-000000000005")
	s.Chats.IndexSearch(delID, "alice", "bob", "delete me")
	s.Chats.Delete(delID)

	ids, err := s.Chats.Search("delete", 10)
	if err != nil {
		t.Fatal("Search:", err)
	}
	if len(ids) != 0 {
		t.Errorf("expected 0 results after delete, got %d", len(ids))
	}
}

func TestStoreCacheStoreAndLoad(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_cache")
	s := store.New(dir)

	id := uuid.New()
	s.Chats.CacheStore(id, "cached value")

	cached, ok := s.Chats.CacheLoad(id)
	if !ok {
		t.Error("expected value in cache after CacheStore")
	}
	if cached != "cached value" {
		t.Errorf("cache = %q, want %q", cached, "cached value")
	}
}

func TestStoreUpdateWorks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_cache_upd")
	s := store.New(dir)

	id := uuid.New()
	_ = s.Chats.PutWithID(id, "original", testSecret)
	_ = s.Chats.Update(id, "modified", testSecret)

	got, err := s.Chats.Get(id, testSecret)
	if err != nil {
		t.Fatal("Get:", err)
	}
	if got != "modified" {
		t.Errorf("got %q, want %q", got, "modified")
	}
}

func TestStoreDeleteClearsCache(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_cache_del")
	s := store.New(dir)

	id := uuid.New()
	_ = s.Chats.PutWithID(id, "delete from cache", testSecret)
	s.Chats.CacheStore(id, "cached")

	_, ok := s.Chats.CacheLoad(id)
	if !ok {
		t.Error("expected value in cache after CacheStore")
	}

	_ = s.Chats.Delete(id)

	_, ok = s.Chats.CacheLoad(id)
	if ok {
		t.Error("expected cache to be cleared after Delete")
	}
}

func TestStorePutWithIDThenUpdateNotFound(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "test_upd_nf")
	s := store.New(dir)

	nonexistent := uuid.MustParse("00000000-0000-0000-0000-000000009999")
	err := s.Chats.Update(nonexistent, "value", testSecret)
	if !os.IsNotExist(err) {
		t.Errorf("expected ErrNotExist, got %v", err)
	}
}
