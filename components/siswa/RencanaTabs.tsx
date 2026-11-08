"use client";

export type RencanaTabKey = "PT" | "WIRAUSAHA" | "UKK";

type RencanaTabItem = {
  key: RencanaTabKey;
  label: string;
};

type RencanaTabsProps = {
  value: RencanaTabKey;
  onChange: (next: RencanaTabKey) => void;
  tabs: RencanaTabItem[];
};

export default function RencanaTabs({ value, onChange, tabs }: RencanaTabsProps) {
  return (
    <div className="sticky top-0 z-20 border-b border-zinc-200 bg-white">
      <div className="flex flex-wrap items-center gap-2 px-1 py-3">
        {tabs.map((tab) => {
          const isActive = tab.key === value;
          return (
            <button
              key={tab.key}
              type="button"
              onClick={() => onChange(tab.key)}
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
