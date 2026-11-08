"use client";

import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import { useState } from "react";

import { AuthProvider } from "@/lib/auth/client";
import type { AuthSnapshot } from "@/lib/auth/types";

export function Providers({
  children,
  initialAuth,
}: {
  children: React.ReactNode;
  initialAuth: AuthSnapshot;
}) {
  const [queryClient] = useState(
    () =>
      new QueryClient({
        defaultOptions: {
          queries: {
            staleTime: 5 * 60 * 1000,
            retry: 1,
          },
        },
      })
  );

  return (
    <QueryClientProvider client={queryClient}>
      <AuthProvider initialAuth={initialAuth}>{children}</AuthProvider>
    </QueryClientProvider>
  );
}
