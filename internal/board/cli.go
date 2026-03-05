package board

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"text/tabwriter"
	"time"
)

func Run(args []string) error {
	if len(args) == 0 {
		printUsage()
		return nil
	}
	store := NewStore()

	switch args[0] {
	case "init":
		return runInit(store, args[1:])
	case "project":
		return runProject(store, args[1:])
	case "update":
		return runUpdate(args[1:])
	case "issue":
		return runIssue(store, args[1:])
	case "watch":
		return runWatch(store, args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func runInit(store *Store, args []string) error {
	if len(args) > 1 {
		return errors.New("usage: board init [project]")
	}
	project := ""
	if len(args) == 1 {
		project = strings.TrimSpace(args[0])
	} else {
		var err error
		project, err = inferProjectFromGitRepo()
		if err != nil {
			return errors.New("usage: board init [project] (or run inside a git repo to auto-detect)")
		}
	}
	projectPath, err := store.InitProject(project)
	if err != nil {
		return err
	}
	fmt.Printf("initialized project %q at %s\n", project, projectPath)
	return nil
}

func inferProjectFromGitRepo() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	root := strings.TrimSpace(string(out))
	if root == "" {
		return "", errors.New("empty git root")
	}
	project := filepath.Base(root)
	project = strings.TrimSpace(project)
	if project == "" || project == "." || project == string(filepath.Separator) {
		return "", errors.New("invalid git repo name")
	}
	return project, nil
}

func runProject(store *Store, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: board project <name>|list|delete <name>")
	}
	if args[0] == "list" {
		if len(args) != 1 {
			return errors.New("usage: board project list")
		}
		projects, err := store.ListProjects()
		if err != nil {
			return err
		}
		for _, project := range projects {
			fmt.Println(project)
		}
		return nil
	}
	if args[0] == "delete" {
		if len(args) != 2 {
			return errors.New("usage: board project delete <name>")
		}
		if err := store.DeleteProject(args[1]); err != nil {
			return err
		}
		fmt.Printf("deleted project %q\n", args[1])
		return nil
	}
	if len(args) != 1 {
		return errors.New("usage: board project <name>")
	}
	return runInit(store, args)
}

func runIssue(store *Store, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: board issue <create|assign|update|list|next> ...")
	}
	switch args[0] {
	case "create":
		return runIssueCreate(store, args[1:])
	case "assign":
		return runIssueAssign(store, args[1:])
	case "update":
		return runIssueUpdate(store, args[1:])
	case "list":
		return runIssueList(store, args[1:])
	case "next":
		return runIssueNext(store, args[1:])
	default:
		return fmt.Errorf("unknown issue command: %s", args[0])
	}
}

func runIssueCreate(store *Store, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: board issue create <project> --title ... --description ... [--assignee ...]")
	}
	project := args[0]
	fs := flag.NewFlagSet("issue create", flag.ContinueOnError)
	title := fs.String("title", "", "issue title")
	description := fs.String("description", "", "issue description")
	assignee := fs.String("assignee", "", "issue assignee")
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board issue create <project> --title ... --description ... [--assignee ...]")
	}
	issue, err := store.CreateIssue(project, *title, *description, *assignee)
	if err != nil {
		return err
	}
	fmt.Printf("created issue %s (%s)\n", issue.ID, issue.Title)
	return nil
}

func runIssueAssign(store *Store, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: board issue assign <project> <issue-id> --assignee ...")
	}
	project := args[0]
	issueID := args[1]
	fs := flag.NewFlagSet("issue assign", flag.ContinueOnError)
	assignee := fs.String("assignee", "", "issue assignee")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board issue assign <project> <issue-id> --assignee ...")
	}
	issue, oldAssignee, err := store.AssignIssue(project, issueID, *assignee)
	if err != nil {
		return err
	}
	fmt.Printf("assigned issue %s from %q to %q\n", issue.ID, oldAssignee, issue.Assignee)
	return nil
}

func runIssueUpdate(store *Store, args []string) error {
	if len(args) < 2 {
		return errors.New("usage: board issue update <project> <issue-id> [--status ...] [--title ...] [--description ...]")
	}
	project := args[0]
	issueID := args[1]
	fs := flag.NewFlagSet("issue update", flag.ContinueOnError)
	title := fs.String("title", "", "new title")
	status := fs.String("status", "", "new status (todo|in_progress|done|cancelled)")
	description := fs.String("description", "", "new description")
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board issue update <project> <issue-id> [--status ...] [--title ...] [--description ...]")
	}

	input := IssueUpdateInput{}
	provided := map[string]bool{}
	fs.Visit(func(f *flag.Flag) {
		provided[f.Name] = true
	})
	if provided["title"] {
		input.Title = title
	}
	if provided["status"] {
		input.Status = status
	}
	if provided["description"] {
		input.Description = description
	}
	if input.Title == nil && input.Status == nil && input.Description == nil {
		return errors.New("at least one of --title, --status, --description is required")
	}

	oldMeta, newMeta, err := store.UpdateIssue(project, issueID, input)
	if err != nil {
		return err
	}
	if oldMeta == newMeta {
		fmt.Printf("no changes for issue %s\n", oldMeta.ID)
		return nil
	}
	fmt.Printf("updated issue %s\n", newMeta.ID)
	return nil
}

