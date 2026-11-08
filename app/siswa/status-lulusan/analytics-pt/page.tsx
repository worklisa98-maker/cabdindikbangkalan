import { redirect } from "next/navigation";

export const dynamic = "force-dynamic";

export default function StatusLulusanAnalyticsPTPage() {
  redirect("/siswa/status-lulusan/pt?tab=PT");
}
