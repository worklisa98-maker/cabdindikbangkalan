import { apiFetch } from "@/lib/api/client";

export type LastUpdatedMeta = {
  prestasiUpdatedAt: string | null;
  siswaPTUpdatedAt: string | null;
  dataSekolahUpdatedAt: string | null;
};

export const fetchLastUpdatedMeta = async (): Promise<LastUpdatedMeta> => {
  const response = await apiFetch("/api/meta/last-updated", {
    cache: "no-store",
  });
  if (!response.ok) {
    throw new Error("Failed to fetch last updated metadata.");
  }
  const payload = (await response.json()) as { data?: LastUpdatedMeta };
  return (
    payload.data ?? {
      prestasiUpdatedAt: null,
      siswaPTUpdatedAt: null,
      dataSekolahUpdatedAt: null,
    }
  );
};
