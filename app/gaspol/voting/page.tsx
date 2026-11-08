"use client";
import { useEffect, useState } from "react";
import { useAuth } from "@/lib/auth/client";
import { apiFetch } from "@/lib/api/client";
import { setToken } from "@/lib/auth";
import type { AuthUser } from "@/lib/auth/types";
import { GaspolVotingPanel } from "@/components/gaspol/GaspolVoting";
import { gaspolField, gaspolButton } from "@/components/gaspol/GaspolRubrics";
export default function VotingPage() {
  const { user, status, setUser } = useAuth();
  const [target, setTarget] = useState<{
    year: number;
    schoolId: number;
    sessionId: number;
  }>();
  const [username, setUsername] = useState("");
  const [password, setPassword] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    const query = new URLSearchParams(window.location.search);
    setTarget({
      year: Number(query.get("year")) || new Date().getFullYear(),
      schoolId: Number(query.get("schoolId")) || 0,
      sessionId: Number(query.get("sessionId")) || 0,
    });
  }, []);
  if (status === "loading" || !target)
    return <p className="p-6">Memuat voting…</p>;
  return (
    <section className="mx-auto max-w-3xl space-y-4 p-4 md:p-8">
      <h1 className="text-2xl font-bold">Voting Ter-Favorit GASPOL Bang</h1>
      {user ? (
        <GaspolVotingPanel
          year={target.year}
          initialSchoolId={target.schoolId}
          initialSessionId={target.sessionId}
          master={user.role === "ADMIN_MASTER"}
          school={user.schoolId != null}
        />
      ) : (
        <form
          className="space-y-4 rounded-2xl border bg-white p-5"
          onSubmit={async (e) => {
            e.preventDefault();
            setBusy(true);
            try {
              const response = await apiFetch(
                "/auth/login",
                {
                  method: "POST",
                  body: JSON.stringify({ username, password }),
                },
                { skipAuthRedirect: true }
              );
              if (!response.ok)
                throw new Error("Username atau password salah.");
              const payload = (await response.json()) as {
                data: { token: string; user: AuthUser };
              };
              setToken(payload.data.token);
              setUser(payload.data.user);
              setPassword("");
            } catch (e) {
              setError((e as Error).message);
            } finally {
              setBusy(false);
            }
          }}
        >
          <p>Login dengan akun yang tercantum dalam daftar pemilih sesi ini.</p>
          <label className="block">
            Username
            <input
              className={gaspolField}
              autoComplete="username"
              value={username}
              onChange={(e) => setUsername(e.target.value)}
              required
            />
          </label>
          <label className="block">
            Password
            <input
              className={gaspolField}
              type="password"
              autoComplete="current-password"
              value={password}
              onChange={(e) => setPassword(e.target.value)}
              required
            />
          </label>
          <button className={gaspolButton} disabled={busy}>
            Login dan lanjutkan pilihan
          </button>
          <p role="alert">{error}</p>
        </form>
      )}
    </section>
  );
}
