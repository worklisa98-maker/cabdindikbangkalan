# Database migration helpers

Use `scripts/migrate_db.ps1` (PowerShell) or `scripts/migrate_db.sh` (Bash) to copy the entire local PostgreSQL database into the Zeabur target while keeping schema migrations in sync via `goose`.

## Prerequisites

- PostgreSQL client tools (`psql`, `pg_dump`, `pg_restore`) and the `goose` CLI must be installed and on `PATH`.
- Set `LOCAL_DATABASE_URL` (for dumps), `ZEABUR_DATABASE_URL` (target), and any SSL hints (`PGSSLMODE`) before running the scripts.
- The repo root (where `migrations/` lives) must be the current working directory when you run a script.

## Environment variables

| Variable              | Description                                                                                                          |
| --------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `LOCAL_DATABASE_URL`  | Connection string to the local Postgres instance. Required for dump-and-restore flows.                               |
| `ZEABUR_DATABASE_URL` | Connection string to the Zeabur Postgres host. Always required.                                                      |
| `PGSSLMODE`           | Defaults to `prefer`; set to `require` if Zeabur insists on SSL.                                                     |
| `MIGRATION_DIR`       | Optional path to the Goose migrations directory (default: `./migrations`).                                           |
| `BACKUP_DIR`          | Destination for timestamped dumps (default: `./backups`). The script will create it automatically.                   |
| `RESET_TARGET`        | When `true`, drop and recreate the `public` schema before restoring. Useful if Zeabur already holds data.            |
| `VERIFY_TABLES`       | Optional comma-separated list of tables (alphanumeric/underscore only) for extra row-count sampling after migration. |

Use `.env.migrate.example` as a template and export the variables per your shell.

## Usage

### PowerShell (Windows)

```powershell
cd <repo-root>
scripts\migrate_db.ps1            # initial dump+restore, assumes empty Zeabur target
scripts\migrate_db.ps1 -Reset     # drop public schema before restoring (same as RESET_TARGET=true)
scripts\migrate_db.ps1 -MigrateOnly  # run goose up + verify without any dump/restore
```

### Bash (Linux/macOS)

```bash
cd <repo-root>
scripts/migrate_db.sh
RESET_TARGET=true scripts/migrate_db.sh --reset   # drop schema + restore
scripts/migrate_db.sh --migrate-only              # run goose + verify only
```

`--reset`/`RESET_TARGET=true` forces a clean schema before restoring the dump; omit it when copying into an empty Zeabur database. The `--migrate-only` flag skips the dump and restore entirely.

## What the scripts do

1. Validate that `psql`, `pg_dump`, `pg_restore`, and `goose` exist.
2. Confirm connectivity to the required databases (`SELECT 1`).
3. Create a timestamped dump (`pg_dump --format=custom --no-owner --no-privileges`) in `BACKUP_DIR`.
4. Optionally drop and recreate the `public` schema on Zeabur if `RESET_TARGET`/`--reset` is requested.
5. Restore the dump with `pg_restore --clean --no-owner --no-privileges`.
6. Run `goose` against Zeabur to reach the latest migration.
7. Report verification metrics: public table count, top 10 tables by `pg_total_relation_size`, and row counts for `VERIFY_TABLES`.

## Troubleshooting

- **SSL errors connecting to Zeabur**: export `PGSSLMODE=require` and confirm your connection string ends with `?sslmode=require`.
- **Permission errors while dropping/creating schema**: ensure the Zeabur user has `CREATE`/`USAGE`/`SELECT` rights on `public`.
- **Target not empty**: you will see an explicit message asking for `RESET_TARGET=true` or `--reset`; run with that flag if you intend to overwrite.
- **Missing `goose` or `pg_restore`**: install the missing tool via package manager and add it to `PATH`.

Any failure stops the script with an explanatory error and a non-zero exit code so you can safely rerun after resolving the issue.
