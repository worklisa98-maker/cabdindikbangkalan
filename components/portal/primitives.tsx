"use client";

import {
  useEffect,
  useRef,
  type ReactNode,
  type ButtonHTMLAttributes,
} from "react";
import Link from "next/link";
import { ArrowUpRight, X } from "lucide-react";
import { cn } from "@/lib/utils";

export function SectionHeader({
  title,
  description,
  actions,
}: {
  title: string;
  description?: string;
  actions?: ReactNode;
}) {
  return (
    <div className="mb-4 flex flex-wrap items-end justify-between gap-3">
      <div>
        <h2 className="text-2xl font-semibold tracking-tight text-slate-950">
          {title}
        </h2>
        {description && (
          <p className="mt-2 max-w-2xl text-sm leading-6 text-slate-600">
            {description}
          </p>
        )}
      </div>
      {actions}
    </div>
  );
}
export function MetricCard({
  label,
  value,
  detail,
  icon,
  accent = "blue",
}: {
  label: string;
  value: ReactNode;
  detail?: string;
  icon?: ReactNode;
  accent?: string;
}) {
  return (
    <div className="portal-card relative overflow-hidden p-4">
      <div className="flex items-center gap-3">
        <span
          className={cn(
            "flex size-10 shrink-0 items-center justify-center rounded-xl",
            accents[accent] ?? "bg-blue-50 text-blue-700"
          )}
        >
          {icon}
        </span>
        <p className="text-sm font-semibold text-slate-600">{label}</p>
      </div>
      <p className="mt-3 text-3xl font-bold tracking-tight text-slate-950">
        {value}
      </p>
      {detail && <p className="mt-2 text-xs text-slate-500">{detail}</p>}
    </div>
  );
}
const accents: Record<string, string> = {
  gaspol: "bg-emerald-50 text-emerald-700",
  mgmp: "bg-violet-50 text-violet-700",
  prestasi: "bg-amber-50 text-amber-700",
  siswa: "bg-teal-50 text-teal-700",
  "halo-kepegawaian": "bg-orange-50 text-orange-700",
  "pengaduan-kritik-saran": "bg-rose-50 text-rose-700",
};
export function ModuleCard({
  title,
  description,
  href,
  icon,
  accent,
  external,
  children,
}: {
  title: string;
  description: string;
  href: string;
  icon: ReactNode;
  accent?: string;
  external?: boolean;
  children?: ReactNode;
}) {
  const solid =
    accent === "gaspol" || accent === "mgmp" || accent === "halo-kepegawaian";
  const surface =
    accent === "gaspol"
      ? "border-emerald-500 bg-emerald-600"
      : accent === "mgmp"
        ? "border-violet-500 bg-violet-600"
        : accent === "halo-kepegawaian"
          ? "border-orange-500 bg-orange-600"
          : accent === "prestasi"
            ? "border-amber-100 bg-amber-50/60"
            : accent === "siswa"
              ? "border-teal-100 bg-teal-50/60"
              : accent === "pengaduan-kritik-saran"
                ? "border-rose-100 bg-rose-50/60"
                : "border-blue-100 bg-blue-50/40";
  return (
    <article
      className={cn(
        "portal-card group flex h-full flex-col border-t-2 p-4 transition-shadow hover:shadow-md",
        surface
      )}
    >
      <span
        className={cn(
          "mb-3 flex size-10 items-center justify-center rounded-xl",
          solid
            ? "bg-white/20 text-white"
            : (accents[accent ?? ""] ?? "bg-blue-50 text-blue-700")
        )}
      >
        {icon}
      </span>
      <Link
        href={href}
        target={external ? "_blank" : undefined}
        rel={external ? "noopener noreferrer" : undefined}
        className={cn(
          "flex items-start justify-between gap-3 text-sm font-bold",
          solid ? "text-white" : "hover:text-primary text-slate-950"
        )}
      >
        {title}
        <ArrowUpRight aria-hidden className="size-4 shrink-0" />
      </Link>
      <p
        className={cn(
          "mt-2 text-xs leading-5",
          solid ? "text-white/90" : "text-slate-600"
        )}
      >
        {description}
      </p>
      {children}
      <Link
        href={href}
        target={external ? "_blank" : undefined}
        rel={external ? "noopener noreferrer" : undefined}
        className={cn(
          "mt-auto flex items-center gap-2 pt-4 text-xs font-semibold",
          solid ? "text-white" : "text-primary"
        )}
      >
        Buka layanan <ArrowUpRight className="size-3.5" aria-hidden />
      </Link>
    </article>
  );
}
export function LoadingState({ label = "Memuat data…" }: { label?: string }) {
  return (
    <div role="status" className="portal-card p-6 text-sm text-slate-600">
      <span className="mr-3 inline-block size-3 animate-pulse rounded-full bg-blue-500 motion-reduce:animate-none" />
      {label}
    </div>
  );
}
export function Button({
  className,
  ...props
}: ButtonHTMLAttributes<HTMLButtonElement>) {
  return <button className={cn("portal-button", className)} {...props} />;
}
export function FormField({
  id,
  label,
  hint,
  error,
  children,
}: {
  id: string;
  label: string;
  hint?: string;
  error?: string;
  children: ReactNode;
}) {
  return (
    <div className="space-y-2">
      <label htmlFor={id} className="block text-sm font-medium text-slate-800">
        {label}
      </label>
      {children}
      {hint && (
        <p id={`${id}-hint`} className="text-xs text-slate-500">
          {hint}
        </p>
      )}
      {error && (
        <p id={`${id}-error`} role="alert" className="text-sm text-red-700">
          {error}
        </p>
      )}
    </div>
  );
}
export function Breadcrumb({
  items,
}: {
  items: { label: string; href?: string }[];
}) {
  return (
    <nav aria-label="Breadcrumb">
      <ol className="flex flex-wrap gap-2 text-sm text-slate-500">
        {items.map((item, index) => (
          <li key={index} className="flex gap-2">
            {index > 0 && <span aria-hidden>/</span>}
            {item.href ? (
              <Link href={item.href}>{item.label}</Link>
            ) : (
              <span aria-current="page">{item.label}</span>
            )}
          </li>
        ))}
      </ol>
    </nav>
  );
}
export function Tabs({
  items,
  value,
  onChange,
}: {
  items: { value: string; label: string }[];
  value: string;
  onChange: (value: string) => void;
}) {
  return (
    <div aria-label="Pilihan tampilan" className="flex gap-2 overflow-x-auto pb-1">
      {items.map((item) => (
        <button
          type="button"
          key={item.value}
          aria-pressed={value === item.value}
          onClick={() => onChange(item.value)}
          className={cn(
            "shrink-0 rounded-lg border px-4 py-2 text-sm",
            value === item.value
              ? "border-primary bg-primary text-white"
              : "border-slate-200 bg-white text-slate-700"
          )}
        >
          {item.label}
        </button>
      ))}
    </div>
  );
}
export function Dialog({
  open,
  onClose,
  title,
  children,
}: {
  open: boolean;
  onClose: () => void;
  title: string;
  children: ReactNode;
}) {
  const ref = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const dialog = ref.current;
    if (open && !dialog?.open) dialog?.showModal();
    else if (!open && dialog?.open) dialog.close();
  }, [open]);
  return (
    <dialog
      ref={ref}
      aria-label={title}
      onCancel={onClose}
      onClose={onClose}
      className="m-auto max-h-[85vh] w-[calc(100%_-_2rem)] max-w-xl overflow-auto rounded-2xl border border-slate-200 bg-white p-6 shadow-xl backdrop:bg-slate-950/40"
    >
      <div className="mb-5 flex items-center justify-between gap-4">
        <h2 className="text-xl font-semibold">{title}</h2>
        <button type="button" onClick={onClose} aria-label="Tutup dialog">
          <X className="size-5" />
        </button>
      </div>
      {children}
    </dialog>
  );
}
export function MapCard({ children }: { children: ReactNode }) {
  return <div className="portal-card overflow-hidden">{children}</div>;
}
