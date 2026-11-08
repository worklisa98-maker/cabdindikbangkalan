#!/usr/bin/env bash
set -euo pipefail

MIGRATE_ONLY=false
RESET_FLAG=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --migrate-only)
      MIGRATE_ONLY=true
      shift
      ;;
    --reset)
      RESET_FLAG=true
      shift
      ;;
    *)
      printf 'ERROR: Unknown flag %s\n' "$1" >&2
      exit 1
      ;;
  esac
done

log() {
  printf '[INFO] %s\n' "$1"
}

fail() {
  printf '[ERROR] %s\n' "$1" >&2
  exit 1
}

check_command() {
  if ! command -v "$1" >/dev/null 2>&1; then
    fail "Required executable '$1' is not available in PATH."
  fi
}

run_psql() {
  local conn="$1"
  local sql="$2"
  log "psql -> $sql"
  if ! output=$(psql "$conn" -q -At -c "$sql" 2>&1); then
    fail "psql failed: $output"
  fi
  printf '%s' "$output"
}

run_psql_lines() {
  local conn="$1"
  local sql="$2"
  run_psql "$conn" "$sql" | awk 'NF'
}

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "$script_dir/.." && pwd)"
cd "$repo_root"

for tool in psql pg_dump pg_restore goose; do
  check_command "$tool"
done

local_url="${LOCAL_DATABASE_URL:-}"
zeabur_url="${ZEABUR_DATABASE_URL:-}"

[[ -n "$zeabur_url" ]] || fail "ZEABUR_DATABASE_URL is required."
[[ "$MIGRATE_ONLY" == true ]] || [[ -n "$local_url" ]] || fail "LOCAL_DATABASE_URL is required for dump-and-restore."

resolve_dir() {
  local value="$1"
  local fallback="$2"
  if [[ -n "$value" ]]; then
    if [[ "$value" == /* ]]; then
      echo "$value"
    else
      echo "$repo_root/$value"
    fi
  else
    echo "$repo_root/$fallback"
  fi
}

migration_dir="$(resolve_dir "${MIGRATION_DIR:-}" "migrations")"
backup_dir="$(resolve_dir "${BACKUP_DIR:-}" "backups")"
mkdir -p "$backup_dir"

if [[ -z "${PGSSLMODE:-}" ]]; then
  export PGSSLMODE=prefer
fi

reset_target=false
if [[ "$RESET_FLAG" == true ]]; then
  reset_target=true
elif [[ "${RESET_TARGET:-}" =~ ^(true|1)$ ]]; then
  reset_target=true
fi

IFS=',' read -ra VERIFY_TABLES_RAW <<< "${VERIFY_TABLES:-}"
verify_tables=()
for raw in "${VERIFY_TABLES_RAW[@]}"; do
  trimmed="${raw#"${raw%%[![:space:]]*}"}"
  trimmed="${trimmed%"${trimmed##*[![:space:]]}"}"
  if [[ -n "$trimmed" && "$trimmed" =~ ^[a-zA-Z0-9_]+$ ]]; then
    verify_tables+=("$trimmed")
  elif [[ -n "$trimmed" ]]; then
    log "Skipping invalid VERIFY_TABLES entry '$trimmed'."
  fi
done

verify_state() {
  local table_count
  table_count="$(run_psql "$zeabur_url" "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';")"
  table_count="${table_count//$'\r'/}"
  table_count="${table_count//$'\n'/}"
  log "Table count: $table_count"

  local top_tables
  top_tables=$(run_psql "$zeabur_url" "SELECT schemaname || '|' || tablename || '|' || pg_total_relation_size(format('%I.%I', schemaname, tablename)) FROM pg_tables WHERE schemaname='public' ORDER BY pg_total_relation_size(format('%I.%I', schemaname, tablename)) DESC LIMIT 10;")
  if [[ -z "$top_tables" ]]; then
    log "No tables found in public schema."
  else
    log "Top tables by size:"
    while IFS= read -r line; do
      IFS='|' read -r schema table bytes <<< "$line"
      size_mb=$(awk "BEGIN { printf \"%.2f\", $bytes / 1024 / 1024 }")
      printf '  %s (schema %s) -> %s MB\n' "$table" "$schema" "$size_mb"
    done <<< "$top_tables"
  fi

  if [[ ${#verify_tables[@]} -gt 0 ]]; then
    log "Row count samples:"
    for table in "${verify_tables[@]}"; do
      run_table_sql="SELECT count(*) FROM public.\"$table\";"
      row_count="$(run_psql "$zeabur_url" "$run_table_sql")"
      row_count="${row_count//$'\r'/}"
      row_count="${row_count//$'\n'/}"
      printf '  %s -> %s rows\n' "$table" "$row_count"
    done
  fi
}

if [[ "$MIGRATE_ONLY" == false ]]; then
  log "Testing local connection"
  run_psql "$local_url" "SELECT 1;"

  log "Testing Zeabur connection"
  run_psql "$zeabur_url" "SELECT 1;"

  timestamp="$(date +"%Y%m%d%H%M%S")"
  dump_file="$backup_dir/local_dump_$timestamp.dump"
  log "Dumping local database to $dump_file"
  echo ">> pg_dump --format=custom --no-owner --no-privileges --file \"$dump_file\" \"$local_url\""
  pg_dump --format=custom --no-owner --no-privileges --file "$dump_file" "$local_url"

  table_count="$(run_psql "$zeabur_url" "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';")"
  table_count="${table_count//$'\r'/}"
  table_count="${table_count//$'\n'/}"
  if [[ -n "$table_count" && "$table_count" -gt 0 && "$reset_target" == false ]]; then
    fail "Target already contains $table_count tables. Set RESET_TARGET=true or run with --reset."
  fi

  if [[ "$reset_target" == true ]]; then
    log "Resetting public schema"
    run_psql "$zeabur_url" "DROP SCHEMA IF EXISTS public CASCADE; CREATE SCHEMA public; GRANT ALL ON SCHEMA public TO public;"
  fi

  log "Restoring dump into Zeabur"
  echo ">> pg_restore --verbose --clean --no-owner --no-privileges --dbname \"$zeabur_url\" \"$dump_file\""
  pg_restore --verbose --clean --no-owner --no-privileges --dbname "$zeabur_url" "$dump_file"
  log "Local backup kept at $dump_file"
else
  log "Testing Zeabur connection"
  run_psql "$zeabur_url" "SELECT 1;"
fi

if [[ ! -d "$migration_dir" ]]; then
  fail "Migration directory '$migration_dir' does not exist."
fi

log "Running goose migrations"
echo ">> goose -dir \"$migration_dir\" postgres \"$zeabur_url\" up"
goose -dir "$migration_dir" postgres "$zeabur_url" up

verify_state

log "Migration workflow completed successfully."
