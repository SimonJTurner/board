package board_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SimonJTurner/board/internal/board"
)

func TestCLIIntegration_IssueLifecycle(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	if _, err := runCLI(t, "init", "demo"); err != nil {
		t.Fatalf("init failed: %v", err)
	}

	if _, err := runCLI(
		t,
		"issue", "create", "demo",
		"--title", "Create some feature",
		"--description", "Initial description",
	); err != nil {
		t.Fatalf("issue create failed: %v", err)
	}

	listOut, err := runCLI(t, "issue", "list", "demo")
	if err != nil {
		t.Fatalf("issue list failed: %v", err)
	}
	assertContains(t, listOut, "DEMO_1001_create_some_feature")
	assertContains(t, listOut, "todo")

	if _, err := runCLI(
		t,
		"issue", "assign", "demo", "DEMO_1001_create_some_feature",
		"--assignee", "agent-b",
	); err != nil {
		t.Fatalf("issue assign failed: %v", err)
	}

	// assign sets status to in_progress by default; verify before update
	listOut, err = runCLI(t, "issue", "list", "demo")
	if err != nil {
		t.Fatalf("issue list after assign failed: %v", err)
	}
	assertContains(t, listOut, "in_progress")
	assertContains(t, listOut, "agent-b")

	if _, err := runCLI(
		t,
		"issue", "update", "demo", "DEMO_1001_create_some_feature",
		"--title", "Create some feature v2",
		"--description", "Updated description",
	); err != nil {
		t.Fatalf("issue update failed: %v", err)
	}

	listOut, err = runCLI(t, "issue", "list", "demo")
	if err != nil {
		t.Fatalf("issue list after update failed: %v", err)
	}
	assertContains(t, listOut, "in_progress")
	assertContains(t, listOut, "agent-b")
	assertContains(t, listOut, "Create some feature v2")

	boardPath := filepath.Join(home, ".board", "demo", "board.json")
	boardBytes, err := os.ReadFile(boardPath)
	if err != nil {
		t.Fatalf("read board.json failed: %v", err)
	}
	assertContains(t, string(boardBytes), "\"next_issue_number\": 1002")
	assertContains(t, string(boardBytes), "\"status\": \"in_progress\"")
	assertContains(t, string(boardBytes), "\"assignee\": \"agent-b\"")

	issuePath := filepath.Join(home, ".board", "demo", "DEMO_1001_create_some_feature.md")
	issueBytes, err := os.ReadFile(issuePath)
	if err != nil {
		t.Fatalf("read issue markdown failed: %v", err)
	}
	assertContains(t, string(issueBytes), "title: Create some feature v2")
	assertContains(t, string(issueBytes), "status: in_progress")
	assertContains(t, string(issueBytes), "assignee: agent-b")
	assertContains(t, string(issueBytes), "Updated description")

	// assign when already assigned to someone else must fail
	_, err = runCLI(t, "issue", "assign", "demo", "DEMO_1001_create_some_feature", "--assignee", "agent-c")
	if err == nil {
		t.Fatal("expected assign to fail when issue already assigned to another user")
	}
	if !strings.Contains(err.Error(), "issue is already assigned to") || !strings.Contains(err.Error(), "agent-b") {
		t.Fatalf("expected error message about already assigned to agent-b, got: %v", err)
	}
}

