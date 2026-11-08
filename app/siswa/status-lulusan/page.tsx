import { redirect } from "next/navigation";

const statusRoutes: Record<string, string> = {
  ANALYTICS_PT: "/siswa/status-lulusan/analisa",
  PT: "/siswa/status-lulusan/pt",
  KERJA: "/siswa/status-lulusan/kerja",
  WIRAUSAHA: "/siswa/status-lulusan/wirausaha",
  BELUM: "/siswa/status-lulusan/belum",
};

type StatusLulusanLandingPageProps = {
  searchParams?: Promise<{
    status?: string;
    tab?: string;
  }>;
};

export default async function StatusLulusanLandingPage({
  searchParams,
}: StatusLulusanLandingPageProps) {
  const params = (await searchParams) ?? {};
  const rawTab = (params.tab ?? "").toUpperCase();
  if (rawTab && statusRoutes[rawTab]) {
    redirect(`${statusRoutes[rawTab]}?tab=${rawTab}`);
  }

  const rawStatus = (params.status ?? "").toUpperCase();
  if (rawStatus && statusRoutes[rawStatus]) {
    redirect(`${statusRoutes[rawStatus]}?tab=${rawStatus}`);
  }

  redirect("/siswa/status-lulusan/analisa?tab=ANALYTICS_PT");
}
