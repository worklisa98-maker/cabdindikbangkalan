#!/usr/bin/env sh
set -e

if [ -z "${DATABASE_URL:-}" ]; then
  echo "DATABASE_URL is required" >&2
  exit 1
fi

ROOT_DIR="$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)"
cd "$ROOT_DIR"

MIGRATION_DIR="./migrations"
if [ -d "./backend/migrations" ]; then
  cd "./backend"
  MIGRATION_DIR="./migrations"
elif [ -d "./db/migrations" ]; then
  MIGRATION_DIR="./db/migrations"
fi

if command -v goose >/dev/null 2>&1; then
  goose -dir "$MIGRATION_DIR" postgres "$DATABASE_URL" up
  exit 0
fi

if command -v go >/dev/null 2>&1; then
  go run github.com/pressly/goose/v3/cmd/goose@v3.26.0 -dir "$MIGRATION_DIR" postgres "$DATABASE_URL" up
  exit 0
fi

echo "goose is not available and go is missing; install goose or include it in the image." >&2
exit 1
