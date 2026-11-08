import { apiFetch } from "@/lib/api/client";

export type ExternalService = {
  url: string;
  description: string;
  is_active: boolean;
  dialog_title?: string;
  dialog_body?: string;
  dialog_button?: string;
};
export type LinkCategory = { id: string; title: string; sort_order: number };
export type ServiceLink = {
  id: string; category_id: string; title: string; description: string;
  url: string; icon: string; sort_order: number; is_active: boolean; is_public: boolean;
};
export type LinkSettings = { categories: LinkCategory[]; links: ServiceLink[]; version?: string };
export type ServiceLinksData = { halo: ExternalService; sinom: ExternalService; pusat_tautan: LinkSettings };

export async function fetchServiceLinks(scope: "public" | "user" | "admin"): Promise<ServiceLinksData> {
  const path = scope === "admin" ? "/admin/master/service-links" : scope === "user" ? "/service-links" : "/public/service-links";
  const response = await apiFetch(path, { cache: "no-store" }, { skipAuthRedirect: scope === "public" });
  if (!response.ok) throw new Error("Gagal memuat Layanan & Tautan.");
  const payload = await response.json();
  return payload.data as ServiceLinksData;
}
