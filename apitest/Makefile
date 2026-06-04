# Turnkey API tests for subgen against real 3x-ui panels in docker.
#
#   make test   — up clean panels, fetch tokens, run `go test -tags apitest ./apitest/...`, down
#   make up     — start the panels and print the env to use them manually
#   make down   — stop and remove the panels (+ anonymous volumes)
#
# Requires: docker, docker compose, free ports 13053/13054.
#
# NOTE: the auth, config and sub (SubSuite) areas need NO panels — they boot subgen and
# run on their own. Run just those in plain CI with:
#     go test -tags apitest -count=1 ./apitest/...
# (the panel-gated suites — users, nodes, SubPanelSuite — skip when the panel env is
# unset). The `test` target below adds the panels so the gated suites run too.

COMPOSE := docker compose -f docker-compose.yml
P1_URL  := http://127.0.0.1:13053
P2_URL  := http://127.0.0.1:13054
CURL    := curl -fsS --noproxy '*'

.PHONY: test up down wait tokens

test:
	@$(COMPOSE) up -d
	@$(MAKE) -s wait
	@P1=$$($(COMPOSE) exec -T panel1 /app/x-ui setting -getApiToken | sed -n 's/.*apiToken: *//p' | tr -d '\r'); \
	 P2=$$($(COMPOSE) exec -T panel2 /app/x-ui setting -getApiToken | sed -n 's/.*apiToken: *//p' | tr -d '\r'); \
	 if [ -z "$$P1" ] || [ -z "$$P2" ]; then echo "failed to read api tokens"; $(COMPOSE) down -v; exit 1; fi; \
	 echo "panels ready; running tests"; \
	 ( cd .. && \
	   SUBGEN_APITEST_PANEL1_URL=$(P1_URL) SUBGEN_APITEST_PANEL1_TOKEN=$$P1 \
	   SUBGEN_APITEST_PANEL2_URL=$(P2_URL) SUBGEN_APITEST_PANEL2_TOKEN=$$P2 \
	   go test -tags apitest -count=1 ./apitest/... ); \
	 status=$$?; \
	 $(COMPOSE) down -v >/dev/null 2>&1 || true; \
	 exit $$status

up:
	$(COMPOSE) up -d
	@$(MAKE) -s wait
	@P1=$$($(COMPOSE) exec -T panel1 /app/x-ui setting -getApiToken | sed -n 's/.*apiToken: *//p' | tr -d '\r'); \
	 P2=$$($(COMPOSE) exec -T panel2 /app/x-ui setting -getApiToken | sed -n 's/.*apiToken: *//p' | tr -d '\r'); \
	 echo "panels up. To run tests manually:"; \
	 echo "  cd .. && SUBGEN_APITEST_PANEL1_URL=$(P1_URL) SUBGEN_APITEST_PANEL1_TOKEN=$$P1 \\"; \
	 echo "    SUBGEN_APITEST_PANEL2_URL=$(P2_URL) SUBGEN_APITEST_PANEL2_TOKEN=$$P2 \\"; \
	 echo "    go test -tags apitest -count=1 ./apitest/..."

down:
	$(COMPOSE) down -v

wait:
	@echo "waiting for panels on 13053/13054..."
	@for i in $$(seq 1 90); do \
	  if $(CURL) $(P1_URL)/ >/dev/null 2>&1 && $(CURL) $(P2_URL)/ >/dev/null 2>&1; then exit 0; fi; \
	  sleep 1; \
	done; \
	echo "panels did not become ready in time"; $(COMPOSE) logs --tail=20; exit 1
