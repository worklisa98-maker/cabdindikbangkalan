import type { CSSProperties, ReactNode } from "react";

import { cn } from "@/lib/utils";

type ReportChartBoxProps = {
  children: ReactNode;
  className?: string;
  chartClassName?: string;
  height?: number;
};

export function ReportChartBox({
  children,
  className,
  chartClassName,
  height = 320,
}: ReportChartBoxProps) {
  const style: CSSProperties = {
    height,
    minHeight: height,
    minWidth: 0,
  };

  return (
    <div
      data-export-chart="1"
      style={style}
      className={cn("report-chart-box w-full rounded-lg border border-slate-200 bg-white p-3", className)}
    >
      <div className={cn("h-full w-full min-w-0", chartClassName)}>{children}</div>
    </div>
  );
}
