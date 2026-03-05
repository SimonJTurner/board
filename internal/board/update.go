package board

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	repo := fs.String("repo", "", "path to agent-board repository")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board update [--repo /path/to/agent-board]")
	}

	repoPath, err := resolveRepoPath(*repo)
	if err != nil {
		return err
	}

	cmd := exec.Command("go", "install", "./cmd/board")
	cmd.Dir = repoPath
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("update failed: %w", err)
	}

	fmt.Printf("updated board executable from %s\n", repoPath)
	return nil
}

func resolveRepoPath(explicit string) (string, error) {
	if p := strings.TrimSpace(explicit); p != "" {
		return validateRepoPath(p)
	}
	if p := strings.TrimSpace(os.Getenv("BOARD_REPO")); p != "" {
		return validateRepoPath(p)
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}
	return validateRepoPath(cwd)
}

func validateRepoPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
		return "", fmt.Errorf("%s is not a board repo (missing go.mod); set --repo or BOARD_REPO", abs)
	}
	if _, err := os.Stat(filepath.Join(abs, "cmd", "board", "main.go")); err != nil {
		return "", fmt.Errorf("%s is not a board repo (missing cmd/board/main.go); set --repo or BOARD_REPO", abs)
	}
	return abs, nil
}
