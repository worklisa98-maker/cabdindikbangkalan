export type AppRole =
  | "GASPOL_VOTER"
  | "ADMIN"
  | "ADMIN_CABDIN"
  | "SCHOOL_ADMIN"
  | "USER_SEKOLAH_GASPOL"
  | "USER_MGMP"
  | "ADMIN_MASTER"
  | "PENGAWAS"
  | "VALIDATOR";

export type AuthUser = {
  id: number;
  username: string;
  role: AppRole;
  schoolId?: number | null;
  namaInstansi?: string | null;
  npsnInstansi?: string | null;
  allowedLevels?: string[] | null;
  schoolLevel?: string | null;
};

export type AuthStatus = "loading" | "authenticated" | "unauthenticated";

export type AuthSnapshot = {
  user: AuthUser | null;
  status: AuthStatus;
};
