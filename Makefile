GO_BUILD=go build -o board ./cmd/board
GO_TEST=go test ./...
GO_INSTALL=go install ./cmd/board

.PHONY: all build test install update release
all: test build

build:
	$(GO_BUILD)

test:
	$(GO_TEST)

install:
	$(GO_INSTALL)

update:
	board update

release:
	@echo "Create GitHub release tag and push; webhook workflow builds binaries."
