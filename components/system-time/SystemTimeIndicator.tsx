"use client";

import { useEffect, useMemo, useState } from "react";

import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import {
  formatSystemTimeTick,
  SYSTEM_TIME_REFRESH_EVENT,
  type SystemTimeStatus,
} from "@/lib/system-time";

const refreshIntervalMs = 15_000;

const readSystemTimeStatus = async (): Promise<SystemTimeStatus | null> => {
  const response = await apiFetch("/system-time", undefined, {
    skipAuthRedirect: true,
    suppressErrorStatuses: [401, 403, 404],
  });
  if (
    response.status === 403 ||
    response.status === 404 ||
    response.status === 401
  ) {
    return null;
  }
  if (!response.ok) {
    throw new Error("Gagal memuat waktu sistem.");
  }
  const payload = (await response.json()) as { data?: SystemTimeStatus };
  return payload.data ?? null;
};

export function SystemTimeIndicator() {
  const { status } = useAuth();
  const [systemTime, setSystemTime] = useState<SystemTimeStatus | null>(null);
  const [receivedAtMs, setReceivedAtMs] = useState(0);
  const [tickMs, setTickMs] = useState(() => Date.now());

  useEffect(() => {
    if (status !== "authenticated") {
      return;
    }

    let cancelled = false;

    const load = async () => {
      try {
        const data = await readSystemTimeStatus();
        if (cancelled) {
          return;
        }
        setSystemTime(data);
        setReceivedAtMs(Date.now());
      } catch {
        if (!cancelled) {
          setSystemTime(null);
        }
      }
    };

    void load();
    const interval = window.setInterval(() => {
      void load();
    }, refreshIntervalMs);
    const onRefresh = () => {
      void load();
    };
    window.addEventListener(SYSTEM_TIME_REFRESH_EVENT, onRefresh);

    return () => {
      cancelled = true;
      window.clearInterval(interval);
      window.removeEventListener(SYSTEM_TIME_REFRESH_EVENT, onRefresh);
    };
  }, [status]);

  useEffect(() => {
    if (!systemTime?.featureEnabled) {
      return;
    }
    const interval = window.setInterval(() => {
      setTickMs(Date.now());
    }, 1_000);
    return () => {
      window.clearInterval(interval);
    };
  }, [systemTime?.featureEnabled]);

  const liveEffectiveTime = useMemo(() => {
    if (!systemTime?.featureEnabled || !systemTime.effectiveTime) {
      return null;
    }
    const baseTime = new Date(systemTime.effectiveTime);
    if (Number.isNaN(baseTime.getTime())) {
      return null;
    }
    const drift = receivedAtMs > 0 ? tickMs - receivedAtMs : 0;
    return new Date(baseTime.getTime() + Math.max(drift, 0));
  }, [receivedAtMs, systemTime, tickMs]);

  if (
    status !== "authenticated" ||
    !systemTime?.featureEnabled ||
    !liveEffectiveTime
  ) {
    return null;
  }

  return (
    <div className="portal-container flex justify-end py-2">
      <div className="flex flex-wrap items-center gap-x-4 gap-y-1 rounded-lg border border-slate-200 bg-white px-3 py-2">
        <div className="flex flex-wrap items-center gap-2">
          <p className="text-[11px] font-semibold tracking-[0.22em] text-slate-500 uppercase">
            Waktu Berlaku
          </p>
          {systemTime.usingOverride ? (
            <span className="rounded-full bg-amber-100 px-2.5 py-1 text-[11px] font-semibold text-amber-800">
              SIMULASI AKTIF
            </span>
          ) : (
            <span className="rounded-full bg-emerald-100 px-2.5 py-1 text-[11px] font-semibold text-emerald-800">
              WAKTU REAL
            </span>
          )}
        </div>
        <p className="text-xs font-semibold text-slate-900">
          {formatSystemTimeTick(liveEffectiveTime)}
        </p>
        <p className="text-[11px] text-slate-500">
          {systemTime.timezone || "Asia/Jakarta"}
        </p>
      </div>
    </div>
  );
}
