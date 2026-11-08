"use client";

import { createContext, useContext, useEffect, useMemo, useState } from "react";
import { useQueryClient } from "@tanstack/react-query";

import { apiFetch } from "@/lib/api/client";
import { clearToken, getToken, setToken } from "@/lib/auth";
import { LOGOUT_BROADCAST_KEY } from "@/lib/idle-logout";
import type { AuthSnapshot, AuthStatus, AuthUser } from "@/lib/auth/types";

type AuthContextValue = {
  user: AuthUser | null;
  status: AuthStatus;
  setUser: (user: AuthUser | null) => void;
  setToken: (token: string) => void;
  logout: () => void;
};

const AuthContext = createContext<AuthContextValue | undefined>(undefined);

const normalizeAuthPayload = (payload: unknown): AuthUser | null => {
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

export const AuthProvider = ({
  children,
  initialAuth,
}: {
  children: React.ReactNode;
  initialAuth?: AuthSnapshot;
}) => {
  const [user, setUser] = useState<AuthUser | null>(initialAuth?.user ?? null);
  const [status, setStatus] = useState<AuthStatus>(
    initialAuth?.status ?? "loading"
  );
  const queryClient = useQueryClient();

  useEffect(() => {
    const token = getToken();
    if (!token) {
      queueMicrotask(() => {
        setUser(null);
        setStatus("unauthenticated");
      });
      return;
    }

    apiFetch("/auth/me")
      .then(async (response) => {
        if (!response.ok) {
          clearToken();
          setUser(null);
          setStatus("unauthenticated");
          return;
        }
        const payload = await response.json().catch(() => null);
        const nextUser = normalizeAuthPayload(payload);
        if (!nextUser) {
          clearToken();
          setUser(null);
          setStatus("unauthenticated");
          return;
        }
        setUser(nextUser);
        setStatus("authenticated");
      })
      .catch(() => {
        clearToken();
        setUser(null);
        setStatus("unauthenticated");
      });
  }, []);

  const value = useMemo<AuthContextValue>(
    () => ({
      user,
      status,
      setUser: (nextUser) => {
        setUser(nextUser);
        setStatus(nextUser ? "authenticated" : "unauthenticated");
      },
      setToken,
      logout: () => {
        queryClient.removeQueries({ queryKey: ["service-links", "user"] });
        queryClient.removeQueries({ queryKey: ["service-links", "admin"] });
        clearToken();
        if (typeof window !== "undefined") {
          window.localStorage.setItem(LOGOUT_BROADCAST_KEY, String(Date.now()));
        }
        setUser(null);
        setStatus("unauthenticated");
      },
    }),
    [user, status, queryClient]
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
};

export const useAuth = () => {
  const context = useContext(AuthContext);
  if (!context) {
    throw new Error("useAuth must be used within AuthProvider");
  }
  return context;
};

export type { AuthSnapshot, AuthStatus, AuthUser };
