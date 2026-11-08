"use client";

import { useEffect, useMemo, useState } from "react";
import readXlsxFile from "read-excel-file";
import { Download, FileSpreadsheet, Plus, Send, Trash2 } from "lucide-react";
import { apiFetch } from "@/lib/api/client";
import {
  Modal,
  PageTitle,
  Panel,
  PublicMGMPNavigation,
  buttonClass,
  inputClass,
  readAPI,
  secondaryButtonClass,
} from "./shared";

type Member = {
  fullName: string;
  identityNumber: string;
  schoolId: number | null;
  schoolName: string;
  position: string;
  email: string;
  whatsapp: string;
  status: string;
  errors?: string[];
};
type References = {
  schools: { id: number; name: string }[];
  subjects: string[];
};
const blankMember = (): Member => ({
  fullName: "",
  identityNumber: "",
  schoolId: null,
  schoolName: "",
  position: "ANGGOTA",
  email: "",
  whatsapp: "",
  status: "AKTIF",
});

const normalizePosition = (value: unknown) => {
  const v = String(value ?? "")
    .trim()
    .toUpperCase();
  if (v === "SEKRETARIS" || v === "WAKIL" || v === "SEKRETARIS/WAKIL")
    return "SEKRETARIS_WAKIL";
  return v || "ANGGOTA";
};

