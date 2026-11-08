"use client";

import { useEffect, useRef, useState } from "react";

type RowActionItem = {
  key: string;
  label: string;
  onSelect: () => void | Promise<void>;
  disabled?: boolean;
  destructive?: boolean;
};

type RowActionDropdownProps = {
  items: RowActionItem[];
  disabled?: boolean;
  label?: string;
};

export type { RowActionItem };

export default function RowActionDropdown({
  items,
  disabled = false,
  label = "Aksi Lainnya",
}: RowActionDropdownProps) {
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!open) return;

    const handleClickOutside = (event: MouseEvent) => {
      if (!rootRef.current) return;
      if (!(event.target instanceof Node)) return;
      if (!rootRef.current.contains(event.target)) {
        setOpen(false);
      }
    };

    const handleEscape = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setOpen(false);
      }
    };

    document.addEventListener("mousedown", handleClickOutside);
    window.addEventListener("keydown", handleEscape);
    return () => {
      document.removeEventListener("mousedown", handleClickOutside);
      window.removeEventListener("keydown", handleEscape);
    };
  }, [open]);

  const hasEnabledItem = items.some((item) => !item.disabled);
  const triggerDisabled = disabled || !hasEnabledItem;

  return (
    <div ref={rootRef} className="relative">
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        disabled={triggerDisabled}
        className="inline-flex h-8 items-center justify-center rounded-lg border border-slate-300 bg-white px-2 text-xs font-semibold text-zinc-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-50"
      >
        {label}
      </button>
      {open ? (
        <div className="absolute right-0 z-20 mt-1 min-w-[220px] rounded-md border border-zinc-200 bg-white p-1 shadow-lg">
          {items.map((item) => (
            <button
              key={item.key}
              type="button"
              onClick={() => {
                if (item.disabled) return;
                setOpen(false);
                void item.onSelect();
              }}
              disabled={item.disabled}
              className={`flex w-full items-center rounded px-2 py-1.5 text-left text-xs transition ${
                item.destructive
                  ? "text-red-700 hover:bg-red-50"
                  : "text-zinc-700 hover:bg-zinc-100"
              } disabled:cursor-not-allowed disabled:text-zinc-400`}
            >
              {item.label}
            </button>
          ))}
        </div>
      ) : null}
    </div>
  );
}

