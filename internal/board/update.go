package board

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
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
	asset, err := releaseAssetName()
	if err != nil {
		return err
	}
	version, err := downloadReleaseAsset(rRepo, asset)
	if err != nil {
		return err
	}
	if version != "" {
		fmt.Printf("updated board to %s from %s (%s)\n", version, rRepo, asset)
	} else {
		fmt.Printf("updated board executable from release %s (%s)\n", rRepo, asset)
	}
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

// releaseAssetName returns the GitHub release asset name for the running binary's
// platform (e.g. board-darwin-arm64). We use runtime.GOOS/GOARCH, not "go env",
// because we overwrite the current executable—so we must fetch the same OS/arch
// as the binary that is running, or the next run will be the wrong format.
func releaseAssetName() (string, error) {
	suffix := ""
	if runtime.GOOS == "windows" {
		suffix = ".exe"
	}
	return fmt.Sprintf("board-%s-%s%s", runtime.GOOS, runtime.GOARCH, suffix), nil
}

// downloadReleaseAsset fetches the asset from the repo's latest release, installs it,
// and returns the release tag (e.g. "v0.2.0") from the redirect URL, or "" if not found.
func downloadReleaseAsset(repo, asset string) (version string, err error) {
	downloadURL := fmt.Sprintf("%s/%s/releases/latest/download/%s", releaseBaseURL, repo, asset)
	client := http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download release asset: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("release asset request returned %d", resp.StatusCode)
	}
	if v := parseReleaseTagFromURL(resp.Request.URL); v != "" {
		version = v
	}
	tmp, err := os.CreateTemp("", "board-update-*")
	if err != nil {
		return "", err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, resp.Body); err != nil {
		return "", err
	}
	if err := tmp.Chmod(0o755); err != nil {
		return "", err
	}
	if err := tmp.Close(); err != nil {
		return "", err
	}
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	if err := os.Rename(tmp.Name(), exe); err != nil {
		return "", err
	}
	return version, nil
}

// parseReleaseTagFromURL extracts the release tag from a GitHub release download URL,
// e.g. "https://github.com/owner/repo/releases/download/v0.2.0/board-darwin-arm64" -> "v0.2.0".
func parseReleaseTagFromURL(u *url.URL) string {
	path := strings.TrimPrefix(u.Path, "/")
	// path is like "owner/repo/releases/download/TAG/asset"
	prefix := "releases/download/"
	if idx := strings.Index(path, prefix); idx >= 0 {
		rest := path[idx+len(prefix):]
		if slash := strings.IndexByte(rest, '/'); slash >= 0 {
			return rest[:slash]
		}
	}
	return ""
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
