"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import {
  getSectionHref,
  type SidebarSection,
  type SidebarUser,
} from "@/lib/navigation/sidebar-menu";

/** Receives the existing role-filtered tree; never invents routes or permissions. */
export function ModuleNavigation({
  sections,
  user,
  pendingCount = 0,
}: {
  sections: SidebarSection[];
  user: SidebarUser;
  pendingCount?: number;
}) {
  const pathname = usePathname();
  const items = sections.flatMap((section) =>
    section.children?.some((child) => child.href === section.href)
      ? section.children
      : [section, ...(section.children ?? [])]
  );
  if (!items.length) return null;
  return (
    <div className="border-b border-blue-100 bg-white/90">
      <nav
        aria-label="Navigasi modul"
        className="portal-container flex gap-1 overflow-x-auto py-2"
      >
        {items.map((item) => {
          const href = getSectionHref(item, user);
          const active = pathname === href;
          return (
            <Link
              key={item.key}
              href={href}
              aria-current={active ? "page" : undefined}
              className={`shrink-0 rounded-lg px-4 py-2 text-sm font-medium ${active ? "bg-blue-50 text-blue-800 ring-1 ring-blue-100 ring-inset" : "text-slate-600 hover:bg-slate-50"}`}
            >
              {item.label}
              {item.key === "pending-validasi" && pendingCount > 0 ? (
                <span className="ml-2 rounded-full bg-amber-100 px-2 py-0.5 text-xs text-amber-800">
                  {pendingCount}
                </span>
              ) : null}
            </Link>
          );
        })}
      </nav>
    </div>
  );
}
