"use client";
import { useState } from "react";
import { useQuery } from "@tanstack/react-query";
import { gaspolRequest, gaspolField } from "./GaspolRubrics";
type Directory = {
  internalCandidates: {
    userId: number;
    username: string;
    displayName: string;
    baseRole: string;
  }[];
  stageSchoolCandidates: {
    schoolId: number;
    schoolName: string;
    npsn: string;
    cohorts: string[];
  }[];
};
function useDirectory(year: number) {
  return useQuery({
    queryKey: ["gaspol-management-directory", year],
    queryFn: () =>
      gaspolRequest<Directory>(`/gaspol/admin/settings/overview?year=${year}`),
    staleTime: 60000,
  });
}
export function GaspolAccountSelect({
  year,
  value,
  onChange,
  multiple = false,
  label = "Pilih akun",
}: {
  year: number;
  value: number[];
  onChange: (value: number[]) => void;
  multiple?: boolean;
  label?: string;
}) {
  const { data, error } = useDirectory(year);
  const [search, setSearch] = useState("");
  const options = (data?.internalCandidates ?? []).filter((v) =>
    `${v.displayName} ${v.username}`
      .toLowerCase()
      .includes(search.toLowerCase())
  );
  return (
    <fieldset className="space-y-2 rounded-xl border p-3">
      <legend className="px-1 text-sm font-semibold">{label}</legend>
      <input
        aria-label={`Cari ${label.toLowerCase()}`}
        className={gaspolField}
        placeholder="Cari nama atau username"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <div className="max-h-44 space-y-2 overflow-auto">
        {options.map((v) => (
          <label key={v.userId} className="flex items-start gap-2 text-sm">
            <input
              type="checkbox"
              checked={value.includes(v.userId)}
              onChange={(e) =>
                onChange(
                  e.target.checked
                    ? multiple
                      ? [...value, v.userId]
                      : [v.userId]
                    : value.filter((id) => id !== v.userId)
                )
              }
            />
            <span>
              {v.displayName} ({v.username})
            </span>
          </label>
        ))}
      </div>
      {error && <p role="alert">{error.message}</p>}
    </fieldset>
  );
}
export function GaspolSchoolSelect({
  year,
  value,
  onChange,
}: {
  year: number;
  value: number[];
  onChange: (value: number[]) => void;
}) {
  const { data, error } = useDirectory(year);
  const [search, setSearch] = useState("");
  const options = (data?.stageSchoolCandidates ?? []).filter(
    (v) =>
      v.cohorts?.includes("TOP_5") &&
      `${v.schoolName} ${v.npsn}`.toLowerCase().includes(search.toLowerCase())
  );
  return (
    <fieldset className="space-y-2 rounded-xl border p-3">
      <legend className="px-1 text-sm font-semibold">
        Sekolah Top 5 resmi
      </legend>
      <input
        aria-label="Cari sekolah Top 5"
        className={gaspolField}
        placeholder="Cari nama sekolah atau NPSN"
        value={search}
        onChange={(e) => setSearch(e.target.value)}
      />
      <div className="max-h-44 space-y-2 overflow-auto">
        {options.map((v) => (
          <label key={v.schoolId} className="flex items-start gap-2 text-sm">
            <input
              type="checkbox"
              checked={value.includes(v.schoolId)}
              onChange={(e) =>
                onChange(
                  e.target.checked
                    ? [...value, v.schoolId]
                    : value.filter((id) => id !== v.schoolId)
                )
              }
            />
            <span>
              {v.schoolName} · {v.npsn}
            </span>
          </label>
        ))}
      </div>
      {error && <p role="alert">{error.message}</p>}
    </fieldset>
  );
}
