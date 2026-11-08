import { HardiknasDetailPage } from "@/components/hardiknas/HardiknasDetailPage";

export default async function LombaHardiknasDetailRoute({
  params,
}: {
  params: Promise<{ id: string }>;
}) {
  const { id } = await params;
  const entryId = Number(id);

  return (
    <div className="min-h-full px-4 py-4 md:px-6 md:py-6">
      <HardiknasDetailPage entryId={Number.isFinite(entryId) ? entryId : 0} />
    </div>
  );
}
