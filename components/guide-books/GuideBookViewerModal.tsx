"use client";

import { useEffect, useMemo, useState } from "react";
import { ChevronLeft, ChevronRight, X } from "lucide-react";

import { GuideBookSlideRenderer } from "@/components/guide-books/GuideBookSlideRenderer";
import {
  GUIDE_BOOK_LAYOUT_TYPES,
  type GuideBookItem,
  type GuideBookSlideItem,
} from "@/lib/guide-books";

type GuideBookViewerModalProps = {
  guide: GuideBookItem | null;
  open: boolean;
  onClose: () => void;
};

const buildSlides = (guide: GuideBookItem | null): GuideBookSlideItem[] => {
  if (!guide) return [];
  if (guide.slides.length > 0) {
    return [...guide.slides].sort((a, b) => a.sortOrder - b.sortOrder);
  }
  if (!guide.contentHtml.trim()) {
    return [];
  }
  return [
    {
      id: 0,
      layoutType: GUIDE_BOOK_LAYOUT_TYPES.FULL_TEXT,
      textHtml: guide.contentHtml,
      image1Url: "",
      image2Url: "",
      columnRatio: "1:1",
      sortOrder: 10,
    },
  ];
};

export function GuideBookViewerModal(props: GuideBookViewerModalProps) {
  if (!props.open || !props.guide) return null;
  return <GuideBookViewerContent key={props.guide.id} {...props} />;
}

function GuideBookViewerContent({
  guide,
  open,
  onClose,
}: GuideBookViewerModalProps) {
  const slides = useMemo(() => buildSlides(guide), [guide]);
  const [activeIndex, setActiveIndex] = useState(0);

  useEffect(() => {
    if (!open) return;
    const handleKeyDown = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        onClose();
      }
      if (slides.length <= 1) return;
      if (event.key === "ArrowLeft") {
        setActiveIndex((current) => (current === 0 ? current : current - 1));
      }
      if (event.key === "ArrowRight") {
        setActiveIndex((current) =>
          current >= slides.length - 1 ? current : current + 1
        );
      }
    };
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose, open, slides.length]);

  if (!open || !guide || slides.length === 0) {
    return null;
  }

  const activeSlide = slides[Math.min(activeIndex, slides.length - 1)];
  const canNavigate = slides.length > 1;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-zinc-950/70 px-3 py-4 sm:px-6">
      <button
        type="button"
        aria-label="Tutup Juknis"
        className="absolute inset-0"
        onClick={onClose}
      />

      <section
        role="dialog"
        aria-modal="true"
        className="relative flex h-full max-h-[92vh] w-full max-w-6xl flex-col overflow-hidden rounded-3xl border border-zinc-200 bg-zinc-100 shadow-2xl"
      >
        <div className="flex items-start justify-between gap-4 border-b border-zinc-200 bg-white px-5 py-4">
          <div className="min-w-0">
            <p className="text-xs font-semibold tracking-[0.18em] text-zinc-500 uppercase">
              Juknis
            </p>
            <h2 className="truncate text-lg font-semibold text-zinc-950">
              {guide.title}
            </h2>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-xl border border-zinc-200 bg-white p-2 text-zinc-600 transition hover:bg-zinc-50"
          >
            <X className="h-5 w-5" />
          </button>
        </div>

        <div className="flex-1 overflow-y-auto p-4 sm:p-6">
          <GuideBookSlideRenderer slide={activeSlide} />
        </div>

        <div className="flex items-center justify-between gap-3 border-t border-zinc-200 bg-white px-5 py-4">
          <div className="text-sm text-zinc-500">
            {slides.length === 1
              ? "1 halaman"
              : `${activeIndex + 1} / ${slides.length}`}
          </div>

          {canNavigate ? (
            <div className="flex items-center gap-2">
              <button
                type="button"
                onClick={() =>
                  setActiveIndex((current) => Math.max(0, current - 1))
                }
                disabled={activeIndex === 0}
                className="inline-flex items-center gap-2 rounded-xl border border-zinc-200 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-zinc-50 disabled:cursor-not-allowed disabled:opacity-50"
              >
                <ChevronLeft className="h-4 w-4" />
                Prev
              </button>
              <button
                type="button"
                onClick={() =>
                  setActiveIndex((current) =>
                    Math.min(slides.length - 1, current + 1)
                  )
                }
                disabled={activeIndex >= slides.length - 1}
                className="inline-flex items-center gap-2 rounded-xl bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-50"
              >
                Next
                <ChevronRight className="h-4 w-4" />
              </button>
            </div>
          ) : (
            <div />
          )}
        </div>
      </section>
    </div>
  );
}