export function MGMPRegistration() {
  const [refs, setRefs] = useState<References>({ schools: [], subjects: [] });
  const [form, setForm] = useState({
    organizationType: "MGMP" as "MGMP" | "JARKOM",
    name: "",
    subject: "",
    region: "Kabupaten Bangkalan",
    managementPeriod: `${new Date().getFullYear()}–${new Date().getFullYear() + 3}`,
  });
  const [members, setMembers] = useState<Member[]>([
    blankMember(),
    blankMember(),
  ]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [success, setSuccess] = useState<{
    registrationId: string;
    name: string;
  } | null>(null);
  useEffect(() => {
    void apiFetch("/api/public/mgmp/references")
      .then(readAPI<References>)
      .then(setRefs)
      .catch(() => setError("Referensi sekolah belum dapat dimuat."));
  }, []);
  const leaders = useMemo(
    () => ({
      chair: members.find((m) => m.position === "KETUA"),
      secretary: members.find((m) => m.position === "SEKRETARIS_WAKIL"),
    }),
    [members]
  );
  const validCount = members.filter(
    (m) =>
      !m.errors?.length && m.fullName && m.schoolName && m.email && m.whatsapp
  ).length;
  const duplicateCount =
    members.length -
    new Set(members.map((m) => m.email.toLowerCase()).filter(Boolean)).size -
    members.filter((m) => !m.email).length;
  const leaderCards: [string, Member | undefined][] = [
    ["Ketua", leaders.chair],
    ["Sekretaris/Wakil", leaders.secretary],
  ];
  const update = (index: number, patch: Partial<Member>) =>
    setMembers((current) =>
      current.map((item, i) =>
        i === index ? { ...item, ...patch, errors: undefined } : item
      )
    );
  const importExcel = async (file: File) => {
    setError("");
    const rows = await readXlsxFile(file);
    const source = rows.slice(1).filter((row) => row.some(Boolean));
    const schoolMap = new Map(
      refs.schools.map((s) => [s.name.trim().toLowerCase(), s])
    );
    const seen = new Set<string>();
    const parsed = source.map((row): Member => {
      const schoolName = String(row[2] ?? "").trim();
      const email = String(row[4] ?? "")
        .trim()
        .toLowerCase();
      const errors: string[] = [];
      const school = schoolMap.get(schoolName.toLowerCase());
      const position = normalizePosition(row[3]);
      const status = String(row[6] ?? "Aktif")
        .trim()
        .toUpperCase();
      if (
        !String(row[0] ?? "").trim() ||
        !schoolName ||
        !email ||
        !String(row[5] ?? "").trim()
      )
        errors.push("Data wajib kosong");
      if (!/^\S+@\S+\.\S+$/.test(email)) errors.push("Email tidak valid");
      if (!/^\+?[0-9 ()-]{9,20}$/.test(String(row[5] ?? "")))
        errors.push("Nomor WA tidak valid");
      if (!school) errors.push("Sekolah tidak ditemukan pada referensi SMK");
      if (
        ![
          "KETUA",
          "SEKRETARIS_WAKIL",
          "BENDAHARA",
          "ANGGOTA",
          "LAINNYA",
        ].includes(position)
      )
        errors.push("Jabatan tidak valid");
      if (!["AKTIF", "NONAKTIF"].includes(status))
        errors.push("Status tidak valid");
      if (seen.has(email)) errors.push("Duplikat anggota");
      if (email) seen.add(email);
      return {
        fullName: String(row[0] ?? "").trim(),
        identityNumber: String(row[1] ?? "").trim(),
        schoolId: school?.id ?? null,
        schoolName,
        position,
        email,
        whatsapp: String(row[5] ?? "").trim(),
        status,
        errors,
      };
    });
    setMembers(parsed.length ? parsed : [blankMember(), blankMember()]);
  };
  const submit = async () => {
    setBusy(true);
    setError("");
    try {
      const invalidRow = members.findIndex((member) => member.errors?.length);
      if (invalidRow >= 0)
        throw new Error(
          `Perbaiki data anggota baris ${invalidRow + 1}: ${members[invalidRow].errors?.join(", ")}.`
        );
      if (!leaders.chair || !leaders.secretary)
        throw new Error("Pilih tepat satu Ketua dan satu Sekretaris/Wakil.");
      if (
        leaders.chair.email.toLowerCase() ===
        leaders.secretary.email.toLowerCase()
      )
        throw new Error("Email Ketua dan Sekretaris/Wakil harus berbeda.");
      const response = await apiFetch("/api/public/mgmp/registrations", {
        method: "POST",
        body: JSON.stringify({ ...form, members }),
      });
      if (response.status >= 500)
        throw new Error(
          "Pendaftaran gagal karena terjadi kesalahan pada server. Silakan coba kembali."
        );
      const data = await readAPI<{ registrationId: string; name: string }>(
        response
      );
      setSuccess(data);
    } catch (e) {
      setError(e instanceof Error ? e.message : "Pendaftaran gagal.");
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="mx-auto min-h-full max-w-7xl px-4 py-6 md:px-6">
      <PublicMGMPNavigation active="register" />
      <PageTitle
        title={`Registrasi ${form.organizationType} SMK`}
        description="Daftarkan organisasi dan anggota pengurus. Akun baru dibuat setelah verifikasi Cabang Dinas."
        actions={
          <a
            href="/api/public/mgmp/template.xlsx"
            className={secondaryButtonClass}
          >
            <Download className="h-4 w-4" />
            Unduh Template Anggota
          </a>
        }
      />
      <div className="grid gap-5 lg:grid-cols-[1fr_1.6fr]">
        <Panel>
          <div className="mb-4 grid grid-cols-2 gap-2"><button className={form.organizationType === "MGMP" ? buttonClass : secondaryButtonClass} onClick={() => setForm({ ...form, organizationType: "MGMP" })}>Daftar MGMP</button><button className={form.organizationType === "JARKOM" ? buttonClass : secondaryButtonClass} onClick={() => setForm({ ...form, organizationType: "JARKOM" })}>Daftar JARKOM</button></div>
          <h2 className="font-bold">Identitas {form.organizationType}</h2>
          <div className="mt-4 space-y-3">
            {(
              [
                ["name", `Nama ${form.organizationType}`],
                ["region", "Wilayah"],
                ["managementPeriod", "Periode kepengurusan"],
              ] as const
            ).map(([key, label]) => (
              <label
                key={key}
                className="block text-sm font-medium text-slate-700"
              >
                {label}
                <input
                  className={`${inputClass} mt-1`}
                  value={form[key]}
                  onChange={(e) => setForm({ ...form, [key]: e.target.value })}
                />
              </label>
            ))}
            <label className="block text-sm font-medium text-slate-700">
              Bidang / Mapel
              <select
                className={`${inputClass} mt-1`}
                value={form.subject}
                onChange={(e) => setForm({ ...form, subject: e.target.value })}
              >
                <option value="">Pilih bidang/mapel</option>
                {refs.subjects.map((item) => (
                  <option key={item}>{item}</option>
                ))}
              </select>
            </label>
          </div>
        </Panel>
        <Panel>
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <h2 className="font-bold">Daftar Anggota</h2>
              <p className="text-xs text-slate-500">
                Email dan WhatsApp wajib. Sekolah dicocokkan dengan master SMK
                SIPRES.
              </p>
            </div>
            <label className={secondaryButtonClass}>
              <FileSpreadsheet className="h-4 w-4" />
              Pilih Excel
              <input
                type="file"
                accept=".xlsx"
                className="hidden"
                onChange={(e) =>
                  e.target.files?.[0] && void importExcel(e.target.files[0])
                }
              />
            </label>
          </div>
          <div className="my-4 grid grid-cols-2 gap-2 text-center text-xs sm:grid-cols-4">
            <div className="rounded-lg bg-slate-100 p-2">
              <b>{members.length}</b>
              <br />
              baris ditemukan
            </div>
            <div className="rounded-lg bg-emerald-50 p-2 text-emerald-700">
              <b>{validCount}</b>
              <br />
              valid
            </div>
            <div className="rounded-lg bg-amber-50 p-2 text-amber-700">
              <b>{members.filter((m) => m.errors?.length).length}</b>
              <br />
              perlu diperbaiki
            </div>
            <div className="rounded-lg bg-red-50 p-2 text-red-700">
              <b>{Math.max(0, duplicateCount)}</b>
              <br />
              duplikat
            </div>
          </div>
          <div className="max-h-[520px] space-y-3 overflow-y-auto pr-1">
            {members.map((m, i) => (
              <div
                key={i}
                className={`rounded-xl border p-3 ${m.errors?.length ? "border-red-300 bg-red-50" : "border-slate-200"}`}
              >
                <div className="grid gap-2 md:grid-cols-2">
                  <input
                    className={inputClass}
                    placeholder="Nama lengkap"
                    value={m.fullName}
                    onChange={(e) => update(i, { fullName: e.target.value })}
                  />
                  <input
                    className={inputClass}
                    placeholder="NIP/NIK (opsional)"
                    value={m.identityNumber}
                    onChange={(e) =>
                      update(i, { identityNumber: e.target.value })
                    }
                  />
                  <select
                    className={inputClass}
                    value={m.schoolId ?? ""}
                    onChange={(e) => {
                      const school = refs.schools.find(
                        (s) => s.id === Number(e.target.value)
                      );
                      update(i, {
                        schoolId: school?.id ?? null,
                        schoolName: school?.name ?? "",
                      });
                    }}
                  >
                    <option value="">Pilih sekolah asal</option>
                    {refs.schools.map((s) => (
                      <option key={s.id} value={s.id}>
                        {s.name}
                      </option>
                    ))}
                  </select>
                  <select
                    className={inputClass}
                    value={m.position}
                    onChange={(e) => update(i, { position: e.target.value })}
                  >
                    {[
                      ["KETUA", "Ketua"],
                      ["SEKRETARIS_WAKIL", "Sekretaris/Wakil"],
                      ["BENDAHARA", "Bendahara"],
                      ["ANGGOTA", "Anggota"],
                      ["LAINNYA", "Lainnya"],
                    ].map(([v, l]) => (
                      <option key={v} value={v}>
                        {l}
                      </option>
                    ))}
                  </select>
                  <input
                    className={inputClass}
                    type="email"
                    placeholder="Email"
                    value={m.email}
                    onChange={(e) => update(i, { email: e.target.value })}
                  />
                  <input
                    className={inputClass}
                    placeholder="Nomor WhatsApp"
                    value={m.whatsapp}
                    onChange={(e) => update(i, { whatsapp: e.target.value })}
                  />
                  <select
                    className={inputClass}
                    value={m.status}
                    onChange={(e) => update(i, { status: e.target.value })}
                  >
                    <option value="AKTIF">Aktif</option>
                    <option value="NONAKTIF">Nonaktif</option>
                  </select>
                  <button
                    className="flex items-center justify-center gap-1 text-sm text-red-600"
                    onClick={() =>
                      setMembers((v) => v.filter((_, x) => x !== i))
                    }
                  >
                    <Trash2 className="h-4 w-4" />
                    Hapus
                  </button>
                </div>
                {m.errors?.length ? (
                  <p className="mt-2 text-xs text-red-700">
                    {m.errors.join(" · ")}
                  </p>
                ) : null}
              </div>
            ))}
          </div>
          <button
            className={`${secondaryButtonClass} mt-3`}
            onClick={() => setMembers((v) => [...v, blankMember()])}
          >
            <Plus className="h-4 w-4" />
            Tambah manual
          </button>
        </Panel>
      </div>
      <Panel className="mt-5">
        <h2 className="font-bold">Konfirmasi Pengurus Utama</h2>
        <div className="mt-3 grid gap-3 md:grid-cols-2">
          {leaderCards.map(([label, m]) => (
            <div key={label} className="rounded-xl bg-blue-50 p-4">
              <b>{label}</b>
              {m ? (
                <div className="mt-1 text-sm text-slate-700">
                  <p>{m.fullName}</p>
                  <p>{m.schoolName}</p>
                  <p>
                    {m.email} · {m.whatsapp}
                  </p>
                </div>
              ) : (
                <p className="mt-1 text-sm text-amber-700">
                  Belum dipilih pada kolom jabatan.
                </p>
              )}
            </div>
          ))}
        </div>
        {error ? (
          <p className="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700">
            {error}
          </p>
        ) : null}
        <div className="mt-4 flex justify-end">
          <button
            className={buttonClass}
            disabled={busy}
            onClick={() => void submit()}
          >
            <Send className="h-4 w-4" />
            {busy ? "Mengirim..." : "Kirim Pendaftaran"}
          </button>
        </div>
      </Panel>
      {success ? (
        <Modal
          title="Pendaftaran berhasil dikirim"
          onClose={() => setSuccess(null)}
        >
          <div className="space-y-3 text-sm">
            <div className="rounded-xl bg-emerald-50 p-4 text-emerald-800">
              <p className="text-xs font-semibold uppercase">ID Registrasi</p>
              <p className="mt-1 text-2xl font-black tracking-wide">
                {success.registrationId}
              </p>
            </div>
            <p>
              <b>Nama {form.organizationType}:</b> {success.name}
            </p>
            <p>
              <b>Status:</b> Menunggu Verifikasi
            </p>
            <p>Simpan ID registrasi ini untuk mengecek status pendaftaran.</p>
            <button
              className={buttonClass}
              onClick={() =>
                navigator.clipboard.writeText(success.registrationId)
              }
            >
              Salin ID
            </button>
          </div>
        </Modal>
      ) : null}
    </div>
  );
}
