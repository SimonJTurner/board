package board

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStorageRoot_usesBOARD_STORAGE_DIR_whenAbsolute(t *testing.T) {
	custom := filepath.Join(t.TempDir(), "custom-board-root")
	if err := os.MkdirAll(custom, 0o755); err != nil {
		t.Fatal(err)
	}
	os.Setenv(storageDirEnv, custom)
	t.Cleanup(func() { os.Unsetenv(storageDirEnv) })

	s := NewStore()
	root, err := s.storageRoot()
	if err != nil {
		t.Fatalf("storageRoot: %v", err)
	}
	if root != custom {
		t.Errorf("storageRoot() = %q, want %q", root, custom)
	}
}

func TestStorageRoot_rejectsBOARD_STORAGE_DIR_whenRelative(t *testing.T) {
	os.Setenv(storageDirEnv, "relative/path")
	t.Cleanup(func() { os.Unsetenv(storageDirEnv) })

	s := NewStore()
	_, err := s.storageRoot()
	if err == nil {
		t.Fatal("expected storageRoot to error for relative path")
	}
	if err != nil && err.Error() == "" {
		t.Errorf("expected error message to mention absolute path, got: %v", err)
	}
}
