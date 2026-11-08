import { redirect } from "next/navigation";
import { getServerAuthSnapshot } from "@/lib/auth/server";
import { MGMPRegistrations } from "@/components/mgmp/MGMPAdministration";
export default async function Page() {
  const { user } = await getServerAuthSnapshot();
  if (!user) redirect("/login");
  if (!["ADMIN", "ADMIN_CABDIN", "ADMIN_MASTER"].includes(user.role))
    redirect("/mgmp");
  return <MGMPRegistrations />;
}
