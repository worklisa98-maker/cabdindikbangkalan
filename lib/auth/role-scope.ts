import type { AppRole } from "@/lib/auth/types";

type RoleLike = string | null | undefined;

export const getPrimaryAppRole = (role: RoleLike): AppRole | null => {
  const normalized = (role ?? "").trim().toUpperCase();

  switch (normalized) {
    case "ADMIN":
    case "ADMIN_CABDIN":
    case "SCHOOL_ADMIN":
    case "USER_SEKOLAH_GASPOL":
    case "USER_MGMP":
    case "ADMIN_MASTER":
    case "PENGAWAS":
    case "GASPOL_VOTER":
    case "VALIDATOR":
      return normalized;
    default:
      return null;
  }
};

export const normalizeAppRole = (role: RoleLike) => {
  const primaryRole = getPrimaryAppRole(role);
  return primaryRole === "ADMIN_CABDIN" ? "ADMIN" : primaryRole ?? "";
};

export const hasRoleAccess = (
  role: RoleLike,
  allowedRoles: readonly (AppRole | "PUBLIC")[]
): boolean => {
  const primaryRole = getPrimaryAppRole(role);
  if (!primaryRole) {
    return allowedRoles.includes("PUBLIC");
  }

  return allowedRoles.some((allowedRole) => {
    if (allowedRole === "PUBLIC") {
      return false;
    }
    if (allowedRole === primaryRole) {
      return true;
    }
    return allowedRole === "ADMIN" && primaryRole === "ADMIN_CABDIN";
  });
};

const scopedSchoolFilterRoles = new Set([
  "ADMIN",
  "ADMIN_CABDIN",
  "ADMIN_MASTER",
  "PENGAWAS",
  "VALIDATOR",
]);

export const canUseScopedSchoolFilter = (role: RoleLike): boolean =>
  scopedSchoolFilterRoles.has(normalizeAppRole(role));

export const isSchoolAdminRole = (role: RoleLike): boolean =>
  normalizeAppRole(role) === "SCHOOL_ADMIN";

export const isPengawasRole = (role: RoleLike): boolean =>
  normalizeAppRole(role) === "PENGAWAS";

export const isAdminMonitoringRole = (role: RoleLike): boolean => {
  const normalized = normalizeAppRole(role);
  return normalized === "ADMIN" || normalized === "ADMIN_MASTER";
};
