# subgen developer tasks.
#
#   make generate     — run go generate across the project (mockgen contracts)
#   make lint         — run golangci-lint (config: .golangci.yaml)
#   make test         — run the unit tests (-race)
#   make integration  — run the repository integration tests against a real temp
#                       SQLite (-tags integration)
#   make apitest      — run the API tests against real 3x-ui panels in docker
#                       (delegates to apitest/Makefile: up panels → run → down)
#   make all          — generate, then lint + test + integration + apitest
#   make hooks        — install the tracked git hooks (core.hooksPath = .githooks);
#                       the pre-push hook runs `make all` before every push
#
# Lint requires golangci-lint v2: https://golangci-lint.run/welcome/install/

.PHONY: generate lint test integration apitest all hooks

generate:
	go generate ./...

lint:
	golangci-lint run ./...

test:
	go test -race ./...

integration:
	go test -tags integration -race -count=1 ./internal/repository/...

apitest:
	$(MAKE) -C apitest test

all: generate lint test integration apitest

hooks:
	git config core.hooksPath .githooks
	@echo "git hooks installed (core.hooksPath = .githooks); pre-push runs 'make all'"
