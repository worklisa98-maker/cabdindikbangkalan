> Arsip dari branch sumber. Hasil validasi integrasi Staging saat ini ada di [laporan integrasi](staging-new-ui-integration.md); klaim database pada dokumen ini bukan hasil sesi integrasi ini.

# Portal redesign — phases 1–3

## Audit and boundaries

- Source: current root Next.js app and `backend/` Go service; `components/backend/` is not the active backend.
- Existing role checks: `lib/navigation/sidebar-menu.ts`, `lib/auth/role-scope.ts`, Go `internal/auth`. Preserve all eight roles and scoped access.
- Existing public APIs: landing settings/summary, accreditation dashboard, achievements, graduate analytics, service links, complaints, MGMP and GASPOL.
- Shared foundation: PageHeader, DataTable, TableFilterBar, EmptyState, Card, StatusBadge, Sidebar. Extend, do not duplicate these components.
- Master schools: `data_sekolah`, unique NPSN. Location additions use this table; no second school master.
- Settings: `/user/settings`; existing landing settings and service visibility remain authoritative.
- No news CMS found in server routes. Homepage information links must point to existing published information; never invent news.
- Seed workbook is initial import only; database remains authoritative. Import must be explicit, transactional and skip any previously edited location.

## Scope

Foundation, homepage, education map and school location settings only. No scoring, ranking, permissions, existing API contracts, or module workflows are rewritten.

## Implemented

- Existing PageHeader, DataTable, TableFilterBar, EmptyState, Sidebar and table/card primitives remain available. New portal primitives supply SectionHeader, MetricCard, ModuleCard, LoadingState, Button, FormField, Breadcrumb, Tabs, Dialog and MapCard. StatusBadge now supports informational blue.
- Public homepage uses grouped PublicNavbar, a native mobile dialog drawer, real API metrics, school/name/NPSN search, role-filtered service cards, official information links and configurable footer. Authenticated navigation and workspace redirects remain intact.
- Map uses dynamically loaded Leaflet with OpenStreetMap attribution; tile/library failure is isolated. Filters cover available school level and status. No district/address fields are invented because the school master has neither. Only finite, paired, in-range coordinates become markers. Unlocated schools remain searchable in the list. Profile opens in an accessible dialog because no separate public school profile route exists.
- New public projection: `GET /api/public/school-locations`. Private `GET/PUT /api/schools/:id/location` uses existing role and multi-school scope checks. Public responses exclude editor identity; authorized settings show editor and timestamp. Existing endpoints are unchanged.
- Settings adds an independent location form under `/user/settings`; input + map preview avoids introducing a drag workflow. Seed never overwrites a populated or manually cleared location.

## Migration and initial seed

1. Apply additive Goose migration `066_add_school_locations.sql` using the project's normal migration procedure. Never reset the database. The down migration intentionally preserves location data for older application versions.
2. From `backend/`, validate: `go run ./cmd/import-school-locations`.
3. After reviewing the report, import to the configured database: `go run ./cmd/import-school-locations --apply`.

The supplied workbook's `Data Map` sheet has 144 rows: dry-run accepted 30 coordinate pairs and skipped 114 incomplete/invalid rows. NPSN is the join key. Missing master records are skipped, never created. Import runs in one transaction and reports updated versus unmatched/already-edited records. Re-running is safe; runtime never reads the workbook.

## Verification

- Phase 1 and phase 2: production builds and type checks passed; targeted lint passed.
- Phase 3: Go unit tests and `go test ./...` passed, and server/import command compile. Optional integration tests requiring their own database URLs remain skipped.
- Coordinate tests cover null pairs, out-of-range values, NaN/infinity, verification without coordinates, duplicate NPSN and seed verification parsing. Handler tests cover anonymous and out-of-scope access.
- Repository lint baseline: 12 errors / 45 warnings before edits. Final full lint: 12 errors / 44 warnings, all existing; new portal files pass targeted lint.
- Local backend/PostgreSQL were unavailable during initial browser checks. Homepage fallback and responsive navigation were inspected; database migration/import and authenticated save round-trip require the local database to be running.
- Final production build and TypeScript check passed. Browser checks at desktop, 390px mobile and 768px tablet confirmed the drawer, grouped menus, service search, uncropped hero branding and API-failure fallback. The tablet document did not overflow horizontally. Console errors observed were backend-unavailable 502 responses.
- Actual migration and `--apply` seed have **not** run: PostgreSQL at localhost:5432 was unreachable and Docker did not respond after startup. No database data was modified. Full marker/filter/save integration verification remains pending that prerequisite.

Map implementation follows the [Leaflet quick start](https://leafletjs.com/examples/quick-start/). Review the [OpenStreetMap tile usage policy](https://operations.osmfoundation.org/policies/tiles/) before scaling hosting; a different tile provider can be substituted without changing school data.
