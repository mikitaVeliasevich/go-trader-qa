.PHONY: build dev run test

build:
	go build -o bin/gtqa ./cmd/gtqa
	go build -o bin/gtqa-server ./cmd/gtqa-server

# Dev server with auto-rebuild on .go changes (like go run, but watches files).
# Requires: go install github.com/air-verse/air@latest
dev:
	@command -v air >/dev/null 2>&1 || { \
		echo "air not found. Install: go install github.com/air-verse/air@latest"; \
		echo "Or run once without watch: make run"; \
		exit 1; \
	}
	air

# One-shot dev start (no file watcher). Re-run after Go API changes.
run:
	go run ./cmd/gtqa-server

test:
	go test ./...
