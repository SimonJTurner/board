package board

import (
	"context"
	"encoding/json"
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
	case "completion":
		return runCompletion(store, args[1:])
	case "__complete":
		return runComplete(store, args[1:])
	case "help", "-h", "--help":
		printUsage()
		return nil
	default:
		return fmt.Errorf("unknown command: %s", args[0])
	}
}

func outputJSON(v interface{}) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
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
		return errors.New("usage: board project <list|delete|archive> ...")
	}
	switch args[0] {
	case "list":
		return runProjectList(store, args[1:])
	case "delete":
		if len(args) != 2 {
			return errors.New("usage: board project delete <name>")
		}
		if err := store.DeleteProject(args[1]); err != nil {
			return err
		}
		fmt.Printf("deleted project %q\n", args[1])
		return nil
	case "archive":
		if len(args) != 2 {
			return errors.New("usage: board project archive <name>")
		}
		if err := store.ArchiveProject(args[1]); err != nil {
			return err
		}
		fmt.Printf("archived project %q\n", args[1])
		return nil
	default:
		return fmt.Errorf("unknown project command: %s", args[0])
	}
}

func runProjectList(store *Store, args []string) error {
	fs := flag.NewFlagSet("project list", flag.ContinueOnError)
	includeArchived := fs.Bool("archived", false, "include archived projects")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board project list [--archived]")
	}
	projects, err := store.ListProjects(*includeArchived)
	if err != nil {
		return err
	}
	for _, project := range projects {
		fmt.Println(project)
	}
	return nil
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
	case "show":
		return runIssueShow(store, args[1:])
	default:
		return fmt.Errorf("unknown issue command: %s", args[0])
	}
}

func runIssueCreate(store *Store, args []string) error {
	projectArg, flagArgs := splitOptionalProjectArg(args)
	fs := flag.NewFlagSet("issue create", flag.ContinueOnError)
	title := fs.String("title", "", "issue title")
	description := fs.String("description", "", "issue description")
	assignee := fs.String("assignee", "", "issue assignee")
	outputJSONFlag := fs.Bool("json", false, "output machine-readable JSON (for agents)")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board issue create [project] --title ... --description ... [--assignee ...] [--json]")
	}
	project, err := resolveProject(projectArg)
	if err != nil {
		return errors.New("usage: board issue create [project] --title ... --description ... (or run inside a git repo to auto-detect project)")
	}
	issue, err := store.CreateIssue(project, *title, *description, *assignee)
	if err != nil {
		return err
	}
	if *outputJSONFlag {
		return outputJSON(issue)
	}
	fmt.Printf("created issue %s (%s)\n", issue.ID, issue.Title)
	return nil
}

func runIssueAssign(store *Store, args []string) error {
	if len(args) < 1 {
		return errors.New("usage: board issue assign [project] <issue-id> --assignee ... [--status ...] [--json]")
	}
	// One or two positional args before flags: [project] <issue-id> or <issue-id>
	projectArg := ""
	issueID := ""
	if len(args) >= 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		projectArg = args[0]
		issueID = args[1]
		args = args[2:]
	} else if len(args) >= 1 && !strings.HasPrefix(args[0], "-") {
		issueID = args[0]
		args = args[1:]
	}
	if issueID == "" {
		return errors.New("usage: board issue assign [project] <issue-id> --assignee ... [--status ...] [--json]")
	}
	project, err := resolveProject(projectArg)
	if err != nil {
		return errors.New("usage: board issue assign [project] <issue-id> --assignee ... (or run inside a git repo to auto-detect project)")
	}
	fs := flag.NewFlagSet("issue assign", flag.ContinueOnError)
	assignee := fs.String("assignee", "", "issue assignee")
	status := fs.String("status", "", "override status (default: in_progress)")
	outputJSONFlag := fs.Bool("json", false, "output machine-readable JSON (for agents)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board issue assign [project] <issue-id> --assignee ... [--status ...] [--json]")
	}
	var statusPtr *string
	if *status != "" {
		statusPtr = status
	}
	issue, oldAssignee, err := store.AssignIssue(project, issueID, *assignee, statusPtr)
	if err != nil {
		return err
	}
	if *outputJSONFlag {
		return outputJSON(issue)
	}
	fmt.Printf("assigned issue %s from %q to %q\n", issue.ID, oldAssignee, issue.Assignee)
	return nil
}

