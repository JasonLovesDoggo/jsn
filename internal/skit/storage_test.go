package skit

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLoadScriptsRun(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "hello")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg := `
name = "Example"
type = "run"

[exec]
default = "!echo hi"
`
	writeFile(t, filepath.Join(dir, ConfigFileName), cfg)
	scripts, err := LoadScripts(root)
	if err != nil {
		t.Fatalf("load scripts: %v", err)
	}
	if len(scripts) != 1 {
		t.Fatalf("expected 1 script, got %d", len(scripts))
	}
	if scripts[0].Name != "Example" {
		t.Fatalf("unexpected name %q", scripts[0].Name)
	}
	cmd, err := scripts[0].Exec.CommandFor("plan9")
	if err != nil {
		t.Fatalf("command for plan9 should fallback to default: %v", err)
	}
	if !cmd.Inline {
		t.Fatalf("expected inline command")
	}
}

func TestStateStore(t *testing.T) {
	root := t.TempDir()
	store, err := NewStateStore(root)
	if err != nil {
		t.Fatal(err)
	}
	if next := store.NextAction("dns"); next != ToggleActionEnable {
		t.Fatalf("expected enable, got %s", next)
	}
	record := RunRecord{Time: time.Now(), Action: string(ToggleActionEnable), Success: true}
	if err := store.Record("dns", ToggleActionEnable, record); err != nil {
		t.Fatalf("record: %v", err)
	}
	if next := store.NextAction("dns"); next != ToggleActionDisable {
		t.Fatalf("expected disable, got %s", next)
	}
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
