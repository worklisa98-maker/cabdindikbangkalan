import { Suspense } from "react";
import StatusLulusanDashboard from "@/components/siswa/status-lulusan/StatusLulusanDashboard";

export default function StatusLulusanBelumPage() {
  return (
    <Suspense fallback={null}>
      <StatusLulusanDashboard status="BELUM" />
    </Suspense>
  );
}
