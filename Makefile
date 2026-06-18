# subgen developer tasks.
#
#   make generate     — run go generate across the project (mockgen contracts)
#   make lint         — run golangci-lint in Docker, byte-identical to CI (see below)
#   make test         — run the unit tests (-race)
#   make integration  — run the repository integration tests against a real temp
#                       SQLite (-tags integration)
#   make apitest      — run the API tests against real 3x-ui panels in docker
#                       (delegates to apitest/Makefile: up panels → run → down)
#   make all          — generate, then lint + test + integration + apitest
#
# Lint is the SINGLE source of truth for how golangci-lint runs: CI (.github/workflows)
# invokes `make lint` too, so local and CI never diverge. It runs the pinned
# golangci/golangci-lint image on the HOST's native architecture — every enabled linter
# is AST/type-based and identical across 64-bit arches, so linux/amd64 (CI) and
# linux/arm64 (Apple Silicon) give the same result with NO emulation (max speed). Module
# downloads and the build/analysis cache persist in the gitignored .lintcache/, so repeat
# runs are fast (CI persists it with actions/cache). Needs only Docker — no local
# golangci-lint install. If your active Docker context can't bind-mount the repo, pass a
# working one: `DOCKER_CONTEXT=desktop-linux make lint`.

.PHONY: generate lint test integration apitest all

# Single source of truth for the golangci-lint version (CI runs `make lint`, so it is
# pinned only here). Keep in sync with .golangci.yaml's `version: "2"`.
GOLANGCI_LINT_VERSION := v2.12.2
LINT_CACHE            := $(CURDIR)/.lintcache

generate:
	go generate ./...

lint:
	@mkdir -p "$(LINT_CACHE)/cache" "$(LINT_CACHE)/gomod"
	docker run --rm \
		-v "$(CURDIR)":/app -w /app \
		-v "$(LINT_CACHE)/cache":/root/.cache \
		-v "$(LINT_CACHE)/gomod":/go/pkg/mod \
		golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) golangci-lint run ./...

test:
	go test -race ./...

integration:
	go test -tags integration -race -count=1 ./internal/repository/...

apitest:
	$(MAKE) -C apitest test

all: generate lint test integration apitest
