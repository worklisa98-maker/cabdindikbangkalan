Param(
    [switch]$MigrateOnly,
    [switch]$Reset
)

Set-StrictMode -Version Latest
$ErrorActionPreference = "Stop"

function Log($Message) {
    Write-Host "[INFO] $Message"
}

function Fail($Message) {
    Write-Host "[ERROR] $Message"
    Exit 1
}

function Check-Command($Name) {
    if (-not (Get-Command $Name -ErrorAction SilentlyContinue)) {
        Fail "Required executable '$Name' is not available in PATH."
    }
}

function Run-PSQL($Connection, $Sql) {
    Log "psql -> $Sql"
    $output = & psql $Connection -q -At -c $Sql 2>&1
    if ($LASTEXITCODE -ne 0) {
        Fail "psql failed ($Connection): $output"
    }
    return $output.Trim()
}

function Run-PSQL-Lines($Connection, $Sql) {
    $raw = Run-PSQL $Connection $Sql
    if ([string]::IsNullOrEmpty($raw)) {
        return @()
    }
    return $raw -split "\r?\n"
}

function Normalize-Path($Value, $Default) {
    if ([string]::IsNullOrWhiteSpace($Value)) {
        return Join-Path $repoRoot $Default
    }
    if ([System.IO.Path]::IsPathRooted($Value)) {
        return $Value
    }
    return Join-Path $repoRoot $Value
}

function Normalize-Dir($Value, $DefaultFolder) {
    $path = Normalize-Path $Value $DefaultFolder
    if (-not (Test-Path $path)) {
        New-Item -ItemType Directory -Force -Path $path | Out-Null
    }
    return $path
}

function Should-ResetTarget($EnvValue, $Flag) {
    if ($Flag) { return $true }
    if (-not $EnvValue) { return $false }
    return ($EnvValue -match '^(true|1)$')
}

function Validate-TableName($Name) {
    return $Name -match '^[a-zA-Z0-9_]+$'
}

function Run-Goose($MigrationDir, $Target) {
    if (-not (Test-Path $MigrationDir)) {
        Fail "Migration directory '$MigrationDir' does not exist."
    }
    Log "Running goose migrations ($MigrationDir)"
    Write-Host ">> goose -dir `"$MigrationDir`" postgres `"$Target`" up"
    & goose -dir $MigrationDir postgres $Target up
    if ($LASTEXITCODE -ne 0) {
        Fail "goose failed."
    }
}

$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Definition
$repoRoot = Resolve-Path (Join-Path $scriptDir "..")
Set-Location $repoRoot

$requiredCmds = @("psql", "pg_dump", "pg_restore", "goose")
foreach ($cmd in $requiredCmds) {
    Check-Command $cmd
}

$zeaburUrl = $env:ZEABUR_DATABASE_URL
if (-not $zeaburUrl) {
    Fail "ZEABUR_DATABASE_URL is required."
}

$localUrl = $env:LOCAL_DATABASE_URL
if (-not $MigrateOnly -and -not $localUrl) {
    Fail "LOCAL_DATABASE_URL is required for dump-and-restore."
}

$migrationDir = Normalize-Path $env:MIGRATION_DIR "migrations"
$backupDir = Normalize-Dir $env:BACKUP_DIR "backups"
$resetTarget = Should-ResetTarget $env:RESET_TARGET $Reset

$verifyTablesRaw = $env:VERIFY_TABLES
$verifyTables = @()
if ($verifyTablesRaw) {
    $verifyTablesRaw.Split(",") | ForEach-Object {
        $part = $_.Trim()
        if (-not [string]::IsNullOrWhiteSpace($part)) {
            if (Validate-TableName $part) {
                $verifyTables += $part
            }
            else {
                Log "Skipping invalid table name '$part' from VERIFY_TABLES."
            }
        }
    }
}

if (-not $env:PGSSLMODE) {
    $env:PGSSLMODE = "prefer"
}

if (-not $MigrateOnly) {
    Log "Testing local connection"
    Run-PSQL $localUrl "SELECT 1;"

    $timestamp = Get-Date -Format "yyyyMMddHHmmss"
    $dumpFile = Join-Path $backupDir "local_dump_$timestamp.dump"
    Log "Dumping local database to $dumpFile"
    Write-Host ">> pg_dump --format=custom --no-owner --no-privileges --file `"$dumpFile`" `"$localUrl`""
    & pg_dump --format=custom --no-owner --no-privileges --file $dumpFile $localUrl
    if ($LASTEXITCODE -ne 0) {
        Fail "pg_dump failed."
    }

    $tableCountRaw = Run-PSQL $zeaburUrl "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
    $tableCount = [int]$tableCountRaw

    if ($tableCount -gt 0 -and -not $resetTarget) {
        Fail "Target contains $tableCount tables; enable RESET_TARGET=true or run with -Reset."
    }

    if ($resetTarget) {
        Log "Resetting target public schema"
        $resetSql = @"
DROP SCHEMA IF EXISTS public CASCADE;
CREATE SCHEMA public;
GRANT ALL ON SCHEMA public TO public;
"@
        Run-PSQL $zeaburUrl $resetSql
    }

    Log "Restoring dump into Zeabur"
    Write-Host ">> pg_restore --verbose --clean --no-owner --no-privileges --dbname `"$zeaburUrl`" `"$dumpFile`""
    & pg_restore --verbose --clean --no-owner --no-privileges --dbname $zeaburUrl $dumpFile
    if ($LASTEXITCODE -ne 0) {
        Fail "pg_restore failed."
    }
    Log "Local backup stored at $dumpFile"
}

Log "Testing Zeabur connection"
Run-PSQL $zeaburUrl "SELECT 1;"

Run-Goose $migrationDir $zeaburUrl

Log "Verifying Zeabur schema"
$tableCount = Run-PSQL $zeaburUrl "SELECT count(*) FROM information_schema.tables WHERE table_schema='public';"
Write-Host "Table count: $tableCount"

$topTables = Run-PSQL-Lines $zeaburUrl @"
SELECT schemaname || '|' || tablename || '|' || pg_total_relation_size(format('%I.%I', schemaname, tablename))
FROM pg_tables
WHERE schemaname='public'
ORDER BY pg_total_relation_size(format('%I.%I', schemaname, tablename)) DESC
LIMIT 10;
"@
if ($topTables.Count -eq 0) {
    Write-Host "No tables found in public schema."
}
foreach ($line in $topTables) {
    $parts = $line -split '\|'
    $bytes = [double]$parts[2]
    $sizeMb = [math]::Round($bytes / 1MB, 2)
    Write-Host "  $($parts[1]) (schema $($parts[0])) -> $sizeMb MB"
}

if ($verifyTables.Count -gt 0) {
    Write-Host "Row counts for VERIFY_TABLES:"
    foreach ($table in $verifyTables) {
        $rowCount = Run-PSQL $zeaburUrl "SELECT count(*) FROM public.""$table"";"
        Write-Host "  $table -> $rowCount"
    }
}

Log "Migration completed successfully."
