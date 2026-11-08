import type { Metadata } from "next";
import { AppSidebar } from "@/components/app-sidebar";
import { Providers } from "./providers";
import IdleLogoutClient from "./IdleLogoutClient";
import "./globals.css";

export const metadata: Metadata = {
  title: "Cabdindik Bangkalan",
  description: "Create By Member Staff",
};

export default function RootLayout({
  children,
}: Readonly<{
  children: React.ReactNode;
}>) {
  return (
    <html lang="en">
      <body className="antialiased">
        <Providers>
          <IdleLogoutClient />
          <div className="flex min-h-screen flex-col bg-zinc-50 text-zinc-900 md:flex-row">
            <AppSidebar />
            <main className="mx-auto flex w-full max-w-6xl flex-1 flex-col gap-8 px-4 py-8 sm:px-6 lg:px-8 lg:py-10">
              {children}
            </main>
          </div>
        </Providers>
      </body>
    </html>
  );
}
