import { redirect } from "next/navigation";
import { GaspolHistoryPage } from "@/components/gaspol/GaspolHistoryPage";
import { getServerAuthSnapshot } from "@/lib/auth/server";

export default async function Page() {
  const { user, status } = await getServerAuthSnapshot();
  if (status === "unauthenticated" || !user) redirect("/login");
  if (
    user.role !== "ADMIN" &&
    user.role !== "ADMIN_CABDIN" &&
    user.role !== "ADMIN_MASTER"
  )
    redirect("/gaspol");
  return <GaspolHistoryPage />;
}
