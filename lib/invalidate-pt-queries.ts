import type { QueryClient } from "@tanstack/react-query";

const ptDerivedQueryPrefixes = [
  "acceptance-analytics",
  "acceptance-offer-school-summary",
  "acceptance-offer-supervisor-comparison",
  "acceptance-analytics-schools",
  "pt-official-coverage",
  "pt-official-coverage-panel",
  "eligible-snbp",
  "status-lulusan-dashboard",
  "status-lulusan-list",
  "status-lulusan-pt-ranking",
  "status-lulusan-analytics-pt",
  "status-lulusan-years",
  "status-lulusan-distribution",
  "status-lulusan-pt-pending-actions",
  "siswa-outcomes-list",
  "siswa-outcomes-analytics",
  "siswa-status-conflict-pending",
  "siswa-status-pending-school",
  "siswa-status-pending-summary",
  "siswa-status-pending-recap",
  "admission-validation-summary",
  "pt-pipeline",
] as const;

export async function invalidatePTDerivedQueries(queryClient: QueryClient) {
  await Promise.all(
    ptDerivedQueryPrefixes.map((queryKey) =>
      queryClient.invalidateQueries({ queryKey: [queryKey] })
    )
  );
}
