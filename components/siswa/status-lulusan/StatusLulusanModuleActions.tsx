"use client";

import Link from "next/link";

import SiswaModuleActions from "@/components/siswa/SiswaModuleActions";
import type { OutcomeStatusValue } from "@/components/siswa/OutcomeStatusTabs";

type StatusLulusanModuleActionsProps = {
  activeStatus: OutcomeStatusValue;
  isSchoolAdmin: boolean;
  onSuccess?: () => void;
};

const alumniTemplateByStatus: Record<OutcomeStatusValue, string> = {
  PT: "/api/legacy-import/templates/higher-ed",
  KERJA: "/api/legacy-import/templates/work",
  WIRAUSAHA: "/api/legacy-import/templates/entrepreneur",
  BELUM: "/api/legacy-import/templates/undefined",
};

const finalTemplateByStatus: Record<OutcomeStatusValue, string> = {
  PT: "/api/siswa/outcomes/template.xlsx?status=PT",
  KERJA: "/api/siswa/outcomes/template.xlsx?status=KERJA",
  WIRAUSAHA: "/api/siswa/outcomes/template.xlsx?status=WIRAUSAHA",
  BELUM: "/api/siswa/outcomes/template.xlsx?status=BELUM",
};

export default function StatusLulusanModuleActions({
  activeStatus,
  isSchoolAdmin,
  onSuccess,
}: StatusLulusanModuleActionsProps) {
  if (!isSchoolAdmin) {
    return null;
  }

  const importUrl = `/api/siswa/status-lulusan/import?status=${activeStatus}`;

  return (
    <div className="space-y-3">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          Aksi Import
        </p>
        <Link
          href="/siswa/status-lulusan/histori"
          className="inline-flex items-center justify-center rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
        >
          Histori Import
        </Link>
      </div>
      {activeStatus === "PT" ? null : (
        <SiswaModuleActions
          title="Final Siswa Akhir"
          sectionLabel="Final Siswa Akhir"
          description="Template Final Siswa Akhir digunakan untuk finalisasi hasil akhir siswa pada tahun berjalan atau data final resmi."
          templateUrl={finalTemplateByStatus[activeStatus]}
          templateButtonLabel="Download Template Final Siswa Akhir"
          importUrl={importUrl}
          importButtonLabel="Import Final Siswa Akhir"
          importCloseButtonLabel="Tutup Import Final Siswa Akhir"
          submitButtonLabel="Submit Import Final Siswa Akhir"
          formatHint={`Gunakan template final siswa akhir sesuai tab ${activeStatus}. Sistem akan membaca jenis template otomatis.`}
          accept=".xlsx,.xlsm"
          isSchoolAdmin={isSchoolAdmin}
          onSuccess={onSuccess}
        />
      )}
      <SiswaModuleActions
        title="Siswa Alumni"
        sectionLabel="Siswa Alumni"
        description="Template Siswa Alumni digunakan untuk pendataan lulusan/alumni tahun sebelumnya."
        templateUrl={alumniTemplateByStatus[activeStatus]}
        templateButtonLabel="Download Template Siswa Alumni"
        importUrl={importUrl}
        importButtonLabel="Import Siswa Alumni"
        importCloseButtonLabel="Tutup Import Siswa Alumni"
        submitButtonLabel="Submit Import Siswa Alumni"
        formatHint={`Gunakan template siswa alumni sesuai tab ${activeStatus}. Sistem akan membaca jenis template otomatis.`}
        accept=".xlsx,.xlsm"
        isSchoolAdmin={isSchoolAdmin}
        onSuccess={onSuccess}
      />
    </div>
  );
}
