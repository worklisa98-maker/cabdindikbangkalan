"use client";

import { useMemo } from "react";
import { usePathname, useRouter } from "next/navigation";

const tabs = [
  {
    value: "ANALYTICS_PT",
    label: "Analytics PT",
    href: "/siswa/status-lulusan/analisa?tab=ANALYTICS_PT",
  },
  {
    value: "PT",
    label: "Perguruan Tinggi",
    href: "/siswa/status-lulusan/pt?tab=PT",
  },
  {
    value: "KERJA",
    label: "Dunia Kerja",
    href: "/siswa/status-lulusan/kerja?tab=KERJA",
  },
  {
    value: "WIRAUSAHA",
    label: "Wirausaha",
    href: "/siswa/status-lulusan/wirausaha?tab=WIRAUSAHA",
  },
  {
    value: "BELUM",
    label: "Belum Melanjutkan",
    href: "/siswa/status-lulusan/belum?tab=BELUM",
  },
] as const;

type StatusLulusanTabValue = (typeof tabs)[number]["value"];

const resolveActiveTab = (pathname: string): StatusLulusanTabValue => {
  if (
    pathname.includes("/siswa/status-lulusan/analisa") ||
    pathname.includes("/siswa/status-lulusan/analytics-pt")
  )
    return "ANALYTICS_PT";
  if (pathname.includes("/siswa/status-lulusan/kerja")) return "KERJA";
  if (pathname.includes("/siswa/status-lulusan/wirausaha")) return "WIRAUSAHA";
  if (pathname.includes("/siswa/status-lulusan/belum")) return "BELUM";
  if (pathname.includes("/siswa/status-lulusan/pt")) return "PT";
  return "ANALYTICS_PT";
};

export default function StatusLulusanStickyTabs() {
  const router = useRouter();
  const pathname = usePathname() ?? "";

  const activeTab = useMemo(() => resolveActiveTab(pathname), [pathname]);

  return (
    <div className="sticky top-0 z-20 border-b border-zinc-200 bg-white/95 backdrop-blur">
      <div className="flex flex-wrap items-center gap-2 rounded-xl border border-zinc-200 bg-white px-3 py-2 shadow-sm">
        {tabs.map((tab) => {
          const isActive = tab.value === activeTab;
          return (
            <button
              key={tab.value}
              type="button"
              onClick={() => router.push(tab.href)}
              className={`rounded-full border px-4 py-2 text-xs font-semibold transition ${
                isActive
                  ? "border-zinc-900 bg-zinc-900 text-white"
                  : "border-zinc-200 bg-white text-zinc-700 hover:border-zinc-900 hover:text-zinc-900"
              }`}
            >
              {tab.label}
            </button>
          );
        })}
      </div>
    </div>
  );
}