func TestCLIIntegration_ProjectListRespectsArchive(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	if _, err := runCLI(t, "init", "alpha"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if _, err := runCLI(t, "init", "beta"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if _, err := runCLI(t, "project", "archive", "beta"); err != nil {
		t.Fatalf("project archive failed: %v", err)
	}

	out, err := runCLI(t, "project", "list")
	if err != nil {
		t.Fatalf("project list failed: %v", err)
	}
	assertContains(t, out, "alpha")
	assertNotContains(t, out, "beta")

	out, err = runCLI(t, "project", "list", "--archived")
	if err != nil {
		t.Fatalf("project list archived failed: %v", err)
	}
	assertContains(t, out, "alpha")
	assertContains(t, out, "beta")
}

func TestCLIIntegration_InitUsesGitRepoNameWhenOmitted(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	repoRoot := filepath.Join(t.TempDir(), "my-repo-name")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatalf("mkdir repo failed: %v", err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v, out=%s", err, string(out))
	}

	runInDir(t, repoRoot, func() {
		if _, err := runCLI(t, "init"); err != nil {
			t.Fatalf("init without project failed: %v", err)
		}
	})

	if _, err := os.Stat(filepath.Join(home, ".board", "my-repo-name", "board.json")); err != nil {
		t.Fatalf("expected board.json for auto-detected project: %v", err)
	}
}

func TestCLIIntegration_IssueListUsesGitRepoNameWhenOmitted(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	repoRoot := filepath.Join(t.TempDir(), "repo-list-default")
	if err := os.MkdirAll(repoRoot, 0o755); err != nil {
		t.Fatalf("mkdir repo failed: %v", err)
	}
	cmd := exec.Command("git", "init")
	cmd.Dir = repoRoot
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v, out=%s", err, string(out))
	}

	runInDir(t, repoRoot, func() {
		if _, err := runCLI(t, "init"); err != nil {
			t.Fatalf("init without project failed: %v", err)
		}
		if _, err := runCLI(
			t,
			"issue", "create", "repo-list-default",
			"--title", "Task Z",
			"--description", "desc",
		); err != nil {
			t.Fatalf("issue create failed: %v", err)
		}

		out, err := runCLI(t, "issue", "list")
		if err != nil {
			t.Fatalf("issue list without project failed: %v", err)
		}
		assertContains(t, out, "REPO_LIST_DEFAULT_1001_task_z")
	})
}

func TestCLIIntegration_IssueListFiltersAndLimitAndNext(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	if _, err := runCLI(t, "init", "demo"); err != nil {
		t.Fatalf("init failed: %v", err)
	}
	if _, err := runCLI(t, "issue", "create", "demo", "--title", "Task 1", "--description", "d1"); err != nil {
		t.Fatalf("create 1 failed: %v", err)
	}
	if _, err := runCLI(t, "issue", "create", "demo", "--title", "Task 2", "--description", "d2"); err != nil {
		t.Fatalf("create 2 failed: %v", err)
	}
	if _, err := runCLI(t, "issue", "create", "demo", "--title", "Task 3", "--description", "d3"); err != nil {
		t.Fatalf("create 3 failed: %v", err)
	}
	if _, err := runCLI(
		t,
		"issue", "update", "demo", "DEMO_1001_task_1",
		"--status", "done",
	); err != nil {
		t.Fatalf("update status failed: %v", err)
	}

	out, err := runCLI(t, "issue", "list", "demo", "--status", "todo")
	if err != nil {
		t.Fatalf("filtered list failed: %v", err)
	}
	assertContains(t, out, "DEMO_1002_task_2")
	assertContains(t, out, "DEMO_1003_task_3")
	assertNotContains(t, out, "DEMO_1001_task_1")

	out, err = runCLI(t, "issue", "list", "demo", "--status", "todo", "--limit", "1")
	if err != nil {
		t.Fatalf("filtered + limit list failed: %v", err)
	}
	assertContains(t, out, "DEMO_1002_task_2")
	assertNotContains(t, out, "DEMO_1003_task_3")

	out, err = runCLI(t, "issue", "next", "demo")
	if err != nil {
		t.Fatalf("issue next failed: %v", err)
	}
	assertContains(t, out, "DEMO_1002_task_2")
	assertNotContains(t, out, "DEMO_1003_task_3")
	assertNotContains(t, out, "DEMO_1001_task_1")
}

func TestCLIIntegration_ProjectDeleteRequiresName(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	if _, err := runCLI(t, "project", "delete"); err == nil {
		t.Fatal("expected project delete without name to fail")
	}
}

func TestCLIIntegration_WatchExecHookReceivesEvents(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	store := board.NewStore()
	if _, err := store.InitProject("demo"); err != nil {
		t.Fatalf("init project failed: %v", err)
	}

	hookFile := filepath.Join(t.TempDir(), "events.log")
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	errCh := make(chan error, 1)
	go func() {
		errCh <- board.Watch(ctx, store, board.WatchConfig{
			Project:  "demo",
			Interval: 50 * time.Millisecond,
			HookCmd:  fmt.Sprintf("cat >> %q", hookFile),
		})
	}()

	if _, err := store.CreateIssue("demo", "Task A", "Desc", "agent-a"); err != nil {
		t.Fatalf("create issue failed: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	if _, _, err := store.AssignIssue("demo", "DEMO_1001_task_a", "agent-b", nil); err != nil {
		t.Fatalf("assign issue failed: %v", err)
	}
	time.Sleep(200 * time.Millisecond)

	status := board.StatusDone
	if _, _, err := store.UpdateIssue("demo", "DEMO_1001_task_a", board.IssueUpdateInput{Status: &status}); err != nil {
		t.Fatalf("update issue failed: %v", err)
	}

	waitFor(t, 3*time.Second, func() bool {
		b, err := os.ReadFile(hookFile)
		if err != nil {
			return false
		}
		s := string(b)
		return strings.Contains(s, board.EventIssueCreated) &&
			strings.Contains(s, board.EventIssueAssigneeChanged) &&
			strings.Contains(s, board.EventIssueStatusChanged)
	})

	cancel()
	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("watch returned error: %v", err)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("watch did not stop after cancel")
	}
}

func runCLI(t *testing.T, args ...string) (string, error) {
	t.Helper()
	origStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe creation failed: %v", err)
	}
	os.Stdout = w

	runErr := board.Run(args)

	_ = w.Close()
	os.Stdout = origStdout

	var buf bytes.Buffer
	_, _ = io.Copy(&buf, r)
	_ = r.Close()

	return buf.String(), runErr
}

func setHome(t *testing.T, home string) {
	t.Helper()
	oldHome, hadHome := os.LookupEnv("HOME")
	if err := os.Setenv("HOME", home); err != nil {
		t.Fatalf("set HOME failed: %v", err)
	}
	t.Cleanup(func() {
		if hadHome {
			_ = os.Setenv("HOME", oldHome)
			return
		}
		_ = os.Unsetenv("HOME")
	})
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q to contain %q", got, want)
	}
}

func assertNotContains(t *testing.T, got, want string) {
	t.Helper()
	if strings.Contains(got, want) {
		t.Fatalf("expected %q to not contain %q", got, want)
	}
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("condition not met before timeout (%s)", timeout)
}

func runInDir(t *testing.T, dir string, fn func()) {
	t.Helper()
	orig, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd failed: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir failed: %v", err)
	}
	t.Cleanup(func() {
		_ = os.Chdir(orig)
	})
	fn()
}
