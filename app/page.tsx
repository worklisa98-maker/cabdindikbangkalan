"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import {
  Building2,
  GraduationCap,
  Trophy,
  Users,
  BookOpen,
  Search,
  Settings2,
} from "lucide-react";
import { apiFetch } from "@/lib/api/client";
import { fetchServiceLinks } from "@/lib/service-links";
import { useAuth } from "@/lib/auth/client";
import {
  defaultLandingSettings,
  type LandingSettings,
} from "@/lib/landing-settings";
import {
  filterVisibleSections,
  getSectionHref,
  sidebarSections,
} from "@/lib/navigation/sidebar-menu";
import { PublicNavbar } from "@/components/portal/public-navbar";
import { PortalFooter } from "@/components/portal/footer";
import { EducationMap } from "@/components/portal/education-map";
import {
  MetricCard,
  ModuleCard,
  SectionHeader,
} from "@/components/portal/primitives";
import type { MapSchool } from "@/lib/school-map";

type Summary = {
  activeSchools: number;
  currentYearGuruTendik: number;
  prestasiTotal: number;
  portalGuruTendik: number;
  portalPrestasiYear: number;
  portalPrestasiTotal: number;
};
async function readPublic<T>(path: string): Promise<T> {
  const response = await apiFetch(path, undefined, { skipAuthRedirect: true });
  if (!response.ok) throw new Error("Data portal belum tersedia");
  const payload = await response.json();
  if (payload.fallback) throw new Error("Data portal belum tersedia");
  return payload.data;
}
const descriptions: Record<string, string> = {
  gaspol: "Penilaian lingkungan sekolah dan pemantauan program GASPOL BANG.",
  mgmp: "Kegiatan dan administrasi MGMP & JARKOM SMK.",
  prestasi: "Data prestasi siswa dan sekolah di Kabupaten Bangkalan.",
  siswa: "Data siswa, status lulusan, dan analitik perguruan tinggi.",
  "halo-kepegawaian": "Layanan informasi dan administrasi kepegawaian.",
  "pengaduan-kritik-saran":
    "Sampaikan pengaduan, kritik, dan saran masyarakat.",
};
const count = (value?: number) =>
  value == null || !Number.isFinite(value)
    ? "—"
    : new Intl.NumberFormat("id-ID").format(value);

