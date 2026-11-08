"use client";

import { PasswordInput } from "@/components/password-input";

type ConfirmPasswordModalProps = {
  isOpen: boolean;
  password: string;
  onPasswordChange: (value: string) => void;
  onConfirm: () => void;
  onCancel: () => void;
  error?: string;
  isSubmitting?: boolean;
};

export function ConfirmPasswordModal({
  isOpen,
  password,
  onPasswordChange,
  onConfirm,
  onCancel,
  error,
  isSubmitting,
}: ConfirmPasswordModalProps) {
  if (!isOpen) return null;

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 px-4">
      <div className="w-full max-w-md rounded-xl bg-white p-5 shadow-lg">
        <div className="space-y-1">
          <h2 className="text-lg font-semibold text-zinc-900">
            Verifikasi Password
          </h2>
          <p className="text-sm text-zinc-600">
            Masukkan password admin sekolah untuk melanjutkan.
          </p>
        </div>
        <div className="mt-4 space-y-2">
          <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Password Admin Sekolah
          </label>
          <PasswordInput
            value={password}
            onChange={(event) => onPasswordChange(event.target.value)}
            className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
            placeholder="Password"
            required
          />
          {error ? <p className="text-sm text-red-600">{error}</p> : null}
        </div>
        <div className="mt-5 flex justify-end gap-2">
          <button
            type="button"
            onClick={onCancel}
            className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
          >
            Batal
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={isSubmitting}
            className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
          >
            {isSubmitting ? "Menyimpan..." : "Konfirmasi"}
          </button>
        </div>
      </div>
    </div>
  );
}

