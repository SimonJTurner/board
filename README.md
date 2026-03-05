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
- `board update [--repo /path/to/agent-board]`
- `board issue create <project> --title "..." --description "..." [--assignee "..."]`
- `board issue assign <project> <issue-id> --assignee "..."`
- `board issue update <project> <issue-id> [--status ...] [--title ...] [--description ...]`
- `board issue list [project] [--status <status>] [--limit <N>]` (if omitted, uses current git repo folder name)
- `board issue next [project]` (same as `issue list --status todo --limit 1`)
- `board watch [project] [--interval 2s] [--hook-cmd "your-command"]` (if omitted, uses current git repo folder name)

## Watch behavior
`watch` emits JSON events to stdout and optionally invokes `--hook-cmd` with the same JSON payload on stdin.

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

From anywhere:
```bash
board update --repo /path/to/agent-board
```

Or set once:
```bash
export BOARD_REPO=/path/to/agent-board
board update
```
