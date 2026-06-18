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
# Lint runs the SAME pinned linux/amd64 golangci-lint image GitHub Actions uses
# (golangci/golangci-lint, version in .github/workflows/ci.yml) — so `make lint` and CI
# never disagree. A host golangci-lint (e.g. darwin/arm64) can differ from CI's
# linux/amd64 binary on some linters (wsl_v5 has bitten us). Needs only Docker; no local
# golangci-lint install. If your active Docker context can't bind-mount the repo, pass a
# working one on the command line, e.g.: `DOCKER_CONTEXT=desktop-linux make lint`.

.PHONY: generate lint test integration apitest all

# Keep in sync with the golangci-lint-action version pinned in .github/workflows/ci.yml.
GOLANGCI_LINT_VERSION := v2.12.2

generate:
	go generate ./...

lint:
	docker run --rm --platform linux/amd64 \
		-v "$(CURDIR)":/app -w /app \
		golangci/golangci-lint:$(GOLANGCI_LINT_VERSION) golangci-lint run ./...

test:
	go test -race ./...

integration:
	go test -tags integration -race -count=1 ./internal/repository/...

apitest:
	$(MAKE) -C apitest test

all: generate lint test integration apitest
