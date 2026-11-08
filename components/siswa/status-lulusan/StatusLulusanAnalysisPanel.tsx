"use client";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  Bar,
  BarChart,
  CartesianGrid,
  Legend,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { jenjangOptions } from "@/lib/shared/jenjangOptions";

type DistributionYear = {
  year: number;
  totalGraduates: number;
  higherEducation: number;
  working: number;
  entrepreneurship: number;
  notContinuing: number;
  unrecorded: number;
  pendingValidation: number;
  finalCount: number;
  dataCoveragePercent: number;
};

const empty: DistributionYear = {
  year: new Date().getFullYear(),
  totalGraduates: 0,
  higherEducation: 0,
  working: 0,
  entrepreneurship: 0,
  notContinuing: 0,
  unrecorded: 0,
  pendingValidation: 0,
  finalCount: 0,
  dataCoveragePercent: 0,
};

export default function StatusLulusanAnalysisPanel() {
  const [year, setYear] = useState(new Date().getFullYear());
  const [jenjang, setJenjang] = useState("ALL");
  const [schoolStatus, setSchoolStatus] = useState("ALL");
  const { data } = useQuery({
    queryKey: ["status-lulusan-distribution", year, jenjang, schoolStatus],
    queryFn: async (): Promise<{
      selected: DistributionYear;
      trend: DistributionYear[];
    }> => {
      const query = new URLSearchParams({ year: String(year) });
      if (jenjang !== "ALL") query.set("jenjang", jenjang);
      if (schoolStatus !== "ALL") query.set("status_sekolah", schoolStatus);
      const response = await apiFetch(
        `/api/siswa/status-lulusan/distribution?${query.toString()}`
      );
      if (!response.ok) throw new Error("Gagal memuat analisa status lulusan.");
      const payload = await response.json();
      return payload?.data ?? { selected: empty, trend: [] };
    },
  });
  const selected = data?.selected ?? { ...empty, year };
  const availableYears = useMemo(() => {
    const values = (data?.trend ?? []).map((item) => item.year);
    if (!values.includes(year)) values.push(year);
    return values.sort((a, b) => b - a);
  }, [data?.trend, year]);
  const percent = (value: number) =>
    selected.totalGraduates > 0
      ? ((value * 100) / selected.totalGraduates).toFixed(1)
      : "0.0";
  const cards = [
    ["Total Lulusan", selected.totalGraduates, "100.0"],
    [
      "Perguruan Tinggi",
      selected.higherEducation,
      percent(selected.higherEducation),
    ],
    ["Bekerja", selected.working, percent(selected.working)],
    [
      "Wirausaha",
      selected.entrepreneurship,
      percent(selected.entrepreneurship),
    ],
    [
      "Belum Melanjutkan",
      selected.notContinuing,
      percent(selected.notContinuing),
    ],
    ["Belum Terdata", selected.unrecorded, percent(selected.unrecorded)],
    [
      "Status Final",
      selected.finalCount,
      selected.dataCoveragePercent.toFixed(1),
    ],
    [
      "Pending Validasi",
      selected.pendingValidation,
      percent(selected.pendingValidation),
    ],
  ] as const;

  return (
    <section className="space-y-6">
      <header>
        <h1 className="text-3xl font-semibold text-zinc-900">
          Analisa Status Lulusan
        </h1>
        <p className="mt-1 text-sm text-zinc-600">
          Sebaran seluruh status lulusan. Data Pending Validasi ditampilkan
          terpisah dan tidak dihitung sebagai status final.
        </p>
      </header>
      <Card>
        <CardContent className="grid gap-3 p-4 md:grid-cols-3">
          <select
            className="rounded-md border p-2 text-sm"
            value={year}
            onChange={(e) => setYear(Number(e.target.value))}
          >
            {availableYears.map((item) => (
              <option key={item}>{item}</option>
            ))}
          </select>
          <select
            className="rounded-md border p-2 text-sm"
            value={jenjang}
            onChange={(e) => setJenjang(e.target.value)}
          >
            <option value="ALL">Semua Jenjang</option>
            {jenjangOptions.map((item) => (
              <option key={item}>{item}</option>
            ))}
          </select>
          <select
            className="rounded-md border p-2 text-sm"
            value={schoolStatus}
            onChange={(e) => setSchoolStatus(e.target.value)}
          >
            <option value="ALL">Negeri & Swasta</option>
            <option value="NEGERI">Negeri</option>
            <option value="SWASTA">Swasta</option>
          </select>
        </CardContent>
      </Card>
      <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
        {cards.map(([label, value, share]) => (
          <Card key={label}>
            <CardContent className="p-4">
              <p className="text-xs font-semibold tracking-wider text-zinc-500 uppercase">
                {label}
              </p>
              <p className="mt-2 text-3xl font-semibold">
                {value.toLocaleString("id-ID")}
              </p>
              <p className="text-sm text-zinc-500">
                {share}% dari total lulusan
              </p>
            </CardContent>
          </Card>
        ))}
      </div>
      <Card>
        <CardContent className="p-5">
          <h2 className="font-semibold">
            Komposisi dan Tren Status Antartahun
          </h2>
          <div className="mt-4 h-96">
            <ResponsiveContainer width="100%" height="100%">
              <BarChart data={data?.trend ?? []}>
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis dataKey="year" />
                <YAxis allowDecimals={false} />
                <Tooltip />
                <Legend />
                <Bar
                  dataKey="higherEducation"
                  name="Perguruan Tinggi"
                  fill="#2563eb"
                />
                <Bar dataKey="working" name="Bekerja" fill="#16a34a" />
                <Bar
                  dataKey="entrepreneurship"
                  name="Wirausaha"
                  fill="#f59e0b"
                />
                <Bar
                  dataKey="notContinuing"
                  name="Belum Melanjutkan"
                  fill="#64748b"
                />
                <Bar dataKey="unrecorded" name="Belum Terdata" fill="#cbd5e1" />
              </BarChart>
            </ResponsiveContainer>
          </div>
        </CardContent>
      </Card>
    </section>
  );
}
