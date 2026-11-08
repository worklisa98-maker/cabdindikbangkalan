import { redirect } from "next/navigation";
import { getServerAuthSnapshot } from "@/lib/auth/server";
import { MGMPHistory } from "@/components/mgmp/MGMPAdministration";
export default async function Page() {
  const { user } = await getServerAuthSnapshot();
  if (!user) redirect("/login");
  if (
    !["ADMIN", "ADMIN_CABDIN", "ADMIN_MASTER", "USER_MGMP"].includes(user.role)
  )
    redirect("/mgmp");
  return <MGMPHistory />;
}
