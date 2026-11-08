"use client";

import NextImage from "next/image";

import type { ReactNode } from "react";

type ReportShellProps = {
  title: string;
  subtitle: string;
  ready: boolean;
  exportMode?: boolean;
  showWatermark?: boolean;
  children: ReactNode;
};

export function ReportShell({
  title,
  subtitle,
  ready,
  exportMode = false,
  showWatermark = true,
  children,
}: ReportShellProps) {
  return (
    <div
      id="report-root"
      data-report="1"
      data-report-ready={ready ? "1" : "0"}
      className="report-shell relative isolate min-h-screen w-full bg-white p-8 text-slate-900"
    >
      {exportMode ? (
        <style jsx global>{`
          html[data-export-mode="1"],
          html[data-export-mode="1"] body {
            background: #ffffff !important;
            background-image: none !important;
          }

          html[data-export-mode="1"] *,
          html[data-export-mode="1"] *::before,
          html[data-export-mode="1"] *::after {
            animation: none !important;
            transition: none !important;
            scroll-behavior: auto !important;
          }

          html[data-export-mode="1"] [data-report-hide-in-export="1"] {
            display: none !important;
          }

          html[data-export-mode="1"] #report-root,
          html[data-export-mode="1"] #report-root section,
          html[data-export-mode="1"] #report-root .rounded-lg,
          html[data-export-mode="1"] #report-root .report-chart-box,
          html[data-export-mode="1"] #report-root table {
            break-inside: avoid;
            page-break-inside: avoid;
          }
        `}</style>
      ) : null}

      {showWatermark ? (
        <div className="report-watermark pointer-events-none fixed inset-0 z-0 flex items-center justify-center">
          <NextImage unoptimized width={224} height={224} loading="eager"
            src="/assets/logo-sipres.png"
            alt="Watermark SIPRES"
            className="h-56 w-56 object-contain opacity-[0.08]"
          />
        </div>
      ) : null}

      <header className="relative z-10 border-b border-slate-200 pb-4">
        <p className="text-xs font-semibold tracking-[0.2em] text-slate-500 uppercase">
          SIPRES Report
        </p>
        <h1 className="mt-1 text-2xl font-semibold text-slate-900">{title}</h1>
        <p className="mt-1 text-sm text-slate-600">{subtitle}</p>
      </header>

      <div className="relative z-10 mt-6 space-y-6">{children}</div>
    </div>
  );
}
