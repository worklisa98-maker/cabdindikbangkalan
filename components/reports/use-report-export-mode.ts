"use client";

import { useEffect } from "react";
import type { ReadonlyURLSearchParams } from "next/navigation";

type ReportExportModeState = {
  isExportMode: boolean;
  showWatermark: boolean;
};

export function useReportExportMode(
  searchParams: ReadonlyURLSearchParams,
  ready: boolean
): ReportExportModeState {
  const isExportMode = searchParams.get("export") === "1";
  const showWatermark = !isExportMode || searchParams.get("watermark") === "1";

  useEffect(() => {
    const root = document.documentElement;
    if (isExportMode) {
      root.dataset.exportMode = "1";
      root.dataset.reportReady = ready ? "1" : "0";
      return () => {
        delete root.dataset.exportMode;
        delete root.dataset.reportReady;
      };
    }

    delete root.dataset.exportMode;
    delete root.dataset.reportReady;
    return undefined;
  }, [isExportMode, ready]);

  return { isExportMode, showWatermark };
}

