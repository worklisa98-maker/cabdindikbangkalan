"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/client";
import { clearToken } from "@/lib/auth";
import { LOGOUT_BROADCAST_KEY, startIdleLogout } from "@/lib/idle-logout";

export default function IdleLogoutClient() {
  const router = useRouter();
  const { status } = useAuth();

  useEffect(() => {
    if (status !== "authenticated") {
      return;
    }

    const cleanup = startIdleLogout({
      onLogout: () => {
        clearToken();
        if (typeof window !== "undefined") {
          window.localStorage.setItem(LOGOUT_BROADCAST_KEY, String(Date.now()));
        }
        router.replace("/login");
      },
    });

    return cleanup;
  }, [status, router]);

  return null;
}
