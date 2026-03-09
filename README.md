# agent-board

[![Go 1.24+](https://img.shields.io/badge/Go-1.24+-00ADD8?logo=go)](https://go.dev/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)
[![Release](https://img.shields.io/github/v/release/SimonJTurner/board?include_prereleases)](https://github.com/SimonJTurner/board/releases)

Small local Go CLI for Trello-style issue tracking for agent workflows.

## Quick start

One-line install (no Go or repo clone required):

```bash
curl -fsSL https://raw.githubusercontent.com/SimonJTurner/board/main/scripts/install.sh | sh
```

Then:

```bash
board init
board issue next
```

Other options: install from this repo (requires [Go 1.24+](#requirements)) with `go install ./cmd/board`, or [download a release](https://github.com/SimonJTurner/board/releases) and put the binary in your PATH.

## Requirements

- **Go 1.24+** (see [go.mod](go.mod))

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
- `board project list [--archived]`
- `board project delete <name>`
- `board project archive <name>`
- `board update [--repo /path/to/board] [--release-repo SimonJTurner/board]`
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

## Build/test/install helpers
Run `make` targets instead of remembering each Go flag.

```bash
make build    # builds ./cmd/board -> board
make test     # runs go test ./...
make install  # runs go install ./cmd/board
make update   # runs board update
make release        # tags/pushes a new semantic version (default patch bump)
make release-major  # bump major version (can still override via MAJOR/MINOR/PATCH)
make release-minor  # bump minor version (can still override via MAJOR/MINOR/PATCH)
```
Ensure your Go bin directory is in `PATH`.

## Update installed executable
From inside this repo:
```bash
board update
```

From anywhere via releases:
```bash
BOARD_RELEASE_REPO=SimonJTurner/board board update
```
Set `BOARD_DEBUG=1` to show underlying errors when a project or issue is not found (default is a short hint, e.g. run `board init <project>`).

Or point directly at the repo:
```bash
board update --repo /path/to/board
```

## GitHub releases
Pushing a semantic version tag (e.g. `v0.2.0`) triggers `.github/workflows/release.yml`, which creates a GitHub Release with auto-generated release notes and uploads the built binaries. Asset names are `board-<GOOS>-<GOARCH>` (with `.exe` on Windows), e.g.:
```
board-linux-amd64
board-linux-arm64
board-darwin-amd64
board-darwin-arm64
board-windows-amd64.exe
```

Install by downloading the matching asset and moving it into your `PATH`:
```bash
curl -L https://github.com/SimonJTurner/board/releases/latest/download/board-$(go env GOOS)-$(go env GOARCH) -o board
chmod +x board
mv board /usr/local/bin/
```

The release workflow runs `go test ./...`, cross-compiles these artifacts, and uploads them for every pushed tag in `.github/workflows/release.yml`. With releases published, `board update` without `--repo` pulls the right binary from GitHub automatically (or you can pass `BOARD_RELEASE_REPO` to target a different repo).

`make release` / `scripts/release.sh` now rounds up the latest `vX.Y.Z` tag, bumps patch, tags and pushes. Use `make release-minor` or `make release-major` to bump the minor/major version before tagging. Override the next version manually with `MAJOR`, `MINOR`, or `PATCH`, e.g. `MAJOR=1 PATCH=0 make release`.

Project archives live in `~/.board/_archive` so they are skipped from `board project list` unless you pass `--archived`. The new `board project archive <name>` command moves the project into that folder for safekeeping.

## License

This project is licensed under the MIT License — see the [LICENSE](LICENSE) file for details.
