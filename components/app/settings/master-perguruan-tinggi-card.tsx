"use client";

import { useCallback, useEffect, useState } from "react";

import { apiFetch } from "@/lib/api/client";
import { formatIndoDateTime } from "@/lib/date";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type MasterPerguruanTinggiItem = {
  id: number;
  nama: string;
  isActive: boolean;
  source?: string | null;
  createdAt?: string | null;
  updatedAt?: string | null;
};

const readMasterPerguruanTinggiError = async (response: Response) => {
  const body = (await response.json().catch(() => null)) as {
    message?: string;
    error?: string;
    details?: unknown;
  } | null;
  if (!body) return "";
  if (typeof body.message === "string" && body.message.trim()) {
    return body.message.trim();
  }
  if (typeof body.details === "string" && body.details.trim()) {
    return body.details.trim();
  }
  if (typeof body.error === "string" && body.error.trim()) {
    return body.error.trim();
  }
  return "";
};

const mapMasterPerguruanTinggiError = (code: string) => {
  switch (code) {
    case "REF_KAMPUS_NAME_REQUIRED":
      return "Nama perguruan tinggi wajib diisi.";
    case "REF_KAMPUS_NAME_EXISTS":
      return "Nama perguruan tinggi sudah ada di master.";
    case "REF_KAMPUS_NOT_FOUND":
      return "Data perguruan tinggi tidak ditemukan.";
    case "INVALID_PAYLOAD":
      return "Perubahan tidak valid.";
    case "INVALID_REF_KAMPUS_ID":
      return "ID perguruan tinggi tidak valid.";
    default:
      return "";
  }
};

const fetchMasterPerguruanTinggi = async (params: {
  q?: string;
  includeInactive?: boolean;
  limit?: number;
}) => {
  const query = new URLSearchParams();
  if (params.q) {
    query.set("q", params.q);
  }
  if (params.includeInactive) {
    query.set("includeInactive", "1");
  }
  if (params.limit) {
    query.set("limit", String(params.limit));
  }
  const response = await apiFetch(
    `/admin/master/ref-kampus${query.toString() ? `?${query.toString()}` : ""}`
  );
  if (!response.ok) {
    const code = await readMasterPerguruanTinggiError(response);
    throw new Error(
      mapMasterPerguruanTinggiError(code) ||
        "Gagal memuat master perguruan tinggi."
    );
  }
  const payload = (await response.json()) as {
    data?: MasterPerguruanTinggiItem[];
  };
  return Array.isArray(payload.data) ? payload.data : [];
};

const createMasterPerguruanTinggi = async (nama: string) => {
  const response = await apiFetch("/admin/master/ref-kampus", {
    method: "POST",
    body: JSON.stringify({ nama }),
  });
  if (!response.ok) {
    const code = await readMasterPerguruanTinggiError(response);
    throw new Error(
      mapMasterPerguruanTinggiError(code) ||
        "Gagal menambahkan perguruan tinggi."
    );
  }
  return response.json();
};

