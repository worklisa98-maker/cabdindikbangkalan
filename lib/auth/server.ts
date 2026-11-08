import "server-only";

import { cookies } from "next/headers";

import { TOKEN_KEY } from "@/lib/auth";
import type { AuthSnapshot, AuthUser } from "@/lib/auth/types";

const getBackendBaseUrl = () => {
  const baseUrl = process.env.BACKEND_URL ?? "";
  return baseUrl.replace(/\/$/, "");
};

const normalizeServerUser = (payload: unknown): AuthUser | null => {
  if (!payload || typeof payload !== "object") {
    return null;
  }

  const data =
    "data" in payload &&
    payload.data &&
    typeof payload.data === "object" &&
    !Array.isArray(payload.data)
      ? payload.data
      : payload;

  if (!data || typeof data !== "object" || Array.isArray(data)) {
    return null;
  }

  const record = data as Record<string, unknown>;
  const id =
    typeof record.id === "number" && Number.isFinite(record.id)
      ? record.id
      : null;
  const username =
    typeof record.username === "string" ? record.username.trim() : "";
  const role = typeof record.role === "string" ? record.role.trim() : "";

  if (id == null || !username || !role) {
    return null;
  }

  return {
    id,
    username,
    role: role as AuthUser["role"],
    schoolId: typeof record.schoolId === "number" ? record.schoolId : null,
    namaInstansi:
      typeof record.namaInstansi === "string" ? record.namaInstansi : null,
    npsnInstansi:
      typeof record.npsnInstansi === "string" ? record.npsnInstansi : null,
    allowedLevels: Array.isArray(record.allowedLevels)
      ? record.allowedLevels.filter(
          (value): value is string => typeof value === "string"
        )
      : null,
    schoolLevel:
      typeof record.schoolLevel === "string"
        ? record.schoolLevel
        : typeof record.school_level === "string"
          ? record.school_level
          : null,
  };
};

export const getServerAuthSnapshot = async (): Promise<AuthSnapshot> => {
  const cookieStore = await cookies();
  const token = cookieStore.get(TOKEN_KEY)?.value?.trim();

  if (!token) {
    return { user: null, status: "unauthenticated" };
  }

  const baseUrl = getBackendBaseUrl();
  if (!baseUrl) {
    return { user: null, status: "loading" };
  }

  try {
    const response = await fetch(`${baseUrl}/auth/me`, {
      method: "GET",
      headers: {
        Authorization: `Bearer ${token}`,
      },
      cache: "no-store",
    });

    if (response.status === 401) {
      return { user: null, status: "unauthenticated" };
    }

    if (!response.ok) {
      return { user: null, status: "loading" };
    }

    const payload = await response.json().catch(() => null);
    const user = normalizeServerUser(payload);

    if (!user) {
      return { user: null, status: "loading" };
    }

    return { user, status: "authenticated" };
  } catch {
    return { user: null, status: "loading" };
  }
};
