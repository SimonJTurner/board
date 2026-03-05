package board_test

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"agent-board/internal/board"
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
		"--assignee", "agent-a",
	); err != nil {
		t.Fatalf("issue create failed: %v", err)
	}

	listOut, err := runCLI(t, "issue", "list", "demo")
	if err != nil {
		t.Fatalf("issue list failed: %v", err)
	}
	assertContains(t, listOut, "DEMO_1001_create_some_feature")
	assertContains(t, listOut, "todo")
	assertContains(t, listOut, "agent-a")

	if _, err := runCLI(
		t,
		"issue", "assign", "demo", "DEMO_1001_create_some_feature",
		"--assignee", "agent-b",
	); err != nil {
		t.Fatalf("issue assign failed: %v", err)
	}

	if _, err := runCLI(
		t,
		"issue", "update", "demo", "DEMO_1001_create_some_feature",
		"--status", "in_progress",
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
}

func TestCLIIntegration_ProjectAliasAndList(t *testing.T) {
	home := t.TempDir()
	setHome(t, home)

	if _, err := runCLI(t, "project", "alpha"); err != nil {
		t.Fatalf("project alias init failed: %v", err)
	}
	if _, err := runCLI(t, "project", "beta"); err != nil {
		t.Fatalf("project alias init failed: %v", err)
	}

	out, err := runCLI(t, "project", "list")
	if err != nil {
		t.Fatalf("project list failed: %v", err)
	}
	assertContains(t, out, "alpha")
	assertContains(t, out, "beta")
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

	if _, _, err := store.AssignIssue("demo", "DEMO_1001_task_a", "agent-b"); err != nil {
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
