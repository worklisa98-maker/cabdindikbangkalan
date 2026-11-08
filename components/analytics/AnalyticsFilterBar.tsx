"use client";

import { TypeaheadInput } from "@/components/typeahead-input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";
import { jenjangOptions } from "@/lib/shared/jenjangOptions";

type RoleType =
  | "ADMIN"
  | "ADMIN_CABDIN"
  | "SCHOOL_ADMIN"
  | "ADMIN_MASTER"
  | "PENGAWAS"
  | "PUBLIC";
type ModuleType = "prestasi" | "siswa-pt";

export type AnalyticsFilterValues = {
  year: string;
  jenjang: string;
  status: string;
  schoolId?: number | null;
  schoolQuery?: string;
  jenisLomba?: string;
};

type SchoolSuggestion = {
  id: number;
  nama_sekolah: string;
  npsn?: string | null;
};

type AnalyticsFilterBarProps = {
  role: RoleType;
  module: ModuleType;
  values: AnalyticsFilterValues;
  availableYears: number[];
  onChange: (next: Partial<AnalyticsFilterValues>) => void;
  schoolSuggestions?: SchoolSuggestion[];
  isSchoolLoading?: boolean;
  onSchoolQueryChange?: (value: string) => void;
  onSchoolSelect?: (school: SchoolSuggestion) => void;
  onSchoolClear?: () => void;
};

const statusOptions = [
  { value: "ALL", label: "Semua" },
  { value: "NEGERI", label: "Negeri" },
  { value: "SWASTA", label: "Swasta" },
];

const jenisLombaOptions = ["ALL", "Siswa", "Guru", "Tendik"];

export function AnalyticsFilterBar({
  role,
  module,
  values,
  availableYears,
  onChange,
  schoolSuggestions = [],
  isSchoolLoading,
  onSchoolQueryChange,
  onSchoolSelect,
  onSchoolClear,
}: AnalyticsFilterBarProps) {
  const yearOptions = availableYears.map(String);
  const canUseJenjangStatus =
    role === "ADMIN" ||
    role === "ADMIN_CABDIN" ||
    role === "ADMIN_MASTER" ||
    role === "PENGAWAS" ||
    role === "PUBLIC";
  const showSchoolFilter =
    role === "ADMIN" ||
    role === "ADMIN_CABDIN" ||
    role === "ADMIN_MASTER" ||
    role === "PENGAWAS";
  const showJenisLomba = role === "SCHOOL_ADMIN" && module === "prestasi";

  return (
    <div className="flex flex-col gap-2 md:flex-row md:flex-wrap md:items-end">
      {showSchoolFilter ? (
        <div className="space-y-1">
          <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Sekolah
          </label>
          <TypeaheadInput
            value={values.schoolQuery ?? ""}
            onValueChange={(value) => {
              onSchoolQueryChange?.(value);
              onChange({ schoolId: null });
            }}
            items={schoolSuggestions.map((school) => ({
              key: school.id,
              label: school.nama_sekolah,
              value: school.nama_sekolah,
              subLabel: school.npsn ?? undefined,
            }))}
            onSelect={(item) => {
              const selected = schoolSuggestions.find(
                (school) => school.id === item.key
              );
              if (selected) {
                onSchoolSelect?.(selected);
              }
            }}
            placeholder="Ketik nama sekolah..."
            emptyText="Sekolah tidak ditemukan"
            isLoading={isSchoolLoading}
            minChars={0}
            showClear={Boolean(values.schoolId || values.schoolQuery)}
            onClear={onSchoolClear}
          />
        </div>
      ) : null}

      {canUseJenjangStatus ? (
        <div className="space-y-1">
          <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Jenjang
          </label>
          <Select
            value={values.jenjang}
            onValueChange={(value) => onChange({ jenjang: value })}
          >
            <SelectTrigger className="w-full md:w-36">
              <SelectValue placeholder="Semua" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value="ALL">Semua</SelectItem>
              {jenjangOptions.map((jenjang) => (
                <SelectItem key={jenjang} value={jenjang}>
                  {jenjang}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      ) : null}

      {canUseJenjangStatus ? (
        <div className="space-y-1">
          <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Status
          </label>
          <Select
            value={values.status}
            onValueChange={(value) => onChange({ status: value })}
          >
            <SelectTrigger className="w-full md:w-36">
              <SelectValue placeholder="Semua" />
            </SelectTrigger>
            <SelectContent>
              {statusOptions.map((status) => (
                <SelectItem key={status.value} value={status.value}>
                  {status.label}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      ) : null}

      <div className="space-y-1">
        <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          Tahun
        </label>
        <Select
          value={values.year}
          onValueChange={(value) => onChange({ year: value })}
          disabled={yearOptions.length === 0}
        >
          <SelectTrigger className="w-full md:w-36">
            <SelectValue placeholder="Semua tahun" />
          </SelectTrigger>
          <SelectContent>
            <SelectItem value="ALL">Semua tahun</SelectItem>
            {yearOptions.map((year) => (
              <SelectItem key={year} value={year}>
                {year}
              </SelectItem>
            ))}
          </SelectContent>
        </Select>
      </div>

      {showJenisLomba ? (
        <div className="space-y-1">
          <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Jenis Lomba
          </label>
          <Select
            value={values.jenisLomba ?? "ALL"}
            onValueChange={(value) => onChange({ jenisLomba: value })}
          >
            <SelectTrigger className="w-full md:w-40">
              <SelectValue placeholder="Semua" />
            </SelectTrigger>
            <SelectContent>
              {jenisLombaOptions.map((option) => (
                <SelectItem key={option} value={option}>
                  {option}
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
      ) : null}
    </div>
  );
}
