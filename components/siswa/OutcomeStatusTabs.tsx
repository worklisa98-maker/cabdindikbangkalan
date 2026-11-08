"use client";

import type { ReactNode } from "react";

const tabItems = [
  { value: "PT", label: "Perguruan Tinggi" },
  { value: "KERJA", label: "Dunia Kerja" },
  { value: "WIRAUSAHA", label: "Wirausaha" },
  { value: "BELUM", label: "Belum Melanjutkan" },
] as const;

export type OutcomeStatusValue = (typeof tabItems)[number]["value"];

type OutcomeStatusTabsProps = {
  value: OutcomeStatusValue;
  onChange: (nextValue: OutcomeStatusValue) => void;
  stickyTop?: number | string;
  rightSlot?: ReactNode;
};

const resolveStickyTop = (stickyTop?: number | string) => {
  if (stickyTop === undefined) return undefined;
  return typeof stickyTop === "number" ? `${stickyTop}px` : stickyTop;
};

export default function OutcomeStatusTabs({
  value,
  onChange,
  stickyTop,
  rightSlot,
}: OutcomeStatusTabsProps) {
  const resolvedTop = resolveStickyTop(stickyTop);

  return (
    <div
      className="sticky top-0 z-20 border-b border-zinc-200 bg-white/95 backdrop-blur"
      style={resolvedTop ? { top: resolvedTop } : undefined}
    >
      <div className="flex flex-wrap items-center justify-between gap-3 rounded-xl border border-zinc-200 bg-white px-3 py-2 shadow-sm">
        <div className="flex flex-wrap gap-2">
          {tabItems.map((tab) => {
            const isActive = tab.value === value;
            return (
              <button
                key={tab.value}
                type="button"
                onClick={() => onChange(tab.value)}
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
        {rightSlot ? <div className="flex items-center gap-2">{rightSlot}</div> : null}
      </div>
    </div>
  );
}
