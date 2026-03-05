# Subagent Output: High-Level Architecture for Agent Board CLI

## Scope
Design a simple, local-first Go CLI that manages project issues in markdown files with board metadata in JSON, plus a watcher + hook extension model.

## System Overview
The system has 5 layers:
1. CLI Layer
- Parses commands/flags and invokes application services.

2. Application Layer
- Implements use-case logic: create issue, assign issue, update issue, watch board.

3. Domain Layer
- Core entities (`Board`, `Issue`, `IssueStatus`) and domain rules (status transitions, required fields).

4. Infrastructure Layer
- Filesystem repository for board/issue persistence.
- File watcher for change detection.
- Hook dispatcher for outgoing integrations.

5. Interface Adapters
- Markdown serializer/parser for issue files.
- JSON serializer/parser for `board.json`.
- Notification adapters (stdout initially, external hooks next).

## Proposed Directory Layout
```text
cmd/board/main.go
internal/cli/
  root.go
  issue_create.go
  issue_assign.go
  issue_update.go
  watch.go
internal/app/
  service.go
  issue_service.go
  watch_service.go
internal/domain/
  board.go
  issue.go
  events.go
internal/store/
  fs_store.go
  markdown_codec.go
  board_json_codec.go
internal/watch/
  watcher.go
  event_mapper.go
internal/hooks/
  dispatcher.go
  stdout_hook.go
  exec_hook.go
```

## Data Model
### `board.json`
Purpose: board index + fast lookup metadata.

Suggested shape:
```json
{
  "project": "my-project",
  "version": 1,
  "issues": [
    {
      "id": "ISSUE-0001",
      "slug": "fix-login-timeout",
      "file": "fix-login-timeout.md",
      "title": "Fix login timeout",
      "status": "todo",
      "assignee": "agent-1",
      "updated_at": "2026-03-05T14:00:00Z"
    }
  ]
}
```

### Issue markdown (`<slug>.md`)
Use front matter + body to keep files human-editable and machine-parseable.

```md
---
id: ISSUE-0001
title: Fix login timeout
status: todo
assignee: agent-1
updated_at: 2026-03-05T14:00:00Z
---
Investigate timeout regression in auth middleware.
```

This satisfies your required fields (`Title`, `Status`, `Description`) while allowing assignment and timestamps.

## Command Flow Design
1. `issue create`
- Validate input.
- Generate ID + slug.
- Write markdown file (atomic temp file + rename).
- Update `board.json` (atomic).
- Emit domain event: `issue_created`.

2. `issue assign`
- Resolve issue by id/slug.
- Update assignee in markdown + board index.
- Emit `issue_assigned`.

3. `issue update`
- Resolve issue.
- Apply patch (status/title/description).
- If status changed, emit `issue_status_changed`; otherwise `issue_updated`.

4. `watch <project>`
- Watch `~/.board/<project>/` for create/write/rename.
- Debounce burst writes.
- Diff old/new issue snapshots.
- Emit normalized events to hook dispatcher.

## Watch + Hook Pattern
Hook interface:
```go
type Hook interface {
    Handle(ctx context.Context, ev Event) error
}
```

Dispatch model:
- Built-in `stdout` hook (default).
- `exec` hook runs external command with event payload over stdin JSON.
- Future adapters (Slack, desktop notification) can implement same interface.

Event payload example:
```json
{
  "type": "issue_status_changed",
  "project": "my-project",
  "issue_id": "ISSUE-0001",
  "slug": "fix-login-timeout",
  "old_status": "todo",
  "new_status": "in_progress",
  "timestamp": "2026-03-05T14:10:00Z"
}
```

## Operational Concerns
- Concurrency: serialize writes with file lock per project.
- Durability: atomic writes for markdown and JSON.
- Recovery: rebuild `board.json` from markdown files if index is missing/corrupt.
- Portability: only depends on user home directory; no cwd coupling.

## Milestone Breakdown
1. MVP storage + issue commands
2. Watch command with stdout events
3. Exec hook integration
4. Hardening (locking, recovery, tests)

## Architecture Decision Summary
- Local filesystem over DB: aligns with simplicity + transparency goals.
- Markdown as issue source: human-readable and easy to edit.
- JSON index: faster lookup/watch diffing than scanning/parsing all files every command.
- Event-driven hooks: clean path to notifs/Slack without coupling core logic.
