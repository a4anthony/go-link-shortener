#!/usr/bin/env bash
#
# Back up the containerized Postgres to a gzipped pg_dump on the host. The prod
# stack runs Postgres in-compose on a named volume (pgdata), so a host-level DB
# backup tool (e.g. Ploi's managed-database backups) never sees it — this script
# is how that data gets off the box.
#
# Idempotent and safe to run from cron. Writes backups/urlshortener-<UTC>.sql.gz
# and prunes copies older than BACKUP_KEEP_DAYS.
#
# Usage:
#   bash scripts/backup.sh                       # dump into ./backups
#   BACKUP_DIR=/mnt/vol bash scripts/backup.sh   # dump elsewhere
#   BACKUP_KEEP_DAYS=30 bash scripts/backup.sh   # change retention
#
# Cron (daily 03:15), from the repo root:
#   15 3 * * * cd /home/ploi/links.a4anthony.com && bash scripts/backup.sh >> backups/backup.log 2>&1
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

ENV_FILE="${ENV_FILE:-.env}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
BACKUP_DIR="${BACKUP_DIR:-$ROOT_DIR/backups}"
BACKUP_KEEP_DAYS="${BACKUP_KEEP_DAYS:-14}"
PG_USER="${PG_USER:-postgres}"
PG_DB="${PG_DB:-urlshortener}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "✗ $ENV_FILE not found in $ROOT_DIR — nothing to back up against." >&2
  exit 1
fi

COMPOSE=(docker compose -f "$COMPOSE_FILE" --env-file "$ENV_FILE")

# UTC timestamp with no colons, so the filename is portable across filesystems.
stamp="$(date -u +%Y%m%dT%H%M%SZ)"
mkdir -p "$BACKUP_DIR"
out="$BACKUP_DIR/${PG_DB}-${stamp}.sql.gz"
tmp="$out.partial"

# Fail if the postgres service isn't up, rather than writing an empty archive.
if [[ -z "$("${COMPOSE[@]}" ps -q postgres 2>/dev/null || true)" ]]; then
  echo "✗ postgres service is not running — start the stack first (scripts/deploy.sh)." >&2
  exit 1
fi

echo "→ Dumping $PG_DB → $out"
# -T: no TTY (cron-safe). Stream pg_dump straight through gzip; write to a
# .partial first and only rename on success, so a crashed dump never leaves a
# truncated file that looks like a good backup.
if "${COMPOSE[@]}" exec -T postgres pg_dump -U "$PG_USER" "$PG_DB" | gzip > "$tmp"; then
  mv "$tmp" "$out"
else
  rm -f "$tmp"
  echo "✗ pg_dump failed — no backup written." >&2
  exit 1
fi

echo "→ Pruning backups older than ${BACKUP_KEEP_DAYS} days"
find "$BACKUP_DIR" -name "${PG_DB}-*.sql.gz" -type f -mtime +"$BACKUP_KEEP_DAYS" -delete

echo "✓ Backup complete: $(du -h "$out" | cut -f1) — $out"
