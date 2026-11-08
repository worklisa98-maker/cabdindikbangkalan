"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";

import { PasswordInput } from "@/components/password-input";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { AUTH_MESSAGE_KEY, getToken, setToken } from "@/lib/auth";
import { useAuth } from "@/lib/auth/client";

export default function LoginPage() {
  const router = useRouter();
  const { setUser } = useAuth();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [infoMessage] = useState<string | null>(() => {
    if (typeof window === "undefined") return null;
    const message = window.localStorage.getItem(AUTH_MESSAGE_KEY);
    if (message) {
      window.localStorage.removeItem(AUTH_MESSAGE_KEY);
    }
    return message;
  });
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (getToken()) {
      router.replace("/");
    }
  }, [router]);

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setIsSubmitting(true);

    const response = await apiFetch(
      "/auth/login",
      {
        method: "POST",
        body: JSON.stringify({ username, password }),
      },
      { skipAuthRedirect: true }
    );

    setIsSubmitting(false);
    if (!response.ok) {
      setError("Username atau password salah.");
      setIsSubmitting(false);
      return;
    }

    const payload = (await response.json()) as {
      data: {
        token: string;
        user: {
          id: number;
          username: string;
          role:
            | "ADMIN"
            | "ADMIN_CABDIN"
            | "SCHOOL_ADMIN"
            | "USER_SEKOLAH_GASPOL"
            | "USER_MGMP"
            | "ADMIN_MASTER"
            | "PENGAWAS"
            | "VALIDATOR";
          schoolId?: number | null;
          namaInstansi?: string | null;
          npsnInstansi?: string | null;
          school_level?: string | null;
        };
      };
    };
    setError(null);
    setToken(payload.data.token);
    setUser({
      id: payload.data.user.id,
      username: payload.data.user.username,
      role: payload.data.user.role,
      schoolId: payload.data.user.schoolId ?? null,
      namaInstansi: payload.data.user.namaInstansi ?? null,
      npsnInstansi: payload.data.user.npsnInstansi ?? null,
      schoolLevel: payload.data.user.school_level ?? null,
    });
    router.push("/");
  };

  return (
    <section className="mx-auto flex w-full max-w-md flex-1 flex-col justify-center">
      <Card>
        <CardContent className="space-y-6">
          <div className="space-y-1">
            <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
              Cabang Dinas Pendidikan
            </p>
            <h1 className="text-2xl font-semibold">Login</h1>
            <p className="text-sm text-zinc-600">
              Masuk untuk mengelola data sekolah.
            </p>
          </div>

          <form className="space-y-4" onSubmit={handleSubmit}>
            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Username
              </label>
              <input
                value={username}
                onChange={(event) => setUsername(event.target.value)}
                className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                required
              />
            </div>
            <div className="space-y-1">
              <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                Password
              </label>
              <PasswordInput
                value={password}
                onChange={(event) => setPassword(event.target.value)}
                className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                required
              />
            </div>
            {infoMessage ? (
              <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                {infoMessage}
              </p>
            ) : null}
            {error ? (
              <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-600">
                {error}
              </p>
            ) : null}
            <button
              type="submit"
              className="w-full rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-3 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
              disabled={isSubmitting}
            >
              {isSubmitting ? "Memproses..." : "Login"}
            </button>
          </form>
        </CardContent>
      </Card>
    </section>
  );
}
