package live

import (
	"bytes"
	"os"
	"path/filepath"
	"testing"
	"time"

	"pkg.jsn.cam/jsn/internal/fsdiff/snapshot"
	"pkg.jsn.cam/jsn/internal/fsdiff/system"
)

func TestOpenStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("OpenStore failed: %v", err)
	}
	defer store.Close()

	// Verify file was created
	if _, err := os.Stat(dbPath); os.IsNotExist(err) {
		t.Fatal("database file was not created")
	}
}

func TestStoreMeta(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	meta := &Meta{
		StartTime: time.Now().Truncate(time.Second),
		RootPath:  "/test/path",
		Interval:  30,
	}

	if err := store.SaveMeta(meta); err != nil {
		t.Fatalf("SaveMeta failed: %v", err)
	}

	loaded, err := store.LoadMeta()
	if err != nil {
		t.Fatalf("LoadMeta failed: %v", err)
	}

	if loaded.RootPath != meta.RootPath {
		t.Errorf("RootPath mismatch: got %s, want %s", loaded.RootPath, meta.RootPath)
	}
	if loaded.Interval != meta.Interval {
		t.Errorf("Interval mismatch: got %d, want %d", loaded.Interval, meta.Interval)
	}
	if !loaded.StartTime.Equal(meta.StartTime) {
		t.Errorf("StartTime mismatch: got %v, want %v", loaded.StartTime, meta.StartTime)
	}
}

func TestStoreBaseline(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Initially no baseline
	if store.HasBaseline() {
		t.Error("HasBaseline should return false for empty store")
	}

	snap := &snapshot.Snapshot{
		SystemInfo: system.SystemInfo{
			Hostname: "testhost",
			Distro:   "test",
		},
		Files: map[string]*snapshot.FileRecord{
			"/test/file1": {Path: "/test/file1", Hash: "abc123", Size: 100},
			"/test/file2": {Path: "/test/file2", Hash: "def456", Size: 200},
		},
		Stats: snapshot.ScanStats{
			FileCount: 2,
			DirCount:  1,
		},
	}

	if err := store.SaveBaseline(snap); err != nil {
		t.Fatalf("SaveBaseline failed: %v", err)
	}

	if !store.HasBaseline() {
		t.Error("HasBaseline should return true after saving")
	}

	loaded, err := store.LoadBaseline()
	if err != nil {
		t.Fatalf("LoadBaseline failed: %v", err)
	}

	if loaded.SystemInfo.Hostname != snap.SystemInfo.Hostname {
		t.Errorf("Hostname mismatch: got %s, want %s", loaded.SystemInfo.Hostname, snap.SystemInfo.Hostname)
	}
	if len(loaded.Files) != len(snap.Files) {
		t.Errorf("Files count mismatch: got %d, want %d", len(loaded.Files), len(snap.Files))
	}
	if loaded.Files["/test/file1"].Hash != "abc123" {
		t.Errorf("File hash mismatch")
	}
}

func TestStoreChanges(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	now := time.Now()
	changes := []*Change{
		{Timestamp: now, Path: "/file1", Type: ChangeAdded, Hash: "hash1", Size: 100},
		{Timestamp: now, Path: "/file2", Type: ChangeModified, Hash: "hash2", Size: 200},
		{Timestamp: now, Path: "/file3", Type: ChangeDeleted},
	}

	if err := store.AppendChanges(changes); err != nil {
		t.Fatalf("AppendChanges failed: %v", err)
	}

	if count := store.ChangeCount(); count != 3 {
		t.Errorf("ChangeCount mismatch: got %d, want 3", count)
	}

	loaded, err := store.LoadChanges()
	if err != nil {
		t.Fatalf("LoadChanges failed: %v", err)
	}

	if len(loaded) != 3 {
		t.Fatalf("Loaded changes count mismatch: got %d, want 3", len(loaded))
	}

	// Check that all changes were loaded (order preserved by key)
	pathSet := make(map[string]bool)
	for _, c := range loaded {
		pathSet[c.Path] = true
	}
	for _, c := range changes {
		if !pathSet[c.Path] {
			t.Errorf("Missing change for path: %s", c.Path)
		}
	}
}