func runIssueList(store *Store, args []string) error {
	projectArg, flagArgs := splitOptionalProjectArg(args)
	fs := flag.NewFlagSet("issue list", flag.ContinueOnError)
	status := fs.String("status", "", "filter by status (todo|in_progress|done|cancelled)")
	limit := fs.Int("limit", 0, "max number of issues to show (0 means no limit)")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board issue list [project] [--status ...] [--limit N]")
	}

	project, err := resolveProject(projectArg)
	if err != nil {
		return errors.New("usage: board issue list [project] [--status ...] [--limit N] (or run inside a git repo to auto-detect)")
	}
	issues, err := store.ListIssues(project)
	if err != nil {
		return err
	}
	if *status != "" {
		if !AllowedStatuses[*status] {
			return fmt.Errorf("invalid status %q (allowed: todo, in_progress, done, cancelled)", *status)
		}
		filtered := make([]IssueMeta, 0, len(issues))
		for _, issue := range issues {
			if issue.Status == *status {
				filtered = append(filtered, issue)
			}
		}
		issues = filtered
	}
	if *limit < 0 {
		return errors.New("limit must be >= 0")
	}
	if *limit > 0 && len(issues) > *limit {
		issues = issues[:*limit]
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NUMBER\tID\tSTATUS\tASSIGNEE\tTITLE")
	for _, issue := range issues {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", issue.Number, issue.ID, issue.Status, issue.Assignee, issue.Title)
	}
	return w.Flush()
}

func runIssueNext(store *Store, args []string) error {
	if len(args) > 1 {
		return errors.New("usage: board issue next [project]")
	}
	if len(args) == 1 {
		return runIssueList(store, []string{args[0], "--status", StatusTodo, "--limit", "1"})
	}
	return runIssueList(store, []string{"--status", StatusTodo, "--limit", "1"})
}

func splitOptionalProjectArg(args []string) (project string, flagArgs []string) {
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		return strings.TrimSpace(args[0]), args[1:]
	}
	return "", args
}

func resolveProject(project string) (string, error) {
	project = strings.TrimSpace(project)
	if project != "" {
		return project, nil
	}
	return inferProjectFromGitRepo()
}

func runWatch(store *Store, args []string) error {
	fs := flag.NewFlagSet("watch", flag.ContinueOnError)
	interval := fs.Duration("interval", 2*time.Second, "poll interval (e.g. 2s)")
	hookCmd := fs.String("hook-cmd", "", "shell command to run per event; event JSON is provided via stdin")
	plain := fs.Bool("plain", false, "disable interactive TUI mode")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() > 1 {
		return errors.New("usage: board watch [project] [--interval 2s] [--hook-cmd \"cmd\"] [--plain]")
	}
	project := ""
	if fs.NArg() == 1 {
		project = strings.TrimSpace(fs.Arg(0))
	} else {
		var err error
		project, err = inferProjectFromGitRepo()
		if err != nil {
			return errors.New("usage: board watch [project] [--interval 2s] [--hook-cmd \"cmd\"] [--plain] (or run inside a git repo to auto-detect)")
		}
	}
	if project == "" {
		return errors.New("project is required")
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	fmt.Printf("watching project %q (poll interval %s)\n", project, interval.String())
	if *hookCmd != "" {
		fmt.Printf("hook command enabled: %s\n", *hookCmd)
	}
	if !*plain && isTerminal(os.Stdin) && isTerminal(os.Stdout) {
		return WatchTUI(ctx, store, WatchConfig{
			Project:   project,
			Interval:  *interval,
			HookCmd:   *hookCmd,
			EnableMap: DefaultEnabledEventTypes,
		})
	}
	return Watch(ctx, store, WatchConfig{
		Project:   project,
		Interval:  *interval,
		HookCmd:   *hookCmd,
		EnableMap: DefaultEnabledEventTypes,
	})
}

func printUsage() {
	fmt.Println("board - local trello-like board for agents")
	fmt.Println("")
	fmt.Println("Commands:")
	fmt.Println("  board init [project]          # uses git repo name when omitted")
	fmt.Println("  board project <name>          # alias for init")
	fmt.Println("  board project list")
	fmt.Println("  board project delete <name>")
	fmt.Println("  board update [--repo /path/to/agent-board]")
	fmt.Println("  board issue create <project> --title ... --description ... [--assignee ...]")
	fmt.Println("  board issue assign <project> <issue-id> --assignee ...")
	fmt.Println("  board issue update <project> <issue-id> [--status ...] [--title ...] [--description ...]")
	fmt.Println("  board issue list [project] [--status ...] [--limit N]")
	fmt.Println("  board issue next [project]    # same as --status todo --limit 1")
	fmt.Println("  board watch [project] [--interval 2s] [--hook-cmd \"cmd\"] [--plain]")
}
