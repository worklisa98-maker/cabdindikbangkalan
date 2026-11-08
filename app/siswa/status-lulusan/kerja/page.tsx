import { Suspense } from "react";
import StatusLulusanDashboard from "@/components/siswa/status-lulusan/StatusLulusanDashboard";

export default function StatusLulusanKerjaPage() {
  return (
    <Suspense fallback={null}>
      <StatusLulusanDashboard status="KERJA" />
    </Suspense>
  );
}