export default function LandingPage() {
  const router = useRouter();
  const { user, status } = useAuth();
  const [search, setSearch] = useState("");
  const effectiveUser = status === "authenticated" ? user : null;
  const restricted =
    effectiveUser?.role === "USER_SEKOLAH_GASPOL" ||
    effectiveUser?.role === "USER_MGMP";
  useEffect(() => {
    if (restricted)
      router.replace(effectiveUser?.role === "USER_MGMP" ? "/mgmp" : "/gaspol");
  }, [restricted, effectiveUser?.role, router]);
  const enabled = status !== "loading" && !restricted;
  const settingsQuery = useQuery({
    queryKey: ["landing-settings-public"],
    queryFn: () =>
      readPublic<{ settings: LandingSettings }>("/public/landing-settings"),
    enabled,
  });
  const summary = useQuery({
    queryKey: ["portal-summary-public"],
    queryFn: () => readPublic<Summary>("/public/landing-summary"),
    enabled,
  });
  const admissions = useQuery({
    queryKey: ["portal-admissions-public"],
    queryFn: () =>
      readPublic<{ tahun_lulus: number; final_count: number }[]>(
        "/public/siswa/analytics/diterima-perguruan-tinggi"
      ),
    enabled,
  });
  const latestGraduates = admissions.data?.reduce<
    { tahun_lulus: number; final_count: number } | undefined
  >(
    (latest, row) =>
      !latest || row.tahun_lulus > latest.tahun_lulus ? row : latest,
    undefined
  );
  const schools = useQuery({
    queryKey: ["school-locations"],
    queryFn: () => readPublic<MapSchool[]>("/public/school-locations"),
    enabled,
    staleTime: 60_000,
    retry: 1,
  });
  const scope = effectiveUser ? "user" : "public";
  const services = useQuery({
    queryKey: ["service-links", scope, effectiveUser?.id ?? "anon"],
    queryFn: () => fetchServiceLinks(scope),
    enabled,
  });
  const sections = useMemo(
    () =>
      filterVisibleSections(sidebarSections, effectiveUser)
        .filter(
          (section) =>
            section.key !== "halo-kepegawaian" || services.data?.halo.is_active
        )
        .filter(
          (section) => section.key !== "sinom" || services.data?.sinom.is_active
        ),
    [effectiveUser, services.data]
  );
  const query = search.trim().toLocaleLowerCase("id-ID");
  const matches = (schools.data ?? []).filter((school) =>
    `${school.name} ${school.npsn}`.toLocaleLowerCase("id-ID").includes(query)
  );
  const settings = settingsQuery.data?.settings ?? defaultLandingSettings;
  if (restricted)
    return <p className="p-6">Mengalihkan akun ke workspace yang sesuai...</p>;
  return (
    <>
      <PublicNavbar />
      <main id="main-content">
        <section className="relative isolate overflow-hidden bg-slate-900 text-white">
          <Image
            src={settings.hero_image_url || "/images/suramadu-hero.webp"}
            alt={settings.hero_image_alt || "Ilustrasi Jembatan Suramadu"}
            fill
            priority
            sizes="100vw"
            className="-z-20 object-cover"
          />
          <div className="absolute inset-0 -z-10 bg-gradient-to-r from-slate-950/90 via-slate-950/60 to-slate-950/20" />
          <div className="portal-container py-12 sm:py-16">
            <p className="mb-4 text-xs font-semibold tracking-wider text-blue-100 uppercase">
              Portal Pendidikan · Cabdin Bangkalan
            </p>
            <h1 className="max-w-2xl text-3xl leading-tight font-bold sm:text-4xl">
              {settings.hero_title ||
                "Bersama Sekolah, Menuju Generasi Emas Bangkalan"}
            </h1>
            <p className="mt-4 max-w-xl text-sm leading-6 text-blue-50">
              {settings.hero_subtitle}
            </p>
            <form
              id="portal-search"
              className="mt-6 max-w-xl scroll-mt-6"
              onSubmit={(event) => {
                event.preventDefault();
                document
                  .getElementById(
                    query && matches.length
                      ? "school-search-results"
                      : "jelajahi"
                  )
                  ?.scrollIntoView({ behavior: "smooth" });
              }}
            >
              <label htmlFor="portal-query" className="sr-only">
                Cari sekolah, NPSN, atau layanan
              </label>
              <div className="flex gap-2 rounded-xl bg-white p-2 shadow-lg">
                <Search className="my-auto ml-2 size-5 shrink-0 text-slate-500" />
                <input
                  id="portal-query"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Cari sekolah, NPSN, atau layanan..."
                  className="min-w-0 flex-1 bg-transparent text-sm text-slate-950 outline-none"
                />
                <button className="portal-button" type="submit">
                  Cari
                </button>
              </div>
            </form>
            <div className="mt-5 flex flex-wrap gap-4 text-sm">
              <a href="#jelajahi">Program & Layanan →</a>
              <a href="#peta-pendidikan">Peta Pendidikan →</a>
              {effectiveUser?.role === "ADMIN_MASTER" && (
                <Link
                  href="/admin/settings/landing"
                  className="flex items-center gap-1"
                >
                  <Settings2 className="size-4" />
                  Kelola Landing
                </Link>
              )}
            </div>
            {!settings.hero_image_url && (
              <p className="mt-5 text-xs text-blue-100">
                Latar merupakan ilustrasi Suramadu.
              </p>
            )}
          </div>
        </section>
        <div className="portal-container space-y-8 py-6">
          <section
            aria-label="Statistik pendidikan"
            className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4"
          >
            <MetricCard
              label="Sekolah Aktif"
              value={count(summary.data?.activeSchools)}
              icon={<Building2 />}
              detail="Sekolah yang telah memperbarui data"
            />
            <MetricCard
              label="Guru & Tendik"
              value={count(summary.data?.portalGuruTendik)}
              icon={<Users />}
            />
            <MetricCard
              label="Prestasi"
              value={count(summary.data?.portalPrestasiTotal)}
              detail={
                summary.data?.portalPrestasiYear
                  ? `Tahun ${summary.data.portalPrestasiYear}`
                  : undefined
              }
              icon={<Trophy />}
              accent="prestasi"
            />
            <MetricCard
              label="Lulusan PT"
              value={count(
                latestGraduates?.final_count ??
                  (admissions.isSuccess ? 0 : undefined)
              )}
              detail={
                latestGraduates?.tahun_lulus
                  ? `Tahun Lulusan ${latestGraduates.tahun_lulus}`
                  : undefined
              }
              icon={<GraduationCap />}
            />
          </section>
          {(summary.isError || admissions.isError) && (
            <p role="status" className="text-sm text-slate-600">
              Statistik belum dapat dimuat. Silakan coba kembali nanti.
            </p>
          )}
          {query && (
            <section id="school-search-results" className="portal-card p-5">
              <SectionHeader title="Hasil pencarian sekolah" />
              <p className="mb-3 text-sm text-slate-600">
                {schools.isPending
                  ? "Memuat sekolah..."
                  : schools.isError
                    ? "Data sekolah belum dapat dimuat."
                    : `${matches.length} sekolah ditemukan`}
              </p>
              <div className="grid max-h-64 gap-2 overflow-auto sm:grid-cols-2">
                {matches.map((school) => (
                  <button
                    key={school.id}
                    type="button"
                    className="rounded-lg border border-slate-200 p-3 text-left text-sm hover:bg-blue-50"
                    onClick={() =>
                      window.dispatchEvent(
                        new CustomEvent("portal-select-school", {
                          detail: school.id,
                        })
                      )
                    }
                  >
                    {school.name}
                    <span className="block text-xs text-slate-500">
                      NPSN {school.npsn}
                    </span>
                  </button>
                ))}
              </div>
            </section>
          )}
          <section id="peta-pendidikan" className="scroll-mt-6">
            <SectionHeader
              title="Peta Pendidikan"
              description="Sebaran sekolah dan komposisi pendidikan Kabupaten Bangkalan."
            />
            <EducationMap />
          </section>
          <section id="jelajahi" className="scroll-mt-6">
            <SectionHeader
              title="Program & Layanan"
              description="Akses layanan pendidikan sesuai hak akses Anda."
            />
            <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-4">
              {sections
                .filter(
                  (section) =>
                    !query ||
                    `${section.label} ${descriptions[section.key] ?? ""}`
                      .toLocaleLowerCase("id-ID")
                      .includes(query)
                )
                .map((section) => {
                  const Icon = section.icon ?? BookOpen;
                  const external =
                    section.external ||
                    (section.key === "sinom" && !!services.data?.sinom.url);
                  return (
                    <ModuleCard
                      key={section.key}
                      title={section.label}
                      description={
                        descriptions[section.key] ??
                        "Informasi dan layanan Cabang Dinas Pendidikan."
                      }
                      href={
                        section.key === "sinom" && services.data?.sinom.url
                          ? services.data.sinom.url
                          : getSectionHref(section, effectiveUser)
                      }
                      icon={<Icon className="size-5" />}
                      accent={section.key}
                      external={external}
                    >
                      {section.children?.map((child) => (
                        <Link
                          key={child.key}
                          href={getSectionHref(child, effectiveUser)}
                          className="mr-3 inline-block text-xs underline"
                        >
                          {child.label}
                        </Link>
                      ))}
                    </ModuleCard>
                  );
                })}
            </div>
          </section>
          <section className="portal-card p-5">
            <SectionHeader
              title="Informasi Cabdin"
              description="Informasi resmi dan panduan layanan pendidikan."
            />
            <div className="flex flex-wrap gap-4 text-sm text-blue-700">
              <Link href="/informasi-pelayanan-cabdin">
                Informasi pelayanan →
              </Link>
              <Link href="/pusat-tautan">Pusat tautan →</Link>
              {settings.guidebook_url && (
                <Link href={settings.guidebook_url}>Panduan layanan →</Link>
              )}
            </div>
            <p className="mt-4 text-sm text-slate-500">
              Berita terbaru belum tersedia.
            </p>
          </section>
        </div>
      </main>
      <PortalFooter settings={settings} />
    </>
  );
}
