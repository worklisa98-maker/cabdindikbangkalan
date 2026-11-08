import { redirect } from "next/navigation";
import { getServerAuthSnapshot } from "@/lib/auth/server";
import { MGMPSettings } from "@/components/mgmp/MGMPAdministration";
export default async function Page() {
  const { user } = await getServerAuthSnapshot();
  if (!user) redirect("/login");
  if (user.role !== "ADMIN_MASTER") redirect("/mgmp");
  return <MGMPSettings />;
}
