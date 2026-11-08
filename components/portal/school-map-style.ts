export const SCHOOL_LEVEL_COLORS = {
  SMA: "#2563eb",
  SMK: "#10b981",
  SLB: "#f59e0b",
} as const;

export const UNKNOWN_LEVEL_COLOR = "#64748b";
export const LOCATION_OUTLINES = {
  verified: "#1e293b",
  unverified: "#94a3b8",
} as const;

export function normalizeSchoolLevel(level: string | null | undefined) {
  return typeof level === "string" ? level.trim().toUpperCase() : "";
}

export function schoolLevelColor(level: string | null | undefined) {
  switch (normalizeSchoolLevel(level)) {
    case "SMA":
      return SCHOOL_LEVEL_COLORS.SMA;
    case "SMK":
      return SCHOOL_LEVEL_COLORS.SMK;
    case "SLB":
      return SCHOOL_LEVEL_COLORS.SLB;
    default:
      return UNKNOWN_LEVEL_COLOR;
  }
}
