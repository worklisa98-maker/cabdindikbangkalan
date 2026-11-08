"use client";

import {
  CartesianGrid,
  Line,
  LineChart,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from "recharts";

import { Card, CardContent } from "@/components/ui/card";

type ChartPoint = {
  year: string;
  count: number;
};

export type PrestasiCategoryChart = {
  category: string;
  data: ChartPoint[];
  maxCount: number;
};

type PrestasiCategoryChartsProps = {
  charts: PrestasiCategoryChart[];
  colors: Record<string, string>;
};

export function PrestasiCategoryCharts({
  charts,
  colors,
}: PrestasiCategoryChartsProps) {
  return (
    <div className="grid gap-6 md:grid-cols-2 lg:grid-cols-3">
      {charts.map((chart) => {
        const hasChartData = chart.data.some((point) => point.count > 0);

        return (
          <Card key={chart.category}>
            <CardContent>
              <div className="space-y-3">
                <div>
                  <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    {chart.category}
                  </p>
                  <p className="text-sm font-semibold text-zinc-700">
                    Total prestasi per tahun
                  </p>
                </div>
                <div className="h-56 w-full sm:h-64 lg:h-72">
                  {hasChartData ? (
                    <ResponsiveContainer width="100%" height="100%">
                      <LineChart data={chart.data}>
                        <CartesianGrid strokeDasharray="3 3" />
                        <XAxis dataKey="year" />
                        <YAxis domain={[0, chart.maxCount]} />
                        <Tooltip />
                        <Line
                          type="monotone"
                          dataKey="count"
                          stroke={colors[chart.category]}
                          strokeWidth={2.5}
                          dot={{ r: 4 }}
                          activeDot={{ r: 6 }}
                        />
                      </LineChart>
                    </ResponsiveContainer>
                  ) : (
                    <div className="flex h-full items-center justify-center rounded-lg border border-dashed border-zinc-200 text-sm text-zinc-500">
                      Data belum tersedia
                    </div>
                  )}
                </div>
              </div>
            </CardContent>
          </Card>
        );
      })}
    </div>
  );
}
