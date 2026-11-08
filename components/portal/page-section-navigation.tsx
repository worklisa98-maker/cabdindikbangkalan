"use client";
import { useEffect, useState } from "react";
import { usePathname } from "next/navigation";

/** Index only sections actually rendered by the module's existing role checks. */
export function PageSectionNavigation() {
  const pathname = usePathname();
  const [items, setItems] = useState<{ id: string; label: string }[]>([]);
  const [active, setActive] = useState("");
  useEffect(() => {
    const main = document.getElementById("main-content");
    if (!main) return;
    let frame = 0;
    let sequence = 0;
    const update = () => {
      const headings = Array.from(
        main.querySelectorAll<HTMLElement>('h2, [data-slot="card-title"]')
      ).filter(
        (heading) =>
          !heading.closest(
            '[role="dialog"], [role="alertdialog"], dialog, .fixed'
          ) && heading.getClientRects().length > 0
      );
      const next = headings
        .map((heading) => {
          if (!heading.id) {
            let id = `module-section-${sequence++}`;
            while (document.getElementById(id))
              id = `module-section-${sequence++}`;
            heading.id = id;
          }
          heading.style.scrollMarginTop = "8rem";
          return { id: heading.id, label: heading.textContent?.trim() ?? "" };
        })
        .filter((item) => item.label);
      setItems((previous) =>
        JSON.stringify(previous) === JSON.stringify(next) ? previous : next
      );
    };
    const observer = new MutationObserver(() => {
      cancelAnimationFrame(frame);
      frame = requestAnimationFrame(update);
    });
    observer.observe(main, {
      childList: true,
      subtree: true,
      characterData: true,
    });
    update();
    return () => {
      observer.disconnect();
      cancelAnimationFrame(frame);
    };
  }, [pathname]);
  if (!items.length) return null;
  return (
    <nav
      aria-label="Navigasi bagian halaman"
      className="flex max-w-full gap-1 overflow-x-auto rounded-xl border border-blue-100 bg-white p-2 shadow-sm"
    >
      {items.map((item) => (
        <a
          key={item.id}
          href={`#${item.id}`}
          onClick={() => setActive(item.id)}
          aria-current={active === item.id ? "location" : undefined}
          className="shrink-0 rounded-lg px-4 py-2 text-sm font-medium text-slate-600 hover:bg-blue-50 focus-visible:bg-blue-50 aria-[current=location]:bg-blue-50 aria-[current=location]:text-blue-800"
        >
          {item.label}
        </a>
      ))}
    </nav>
  );
}
