package main

import (
	"os"
	"path/filepath"
	"testing"

	"pkg.jsn.cam/jsn/internal/skit"
)

func TestSanitizeSlug(t *testing.T) {
	cases := map[string]string{
		"":                 "",
		" simple ":         "simple",
		"HELLO WORLD":      "hello-world",
		"foo__bar":         "foo-bar",
		"foo--bar":         "foo-bar",
		"123 start":        "123-start",
		"invalid!@#":       "invalid",
		"already-good":     "already-good",
		" spaces   inside": "spaces-inside",
	}
	for in, want := range cases {
		if got := sanitizeSlug(in); got != want {
			t.Fatalf("sanitizeSlug(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestWindowStart(t *testing.T) {
	type args struct {
		cursor int
		total  int
		window int
	}
	cases := []struct {
		args args
		want int
	}{
		{args{0, 5, 5}, 0},
		{args{4, 10, 5}, 2},
		{args{9, 10, 5}, 5},
		{args{1, 2, 5}, 0},
	}
	for _, tc := range cases {
		got := windowStart(tc.args.cursor, tc.args.total, tc.args.window)
		if got != tc.want {
			t.Fatalf("windowStart%+v = %d, want %d", tc.args, got, tc.want)
		}
	}
}

func TestCreateScript(t *testing.T) {
	tmp := t.TempDir()
	m := newModel(tmp, []*skit.Script{}, nil, nil)
	cmd := m.createScript("demo script")
	msg := cmd()
	update, ok := msg.(scriptsUpdatedMsg)
	if !ok {
		t.Fatalf("createScript returned %T, want scriptsUpdatedMsg", msg)
	}
	if update.err != nil {
		t.Fatalf("createScript err: %v", update.err)
	}
	slug := "demo-script"
	manifest := filepath.Join(tmp, slug, skit.ConfigFileName)
	if _, err := os.Stat(manifest); err != nil {
		t.Fatalf("manifest missing: %v", err)
	}
	run := filepath.Join(tmp, slug, "run.sh")
	info, err := os.Stat(run)
	if err != nil {
		t.Fatalf("run script missing: %v", err)
	}
	if info.Mode()&0o111 == 0 {
		t.Fatalf("run script not executable: mode %v", info.Mode())
	}
}

func TestDisplayName(t *testing.T) {
	if got := displayName("cloudflare-dns"); got != "Cloudflare Dns" {
		t.Fatalf("displayName mismatch: %q", got)
	}
	if got := displayName(""); got != "New Script" {
		t.Fatalf("empty displayName mismatch: %q", got)
	}
}

func TestCommandPathFor(t *testing.T) {
	run := &skit.Script{
		Type: skit.ScriptTypeRun,
		Exec: skit.CommandMap{"default": "/tmp/run.sh"},
	}
	if path, err := commandPathFor(run, ""); err != nil || path != "/tmp/run.sh" {
		t.Fatalf("run commandPathFor: %v %s", err, path)
	}

	tog := &skit.Script{
		Type: skit.ScriptTypeToggle,
		Toggle: skit.ToggleSpec{
			Enable:  skit.CommandMap{"default": "/tmp/enable.sh"},
			Disable: skit.CommandMap{"default": "/tmp/disable.sh"},
		},
	}
	if _, err := commandPathFor(tog, ""); err == nil {
		t.Fatalf("expected error when no toggle action provided")
	}
	if path, err := commandPathFor(tog, skit.ToggleActionEnable); err != nil || path != "/tmp/enable.sh" {
		t.Fatalf("enable path mismatch: %v %s", err, path)
	}
}
