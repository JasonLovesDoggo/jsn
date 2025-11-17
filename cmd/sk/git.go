package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

func ensureGitRepo(dir, url, branch string) error {
	if url == "" {
		return nil
	}
	if isGitRepo(dir) {
		return nil
	}
	empty, err := isDirEmpty(dir)
	if err != nil {
		return err
	}
	if !empty {
		return fmt.Errorf("%s exists but is not a git repo; move it or set SKIT_HOME", dir)
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "--branch", branch, "--single-branch")
	}
	args = append(args, url, dir)
	return runGit(".", args...)
}

func gitSyncChanges(dir, branch string) error {
	if !isGitRepo(dir) {
		return errors.New("scripts directory is not a git repository")
	}
	targetBranch := branch
	if targetBranch == "" {
		b, err := currentBranch(dir)
		if err != nil {
			return err
		}
		targetBranch = b
	}
	if err := runGit(dir, "pull", "--rebase", "origin", targetBranch); err != nil {
		return err
	}
	if err := runGit(dir, "add", "."); err != nil {
		return err
	}
	msg := fmt.Sprintf("sk sync %s", time.Now().Format(time.RFC3339))
	if err := gitCommit(dir, msg); err != nil {
		return err
	}
	return runGit(dir, "push", "origin", targetBranch)
}

func gitCommit(dir, message string) error {
	cmd := exec.Command("git", "-C", dir, "commit", "-m", message)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if bytes.Contains(output, []byte("nothing to commit")) {
			return nil
		}
		return fmt.Errorf("git commit: %s", bytes.TrimSpace(output))
	}
	return nil
}

func currentBranch(dir string) (string, error) {
	var buf bytes.Buffer
	cmd := exec.Command("git", "-C", dir, "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git rev-parse: %s", strings.TrimSpace(buf.String()))
	}
	return strings.TrimSpace(buf.String()), nil
}

func runGit(dir string, args ...string) error {
	cmd := exec.Command("git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	var buf bytes.Buffer
	cmd.Stdout = os.Stdout
	cmd.Stderr = &buf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(buf.String()))
	}
	return nil
}

func isGitRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}

func isDirEmpty(dir string) (bool, error) {
	f, err := os.Open(dir)
	if err != nil {
		return false, err
	}
	defer f.Close()
	_, err = f.Readdirnames(1)
	if errors.Is(err, io.EOF) {
		return true, nil
	}
	return false, err
}
