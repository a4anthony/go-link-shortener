#!/usr/bin/env bash
#
# Host-agnostic production deploy. This is the SINGLE source of truth for how a
# deploy happens — Ploi, a manual SSH session, or a future CI all just invoke
# `bash scripts/deploy.sh`. No deploy logic lives in any host's UI, so switching
# hosts means pointing a different trigger at this same script — zero rewrite.
#
# Idempotent: handles both the first deploy and updates. `docker compose up`
# creates what's missing and recreates only services whose image changed. There
# is no separate migrate step — the app applies its embedded golang-migrate
# migrations on startup (see internal/repository/migrate.go), gated behind
# Postgres becoming healthy, so it never races an unmigrated schema.
#
# Usage:
#   bash scripts/deploy.sh                    # build + up (app self-migrates)
#   DEPLOY_BRANCH=main bash scripts/deploy.sh # pin the branch to reset to
#   DEPLOY_PULL=0 bash scripts/deploy.sh      # skip git fetch (deploy current tree)
#
# Env knobs (all optional, sane defaults):
#   DEPLOY_BRANCH     branch to hard-reset to before building   (default: main)
#   DEPLOY_PULL       "1" fetch+reset to origin, "0" skip        (default: 1)
#   DEPLOY_CACHE_KEEP build-cache size cap for the prunes        (default: 6GB)
#   ENV_FILE          env file passed to compose                 (default: .env)
#   COMPOSE_FILE      compose file                               (default: docker-compose.prod.yml)
set -euo pipefail

# Resolve the repo root from this script's location, so the deploy works no
# matter what directory the host's hook cd'd into.
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

DEPLOY_BRANCH="${DEPLOY_BRANCH:-main}"
DEPLOY_PULL="${DEPLOY_PULL:-1}"
DEPLOY_CACHE_KEEP="${DEPLOY_CACHE_KEEP:-6GB}"
ENV_FILE="${ENV_FILE:-.env}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"

# Drop dangling images and cap the build cache BY SIZE. Called twice: BEFORE
# the build (so a deploy starts with headroom on a size-constrained host — a
# build that dies on a full disk aborts the script before the post-deploy prune
# can ever reclaim anything, which strands the host full) and AFTER the swap (to
# collect the images the deploy just superseded). `--max-used-space` evicts
# least-recently-used cache first, so the layers this build is about to reuse
# survive the cap. The cap flag was renamed across Docker versions
# (--keep-storage → --max-used-space); try the new flag, then the old, then a
# plain prune — never silently no-op, which would let the disk creep back to full.
reclaim_disk() {
  docker image prune -f >/dev/null 2>&1 || true
  docker builder prune -f --max-used-space "$DEPLOY_CACHE_KEEP" >/dev/null 2>&1 \
    || docker builder prune -f --keep-storage "$DEPLOY_CACHE_KEEP" >/dev/null 2>&1 \
    || docker builder prune -f >/dev/null 2>&1 \
    || true
}

# Fail loudly + early on the most common misconfiguration.
if [[ ! -f "$ENV_FILE" ]]; then
  echo "✗ $ENV_FILE not found in $ROOT_DIR — copy .env.prod.example and fill it in." >&2
  exit 1
fi
if ! command -v docker >/dev/null 2>&1; then
  echo "✗ docker not found on PATH. Install Docker on this host first." >&2
  exit 1
fi

COMPOSE=(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE")

if [[ "$DEPLOY_PULL" == "1" ]]; then
  echo "→ Fetching origin/$DEPLOY_BRANCH"
  git fetch --prune origin
  git reset --hard "origin/$DEPLOY_BRANCH"
  git submodule update --init --recursive 2>/dev/null || true
else
  echo "→ Skipping git fetch (DEPLOY_PULL=0) — deploying the current working tree"
fi

echo "→ Reclaiming disk before build (cache cap: $DEPLOY_CACHE_KEEP)"
reclaim_disk

echo "→ Building images (only changed layers rebuild)"
"${COMPOSE[@]}" build

echo "→ (Re)creating services — app applies migrations on startup once Postgres is healthy"
"${COMPOSE[@]}" up -d --remove-orphans

echo "→ Pruning dangling images + capping build cache"
# Collect the images this deploy just superseded (their tags became untagged
# when the rebuild took the tag). The size cap bounds the cache regardless of
# deploy frequency while keeping recent layers for a fast next build.
reclaim_disk

# Block until every long-running service is healthy/running, so the deploy fails
# loudly instead of "succeeding" while a container is crash-looping. postgres and
# redis expose healthchecks; app and web have none, so we fall back to their
# running state.
echo "→ Waiting for services to become healthy"
DEPLOY_HEALTH_TIMEOUT="${DEPLOY_HEALTH_TIMEOUT:-180}"
deadline=$(( SECONDS + DEPLOY_HEALTH_TIMEOUT ))
services=(postgres redis app web)
while :; do
  unhealthy=""
  for svc in "${services[@]}"; do
    # A service not in the compose file / not running yet → treat as pending.
    cid="$("${COMPOSE[@]}" ps -q "$svc" 2>/dev/null || true)"
    if [[ -z "$cid" ]]; then unhealthy+="$svc "; continue; fi
    # Prefer the healthcheck status; fall back to running state for services
    # without a healthcheck (app, web).
    health="$(docker inspect -f '{{if .State.Health}}{{.State.Health.Status}}{{else}}{{.State.Status}}{{end}}' "$cid" 2>/dev/null || echo "unknown")"
    case "$health" in
      healthy|running) ;;                      # good
      *) unhealthy+="$svc($health) " ;;
    esac
  done
  [[ -z "$unhealthy" ]] && break
  if (( SECONDS >= deadline )); then
    echo "✗ Timed out after ${DEPLOY_HEALTH_TIMEOUT}s — not healthy: $unhealthy" >&2
    "${COMPOSE[@]}" ps
    exit 1
  fi
  sleep 3
done

echo "✓ Deploy complete — all services healthy"
"${COMPOSE[@]}" ps
