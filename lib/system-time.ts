export const SYSTEM_TIME_REFRESH_EVENT = "system-time:refresh";
export const SYSTEM_TIME_TIMEZONE = "Asia/Jakarta";

export type SystemTimeAuditEntry = {
  id: number;
  action: string;
  actorUserId?: number | null;
  actorName: string;
  actorRole: string;
  overrideBaseTime?: string | null;
  overrideStartedRealAt?: string | null;
  payload?: unknown;
  createdAt: string;
};

export type SystemTimeStatus = {
  featureEnabled: boolean;
  canManage: boolean;
  usingOverride: boolean;
  timezone: string;
  realTime: string;
  effectiveTime: string;
  overrideBaseTime?: string | null;
  overrideStartedRealAt?: string | null;
  updatedBy?: number | null;
  updatedAt?: string | null;
  recentAudits?: SystemTimeAuditEntry[];
};

export const dispatchSystemTimeRefresh = () => {
  if (typeof window === "undefined") {
    return;
  }
  window.dispatchEvent(new CustomEvent(SYSTEM_TIME_REFRESH_EVENT));
};

export const formatSystemTime = (value?: string | null) => {
  if (!value) {
    return "-";
  }
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat("id-ID", {
    timeZone: SYSTEM_TIME_TIMEZONE,
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(date);
};

export const formatSystemTimeTick = (value: Date) =>
  new Intl.DateTimeFormat("id-ID", {
    timeZone: SYSTEM_TIME_TIMEZONE,
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  }).format(value);
