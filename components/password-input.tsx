"use client";

import { useState } from "react";
import type { ChangeEvent } from "react";
import * as LucideIcons from "lucide-react";

type PasswordInputProps = {
  value: string;
  onChange: (event: ChangeEvent<HTMLInputElement>) => void;
  readOnly?: boolean;
  disabled?: boolean;
  className?: string;
  placeholder?: string;
  required?: boolean;
};

export function PasswordInput({
  value,
  onChange,
  readOnly,
  disabled,
  className,
  placeholder,
  required,
}: PasswordInputProps) {
  const [visible, setVisible] = useState(false);
  const ariaLabel = visible ? "Sembunyikan password" : "Tampilkan password";
  const EyeIcon = LucideIcons.Eye;
  const EyeClosedIcon =
    (LucideIcons as { EyeClosed?: typeof LucideIcons.Eye }).EyeClosed ??
    LucideIcons.EyeOff;

  return (
    <div className="relative">
      <input
        type={visible ? "text" : "password"}
        value={value}
        onChange={onChange}
        readOnly={readOnly}
        disabled={disabled}
        placeholder={placeholder}
        required={required}
        className={`${className ?? ""} pr-24`}
      />
      <button
        type="button"
        onClick={() => setVisible((prev) => !prev)}
        className="absolute top-1/2 right-2 -translate-y-1/2 text-zinc-600"
        aria-label={ariaLabel}
      >
        {visible ? (
          <EyeIcon className="h-4 w-4" />
        ) : (
          <EyeClosedIcon className="h-4 w-4" />
        )}
      </button>
    </div>
  );
}
