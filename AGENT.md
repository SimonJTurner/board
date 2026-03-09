# Agent usage guide

This document describes how to use the `board` CLI from scripts and AI agents. Use `--json` for machine-readable output and rely on project inference when running inside a git repo.

## Project inference

When run from inside a git repository, these commands infer the project from the repo folder name. You can omit the project argument:

- `board issue create`
- `board issue list`
- `board issue next`
- `board issue show`
- `board issue assign`
- `board issue update`
- `board watch`

Example: from `~/code/my-app` (git root), `board issue next` uses project `my-app`.

## JSON output (`--json`)

Append `--json` to get structured JSON on stdout. Errors still go to stderr; exit code 0 means success.

| Command | JSON shape |
|--------|------------|
| `issue create` | Single `IssueMeta` object |
| `issue assign` | Single `IssueMeta` object |
| `issue update` | Single `IssueMeta` object |
| `issue list` | Array of `IssueMeta` |
| `issue next` | Array of 0 or 1 `IssueMeta` |
| `issue show` | Single object: `IssueMeta` fields plus `description`, `created_at`, `updated_at`, `file` |

`IssueMeta` fields: `id`, `number`, `file`, `title`, `status`, `assignee`, `updated_at`.

## Typical agent flows

### Create an issue

```bash
board issue create --title "Implement login" --description "Add OAuth flow" --json
```

With explicit project:

```bash
board issue create my-project --title "Implement login" --description "Add OAuth flow" --json
```

### Get the next todo issue (pull one)

```bash
board issue next --json
```

Returns an array of 0 or 1 issue. Parse the first element for `id`, `title`, `description` (use `issue show` for full body).

### Show full issue by ID

```bash
board issue show <issue-id> --json
```

Returns full issue including `description`, `file` path, and timestamps.

### Assign and start working

```bash
board issue assign <issue-id> --assignee "agent-1" --status in_progress --json
```

### Update status or fields

```bash
board issue update <issue-id> --status done --json
board issue update <issue-id> --description "Updated body" --json
```

### List issues (filtered)

```bash
board issue list --status todo --limit 5 --json
board issue list --status in_progress --json
```

## Exit codes and stderr

- **0**: success; with `--json`, JSON is on stdout only.
- **Non-zero**: failure; error message on stderr. Do not parse stdout as JSON.

## One-shot workflow example

```bash
# From repo root
board init                                    # once per project
ISSUE=$(board issue next --json | jq -r '.[0].id')
board issue show "$ISSUE" --json              # get full issue
board issue assign "$ISSUE" --assignee "bot" --status in_progress --json
# ... do work ...
board issue update "$ISSUE" --status done --json
```
