"use client";
import { MapBoundary } from "./map-boundary";
import dynamic from "next/dynamic";
import { useCallback, useEffect, useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api/client";
import { hasCoordinates, type MapSchool } from "@/lib/school-map";
import { Dialog, LoadingState, MapCard } from "./primitives";
import { PieChart, Pie, Cell, ResponsiveContainer } from "recharts";
import { Layers, MapPin, ChevronDown } from "lucide-react";
import { StatusBadge } from "@/components/ui/status-badge";
import {
  LOCATION_OUTLINES,
  SCHOOL_LEVEL_COLORS,
  UNKNOWN_LEVEL_COLOR,
  normalizeSchoolLevel,
  schoolLevelColor,
} from "./school-map-style";
const Canvas = dynamic(() => import("./school-map-canvas"), {
  ssr: false,
  loading: () => <LoadingState label="Menyiapkan peta…" />,
});
export function EducationMap() {
  const [level, setLevel] = useState("");
  const [status, setStatus] = useState("");
  const [search, setSearch] = useState("");
  const [selectedId, setSelectedId] = useState<number | null>(null);
  const [profile, setProfile] = useState<MapSchool | null>(null);
  const data = useQuery({
    queryKey: ["school-locations"],
    queryFn: async () => {
      const response = await apiFetch("/public/school-locations", undefined, {
        skipAuthRedirect: true,
      });
      if (!response.ok) throw new Error("Lokasi belum tersedia");
      return (await response.json()).data as MapSchool[];
    },
    staleTime: 60_000,
    retry: 1,
  });
  const filtered = useMemo(
    () =>
      (data.data ?? []).filter(
        (s) =>
          (!level || normalizeSchoolLevel(s.level) === level) &&
          (!status || s.status.toLowerCase() === status.toLowerCase()) &&
          `${s.name} ${s.npsn}`.toLowerCase().includes(search.toLowerCase())
      ),
    [data.data, level, status, search]
  );
  const located = filtered.filter(hasCoordinates);
  const select = useCallback((school: MapSchool) => {
    setSelectedId(school.id);
    setProfile(school);
  }, []);
  useEffect(() => {
    const listener = (event: Event) => {
      const id = (event as CustomEvent<number>).detail;
      setLevel("");
      setStatus("");
      setSearch("");
      setSelectedId(id);
      setProfile(data.data?.find((s) => s.id === id) ?? null);
      document
        .getElementById("peta-pendidikan")
        ?.scrollIntoView({ block: "start" });
    };
    window.addEventListener("portal-select-school", listener);
    return () => window.removeEventListener("portal-select-school", listener);
  }, [data.data]);
  const composition = Object.keys(SCHOOL_LEVEL_COLORS).map((name) => ({
    name,
    value: filtered.filter((s) => normalizeSchoolLevel(s.level) === name)
      .length,
    color: schoolLevelColor(name),
  }));
  const available = !data.isPending && !data.isError;
  return (
    <div className="grid items-start gap-4 xl:grid-cols-[minmax(0,1fr)_250px]">
      <MapCard>
        <div className="flex flex-wrap items-center justify-between gap-3 border-b border-slate-200 px-4 py-3">
          <p className="flex items-center gap-2 text-sm font-semibold">
            <MapPin className="size-4 text-blue-600" />
            Sebaran Sekolah
          </p>
          <span className="text-xs text-slate-500">
            {available
              ? located.length + " titik / " + filtered.length + " sekolah"
              : "Menunggu data lokasi"}
          </span>
        </div>
        <div className="grid lg:grid-cols-[minmax(0,1fr)_190px]">
          <div className="relative min-w-0">
            <MapBoundary>
              <Canvas
                schools={filtered}
                selectedId={selectedId}
                onSelect={select}
              />
            </MapBoundary>
            {!available && (
              <div className="absolute inset-x-3 bottom-8 z-[500] rounded-xl border border-slate-200 bg-white/95 p-3 shadow-sm">
                <p role="status" className="text-sm font-semibold">
                  {data.isPending
                    ? "Memuat lokasi sekolah…"
                    : "Data lokasi belum dapat dimuat"}
                </p>
                <p className="mt-1 text-xs text-slate-500">
                  Peta dasar tetap tersedia. Marker akan muncul setelah data
                  lokasi tersedia.
                </p>
                {data.isError && (
                  <button
                    type="button"
                    onClick={() => data.refetch()}
                    className="mt-2 text-xs font-semibold text-blue-700"
                  >
                    Coba lagi →
                  </button>
                )}
              </div>
            )}
            {available && located.length === 0 && (
              <p
                role="status"
                className="absolute inset-x-3 bottom-8 z-[500] rounded-xl bg-white/95 p-3 text-sm shadow-sm"
              >
                Belum ada koordinat untuk pilihan ini.
              </p>
            )}
          </div>
          <div className="border-t border-slate-200 bg-slate-50/50 p-3 lg:border-t-0 lg:border-l">
            <div aria-label="Jenjang peta" className="grid grid-cols-4 gap-1">
              {["", "SMA", "SMK", "SLB"].map((value) => (
                <button
                  key={value}
                  type="button"
                  aria-pressed={level === value}
                  onClick={() => setLevel(value)}
                  className={
                    "rounded-md px-1 py-2 text-[11px] font-semibold " +
                    (level === value
                      ? "bg-blue-600 text-white"
                      : "bg-white text-slate-600")
                  }
                >
                  {value || "Semua"}
                </button>
              ))}
            </div>
            <details className="mt-3" open>
              <summary className="flex cursor-pointer list-none items-center justify-between text-xs font-bold text-slate-800">
                <span className="flex items-center gap-1.5">
                  <Layers className="size-3.5" />
                  Filter & layer
                </span>
                <ChevronDown className="size-3.5" />
              </summary>
              <fieldset className="mt-3 space-y-1">
                <legend className="sr-only">Layer data</legend>
                {[
                  "Sebaran Sekolah",
                  "Akreditasi",
                  "Prestasi",
                  "Lulusan PT",
                  "GASPOL BANG",
                  "MGMP & JARKOM",
                ].map((label, index) => (
                  <label
                    key={label}
                    className={
                      "flex items-center gap-2 rounded-md px-2 py-2 text-xs " +
                      (index === 0
                        ? "bg-blue-100 text-blue-800"
                        : "text-slate-400")
                    }
                    title={index ? "Layer belum tersedia" : undefined}
                  >
                    <input
                      type="radio"
                      name="map-layer"
                      checked={index === 0}
                      readOnly
                      disabled={index !== 0}
                    />
                    {label}
                  </label>
                ))}
              </fieldset>
              <label className="mt-4 block text-xs font-bold">
                Status sekolah
                <select
                  aria-label="Status sekolah peta"
                  value={status}
                  onChange={(e) => setStatus(e.target.value)}
                  className="portal-input mt-2"
                >
                  <option value="">Semua status</option>
                  <option>Negeri</option>
                  <option>Swasta</option>
                </select>
              </label>
              <label className="mt-3 block text-xs font-bold">
                Cari sekolah
                <input
                  className="portal-input mt-2"
                  placeholder="Nama atau NPSN"
                  value={search}
                  onChange={(e) => setSearch(e.target.value)}
                />
              </label>
              <div
                aria-label="Legenda marker sekolah"
                className="mt-3 space-y-2 text-[11px] leading-4 text-slate-600"
              >
                <p className="font-semibold text-slate-800">Warna jenjang</p>
                <ul className="flex flex-wrap gap-x-3 gap-y-2">
                  {Object.entries(SCHOOL_LEVEL_COLORS).map(([name, color]) => (
                    <li key={name} className="flex items-center gap-1.5">
                      <span
                        aria-hidden="true"
                        className="size-3 shrink-0 rounded-full"
                        style={{ backgroundColor: color, opacity: 0.85 }}
                      />
                      {name}
                    </li>
                  ))}
                  {located.some(
                    (s) => schoolLevelColor(s.level) === UNKNOWN_LEVEL_COLOR
                  ) && (
                    <li className="flex items-center gap-1.5">
                      <span
                        aria-hidden="true"
                        className="size-3 shrink-0 rounded-full"
                        style={{
                          backgroundColor: UNKNOWN_LEVEL_COLOR,
                          opacity: 0.85,
                        }}
                      />
                      Jenjang belum dikenal
                    </li>
                  )}
                </ul>
                <p className="pt-1 font-semibold text-slate-800">
                  Outline lokasi
                </p>
                <ul className="space-y-2">
                  {[
                    [LOCATION_OUTLINES.verified, "Gelap: terverifikasi"],
                    [
                      LOCATION_OUTLINES.unverified,
                      "Abu-abu: belum terverifikasi",
                    ],
                  ].map(([color, label]) => (
                    <li key={label} className="flex items-start gap-1.5">
                      <span
                        aria-hidden="true"
                        className="mt-0.5 size-3 shrink-0 rounded-full border-2"
                        style={{ borderColor: color }}
                      />
                      <span>{label}</span>
                    </li>
                  ))}
                </ul>
              </div>
            </details>
          </div>
        </div>
        <details className="border-t border-slate-200 p-5">
          <summary className="text-primary cursor-pointer text-sm font-semibold">
            Lihat daftar sekolah ({available ? filtered.length : "—"})
          </summary>
          <div className="mt-4 grid max-h-80 gap-2 overflow-auto sm:grid-cols-2">
            {filtered.map((s) => (
              <button
                id={`sekolah-${s.id}`}
                type="button"
                key={s.id}
                onClick={() => select(s)}
                className="rounded-xl border border-slate-200 p-3 text-left"
              >
                <p className="text-sm font-medium">{s.name}</p>
                <p className="mt-1 text-xs text-slate-500">
                  {s.npsn} · {s.level} ·{" "}
                  {hasCoordinates(s)
                    ? "Lihat profil & lokasi"
                    : "Koordinat belum tersedia"}
                </p>
              </button>
            ))}
          </div>
        </details>
        <Dialog
          open={Boolean(profile)}
          onClose={() => setProfile(null)}
          title="Profil Sekolah"
        >
          {profile && (
            <div className="space-y-4">
              <h3 className="text-lg font-semibold">{profile.name}</h3>
              <dl className="grid grid-cols-2 gap-3 text-sm">
                <dt>NPSN</dt>
                <dd>{profile.npsn}</dd>
                <dt>Jenjang</dt>
                <dd>{profile.level || "—"}</dd>
                <dt>Status</dt>
                <dd>{profile.status || "—"}</dd>
                {profile.accreditation && (
                  <>
                    <dt>Akreditasi</dt>
                    <dd>{profile.accreditation}</dd>
                  </>
                )}
              </dl>
              <StatusBadge
                variant={profile.location_verified ? "success" : "warning"}
              >
                {profile.location_verified
                  ? "Lokasi terverifikasi"
                  : "Lokasi belum terverifikasi"}
              </StatusBadge>
              <p className="text-xs break-words text-slate-500">
                {profile.location_source &&
                  `Sumber: ${profile.location_source}`}
              </p>
              {!hasCoordinates(profile) && (
                <p className="text-sm text-slate-600">
                  Koordinat sekolah ini belum tersedia.
                </p>
              )}
            </div>
          )}
        </Dialog>
      </MapCard>
      <aside className="grid gap-3 sm:grid-cols-2 xl:grid-cols-1">
        <section className="portal-card p-4">
          <h3 className="text-sm font-bold text-slate-900">
            Komposisi Sekolah
          </h3>
          <p className="mt-1 text-xs text-slate-500">
            Sesuai pilihan filter peta
          </p>
          {available && filtered.length > 0 ? (
            <>
              <div className="relative mt-2 h-40">
                <ResponsiveContainer
                  width="100%"
                  height="100%"
                  initialDimension={{ width: 216, height: 160 }}
                >
                  <PieChart>
                    <Pie
                      data={composition}
                      dataKey="value"
                      innerRadius={48}
                      outerRadius={68}
                      stroke="none"
                      isAnimationActive={false}
                    >
                      {composition.map((item) => (
                        <Cell key={item.name} fill={item.color} />
                      ))}
                    </Pie>
                  </PieChart>
                </ResponsiveContainer>
                <div className="pointer-events-none absolute inset-0 flex flex-col items-center justify-center">
                  <strong className="text-2xl text-slate-900">
                    {filtered.length}
                  </strong>
                  <span className="text-xs text-slate-500">Sekolah</span>
                </div>
              </div>
              <div className="space-y-2">
                {composition.map((item) => (
                  <div
                    key={item.name}
                    className="flex items-center gap-2 text-xs"
                  >
                    <span
                      className="size-2 rounded-full"
                      style={{ background: item.color }}
                    />
                    {item.name}
                    <span className="ml-auto font-semibold">{item.value}</span>
                  </div>
                ))}
              </div>
            </>
          ) : (
            <div className="flex h-40 flex-col items-center justify-center text-sm text-slate-500">
              <span className="mb-2 text-3xl text-slate-300">—</span>
              {data.isPending
                ? "Memuat ringkasan…"
                : "Ringkasan belum tersedia"}
            </div>
          )}
        </section>
        <section className="portal-card p-4">
          <h3 className="mb-4 text-sm font-bold text-slate-900">
            Status Akreditasi
          </h3>
          {["A", "B", "C", "Belum"].map((label) => {
            const total = filtered.filter((s) =>
              label === "Belum"
                ? !["A", "B", "C"].includes(s.accreditation.toUpperCase())
                : s.accreditation.toUpperCase() === label
            ).length;
            return (
              <div
                key={label}
                className="mb-3 grid grid-cols-[38px_1fr_24px] items-center gap-2 text-xs"
              >
                <span>{label}</span>
                <meter
                  aria-label={"Akreditasi " + label}
                  min={0}
                  max={Math.max(1, filtered.length)}
                  value={available ? total : 0}
                  className="h-3 w-full accent-blue-600"
                />
                <span className="text-right font-semibold">
                  {available ? total : "—"}
                </span>
              </div>
            );
          })}
          <p className="mt-3 text-[11px] leading-4 text-slate-500">
            {available
              ? "Berdasarkan data master sekolah."
              : "Data belum tersedia. Tidak ada angka perkiraan."}
          </p>
        </section>
      </aside>
    </div>
  );
}
