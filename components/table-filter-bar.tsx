"use client";

import type { ReactNode } from "react";

type TableFilterBarProps = {
  children: ReactNode;
};

export function TableFilterBar({ children }: TableFilterBarProps) {
  return (
    <div className="flex flex-col gap-3 rounded-xl border border-slate-200/60 bg-white p-4 shadow-sm transition-all duration-300 hover:shadow-md md:flex-row md:flex-wrap md:items-end md:justify-between">
      {children}
    </div>
  );
}
