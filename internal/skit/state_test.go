package skit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestStateStoreDelete(t *testing.T) {
	dir := t.TempDir()
	store, err := NewStateStore(dir)
	if err != nil {
		t.Fatalf("NewStateStore: %v", err)
	}
	record := RunRecord{Time: time.Now(), Action: string(ToggleActionEnable), Success: true}
	if err := store.Record("demo", ToggleActionEnable, record); err != nil {
		t.Fatalf("Record: %v", err)
	}
	path := filepath.Join(dir, "demo.json")
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("state file missing: %v", err)
	}
	if err := store.Delete("demo"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("state file still exists")
	}
}
