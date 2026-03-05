package board

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateRepoPath(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "go.mod"), []byte("module x\n"), 0o644); err != nil {
		t.Fatalf("write go.mod failed: %v", err)
	}
	if err := os.MkdirAll(filepath.Join(repo, "cmd", "board"), 0o755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repo, "cmd", "board", "main.go"), []byte("package main\n"), 0o644); err != nil {
		t.Fatalf("write main.go failed: %v", err)
	}

	got, err := validateRepoPath(repo)
	if err != nil {
		t.Fatalf("validateRepoPath returned error: %v", err)
	}
	if got == "" {
		t.Fatal("validateRepoPath returned empty path")
	}
}

func TestValidateRepoPathRejectsInvalidDir(t *testing.T) {
	if _, err := validateRepoPath(t.TempDir()); err == nil {
		t.Fatal("expected validateRepoPath to fail for invalid dir")
	}
}
