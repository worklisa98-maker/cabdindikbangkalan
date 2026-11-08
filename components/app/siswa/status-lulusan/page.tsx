import { redirect } from "next/navigation";

import StatusLulusanDashboard from "@/components/siswa/status-lulusan/StatusLulusanDashboard";

const statusRoutes: Record<string, string> = {
  PT: "/siswa/status-lulusan/pt",
  KERJA: "/siswa/status-lulusan/kerja",
  WIRAUSAHA: "/siswa/status-lulusan/wirausaha",
  BELUM: "/siswa/status-lulusan/belum",
};

type StatusLulusanLandingPageProps = {
  searchParams?: {
    status?: string;
  };
};

export default function StatusLulusanLandingPage({
  searchParams,
}: StatusLulusanLandingPageProps) {
  const rawStatus = (searchParams?.status ?? "").toUpperCase();
  if (rawStatus && statusRoutes[rawStatus]) {
    redirect(statusRoutes[rawStatus]);
  }
  return <StatusLulusanDashboard status="PT" />;
}
