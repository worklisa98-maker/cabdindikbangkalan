> Arsip dari branch sumber. Hasil validasi integrasi Staging saat ini ada di [laporan integrasi](staging-new-ui-integration.md); klaim database pada dokumen ini bukan hasil sesi integrasi ini.

# Visual fidelity pass — Phase 1–3 only

Primary reference: `app/assets/Bangkalan Education Portal UI Collage.png`, provided by the user. Work stops at the homepage, shared portal components, map presentation and location settings presentation. Phase 4–6 are not included.

## Changes in this pass

- `app/page.tsx`: compact full-width scenic hero, five metric cards, map before services, denser service grid, honest news empty state. Existing landing settings and role-filtered destinations remain authoritative. Lulusan PT uses the existing public admission analytics KPI; no API changes. No total-enrollment endpoint was identified, so Peserta Didik explicitly displays unavailable instead of repurposing final-year counts.
- `app/globals.css`: portal container widened to 1440px, portal card radius reduced.
- `components/portal/public-navbar.tsx`: official existing East Java logo, full Cabdin identity and province name, compact header, active Beranda treatment. Existing grouping, service visibility, native dialog drawer and routes retained.
- `components/portal/primitives.tsx`: icon-based metric treatment, compact color-accented module cards with visible actions; existing primitives extended, no duplicate design system.
- `components/portal/education-map.tsx`: dominant map, adjacent level/filter/layer controls, composition ring using existing Recharts, accreditation meters, disabled unsupported layers. Charts use fetched master schools and follow existing filters. Error states show the real OpenStreetMap basemap without fabricated markers. Search, profile dialog, coordinates, map library and API remain unchanged.
- `components/portal/school-location-settings.tsx`: two-column form/preview at desktop; same fields, validation and save operation.
- `components/portal/footer.tsx`: four columns, official identity, existing navigation/program links, configured contact details and copyright. No invented social, privacy or terms destinations.
- `public/images/suramadu-hero.webp`: generated illustrative backdrop inspired by the supplied reference, 1800×600, ~211 KB. Clearly labelled illustration; official logo was not generated. Configured hero image still overrides the fallback.

No new component files or dependencies were added for this pass. Existing Leaflet and Recharts are reused. Backend files, migrations, importer, database behavior, access rules, scoring and workflows were not edited in this pass.

## Validation and limits

- Production build and TypeScript check pass.
- Targeted lint for homepage and portal components passes. Full-repository lint retains existing errors (see baseline in `portal-redesign.md`).
- Browser verified desktop hero, actual map tiles, filter selection, disabled layers, fallback states, mobile layout and native navigation drawer. No dummy school data was injected for screenshots.
- Mobile 390px and tablet 768px passed horizontal-overflow checks; the filter disclosure collapses correctly. Unauthenticated location API still returns 401. Full lint reports 12 existing errors and 44 warnings, matching the preceding phase baseline.
- Local backend on port 8080 remains unavailable: actual statistics, markers, chart values and authenticated settings save cannot be integration-tested. Console/API failures observed are 502 backend-connection errors; map basemap remains usable.
- Mockup examples of populated figures, news, satellite imagery, extra layer functionality and profile photos are not presented as real data. Existing OpenStreetMap basemap and explicit fallbacks preserve behavior and avoid adding another provider or map library.
- Main remaining regression risk is the unverified authenticated form layout/save and populated chart/marker presentation until the existing backend and database are running.
