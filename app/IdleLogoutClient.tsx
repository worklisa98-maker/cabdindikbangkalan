"use client";

import { useEffect } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/client";
import { startIdleLogout } from "@/lib/idle-logout";

export default function IdleLogoutClient() {
  const router = useRouter();
  const { status, logout } = useAuth();

  useEffect(() => {
    if (status !== "authenticated") {
      return;
    }

    const cleanup = startIdleLogout({
      onLogout: () => {
        logout();
        router.replace("/login");
      },
    });

    return cleanup;
  }, [status, router, logout]);

  return null;
}
