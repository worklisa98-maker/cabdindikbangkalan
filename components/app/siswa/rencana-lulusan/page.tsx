import { Suspense } from "react";

import RencanaLulusanClient from "./RencanaLulusanClient";

export default function RencanaLulusanPage() {
  return (
    <Suspense
      fallback={
        <section className="rounded-xl border border-dashed border-zinc-200 bg-white p-6 text-sm text-zinc-500">
          Memuat...
        </section>
      }
    >
      <RencanaLulusanClient />
    </Suspense>
  );
}
