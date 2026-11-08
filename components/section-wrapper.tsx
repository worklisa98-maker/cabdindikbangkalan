import type { ComponentPropsWithoutRef, ElementType, ReactNode } from "react";

import { cn } from "@/lib/utils";

type SectionWrapperProps<T extends ElementType = "section"> = {
  as?: T;
  title?: string;
  description?: string;
  eyebrow?: string;
  actions?: ReactNode;
  children: ReactNode;
  className?: string;
  bodyClassName?: string;
} & Omit<ComponentPropsWithoutRef<T>, "as" | "children" | "className">;

export function SectionWrapper<T extends ElementType = "section">({
  as,
  title,
  description,
  eyebrow,
  actions,
  children,
  className,
  bodyClassName,
  ...props
}: SectionWrapperProps<T>) {
  const Comp = (as ?? "section") as ElementType;

  return (
    <Comp className={cn("space-y-6 py-12 md:py-16", className)} {...props}>
      {title || description || eyebrow || actions ? (
        <div className="flex flex-col gap-3 sm:flex-row sm:items-end sm:justify-between">
          <div>
            {eyebrow ? (
              <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
                {eyebrow}
              </p>
            ) : null}
            {title ? (
              <h2 className="mt-1 text-xl font-semibold text-slate-900 md:text-2xl">
                {title}
              </h2>
            ) : null}
            {description ? (
              <p className="mt-1 text-sm text-slate-500">{description}</p>
            ) : null}
          </div>
          {actions ? <div className="flex flex-wrap items-center gap-2">{actions}</div> : null}
        </div>
      ) : null}
      <div className={cn("space-y-5", bodyClassName)}>{children}</div>
    </Comp>
  );
}
