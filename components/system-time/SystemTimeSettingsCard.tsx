"use client";

import { useEffect, useMemo, useState } from "react";
import { RotateCcw, Save, ShieldAlert } from "lucide-react";

import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import {
  dispatchSystemTimeRefresh,
  formatSystemTime,
  formatSystemTimeTick,
  SYSTEM_TIME_REFRESH_EVENT,
  type SystemTimeStatus,
} from "@/lib/system-time";

const inputClass =
  "w-full rounded-lg border border-slate-200 bg-white px-3 py-2 text-sm text-slate-900 outline-none transition focus:border-sky-400";

const buttonBaseClass =
  "inline-flex items-center justify-center rounded-lg px-3 py-2 text-sm font-semibold transition disabled:cursor-not-allowed disabled:opacity-60";

const fetchAdminSystemTime = async (): Promise<SystemTimeStatus | null> => {
  const response = await apiFetch("/admin/system-time", undefined, {
    skipAuthRedirect: true,
  });
  if (response.status === 403 || response.status === 404) {
    return null;
  }
  if (!response.ok) {
    throw new Error("Gagal memuat status simulasi waktu sistem.");
  }
  const payload = (await response.json()) as { data?: SystemTimeStatus };
  return payload.data ?? null;
};

const toDateTimeLocalValue = (value?: string | null) => {
  if (!value) {
    return "";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "";
  }
  const parts = new Intl.DateTimeFormat("sv-SE", {
    timeZone: "Asia/Jakarta",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).formatToParts(date);
  const get = (type: Intl.DateTimeFormatPartTypes) =>
    parts.find((part) => part.type === type)?.value ?? "00";
  return `${get("year")}-${get("month")}-${get("day")}T${get("hour")}:${get(
    "minute"
  )}:${get("second")}`;
};

export function SystemTimeSettingsCard() {
  const [status, setStatus] = useState<SystemTimeStatus | null>(null);
  const [loading, setLoading] = useState(true);
  const [busyAction, setBusyAction] = useState<"" | "save" | "reset">("");
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [overrideValue, setOverrideValue] = useState("");
  const [receivedAtMs, setReceivedAtMs] = useState(0);
  const [tickMs, setTickMs] = useState(() => Date.now());

  useEffect(() => {
    let cancelled = false;

    const load = async () => {
      setLoading(true);
      try {
        const nextStatus = await fetchAdminSystemTime();
        if (cancelled) {
          return;
        }
        setStatus(nextStatus);
        setOverrideValue(toDateTimeLocalValue(nextStatus?.overrideBaseTime));
        setReceivedAtMs(Date.now());
      } catch (loadError) {
        if (!cancelled) {
          setError(
            loadError instanceof Error
              ? loadError.message
              : "Gagal memuat simulasi waktu sistem."
          );
        }
      } finally {
        if (!cancelled) {
          setLoading(false);
        }
      }
    };

    void load();
    const onRefresh = () => {
      void load();
    };
    window.addEventListener(SYSTEM_TIME_REFRESH_EVENT, onRefresh);
    return () => {
      cancelled = true;
      window.removeEventListener(SYSTEM_TIME_REFRESH_EVENT, onRefresh);
    };
  }, []);

  useEffect(() => {
    if (!status?.featureEnabled) {
      return;
    }
    const interval = window.setInterval(() => {
      setTickMs(Date.now());
    }, 1_000);
    return () => {
      window.clearInterval(interval);
    };
  }, [status?.featureEnabled]);

  const liveEffectiveTime = useMemo(() => {
    if (!status?.effectiveTime) {
      return null;
    }
    const baseTime = new Date(status.effectiveTime);
    if (Number.isNaN(baseTime.getTime())) {
      return null;
    }
    const drift = receivedAtMs > 0 ? tickMs - receivedAtMs : 0;
    return new Date(baseTime.getTime() + Math.max(drift, 0));
  }, [receivedAtMs, status?.effectiveTime, tickMs]);

  const handleSaveOverride = async () => {
    setError("");
    setMessage("");
    setBusyAction("save");
    try {
      const response = await apiFetch("/admin/system-time/override", {
        method: "POST",
        body: JSON.stringify({ overrideDatetime: overrideValue }),
      });
      const payload = (await response.json().catch(() => null)) as {
        data?: SystemTimeStatus;
        message?: string;
      } | null;
      if (!response.ok || !payload?.data) {
        throw new Error(
          payload?.message === "INVALID_OVERRIDE_DATETIME"
            ? "Gunakan tanggal dan jam lengkap sampai detik dalam timezone Asia/Jakarta."
            : "Gagal menyimpan waktu simulasi."
        );
      }
      setStatus(payload.data);
      setOverrideValue(toDateTimeLocalValue(payload.data.overrideBaseTime));
      setReceivedAtMs(Date.now());
      setMessage("Waktu simulasi berhasil disimpan.");
      dispatchSystemTimeRefresh();
    } catch (saveError) {
      setError(
        saveError instanceof Error
          ? saveError.message
          : "Gagal menyimpan waktu simulasi."
      );
    } finally {
      setBusyAction("");
    }
  };

  const handleReset = async () => {
    setError("");
    setMessage("");
    setBusyAction("reset");
    try {
      const response = await apiFetch("/admin/system-time/reset", {
        method: "POST",
      });
      const payload = (await response.json().catch(() => null)) as {
        data?: SystemTimeStatus;
      } | null;
      if (!response.ok || !payload?.data) {
        throw new Error("Gagal mengembalikan waktu sistem ke waktu real.");
      }
      setStatus(payload.data);
      setOverrideValue("");
      setReceivedAtMs(Date.now());
      setMessage("Waktu sistem kembali memakai waktu real server.");
      dispatchSystemTimeRefresh();
    } catch (resetError) {
      setError(
        resetError instanceof Error
          ? resetError.message
          : "Gagal mengembalikan waktu sistem ke waktu real."
      );
    } finally {
      setBusyAction("");
    }
  };

  if (
    !loading &&
    !error &&
    (!status || !status.featureEnabled || !status.canManage)
  ) {
    return null;
  }

  return (
    <Card>
      <CardContent className="space-y-4">
        <div className="flex flex-wrap items-start justify-between gap-3">
          <div>
            <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Simulasi Waktu Sistem
            </p>
            <h2 className="text-lg font-semibold">
              Override waktu efektif global
            </h2>
            <p className="text-sm text-zinc-600">
              Hanya aktif di staging dan hanya bisa diubah oleh ADMIN_MASTER.
              Semua rule berbasis waktu yang sudah memakai helper global akan
              mengikuti waktu efektif ini.
            </p>
          </div>
          <div className="rounded-full border border-slate-200 bg-slate-50 px-3 py-2 text-xs font-semibold text-slate-700">
            {status?.usingOverride ? "SIMULASI AKTIF" : "WAKTU REAL"}
          </div>
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

        {loading ? (
          <p className="text-sm text-zinc-500">Memuat status waktu sistem...</p>
        ) : (
          <>
            <div className="grid gap-3 md:grid-cols-2">
              <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
                <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                  Waktu Real Server
                </p>
                <p className="mt-2 text-sm font-semibold text-slate-900">
                  {formatSystemTime(status?.realTime)}
                </p>
              </div>
              <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
                <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                  Waktu Efektif Sistem
                </p>
                <p className="mt-2 text-sm font-semibold text-slate-900">
                  {liveEffectiveTime
                    ? formatSystemTimeTick(liveEffectiveTime)
                    : "-"}
                </p>
              </div>
            </div>

            <div className="grid gap-3 md:grid-cols-2">
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                  Override datetime (Asia/Jakarta)
                </label>
                <input
                  type="datetime-local"
                  step={1}
                  value={overrideValue}
                  onChange={(event) => setOverrideValue(event.target.value)}
                  className={inputClass}
                />
                <p className="text-xs text-slate-500">
                  Input wajib sampai detik. Setelah disimpan, waktu simulasi
                  akan terus berjalan.
                </p>
              </div>
              <div className="space-y-2 rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
                <div className="flex items-start gap-2">
                  <ShieldAlert className="mt-0.5 h-4 w-4 shrink-0" />
                  <p>
                    Simulasi waktu hanya mengubah waktu efektif sistem. Data
                    bisnis yang sudah tersimpan di database tetap ada walaupun
                    nanti waktu dikembalikan ke real.
                  </p>
                </div>
                <p>
                  Override base:{" "}
                  <span className="font-semibold">
                    {formatSystemTime(status?.overrideBaseTime)}
                  </span>
                </p>
                <p>
                  Override dimulai pada:{" "}
                  <span className="font-semibold">
                    {formatSystemTime(status?.overrideStartedRealAt)}
                  </span>
                </p>
              </div>
            </div>

            <div className="flex flex-wrap gap-2">
              <button
                type="button"
                onClick={() => void handleSaveOverride()}
                disabled={busyAction !== "" || !overrideValue}
                className={`${buttonBaseClass} bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] text-white shadow-sm hover:opacity-95`}
              >
                <Save className="mr-2 h-4 w-4" />
                {busyAction === "save"
                  ? "Menyimpan..."
                  : "Simpan Waktu Simulasi"}
              </button>
              <button
                type="button"
                onClick={() => void handleReset()}
                disabled={busyAction !== ""}
                className={`${buttonBaseClass} border border-slate-300 bg-white text-slate-700 hover:bg-slate-50`}
              >
                <RotateCcw className="mr-2 h-4 w-4" />
                {busyAction === "reset"
                  ? "Memulihkan..."
                  : "Kembali ke Waktu Real"}
              </button>
            </div>

            <div className="space-y-2 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
              <p>
                Feature ini khusus testing staging. Production tidak mendukung
                override waktu.
              </p>
              <p>
                Gunakan badge global di pojok kanan atas untuk memastikan
                halaman lain membaca waktu efektif yang sama.
              </p>
            </div>

            <div className="space-y-3">
              <div>
                <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                  Audit Trail
                </p>
                <p className="text-sm text-slate-600">
                  Jejak perubahan waktu simulasi yang tersimpan di database.
                </p>
              </div>

              {(status?.recentAudits ?? []).length === 0 ? (
                <p className="rounded-md border border-dashed border-slate-200 px-3 py-3 text-sm text-slate-500">
                  Belum ada audit simulasi waktu sistem.
                </p>
              ) : (
                <div className="overflow-x-auto rounded-xl border border-slate-200">
                  <table className="min-w-full divide-y divide-slate-200 text-sm">
                    <thead className="bg-slate-50">
                      <tr>
                        <th className="px-3 py-2 text-left font-semibold text-slate-700">
                          Waktu
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-700">
                          Aksi
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-700">
                          Aktor
                        </th>
                        <th className="px-3 py-2 text-left font-semibold text-slate-700">
                          Override
                        </th>
                      </tr>
                    </thead>
                    <tbody className="divide-y divide-slate-100 bg-white">
                      {(status?.recentAudits ?? []).map((item) => (
                        <tr key={item.id}>
                          <td className="px-3 py-2 text-slate-600">
                            {formatSystemTime(item.createdAt)}
                          </td>
                          <td className="px-3 py-2 font-semibold text-slate-900">
                            {item.action}
                          </td>
                          <td className="px-3 py-2 text-slate-600">
                            {item.actorName || "-"} ({item.actorRole || "-"})
                          </td>
                          <td className="px-3 py-2 text-slate-600">
                            {formatSystemTime(item.overrideBaseTime)}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              )}
            </div>
          </>
        )}
      </CardContent>
    </Card>
  );
}
