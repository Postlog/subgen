# subgen developer tasks.
#
#   make lint         — run golangci-lint (config: .golangci.yaml)
#   make test         — run the unit tests (-race)
#   make integration  — run the repository integration tests against a real temp
#                       SQLite (-tags integration)
#   make apitest      — run the API tests against real 3x-ui panels in docker
#                       (delegates to apitest/Makefile: up panels → run → down)
#
# Lint requires golangci-lint v2: https://golangci-lint.run/welcome/install/

.PHONY: lint test integration apitest

lint:
	golangci-lint run ./...

test:
	go test -race ./...

integration:
	go test -tags integration -race -count=1 ./internal/repository/...

apitest:
	$(MAKE) -C apitest test
