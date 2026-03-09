# Agent Board CLI: Use Cases and Plan

## Product Goal
Build a small Go CLI (`board`) that provides Trello-like issue tracking for agents using local files under `~/.board/<project>/`.

## Core Use Cases
1. Create issue
- As a user, I can run a command to create a new issue in a project.
- Expected outcome: a new `issue_name.md` file appears and project metadata is updated.

2. Assign issue
- As a user, I can assign an issue to an agent/user.
- Expected outcome: assignment is persisted and visible in issue data.

3. Update issue
- As a user, I can update issue status and description.
- Expected outcome: issue markdown content and index metadata stay consistent.

4. Portable execution from anywhere
- As a user, I can run `board` from any directory on my machine.
- Expected outcome: CLI uses `~/.board/<project>/` regardless of current working directory.

5. Watch for changes with notifications/hooks
- As a user, I can run `board watch <project-name>`.
- Expected outcome: on issue created or status change, I get a notification/event.
- Extensibility: a hook pattern lets me plug in custom handlers (desktop notifications, Slack, etc.).

## Data and Storage Use Cases
1. Project structure
- Each project is stored in `~/.board/<project>/`.
- `board.json` exists at the project root.
- Each issue is a sibling markdown file: `<issue_name>.md`.

2. Issue fields
- Each issue contains at least: `Title`, `Status`, `Description`.

3. Source of truth
- Markdown files are human-readable source records.
- `board.json` acts as project index/metadata for quick lookup and watch event correlation.

## CLI Surface (Initial)
- `board issue create <project> --title ... --description ... [--assignee ...]`
- `board issue assign <project> <issue-id-or-slug> --assignee ...`
- `board issue update <project> <issue-id-or-slug> [--status ...] [--description ...] [--title ...]`
- `board watch <project>`

## Delivery Plan
1. Scaffold Go CLI app
- Initialize module, command routing, shared config path resolver (`~/.board`).

2. Define domain models and storage contract
- `Board`, `Issue`, status enum, repository interface.
- Implement filesystem repository for `board.json` + issue markdown files.

3. Implement write commands
- `issue create`, `issue assign`, `issue update`.
- Ensure atomic writes and basic validation for status values.

4. Implement read/list helpers (minimal supporting functionality)
- Internal helpers for loading board and issue lookup by slug/id.

5. Implement watch subsystem
- File watcher on project directory.
- Event mapper for `issue_created` and `issue_status_changed`.
- Hook interface + one default stdout notifier.

6. Add hooks framework
- Define hook payload schema.
- Add command/config option for executable hook targets.

7. Packaging and run-anywhere support
- Build/install instructions (`go install`), PATH guidance.

8. Testing and acceptance checks
- Unit tests for parser/storage/event mapping.
- Basic integration tests for CLI flows.

## Acceptance Criteria (Phase 1)
- Can create, assign, and update issues via CLI.
- Project data persists in `~/.board/<project>/board.json` + `*.md` files.
- `board watch <project>` detects issue creation and status changes.
- At least one hook mechanism is available and documented.
