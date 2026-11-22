package checkpoint

import (
	"path/filepath"
	"testing"
)

func TestSaveLoadAndClear(t *testing.T) {
	path := filepath.Join(t.TempDir(), "progress.json")
	store := New(path)

	key := Key("Animation", false, 42)
	store.Add(key, 111, 999)
	store.Add(key, 222, 888)
	store.Add(Key("Animation", true, 42), 111, 777)
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}

	loaded := New(path)
	if err := loaded.Load(); err != nil {
		t.Fatal(err)
	}
	if got, ok := loaded.Lookup(key, 111); !ok || got != 999 {
		t.Fatalf("lookup got %d %v", got, ok)
	}
	if _, ok := loaded.Lookup(Key("Animation", false, 43), 111); ok {
		t.Fatal("different creator must not match")
	}
	if got, _ := loaded.Lookup(Key("Animation", true, 42), 111); got != 777 {
		t.Fatalf("group key got %d", got)
	}
	if loaded.Count() != 3 {
		t.Fatalf("count %d", loaded.Count())
	}

	if removed := loaded.Clear(); removed != 3 {
		t.Fatalf("removed %d", removed)
	}
	again := New(path)
	again.Load()
	if again.Count() != 0 {
		t.Fatalf("expected empty after clear, got %d", again.Count())
	}
}

func TestLoadMissingFileIsFine(t *testing.T) {
	if err := New(filepath.Join(t.TempDir(), "nope.json")).Load(); err != nil {
		t.Fatal(err)
	}
}
