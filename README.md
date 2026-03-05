# agent-board

Small local Go CLI for Trello-style issue tracking for agent workflows.

## Storage
Each project lives at:

`~/.board/<project>/`

Files:
- `board.json` (metadata index only)
- `<PROJECT_SLUG>_<NUMBER>_<TITLE_SLUG>.md` issue files

Example issue filename:
- `FOO_1001_create_some_feature.md`

## Issue fields
Issue markdown stores:
- `Title`
- `Status`
- `Description`
- `Assignee`
- timestamps

Allowed statuses:
- `todo`
- `in_progress`
- `done`
- `cancelled`

Transitions are unrestricted.

## Commands
- `board init [project]` (if omitted, uses current git repo folder name)
- `board project <name>` (alias for init)
- `board project list`
- `board project delete <name>`
- `board update [--repo /path/to/agent-board] [--release-repo owner/repo]`
- `board issue create <project> --title "..." --description "..." [--assignee "..."]`
- `board issue assign <project> <issue-id> --assignee "..."`
- `board issue update <project> <issue-id> [--status ...] [--title ...] [--description ...]`
- `board issue list [project] [--status <status>] [--limit <N>]` (if omitted, uses current git repo folder name)
- `board issue next [project]` (same as `issue list --status todo --limit 1`)
- `board watch [project] [--interval 2s] [--hook-cmd "your-command"] [--plain]` (if omitted, uses current git repo folder name)

## Watch behavior
By default, `watch` launches an interactive TUI:
- `j` / `k` moves selection
- `Enter` opens issue details; in details view, `Enter` opens the issue file in `$EDITOR` (fallback `vim`)
- `b` goes back from details
- `q` quits

The bottom footer is sticky and always shows remaining `todo` count.

Use `--plain` to disable TUI and print one line per event.

Hooks still work in both modes: `--hook-cmd` receives JSON event payload on stdin.

Event types are defined in `internal/board/types.go` via constants and `DefaultEnabledEventTypes`:
- `issue_created`
- `issue_status_changed`
- `issue_assignee_changed`
- `issue_title_changed`
- `issue_description_changed`

Hook failures are best-effort and logged; they do not stop watch execution.

Note: watch currently diffs `board.json` metadata. Manual edits directly to issue markdown files may not be detected unless metadata is also updated through CLI commands.

## Build
```bash
go build -o board ./cmd/board
```

## Install (run from anywhere)
```bash
go install ./cmd/board
```
Ensure your Go bin directory is in `PATH`.

## Update installed executable
From inside this repo:
```bash
board update
```

From anywhere via releases:
```bash
BOARD_RELEASE_REPO=owner/repo board update
```

Or point directly at the repo:
```bash
board update --repo /path/to/agent-board
```

## GitHub releases
Create a GitHub release (semantic tag such as `v0.2.0`) and upload the binaries produced by `.github/workflows/release.yml`. Assets must be named `board-<GOOS>-<GOARCH>` (with `.exe` on Windows), e.g.:
```
board-linux-amd64
board-linux-arm64
board-darwin-amd64
board-darwin-arm64
board-windows-amd64.exe
```

Install by downloading the matching asset and moving it into your `PATH`:
```bash
curl -L https://github.com/<owner>/agent-board/releases/latest/download/board-$(go env GOOS)-$(go env GOARCH) -o board
chmod +x board
mv board /usr/local/bin/
```

The release workflow runs `go test ./...`, cross-compiles these artifacts, and uploads them for every pushed tag in `.github/workflows/release.yml`. With releases published, `board update` without `--repo` pulls the right binary from GitHub automatically (or you can pass `BOARD_RELEASE_REPO` to target a different repo).
