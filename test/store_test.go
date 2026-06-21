package store_test

import (
	"os"
	"testing"

	"github.com/JspBack/end-to-end-chat/store"
)

const testSecret = "test-secret-key-for-testing"

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "store-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())
	s := store.New(f.Name())
	t.Cleanup(func() { os.Remove(f.Name()) })
	return s
}

func TestNewCreatesFile(t *testing.T) {
	f, err := os.CreateTemp(t.TempDir(), "store-*.db")
	if err != nil {
		t.Fatal(err)
	}
	f.Close()
	os.Remove(f.Name())

	_ = store.New(f.Name())

	if _, err = os.Stat(f.Name()); os.IsNotExist(err) {
		t.Error("New did not create the file")
	}
}

func TestPutGet(t *testing.T) {
	s := newTestStore(t)

	id, err := s.Put("hello", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if id == "" {
		t.Fatal("expected non-empty id")
	}

	got, err := s.Get(id, testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if got != "hello" {
		t.Fatalf("got %q, want %q", got, "hello")
	}
}

func TestGetMissingKey(t *testing.T) {
	s := newTestStore(t)
	_, err := s.Get("nonexistent", testSecret)
	if !os.IsNotExist(err) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestUpdateExisting(t *testing.T) {
	s := newTestStore(t)

	id, err := s.Put("first", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Update(id, "second", testSecret); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Get(id, testSecret)
	if got != "second" {
		t.Fatalf("got %q, want %q", got, "second")
	}
}

func TestUpdateMissing(t *testing.T) {
	s := newTestStore(t)
	err := s.Update("nonexistent", "value", testSecret)
	if !os.IsNotExist(err) {
		t.Fatalf("expected ErrNotExist, got %v", err)
	}
}

func TestDelete(t *testing.T) {
	s := newTestStore(t)

	id, err := s.Put("value", testSecret)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Delete(id); err != nil {
		t.Fatal(err)
	}

	_, err = s.Get(id, testSecret)
	if !os.IsNotExist(err) {
		t.Fatal("expected ErrNotExist after delete")
	}
}
