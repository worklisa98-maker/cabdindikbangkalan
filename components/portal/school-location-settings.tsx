"use client";
import { SchoolLocationImport } from "./school-location-import";
import dynamic from "next/dynamic";
import { MapBoundary } from "./map-boundary";
import { useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import { resolveLocationSchool } from "@/lib/school-location-context";
import { hasCoordinates, type MapSchool } from "@/lib/school-map";
import { Button, FormField, LoadingState, SectionHeader } from "./primitives";
import { EmptyState } from "@/components/empty-state";
const Canvas = dynamic(() => import("./school-map-canvas"), { ssr: false });
async function read<T>(path: string): Promise<T> {
  const response = await apiFetch(path);
  if (!response.ok) throw new Error("Data lokasi gagal dimuat");
  return (await response.json()).data;
}
function LocationForm({ school }: { school: MapSchool }) {
  const client = useQueryClient();
  const [latitude, setLatitude] = useState(school.latitude?.toString() ?? "");
  const [longitude, setLongitude] = useState(
    school.longitude?.toString() ?? ""
  );
  const [source, setSource] = useState(school.location_source);
  const [verified, setVerified] = useState(school.location_verified);
  const [saving, setSaving] = useState(false);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [saved, setSaved] = useState(school);
  const point = {
    ...school,
    latitude: latitude.trim() ? Number(latitude) : null,
    longitude: longitude.trim() ? Number(longitude) : null,
    location_verified: verified,
  };
  const valid = hasCoordinates(point);
  async function save(event: React.FormEvent) {
    event.preventDefault();
    setError("");
    setMessage("");
    if (
      (!valid && (latitude.trim() || longitude.trim())) ||
      (!valid && verified)
    ) {
      setError(
        "Isi pasangan koordinat yang valid. Lokasi kosong tidak dapat diverifikasi."
      );
      return;
    }
    setSaving(true);
    try {
      const response = await apiFetch(`/schools/${school.id}/location`, {
        method: "PUT",
        body: JSON.stringify({
          latitude: point.latitude,
          longitude: point.longitude,
          location_source: source,
          location_verified: verified,
        }),
      });
      if (!response.ok)
        throw new Error(
          response.status === 403
            ? "Anda tidak memiliki akses mengubah lokasi sekolah ini."
            : "Lokasi belum berhasil disimpan. Silakan coba kembali."
        );
      const result = await response.json();
      client.setQueryData(["school-location", school.id], result.data);
      setSaved(result.data);
      setMessage("Lokasi sekolah berhasil disimpan.");
      await client.invalidateQueries({ queryKey: ["school-locations"] });
    } catch (cause) {
      setError(
        cause instanceof Error ? cause.message : "Gagal menyimpan lokasi."
      );
    } finally {
      setSaving(false);
    }
  }
  return (
    <form onSubmit={save} className="mt-5 grid gap-5 lg:grid-cols-2">
      <div className="lg:col-span-2">
        <h3 className="font-semibold">{school.name}</h3>
        <p className="text-sm text-slate-500">
          NPSN: {school.npsn || "Belum tersedia"}
        </p>
      </div>
      <fieldset
        disabled={saving}
        className="grid content-start gap-4 rounded-xl border border-slate-200 bg-slate-50/70 p-4 sm:grid-cols-2"
      >
        <FormField
          id="school-latitude"
          label="Latitude"
          hint="Rentang -90 sampai 90."
        >
          <input
            id="school-latitude"
            aria-describedby="school-latitude-hint"
            className="portal-input"
            type="number"
            step="any"
            min="-90"
            max="90"
            value={latitude}
            onChange={(e) => setLatitude(e.target.value)}
          />
        </FormField>
        <FormField
          id="school-longitude"
          label="Longitude"
          hint="Rentang -180 sampai 180."
        >
          <input
            id="school-longitude"
            aria-describedby="school-longitude-hint"
            className="portal-input"
            type="number"
            step="any"
            min="-180"
            max="180"
            value={longitude}
            onChange={(e) => setLongitude(e.target.value)}
          />
        </FormField>
        <FormField id="location-source" label="Sumber koordinat">
          <input
            id="location-source"
            className="portal-input"
            maxLength={1000}
            value={source}
            onChange={(e) => setSource(e.target.value)}
            placeholder="Sumber resmi atau hasil pemeriksaan lapangan"
          />
        </FormField>
        <label className="flex items-center gap-3 text-sm">
          <input
            type="checkbox"
            checked={verified}
            onChange={(e) => setVerified(e.target.checked)}
          />
          Lokasi terverifikasi
        </label>
      </fieldset>
      {valid ? (
        <div className="overflow-hidden rounded-xl border">
          <MapBoundary>
            <Canvas schools={[point]} compact />
          </MapBoundary>
        </div>
      ) : (
        <EmptyState
          title="Preview lokasi belum tersedia"
          description="Masukkan latitude dan longitude yang valid untuk menampilkan titik."
        />
      )}
      <p className="text-xs text-slate-500 lg:col-span-2">
        Terakhir diperbarui:{" "}
        {saved.location_updated_at
          ? new Date(saved.location_updated_at).toLocaleString("id-ID", {
              timeZone: "Asia/Jakarta",
            }) + " WIB"
          : "Belum pernah"}
        {saved.location_updated_by ? ` oleh ${saved.location_updated_by}` : ""}
      </p>
      {message && (
        <p role="status" className="text-sm text-emerald-700">
          {message}
        </p>
      )}
      {error && (
        <p role="alert" className="text-sm text-red-700">
          {error}
        </p>
      )}
      <Button disabled={saving} type="submit">
        {saving ? "Menyimpan…" : "Simpan lokasi sekolah"}
      </Button>
    </form>
  );
}
function SelectedLocation({ id }: { id: number }) {
  const location = useQuery({
    queryKey: ["school-location", id],
    queryFn: () => read<MapSchool>(`/schools/${id}/location`),
  });
  if (location.isPending) return <LoadingState />;
  if (location.isError)
    return (
      <EmptyState
        title="Lokasi sekolah gagal dimuat"
        action={<Button onClick={() => location.refetch()}>Coba lagi</Button>}
      />
    );
  return <LocationForm key={id} school={location.data} />;
}
export function SchoolLocationSettings() {
  const { user, status } = useAuth();
  const isSchoolAccount = user?.role === "SCHOOL_ADMIN";
  const [selected, setSelected] = useState("");
  const [search, setSearch] = useState("");
  const schools = useQuery({
    queryKey: ["location-settings-schools", user?.id],
    queryFn: () =>
      read<{ id: number; namaSekolah: string }[]>("/user/me/schools"),
    enabled: status === "authenticated",
  });
  const ownSchool = resolveLocationSchool(user, schools.data ?? [], selected);
  const id = ownSchool ? String(ownSchool.id) : "";
  const visibleSchools = schools.data?.filter(
    (school) =>
      String(school.id) === id ||
      school.namaSekolah
        .toLocaleLowerCase("id-ID")
        .includes(search.toLocaleLowerCase("id-ID"))
  );
  return (
    <section id="lokasi-sekolah" className="portal-card scroll-mt-6 p-4 sm:p-6">
      <SectionHeader
        title="Lokasi Sekolah"
        description="Kelola titik sekolah yang ditampilkan pada peta pendidikan. Kosongkan kedua koordinat untuk menyembunyikan marker."
      />
      {schools.isPending ? (
        <LoadingState />
      ) : schools.isError ? (
        <EmptyState
          title="Daftar sekolah gagal dimuat"
          action={<Button onClick={() => schools.refetch()}>Coba lagi</Button>}
        />
      ) : (
        <>
          {isSchoolAccount ? (
            ownSchool ? (
              <div className="rounded-xl border border-blue-100 bg-blue-50 p-4">
                <p className="text-xs font-medium text-blue-700">
                  Sekolah terhubung
                </p>
                <h3 className="mt-1 font-semibold text-slate-900">
                  {ownSchool.namaSekolah}
                </h3>
              </div>
            ) : (
              <EmptyState
                title="Akun belum terhubung dengan sekolah"
                description="Mapping sekolah pada akun tidak ditemukan. Hubungi administrator untuk memeriksa keterkaitan akun; lokasi sekolah lain tidak dapat dipilih."
              />
            )
          ) : (
            <>
              <FormField id="location-school-search" label="Cari sekolah">
                <input
                  id="location-school-search"
                  className="portal-input mb-3"
                  value={search}
                  onChange={(event) => setSearch(event.target.value)}
                  placeholder="Ketik nama sekolah dalam cakupan akses Anda"
                />
              </FormField>
              <FormField id="location-school" label="Sekolah">
                <select
                  id="location-school"
                  className="portal-input"
                  value={id}
                  onChange={(e) => setSelected(e.target.value)}
                >
                  <option value="">Pilih sekolah</option>
                  {visibleSchools?.map((s) => (
                    <option key={s.id} value={s.id}>
                      {s.namaSekolah}
                    </option>
                  ))}
                </select>
              </FormField>
            </>
          )}
          {id && <SelectedLocation key={id} id={Number(id)} />}
        </>
      )}
      {user?.role === "ADMIN_MASTER" && <SchoolLocationImport onImported={() => setSelected("")} />}
    </section>
  );
}
