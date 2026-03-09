# Contributing to agent-board

Thanks for your interest in contributing.

## Development setup

- **Go** is required. Ensure your Go bin directory is in `PATH`.
- Clone the repo and from the repo root run:
  - `make build` — builds the `board` binary
  - `make test` — runs tests
  - `make install` — installs `board` into your Go bin path

## How to contribute

1. Open an issue or pick one from the board (e.g. `board issue next <project>`).
2. Implement your change, keeping tests and README in mind.
3. Run `make test` before submitting.
4. Submit a pull request with a clear description of the change.

## Code and conventions

- Follow existing style in the codebase.
- Prefer small, focused changes and conventional commit messages.

## Board CLI

This project is a local Trello-like board for agents. See [README.md](README.md) for commands (`board init`, `board issue list`, `board issue next`, etc.) and storage layout.
