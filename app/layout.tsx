import type { Metadata } from "next";
import { Suspense } from "react";
import { AppShell } from "@/components/app-shell";
import { getServerAuthSnapshot } from "@/lib/auth/server";
import { Providers } from "./providers";
import IdleLogoutClient from "./IdleLogoutClient";
import "./globals.css";
import { BackupReminder } from "@/components/backup-reminder";

export const metadata: Metadata = {
  title: "Cabdindik Bangkalan",
  description: "Create By Member Staff",
};

export default async function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  const initialAuth = await getServerAuthSnapshot();

  return (
    <html lang="en">
      <body className="antialiased">
        <Providers initialAuth={initialAuth}>
          <IdleLogoutClient />
          <BackupReminder />
          <Suspense fallback={<main className="min-h-screen w-full bg-white" />}>
            <AppShell>{children}</AppShell>
          </Suspense>
        </Providers>
      </body>
    </html>
  );
}