func TestStoreChangesKeyUniqueness(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Create many changes with the same timestamp to test key uniqueness
	now := time.Now()
	changes := make([]*Change, 100)
	for i := 0; i < 100; i++ {
		changes[i] = &Change{
			Timestamp: now, // Same timestamp
			Path:      filepath.Join("/test", string(rune('a'+i%26)), "file"),
			Type:      ChangeAdded,
		}
	}

	if err := store.AppendChanges(changes); err != nil {
		t.Fatalf("AppendChanges failed: %v", err)
	}

	// All 100 changes should be stored (no overwrites)
	if count := store.ChangeCount(); count != 100 {
		t.Errorf("ChangeCount mismatch: got %d, want 100 (key collision occurred)", count)
	}
}

func TestStoreContent(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	hash := "abc123"
	content := []byte("test content data")

	// Initially doesn't exist
	if store.ContentExists(hash) {
		t.Error("ContentExists should return false initially")
	}

	if err := store.SaveContent(hash, content); err != nil {
		t.Fatalf("SaveContent failed: %v", err)
	}

	if !store.ContentExists(hash) {
		t.Error("ContentExists should return true after saving")
	}

	loaded, err := store.LoadContent(hash)
	if err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}

	if !bytes.Equal(loaded, content) {
		t.Errorf("Content mismatch: got %q, want %q", loaded, content)
	}
}

func TestStoreContentDeduplication(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	hash := "abc123"
	content1 := []byte("original content")
	content2 := []byte("different content")

	if err := store.SaveContent(hash, content1); err != nil {
		t.Fatalf("SaveContent failed: %v", err)
	}

	// Save again with different content - should be deduplicated (not overwritten)
	if err := store.SaveContent(hash, content2); err != nil {
		t.Fatalf("SaveContent (second) failed: %v", err)
	}

	loaded, err := store.LoadContent(hash)
	if err != nil {
		t.Fatalf("LoadContent failed: %v", err)
	}

	// Should still be original content (deduplication)
	if !bytes.Equal(loaded, content1) {
		t.Errorf("Content was overwritten instead of deduplicated")
	}
}

func TestStoreExportJSON(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Set up test data
	meta := &Meta{
		StartTime: time.Now().Truncate(time.Second),
		RootPath:  "/test",
		Interval:  30,
	}
	store.SaveMeta(meta)

	snap := &snapshot.Snapshot{
		SystemInfo: system.SystemInfo{Hostname: "test"},
		Files:      map[string]*snapshot.FileRecord{},
		Stats:      snapshot.ScanStats{},
	}
	store.SaveBaseline(snap)

	changes := []*Change{
		{Timestamp: time.Now(), Path: "/file1", Type: ChangeAdded},
	}
	store.AppendChanges(changes)

	store.SaveContent("hash1", []byte("content"))

	// Export
	var buf bytes.Buffer
	if err := store.ExportJSON(&buf); err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// Basic validation - should contain expected strings
	exported := buf.String()
	if len(exported) == 0 {
		t.Error("ExportJSON produced empty output")
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"meta"`)) {
		t.Error("Export missing meta section")
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"baseline"`)) {
		t.Error("Export missing baseline section")
	}
	if !bytes.Contains(buf.Bytes(), []byte(`"changes"`)) {
		t.Error("Export missing changes section")
	}
}

func TestStoreLoadMetaNotFound(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	_, err := store.LoadMeta()
	if err == nil {
		t.Error("LoadMeta should fail on empty store")
	}
}

func TestStoreLoadBaselineNotFound(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	_, err := store.LoadBaseline()
	if err == nil {
		t.Error("LoadBaseline should fail on empty store")
	}
}

func TestStoreLoadContentNotFound(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	_, err := store.LoadContent("nonexistent")
	if err == nil {
		t.Error("LoadContent should fail for nonexistent hash")
	}
}

func TestAppendChangesEmpty(t *testing.T) {
	store := newTestStore(t)
	defer store.Close()

	// Should not error on empty slice
	if err := store.AppendChanges(nil); err != nil {
		t.Errorf("AppendChanges(nil) failed: %v", err)
	}
	if err := store.AppendChanges([]*Change{}); err != nil {
		t.Errorf("AppendChanges([]) failed: %v", err)
	}

	if count := store.ChangeCount(); count != 0 {
		t.Errorf("ChangeCount should be 0, got %d", count)
	}
}

// Helper function to create a test store
func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")

	store, err := OpenStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create test store: %v", err)
	}
	return store
}
