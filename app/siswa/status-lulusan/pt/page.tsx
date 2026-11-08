import { Suspense } from "react";
import StatusLulusanDashboard from "@/components/siswa/status-lulusan/StatusLulusanDashboard";
import StatusLulusanAnalyticsPTPanel from "@/components/siswa/status-lulusan/StatusLulusanAnalyticsPTPanel";
import PTOfficialCoveragePanel from "@/components/siswa/status-lulusan/PTOfficialCoveragePanel";

export default function StatusLulusanPtPage() {
  return (
    <Suspense fallback={null}>
      <div className="space-y-8">
        <StatusLulusanDashboard status="PT" />
        <PTOfficialCoveragePanel />
        <StatusLulusanAnalyticsPTPanel />
      </div>
    </Suspense>
  );
}
