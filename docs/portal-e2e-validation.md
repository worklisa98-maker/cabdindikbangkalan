> Arsip dari branch sumber. Hasil validasi integrasi Staging saat ini ada di [laporan integrasi](staging-new-ui-integration.md); klaim database pada dokumen ini bukan hasil sesi integrasi ini.

# Phase 1–3 integration validation — 5 October 2026

## Completed against the running local backend

- Public location, landing summary, dashboard and graduate analytics APIs return HTTP 200 through the Next.js proxy.
- Homepage matches API values: 144 schools, 2,571 teachers/staff, 550 achievements and 330 university admissions. Total enrollment remains explicitly unavailable.
- School composition: SMA 69, SMK 71, SLB 4. Accreditation: A 19, B 78, C 46, unaccredited 1.
- SMA filter returns 69 schools; SMA + Negeri returns 10. An unmatched NPSN produces zero results. Global NPSN search 20531146 opens SMA NEGERI 1 BANGKALAN's profile.
- Public GET and PUT attempts to `/api/schools/21/location` without authentication return 401. The public school projection does not expose audit fields. Existing Go access-scope and coordinate-validation tests pass.
- All ten homepage service destinations respond with HTTP 200 (route smoke check, not full module workflow coverage).
- Marker click opens its popup; its profile button opens the correct school dialog; Escape closes the dialog. Verified status and source are displayed.
- Mobile drawer and grouped navigation work. At 390px and 768px viewports, document widths equal scroll widths (375px and 753px respectively).

## Initial coordinate import

Before import, the actual database contained 144 schools and zero coordinate pairs. The existing importer read `backend/assets/data-sekolah-map-bangkalan.xlsx`: 30 valid coordinate pairs, 114 skipped rows.

The first invocation could not connect because `config.Load()` stops reading dotenv files when `backend/.env` is missing, leaving the project-root configuration unread. The importer now falls back to `../.env` when DATABASE_URL is empty; existing environment values are preserved. This change is local to the importer.

Import committed 30 updates, with no new school records. A second invocation using the corrected configuration updated zero rows and skipped all 30 previously populated records. The public API and rendered map both show 30 located schools out of 144. All 30 are marked verified according to the supplied workbook; source accuracy was not independently re-surveyed.

SMA NEGERI 1 BANGKALAN (ID 21, NPSN 20531146) now has latitude -7.0303 and longitude 112.7584, sourced from the workbook's Kemendikdasmen reference URL.

## Checks and remaining boundary

- Production build, TypeScript check, targeted portal lint, `git diff --check`, and `go test ./...` pass.
- A Recharts initial-size warning observed with populated data was addressed by providing initial dimensions matching the chart container height.
- After restarting the production frontend and reloading, the fresh browser-console window contained no warnings or errors. Earlier entries from before the restart are not counted as fresh failures.
- Authenticated follow-up completed using the user's SCHOOL_ADMIN session for SMA Negeri 1 Bangkalan. The school selector exposes only that school; imported coordinates and source load correctly.
- An incomplete coordinate pair is rejected with the form error. Latitude 91 is rejected by native range validation (`rangeOverflow: true`). No invalid coordinates were saved.
- Unchecking verified and saving persisted to the public API and survived a settings reload. The map profile then displayed "Lokasi belum terverifikasi". Audit attribution displayed `admin-SMAN1bangkalan` and the save time.
- Clearing both coordinates with verified unchecked saved successfully and reduced public located-school count from 30 to 29. Re-running the importer updated zero rows and skipped all 30 seed records, preserving the manual clear.
- Original coordinates (-7.0303, 112.7584), source and verified status were restored through the UI; the success message and audit were visible. The final API contains 30 located schools and no public audit-field leak. The audit timestamp/editor intentionally reflect this real validation save.
- Full live role-matrix testing (ADMIN_MASTER, ADMIN_CABDIN, PENGAWAS and disallowed-role sessions) has not been performed; only SCHOOL_ADMIN was supplied. Existing unit tests cover out-of-scope denial. Direct private-API browser navigation was blocked by the browser client, so it is not counted as an authenticated 403 test.
- This report supersedes the backend-unavailable limitations in the earlier redesign/refinement reports for the checks completed above. Phase 4–6 remain outside scope.
