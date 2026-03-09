package board

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

const releaseBaseURL = "https://github.com"

func runUpdate(args []string) error {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	repo := fs.String("repo", "", "path to local agent-board repository")
	releaseRepo := fs.String("release-repo", "", "GitHub owner/repo for releases (default: SimonJTurner/board)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board update [--repo /path/to/board] [--release-repo owner/repo]")
	}

	// Local repo only when explicitly requested (for contributors testing)
	if p := strings.TrimSpace(*repo); p != "" {
		repoPath, err := validateRepoPath(p)
		if err != nil {
			return err
		}
		return installFromRepo(repoPath)
	}

	rRepo, err := resolveReleaseRepo(*releaseRepo)
	if err != nil {
		return err
	}
	asset := assetName()
	if err := downloadReleaseAsset(rRepo, asset); err != nil {
		return err
	}
	fmt.Printf("updated board executable from release %s (%s)\n", rRepo, asset)
	return nil
}

func installFromRepo(repoPath string) error {
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

const defaultReleaseRepo = "SimonJTurner/board"

func resolveReleaseRepo(explicit string) (string, error) {
	if repo, err := normalizeReleaseRepo(explicit); err == nil && repo != "" {
		return repo, nil
	}
	if repo, err := normalizeReleaseRepo(os.Getenv("BOARD_RELEASE_REPO")); err == nil && repo != "" {
		return repo, nil
	}
	if repo, err := inferReleaseRepoFromGit(); err == nil && repo != "" {
		return repo, nil
	}
	return defaultReleaseRepo, nil
}

func normalizeReleaseRepo(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	value = strings.TrimSuffix(value, ".git")
	if strings.HasPrefix(value, "https://") || strings.HasPrefix(value, "http://") {
		parts := strings.Split(value, "/")
		if len(parts) < 2 {
			return "", fmt.Errorf("invalid release repo url: %s", value)
		}
		value = strings.Join(parts[len(parts)-2:], "/")
	}
	if strings.HasPrefix(value, "git@") {
		if idx := strings.Index(value, ":"); idx != -1 {
			value = value[idx+1:]
		}
	}
	if value == "" || strings.Count(value, "/") != 1 {
		return "", fmt.Errorf("release repo must be owner/repo (got %q)", value)
	}
	return value, nil
}

func inferReleaseRepoFromGit() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	out, err := cmd.Output()
	if err != nil {
		// Fall through to default
		return "", nil
	}
	repo, err := normalizeReleaseRepo(string(out))
	if err != nil {
		return "", err
	}
	return repo, nil
}

func assetName() string {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return fmt.Sprintf("board-%s-%s%s", runtime.GOOS, runtime.GOARCH, suffix)
}

func downloadReleaseAsset(repo, asset string) error {
	url := fmt.Sprintf("%s/%s/releases/latest/download/%s", releaseBaseURL, repo, asset)
	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to download release asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("release asset request returned %d", resp.StatusCode)
	}
	tmp, err := os.CreateTemp("", "board-update-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return err
	}
	if err := tmp.Chmod(0o755); err != nil {
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		return err
	}
	return nil
}

func validateRepoPath(path string) (string, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(filepath.Join(abs, "go.mod")); err != nil {
		return "", fmt.Errorf("%s is not a board repo (missing go.mod)", abs)
	}
	if _, err := os.Stat(filepath.Join(abs, "cmd", "board", "main.go")); err != nil {
		return "", fmt.Errorf("%s is not a board repo (missing cmd/board/main.go)", abs)
	}
	return abs, nil
}
