import * as React from "react";

import { cn } from "@/lib/utils";

type StatusBadgeVariant = "success" | "warning" | "danger" | "neutral" | "info";

const variantClasses: Record<StatusBadgeVariant, string> = {
  info: "border-blue-200 bg-blue-50 text-blue-700",
  success: "border-emerald-200/80 bg-emerald-50 text-emerald-700",
  warning: "border-amber-200/80 bg-amber-50 text-amber-700",
  danger: "border-red-200/80 bg-red-50 text-red-700",
  neutral: "border-slate-200 bg-slate-50 text-slate-700",
};

type StatusBadgeProps = React.ComponentProps<"span"> & {
  variant?: StatusBadgeVariant;
};

export function StatusBadge({
  variant = "neutral",
  className,
  ...props
}: StatusBadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center rounded-full border px-2.5 py-1 text-xs font-semibold shadow-[inset_0_1px_0_rgba(255,255,255,0.65)]",
        variantClasses[variant],
        className
      )}
      {...props}
    />
  );
}
