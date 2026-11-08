import type { ReactNode } from "react";

/** In-page navigation keeps form state, queries, and module workflows mounted. */
export function SectionNavigation({ label, items, children }: {
  label: string;
  items: { id: string; label: string }[];
  children?: ReactNode;
}) {
  return <nav aria-label={label} className="mb-5 flex gap-1 overflow-x-auto rounded-xl border border-blue-100 bg-white p-2 shadow-sm">
    {items.map(item => <a key={item.id} href={`#${item.id}`} className="shrink-0 rounded-lg px-4 py-2 text-sm font-semibold text-slate-600 transition-colors hover:bg-blue-50 hover:text-blue-800 focus-visible:bg-blue-50">{item.label}</a>)}
    {children}
  </nav>;
}
