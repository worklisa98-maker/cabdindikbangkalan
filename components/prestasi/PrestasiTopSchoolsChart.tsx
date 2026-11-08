"use client";

import {
  Bar,
  BarChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { EmptyState } from "@/components/empty-state";
import { Card, CardContent } from "@/components/ui/card";

type TopSchool = {
  school: string;
  total: number;
};

const splitSchoolLabel = (label: string) => {
  const words = label.trim().split(/\s+/);
  if (words.length <= 1) return [label];
  if (words.length === 2) return [words[0], words[1]];
  if (words.length === 3) return [...words];
  const firstCut = Math.ceil(words.length / 3);
  const secondCut = Math.ceil((2 * words.length) / 3);
  return [
    words.slice(0, firstCut).join(" "),
    words.slice(firstCut, secondCut).join(" "),
    words.slice(secondCut).join(" "),
  ].filter(Boolean);
};

const SchoolAxisTick = ({
  x = 0,
  y = 0,
  payload,
}: {
  x?: number;
  y?: number;
  payload?: { value?: string };
}) => {
  const label = payload?.value ?? "";
  const lines = splitSchoolLabel(label);
  return (
    <text x={x} y={y} textAnchor="middle" fill="#52525b" fontSize={11}>
      {lines.map((line, index) => (
        <tspan key={`${label}-${index}`} x={x} dy={index === 0 ? 9 : 13}>
          {line}
        </tspan>
      ))}
    </text>
  );
};

type PrestasiTopSchoolsChartProps = {
  topSchools: TopSchool[];
  barColor: string;
};

export function PrestasiTopSchoolsChart({
  topSchools,
  barColor,
}: PrestasiTopSchoolsChartProps) {
  const hasChartData = topSchools.some((item) => item.total > 0);

  return (
    <Card>
      <CardContent className="space-y-4">
        <div>
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Rekapitulasi
          </p>
          <p className="text-sm font-semibold text-zinc-700">
            10 Besar Sekolah Paling Berprestasi se-Bangkalan
          </p>
        </div>
        <div className="h-64 w-full sm:h-72 lg:h-80">
          {!hasChartData ? (
            <EmptyState
              title="Tidak ada data prestasi"
              description="Top sekolah akan muncul setelah data pada filter ini tersedia."
              className="h-full min-h-0"
            />
          ) : (
            <ResponsiveContainer width="100%" height="100%">
              <BarChart
                data={topSchools}
                margin={{ top: 8, right: 16, left: 0, bottom: 32 }}
              >
                <CartesianGrid strokeDasharray="3 3" />
                <XAxis
                  dataKey="school"
                  interval={0}
                  height={64}
                  tick={<SchoolAxisTick />}
                />
                <YAxis allowDecimals={false} />
                <Tooltip />
                <Bar dataKey="total" fill={barColor} />
              </BarChart>
            </ResponsiveContainer>
          )}
        </div>
      </CardContent>
    </Card>
  );
}