func runIssueUpdate(store *Store, args []string) error {
	if len(args) < 1 {
		return errors.New("usage: board issue update [project] <issue-id> [--status ...] [--title ...] [--description ...] [--assignee ...] [--json]")
	}
	projectArg := ""
	issueID := ""
	if len(args) >= 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		projectArg = args[0]
		issueID = args[1]
		args = args[2:]
	} else if len(args) >= 1 && !strings.HasPrefix(args[0], "-") {
		issueID = args[0]
		args = args[1:]
	}
	if issueID == "" {
		return errors.New("usage: board issue update [project] <issue-id> [--status ...] [--title ...] [--description ...] [--assignee ...] [--json]")
	}
	project, err := resolveProject(projectArg)
	if err != nil {
		return errors.New("usage: board issue update [project] <issue-id> ... (or run inside a git repo to auto-detect project)")
	}
	fs := flag.NewFlagSet("issue update", flag.ContinueOnError)
	title := fs.String("title", "", "new title")
	status := fs.String("status", "", "new status (todo|in_progress|done|cancelled)")
	description := fs.String("description", "", "new description")
	assignee := fs.String("assignee", "", "new assignee")
	outputJSONFlag := fs.Bool("json", false, "output machine-readable JSON (for agents)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board issue update [project] <issue-id> [--status ...] [--title ...] [--description ...] [--assignee ...] [--json]")
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
	if provided["assignee"] {
		input.Assignee = assignee
	}
	if input.Title == nil && input.Status == nil && input.Description == nil && input.Assignee == nil {
		return errors.New("at least one of --title, --status, --description, --assignee is required")
	}

	oldMeta, newMeta, err := store.UpdateIssue(project, issueID, input)
	if err != nil {
		return err
	}
	if *outputJSONFlag {
		return outputJSON(newMeta)
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
	outputJSONFlag := fs.Bool("json", false, "output machine-readable JSON (for agents)")
	if err := fs.Parse(flagArgs); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board issue list [project] [--status ...] [--limit N] [--json]")
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

	if *outputJSONFlag {
		return outputJSON(issues)
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
	fmt.Fprintln(w, "NUMBER\tID\tSTATUS\tASSIGNEE\tTITLE")
	for _, issue := range issues {
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n", issue.Number, issue.ID, issue.Status, issue.Assignee, issue.Title)
	}
	return w.Flush()
}

func runIssueNext(store *Store, args []string) error {
	projectArg, rest := splitOptionalProjectArg(args)
	if len(rest) > 0 && rest[0] != "--json" {
		return errors.New("usage: board issue next [project] [--json]")
	}
	jsonFlag := false
	for i := 0; i < len(rest); i++ {
		if rest[i] == "--json" {
			jsonFlag = true
			break
		}
	}
	listArgs := []string{}
	if projectArg != "" {
		listArgs = append(listArgs, projectArg)
	}
	listArgs = append(listArgs, "--status", StatusTodo, "--limit", "1")
	if jsonFlag {
		listArgs = append(listArgs, "--json")
	}
	return runIssueList(store, listArgs)
}

func runIssueShow(store *Store, args []string) error {
	if len(args) == 0 {
		return errors.New("usage: board issue show [project] <issue-id> [--json]")
	}
	// args could be: [project, issueID, --json] or [issueID, --json] (issueID might look like FOO_1001_title)
	projectArg := ""
	issueID := ""
	rest := args
	if len(args) >= 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		projectArg = args[0]
		issueID = args[1]
		rest = args[2:]
	} else if len(args) >= 1 && !strings.HasPrefix(args[0], "-") {
		issueID = args[0]
		rest = args[1:]
	}
	if issueID == "" {
		return errors.New("usage: board issue show [project] <issue-id> [--json]")
	}
	fs := flag.NewFlagSet("issue show", flag.ContinueOnError)
	outputJSONFlag := fs.Bool("json", false, "output machine-readable JSON (for agents)")
	if err := fs.Parse(rest); err != nil {
		return err
	}
	if fs.NArg() != 0 {
		return errors.New("usage: board issue show [project] <issue-id> [--json]")
	}
	project, err := resolveProject(projectArg)
	if err != nil {
		return errors.New("usage: board issue show [project] <issue-id> (or run inside a git repo to auto-detect project)")
	}
	doc, meta, err := store.GetIssue(project, issueID)
	if err != nil {
		return err
	}
	if *outputJSONFlag {
		out := IssueShowOutput{IssueDoc: doc, File: meta.File}
		return outputJSON(out)
	}
	fmt.Printf("id: %s\n", doc.ID)
	fmt.Printf("number: %d\n", doc.Number)
	fmt.Printf("title: %s\n", doc.Title)
	fmt.Printf("status: %s\n", doc.Status)
	fmt.Printf("assignee: %s\n", doc.Assignee)
	fmt.Printf("file: %s\n", meta.File)
	fmt.Printf("created_at: %s\n", doc.CreatedAt.Format(time.RFC3339))
	fmt.Printf("updated_at: %s\n", doc.UpdatedAt.Format(time.RFC3339))
	fmt.Println("---")
	fmt.Println(doc.Description)
	return nil
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
	fmt.Println("  board project list [--archived]")
	fmt.Println("  board project delete <name>")
	fmt.Println("  board project archive <name>")
	fmt.Println("  board update [--repo /path]  # default: GitHub release; --repo for local build")
	fmt.Println("  board issue create [project] --title ... --description ... [--assignee ...] [--json]")
	fmt.Println("  board issue assign [project] <issue-id> --assignee ... [--status ...] [--json]")
	fmt.Println("  board issue update [project] <issue-id> [--status ...] [--title ...] [--description ...] [--assignee ...] [--json]")
	fmt.Println("  board issue list [project] [--status ...] [--limit N] [--json]")
	fmt.Println("  board issue next [project] [--json]   # same as list --status todo --limit 1")
	fmt.Println("  board issue show [project] <issue-id> [--json]")
	fmt.Println("  board watch [project] [--interval 2s] [--hook-cmd \"cmd\"] [--plain]")
	fmt.Println("  board completion <bash|zsh>   # print shell completion script")
}
