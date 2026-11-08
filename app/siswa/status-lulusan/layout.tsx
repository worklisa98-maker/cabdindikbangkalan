"use client";

import type { ReactNode } from "react";
import { usePathname } from "next/navigation";

import StatusLulusanStickyTabs from "@/components/siswa/status-lulusan/StatusLulusanStickyTabs";

export default function StatusLulusanLayout({
  children,
}: {
  children: ReactNode;
}) {
  const pathname = usePathname() ?? "";
  const isPendingValidationRoute = pathname.includes("/pending-validasi");

  return (
    <div className="space-y-6">
      {!isPendingValidationRoute ? <StatusLulusanStickyTabs /> : null}
      {children}
    </div>
  );
}