const updateMasterPerguruanTinggi = async (
  id: number,
  payload: { nama?: string; isActive?: boolean }
) => {
  const response = await apiFetch(`/admin/master/ref-kampus/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const code = await readMasterPerguruanTinggiError(response);
    throw new Error(
      mapMasterPerguruanTinggiError(code) ||
        "Gagal memperbarui perguruan tinggi."
    );
  }
  return response.json();
};

const sourceLabel = (source?: string | null) => {
  switch (source) {
    case "manual":
      return "Manual";
    case "seed_csv":
      return "Seed CSV";
    default:
      return source?.trim() || "-";
  }
};

export function MasterPerguruanTinggiCard() {
  const [items, setItems] = useState<MasterPerguruanTinggiItem[]>([]);
  const [query, setQuery] = useState("");
  const [includeInactive, setIncludeInactive] = useState(true);
  const [createName, setCreateName] = useState("");
  const [editingId, setEditingId] = useState<number | null>(null);
  const [editingName, setEditingName] = useState("");
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [savingId, setSavingId] = useState<number | null>(null);

  const debouncedQuery = useDebouncedValue(query, 300);

  const loadItems = useCallback(async () => {
    setIsLoading(true);
    setError("");
    try {
      const records = await fetchMasterPerguruanTinggi({
        q: debouncedQuery.trim(),
        includeInactive,
        limit: 500,
      });
      setItems(records);
    } catch (loadErr) {
      setError(
        loadErr instanceof Error
          ? loadErr.message
          : "Gagal memuat master perguruan tinggi."
      );
    } finally {
      setIsLoading(false);
    }
  }, [debouncedQuery, includeInactive]);

  useEffect(() => {
    void loadItems();
  }, [loadItems]);

  const handleCreate = async () => {
    setIsCreating(true);
    setError("");
    setMessage("");
    try {
      await createMasterPerguruanTinggi(createName);
      setCreateName("");
      setMessage("Perguruan tinggi baru berhasil ditambahkan.");
      await loadItems();
    } catch (createErr) {
      setError(
        createErr instanceof Error
          ? createErr.message
          : "Gagal menambahkan perguruan tinggi."
      );
    } finally {
      setIsCreating(false);
    }
  };

  const startEdit = (item: MasterPerguruanTinggiItem) => {
    setEditingId(item.id);
    setEditingName(item.nama);
    setError("");
    setMessage("");
  };

  const cancelEdit = () => {
    setEditingId(null);
    setEditingName("");
  };

  const handleSaveEdit = async (id: number) => {
    setSavingId(id);
    setError("");
    setMessage("");
    try {
      await updateMasterPerguruanTinggi(id, { nama: editingName });
      setMessage("Nama perguruan tinggi berhasil diperbarui.");
      cancelEdit();
      await loadItems();
    } catch (saveErr) {
      setError(
        saveErr instanceof Error
          ? saveErr.message
          : "Gagal memperbarui perguruan tinggi."
      );
    } finally {
      setSavingId(null);
    }
  };

  const handleToggleActive = async (
    item: MasterPerguruanTinggiItem,
    nextValue: boolean
  ) => {
    setSavingId(item.id);
    setError("");
    setMessage("");
    try {
      await updateMasterPerguruanTinggi(item.id, { isActive: nextValue });
      setMessage(
        nextValue
          ? "Perguruan tinggi berhasil diaktifkan."
          : "Perguruan tinggi berhasil dinonaktifkan."
      );
      await loadItems();
    } catch (toggleErr) {
      setError(
        toggleErr instanceof Error
          ? toggleErr.message
          : "Gagal memperbarui status perguruan tinggi."
      );
    } finally {
      setSavingId(null);
    }
  };

  return (
    <Card>
      <CardContent className="space-y-4">
        <div className="space-y-1">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Master Perguruan Tinggi
          </p>
          <h2 className="text-lg font-semibold">Kelola Referensi Kampus/PT</h2>
          <p className="text-sm text-zinc-600">
            Referensi runtime kampus memakai tabel <code>ref_kampus</code>.
            Perubahan di sini langsung dipakai dropdown, suggestion, template,
            dan validasi yang membaca referensi PT dari database.
          </p>
        </div>

        <div className="grid gap-3 md:grid-cols-[minmax(0,1fr)_auto]">
          <input
            value={createName}
            onChange={(event) => setCreateName(event.target.value)}
            placeholder="Tambah nama perguruan tinggi baru"
            className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
          />
          <button
            type="button"
            onClick={handleCreate}
            disabled={isCreating}
            className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isCreating ? "Menyimpan..." : "Tambah PT"}
          </button>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <input
            value={query}
            onChange={(event) => setQuery(event.target.value)}
            placeholder="Cari nama perguruan tinggi"
            className="min-w-[240px] flex-1 rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
          />
          <label className="flex items-center gap-2 text-sm text-zinc-700">
            <input
              type="checkbox"
              checked={includeInactive}
              onChange={(event) => setIncludeInactive(event.target.checked)}
              className="h-4 w-4 rounded border-zinc-300"
            />
            Tampilkan nonaktif
          </label>
        </div>

        {error ? (
          <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
            {error}
          </p>
        ) : null}
        {message ? (
          <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
            {message}
          </p>
        ) : null}

        <Table wrapperClassName="max-h-[420px] overflow-y-auto">
          <TableHeader>
            <TableRow>
              <TableHead className="sticky top-0 z-10 bg-white">Nama</TableHead>
              <TableHead className="sticky top-0 z-10 bg-white">Sumber</TableHead>
              <TableHead className="sticky top-0 z-10 bg-white">Status</TableHead>
              <TableHead className="sticky top-0 z-10 bg-white">
                Diperbarui
              </TableHead>
              <TableHead className="sticky top-0 z-10 bg-white text-center">
                Aksi
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {isLoading ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-sm text-zinc-500">
                  Memuat data...
                </TableCell>
              </TableRow>
            ) : items.length === 0 ? (
              <TableRow>
                <TableCell colSpan={5} className="text-center text-sm text-zinc-500">
                  Belum ada data perguruan tinggi yang cocok.
                </TableCell>
              </TableRow>
            ) : (
              items.map((item) => {
                const isRowSaving = savingId === item.id;
                const isEditing = editingId === item.id;
                return (
                  <TableRow key={item.id}>
                    <TableCell className="align-top">
                      {isEditing ? (
                        <input
                          value={editingName}
                          onChange={(event) => setEditingName(event.target.value)}
                          className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                        />
                      ) : (
                        <div className="font-semibold text-zinc-900">{item.nama}</div>
                      )}
                    </TableCell>
                    <TableCell className="align-top text-sm text-zinc-600">
                      {sourceLabel(item.source)}
                    </TableCell>
                    <TableCell className="align-top">
                      <span
                        className={`inline-flex rounded-full px-2.5 py-1 text-xs font-semibold ${
                          item.isActive
                            ? "bg-emerald-50 text-emerald-700"
                            : "bg-zinc-100 text-zinc-600"
                        }`}
                      >
                        {item.isActive ? "Aktif" : "Nonaktif"}
                      </span>
                    </TableCell>
                    <TableCell className="align-top text-sm text-zinc-600">
                      {item.updatedAt ? formatIndoDateTime(item.updatedAt) : "-"}
                    </TableCell>
                    <TableCell className="align-top">
                      <div className="flex flex-wrap items-center justify-center gap-2">
                        {isEditing ? (
                          <>
                            <button
                              type="button"
                              onClick={() => void handleSaveEdit(item.id)}
                              disabled={isRowSaving}
                              className="rounded-lg border border-emerald-200 bg-emerald-50 px-3 py-1.5 text-sm font-semibold text-emerald-700 transition hover:bg-emerald-100 disabled:cursor-not-allowed disabled:opacity-60"
                            >
                              Simpan
                            </button>
                            <button
                              type="button"
                              onClick={cancelEdit}
                              disabled={isRowSaving}
                              className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                            >
                              Batal
                            </button>
                          </>
                        ) : (
                          <>
                            <button
                              type="button"
                              onClick={() => startEdit(item)}
                              disabled={isRowSaving}
                              className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
                            >
                              Edit
                            </button>
                            <button
                              type="button"
                              onClick={() =>
                                void handleToggleActive(item, !item.isActive)
                              }
                              disabled={isRowSaving}
                              className={`rounded-lg px-3 py-1.5 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-60 ${
                                item.isActive
                                  ? "border border-amber-200 bg-amber-50 text-amber-700 hover:bg-amber-100"
                                  : "border border-emerald-200 bg-emerald-50 text-emerald-700 hover:bg-emerald-100"
                              }`}
                            >
                              {item.isActive ? "Nonaktifkan" : "Aktifkan"}
                            </button>
                          </>
                        )}
                      </div>
                    </TableCell>
                  </TableRow>
                );
              })
            )}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  );
}
