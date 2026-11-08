import { Suspense } from "react";
import StatusLulusanDashboard from "@/components/siswa/status-lulusan/StatusLulusanDashboard";

export default function StatusLulusanWirausahaPage() {
  return (
    <Suspense fallback={null}>
      <StatusLulusanDashboard status="WIRAUSAHA" />
    </Suspense>
  );
}
