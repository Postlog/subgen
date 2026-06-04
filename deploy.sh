#!/usr/bin/env bash
# Deploy subgen as a Docker container: build the image locally, ship it to the
# server (docker save | ssh | docker load — no registry needed), refresh the repo
# there (for docker-compose.yml + .env) and (re)start via docker compose. The node
# needs Docker, but NOT Go. You'll be prompted for the server's sudo password.
#
# Cleanly migrates from the old systemd deploy: the SQLite store stays at
# db/subgen.db (the compose ./db bind mount, chowned to the image's nonroot
# uid), and the old systemd unit is stopped + disabled.
#
# This does NOT push to git — commit and `git push` yourself when you want origin
# updated. The server's `git pull` only refreshes docker-compose.yml.
#
# Usage:
#   ./deploy.sh
#   SSH_HOST=ru1.freedom.postlog.ru SSH_PORT=61022 SSH_USER=server ./deploy.sh
#
# One-time server prerequisites:
#   - Docker installed; the deploy user is in the `docker` group (so `docker` runs
#     without sudo — the save|load pipe is non-interactive).
#   - git clone the repo to ~/subgen
#   - cp .env.example .env and fill it in (set SUBGEN_LISTEN=0.0.0.0:2097),
#     chmod 600.
set -euo pipefail

SSH_HOST="${SSH_HOST:-ru1.freedom.postlog.ru}"
SSH_PORT="${SSH_PORT:-61022}"
SSH_USER="${SSH_USER:-server}"
REPO_DIR="${REPO_DIR:-subgen}"   # relative to the remote $HOME
BRANCH="${BRANCH:-main}"
IMAGE="${IMAGE:-subgen:latest}"
HERE="$(cd "$(dirname "$0")" && pwd)"

echo "[1/4] build image $IMAGE (linux/amd64, static → distroless)"
docker build --platform linux/amd64 -t "$IMAGE" "$HERE"

echo "[2/4] ship image to $SSH_HOST (docker save | ssh | docker load)"
docker save "$IMAGE" | gzip | \
  ssh -p "$SSH_PORT" -o ConnectTimeout=15 "$SSH_USER@$SSH_HOST" 'gunzip | docker load'

echo "[3/4] refresh repo, cut over from systemd, start container"
ssh -t -p "$SSH_PORT" "$SSH_USER@$SSH_HOST" "REPO_DIR='$REPO_DIR' BRANCH='$BRANCH' bash -s" <<'REMOTE'
set -euo pipefail
cd "$HOME/$REPO_DIR"
git fetch origin "$BRANCH" && git checkout "$BRANCH" 2>/dev/null || true
git pull --ff-only origin "$BRANCH" || echo "(repo not fast-forwarded; using shipped image + existing compose)"

if [ ! -f .env ]; then
  echo "ERROR: .env is missing. Create it from .env.example and re-run." >&2
  exit 1
fi

# Persisted store: keep it where the systemd deploy had it (db) and make it
# writable by the image's nonroot user (uid 65532).
mkdir -p db
sudo chown -R 65532:65532 db

echo "== start container =="
docker compose up -d --no-build   # uses the image loaded in step 2
sleep 2
docker compose ps
REMOTE

echo "[4/4] Done."
