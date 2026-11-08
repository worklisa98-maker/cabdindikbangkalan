"use client";

import { useCallback, useEffect, useState } from "react";

import { apiFetch } from "@/lib/api/client";
import { formatIndoDateTime } from "@/lib/date";
import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";

type PtChoiceRuleItem = {
  id: number;
  tahun_lulus?: number | null;
  jalur: "SNBP" | "SNBT" | "MANDIRI" | "KEDINASAN" | "LAINNYA";
  max_choices: number;
  is_active: boolean;
  updated_at?: string | null;
};

const JALUR_OPTIONS: PtChoiceRuleItem["jalur"][] = [
  "SNBP",
  "SNBT",
  "MANDIRI",
  "KEDINASAN",
  "LAINNYA",
];

const readApiError = async (response: Response) => {
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

const mapApiError = (code: string) => {
  switch (code) {
    case "PT_CHOICE_RULE_JALUR_INVALID":
      return "Jalur wajib salah satu dari SNBP, SNBT, MANDIRI, KEDINASAN, atau LAINNYA.";
    case "PT_CHOICE_RULE_MAX_CHOICES_INVALID":
      return "Maksimal pilihan harus bernilai 1 sampai 4.";
    case "PT_CHOICE_RULE_ALREADY_EXISTS":
      return "Rule aktif untuk kombinasi tahun lulus dan jalur tersebut sudah ada.";
    case "PT_CHOICE_RULE_NOT_FOUND":
      return "Rule pilihan PT tidak ditemukan.";
    case "INVALID_PT_CHOICE_RULE_ID":
      return "ID rule pilihan PT tidak valid.";
    case "INVALID_PAYLOAD":
      return "Payload pengaturan tidak valid.";
    default:
      return "";
  }
};

const fetchRules = async () => {
  const response = await apiFetch("/admin/master/pt-choice-rules");
  if (!response.ok) {
    const code = await readApiError(response);
    throw new Error(mapApiError(code) || "Gagal memuat pengaturan pilihan PT.");
  }
  const payload = (await response.json()) as {
    data?: { rules?: PtChoiceRuleItem[] };
  };
  return payload.data?.rules ?? [];
};

const createRule = async (payload: {
  tahun_lulus?: number | null;
  jalur: PtChoiceRuleItem["jalur"];
  max_choices: number;
  is_active: boolean;
}) => {
  const response = await apiFetch("/admin/master/pt-choice-rules", {
    method: "POST",
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const code = await readApiError(response);
    throw new Error(mapApiError(code) || "Gagal menambahkan rule pilihan PT.");
  }
};

const updateRule = async (
  id: number,
  payload: Partial<{
    tahun_lulus: number | null;
    jalur: PtChoiceRuleItem["jalur"];
    max_choices: number;
    is_active: boolean;
  }>
) => {
  const response = await apiFetch(`/admin/master/pt-choice-rules/${id}`, {
    method: "PATCH",
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const code = await readApiError(response);
    throw new Error(mapApiError(code) || "Gagal memperbarui rule pilihan PT.");
  }
};

export function PtChoiceRulesCard() {
  const [items, setItems] = useState<PtChoiceRuleItem[]>([]);
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  const [isLoading, setIsLoading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [savingId, setSavingId] = useState<number | null>(null);
  const [newYear, setNewYear] = useState("");
  const [newJalur, setNewJalur] =
    useState<PtChoiceRuleItem["jalur"]>("SNBP");
  const [newMaxChoices, setNewMaxChoices] = useState("4");
  const [newIsActive, setNewIsActive] = useState(true);

  const loadItems = useCallback(async () => {
    setIsLoading(true);
    setError("");
    try {
      const next = await fetchRules();
      setItems(next);
    } catch (loadErr) {
      setError(
        loadErr instanceof Error
          ? loadErr.message
          : "Gagal memuat pengaturan pilihan PT."
      );
    } finally {
      setIsLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadItems();
  }, [loadItems]);

  const handleCreate = async () => {
    setIsCreating(true);
    setError("");
    setMessage("");
    try {
      const parsedYear = newYear.trim() ? Number(newYear) : null;
      await createRule({
        tahun_lulus:
          parsedYear && Number.isFinite(parsedYear) ? parsedYear : null,
        jalur: newJalur,
        max_choices: Number(newMaxChoices) || 4,
        is_active: newIsActive,
      });
      setNewYear("");
      setNewMaxChoices("4");
      setNewIsActive(true);
      setMessage("Rule pilihan PT berhasil ditambahkan.");
      await loadItems();
    } catch (createErr) {
      setError(
        createErr instanceof Error
          ? createErr.message
          : "Gagal menambahkan rule pilihan PT."
      );
    } finally {
      setIsCreating(false);
    }
  };

  const handleQuickUpdate = async (
    item: PtChoiceRuleItem,
    payload: { max_choices?: number; is_active?: boolean }
  ) => {
    setSavingId(item.id);
    setError("");
    setMessage("");
    try {
      await updateRule(item.id, payload);
      setMessage("Rule pilihan PT berhasil diperbarui.");
      await loadItems();
    } catch (updateErr) {
      setError(
        updateErr instanceof Error
          ? updateErr.message
          : "Gagal memperbarui rule pilihan PT."
      );
    } finally {
      setSavingId(null);
    }
  };

  return (
    <Card className="border-zinc-200/80 bg-white/95 shadow-sm">
      <CardContent className="space-y-4 p-6">
        <div className="space-y-1">
          <p className="text-xs font-semibold tracking-[0.3em] text-zinc-500 uppercase">
            Pengaturan Pilihan PT
          </p>
          <h3 className="text-lg font-semibold text-zinc-900">
            Rule aktif per jalur dan tahun lulus
          </h3>
          <p className="text-sm text-zinc-600">
            Template tetap 4 slot. Backend memakai rule ini untuk membatasi
            input aktif per jalur dan tahun lulus.
          </p>
        </div>

        <div className="grid gap-3 md:grid-cols-4">
          <label className="space-y-1 text-sm text-zinc-700">
            <span className="font-medium">Tahun Lulus</span>
            <input
              value={newYear}
              onChange={(event) => setNewYear(event.target.value)}
              placeholder="Kosong = Default"
              className="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none transition focus:border-zinc-400"
            />
          </label>
          <label className="space-y-1 text-sm text-zinc-700">
            <span className="font-medium">Jalur</span>
            <select
              value={newJalur}
              onChange={(event) =>
                setNewJalur(event.target.value as PtChoiceRuleItem["jalur"])
              }
              className="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none transition focus:border-zinc-400"
            >
              {JALUR_OPTIONS.map((jalur) => (
                <option key={jalur} value={jalur}>
                  {jalur}
                </option>
              ))}
            </select>
          </label>
          <label className="space-y-1 text-sm text-zinc-700">
            <span className="font-medium">Maksimal Pilihan</span>
            <select
              value={newMaxChoices}
              onChange={(event) => setNewMaxChoices(event.target.value)}
              className="w-full rounded-xl border border-zinc-200 bg-white px-3 py-2 text-sm outline-none transition focus:border-zinc-400"
            >
              <option value="1">1</option>
              <option value="2">2</option>
              <option value="3">3</option>
              <option value="4">4</option>
            </select>
          </label>
          <label className="space-y-2 text-sm text-zinc-700">
            <span className="font-medium">Status</span>
            <label className="flex items-center gap-2 rounded-xl border border-zinc-200 px-3 py-2">
              <input
                type="checkbox"
                checked={newIsActive}
                onChange={(event) => setNewIsActive(event.target.checked)}
              />
              <span>Aktif</span>
            </label>
          </label>
        </div>

        <div className="flex flex-wrap items-center gap-3">
          <button
            type="button"
            onClick={() => void handleCreate()}
            disabled={isCreating}
            className="rounded-full bg-zinc-900 px-4 py-2 text-sm font-semibold text-white transition hover:bg-zinc-700 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {isCreating ? "Menyimpan..." : "Tambah Rule"}
          </button>
          {message ? <p className="text-sm text-emerald-700">{message}</p> : null}
          {error ? <p className="text-sm text-rose-700">{error}</p> : null}
        </div>

        <div className="overflow-x-auto rounded-2xl border border-zinc-200">
          <Table>
            <TableHeader>
              <TableRow>
                <TableHead>Tahun Lulus</TableHead>
                <TableHead>Jalur</TableHead>
                <TableHead>Maksimal</TableHead>
                <TableHead>Status</TableHead>
                <TableHead>Updated</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {items.length === 0 ? (
                <TableRow>
                  <TableCell colSpan={5} className="py-6 text-center text-zinc-500">
                    {isLoading ? "Memuat..." : "Belum ada rule pilihan PT."}
                  </TableCell>
                </TableRow>
              ) : (
                items.map((item) => (
                  <TableRow key={item.id}>
                    <TableCell>{item.tahun_lulus ?? "Default"}</TableCell>
                    <TableCell>{item.jalur}</TableCell>
                    <TableCell>
                      <select
                        value={String(item.max_choices)}
                        onChange={(event) =>
                          void handleQuickUpdate(item, {
                            max_choices: Number(event.target.value),
                          })
                        }
                        disabled={savingId === item.id}
                        className="rounded-lg border border-zinc-200 bg-white px-2 py-1 text-sm"
                      >
                        <option value="1">1</option>
                        <option value="2">2</option>
                        <option value="3">3</option>
                        <option value="4">4</option>
                      </select>
                    </TableCell>
                    <TableCell>
                      <label className="flex items-center gap-2 text-sm">
                        <input
                          type="checkbox"
                          checked={item.is_active}
                          disabled={savingId === item.id}
                          onChange={(event) =>
                            void handleQuickUpdate(item, {
                              is_active: event.target.checked,
                            })
                          }
                        />
                        <span>{item.is_active ? "Aktif" : "Nonaktif"}</span>
                      </label>
                    </TableCell>
                    <TableCell>
                      {item.updated_at ? formatIndoDateTime(item.updated_at) : "-"}
                    </TableCell>
                  </TableRow>
                ))
              )}
            </TableBody>
          </Table>
        </div>
      </CardContent>
    </Card>
  );
}
