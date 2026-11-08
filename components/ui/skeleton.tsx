import * as React from "react";

import { cn } from "@/lib/utils";

function Skeleton({
  className,
  ...props
}: React.ComponentProps<"div">) {
  return (
    <div
      aria-hidden="true"
      className={cn(
        "animate-pulse rounded-xl border border-slate-200/70 bg-slate-100/90",
        className
      )}
      {...props}
    />
  );
}

export { Skeleton };
