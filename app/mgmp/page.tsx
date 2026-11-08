import { redirect } from "next/navigation";
import { getServerAuthSnapshot } from "@/lib/auth/server";
import { MGMPDashboard } from "@/components/mgmp/MGMPDashboard";

export default async function Page() {
  const { user } = await getServerAuthSnapshot();
  if (!user) redirect("/mgmp/register");
  if (
    !["ADMIN", "ADMIN_CABDIN", "ADMIN_MASTER", "USER_MGMP"].includes(user.role)
  )
    redirect("/");
  return <MGMPDashboard />;
}
