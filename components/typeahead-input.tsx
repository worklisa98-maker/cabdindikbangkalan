"use client";

import { useMemo, useState } from "react";

type TypeaheadItem = {
  key: string | number;
  label: string;
  value: string;
  subLabel?: string;
};

type TypeaheadInputProps = {
  value: string;
  onValueChange: (value: string) => void;
  items: TypeaheadItem[];
  onSelect: (item: TypeaheadItem) => void;
  placeholder?: string;
  emptyText?: string;
  isLoading?: boolean;
  disabled?: boolean;
  minChars?: number;
  showClear?: boolean;
  onClear?: () => void;
};

export function TypeaheadInput({
  value,
  onValueChange,
  items,
  onSelect,
  placeholder,
  emptyText,
  isLoading,
  disabled,
  minChars = 2,
  showClear,
  onClear,
}: TypeaheadInputProps) {
  const [isOpen, setIsOpen] = useState(false);
  const [activeIndex, setActiveIndex] = useState(-1);
  const normalizedItems = useMemo(() => items ?? [], [items]);
  const canShowList = value.trim().length >= minChars;

  const handleKeyDown = (event: React.KeyboardEvent<HTMLInputElement>) => {
    if (!isOpen) return;
    if (event.key === "ArrowDown") {
      event.preventDefault();
      setActiveIndex((prev) => Math.min(prev + 1, normalizedItems.length - 1));
      return;
    }
    if (event.key === "ArrowUp") {
      event.preventDefault();
      setActiveIndex((prev) => Math.max(prev - 1, 0));
      return;
    }
    if (event.key === "Enter") {
      if (activeIndex >= 0 && normalizedItems[activeIndex]) {
        event.preventDefault();
        onSelect(normalizedItems[activeIndex]);
        setIsOpen(false);
      }
      return;
    }
    if (event.key === "Escape") {
      setIsOpen(false);
      return;
    }
  };

  return (
    <div className="relative">
      <input
        type="text"
        value={value}
        onChange={(event) => {
          onValueChange(event.target.value);
          const nextValue = event.target.value;
          if (nextValue.trim().length >= minChars) {
            setIsOpen(true);
            setActiveIndex(-1);
          } else {
            setIsOpen(false);
            setActiveIndex(-1);
          }
        }}
        onFocus={() => {
          if (canShowList) {
            setIsOpen(true);
          }
        }}
        onBlur={() => window.setTimeout(() => setIsOpen(false), 150)}
        onKeyDown={handleKeyDown}
        placeholder={placeholder}
        disabled={disabled}
        className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
      />
      {showClear && onClear ? (
        <button
          type="button"
          onClick={onClear}
          className="absolute top-2 right-2 text-xs text-zinc-500 hover:text-zinc-800"
        >
          Reset
        </button>
      ) : null}
      {isOpen && canShowList ? (
        <div className="absolute z-20 mt-2 max-h-64 w-full overflow-auto rounded-md border border-zinc-200 bg-white shadow-lg">
          {isLoading ? (
            <div className="px-3 py-2 text-sm text-zinc-500">Memuat...</div>
          ) : normalizedItems.length === 0 ? (
            <div className="px-3 py-2 text-sm text-zinc-500">
              {emptyText ?? "Tidak ada hasil."}
            </div>
          ) : (
            normalizedItems.map((item, index) => (
              <button
                key={item.key}
                type="button"
                onMouseDown={(event) => event.preventDefault()}
                onClick={() => {
                  onSelect(item);
                  setIsOpen(false);
                }}
                className={`block w-full px-3 py-2 text-left text-sm ${
                  index === activeIndex
                    ? "bg-zinc-100 text-zinc-900"
                    : "text-zinc-800 hover:bg-zinc-100"
                }`}
              >
                <span className="font-medium">{item.label}</span>
                {item.subLabel ? (
                  <span className="ml-2 text-xs text-zinc-500">
                    {item.subLabel}
                  </span>
                ) : null}
              </button>
            ))
          )}
        </div>
      ) : null}
    </div>
  );
}
