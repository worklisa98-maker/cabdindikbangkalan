"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import readXlsxFile from "read-excel-file";
import {
  CalendarCheck,
  ClipboardList,
  ExternalLink,
  Eye,
  Plus,
  Power,
  RotateCcw,
  Search,
  Users,
} from "lucide-react";
import { useAuth } from "@/lib/auth/client";
import { apiFetch } from "@/lib/api/client";
import {
  Modal,
  PageTitle,
  Panel,
  StatusPill,
  buttonClass,
  inputClass,
  readAPI,
  secondaryButtonClass,
} from "./shared";

type Activity = {
  id: number;
  title: string;
  activityDate: string;
  venue: string;
  description?: string;
  participantCount: number;
  activeMemberCount?: number;
  attendanceUrl: string;
  documentationUrl: string;
  notes?: string;
  createdBy?: string;
  createdByPosition?: string;
  updatedBy?: string;
  updatedByPosition?: string;
  attendance?: Attendance[];
};
type Attendance = {
  memberId: number;
  fullName?: string;
  schoolName?: string;
  status: "PRESENT" | "ABSENT";
  absenceType?: "EXCUSED" | "UNEXCUSED" | "";
  absenceNote?: string;
};
type Member = {
  id: number;
  fullName: string;
  identityNumber?: string;
  schoolId?: number | null;
  schoolName: string;
  position: string;
  email: string;
  whatsapp: string;
  status: string;
  deactivationReason?: string;
  deactivationNote?: string;
  deactivatedAt?: string;
  attendanceStats?: { activities: number; present: number; excused: number; unexcused: number };
};
type Workspace = {
  group: {
    id: number;
    organizationType: "MGMP" | "JARKOM";
    name: string;
    subject: string;
    region: string;
    status: string;
  };
  summary: {
    monthCount: number;
    yearCount: number;
    lastActivity?: string;
    monthStatus: string;
  };
  activities: Activity[];
  members: Member[];
  notifications?: { id: number; memberName: string; reason: string; note?: string; effectiveDate: string }[];
};
type MonitorItem = {
  id: number;
  organizationType: "MGMP" | "JARKOM";
  name: string;
  subject: string;
  region: string;
  groupStatus: string;
  monthStatus: string;
  monthCount: number;
  yearCount: number;
  lastActivity?: string;
  missingStreak?: number;
};
type Monitoring = {
  kpi: Record<string, number>;
  items: MonitorItem[];
  year: number;
  month: number;
  warningThreshold: number;
};
type GroupDetail = {
  id: number;
  organizationType: "MGMP" | "JARKOM";
  name: string;
  subject: string;
  region: string;
  period?: string;
  status: string;
  deactivationReason?: string;
  accounts: {
    userId: number;
    name: string;
    position: string;
    username: string;
    isActive: boolean;
  }[];
  members: Member[];
  activities: Activity[];
  history: {
    eventType: string;
    actor: string;
    reason?: string;
    createdAt: string;
  }[];
};
const emptyActivity = {
  title: "",
  activityDate: new Date().toISOString().slice(0, 10),
  venue: "",
  description: "",
  attendanceUrl: "",
  documentationUrl: "",
  notes: "",
};
const monthNames = [
  "Januari",
  "Februari",
  "Maret",
  "April",
  "Mei",
  "Juni",
  "Juli",
  "Agustus",
  "September",
  "Oktober",
  "November",
  "Desember",
];
const deactivationReasons = [
  "Tidak aktif beberapa bulan",
  "Kepengurusan tidak berjalan",
  "Tidak ada laporan kegiatan",
  "Keputusan pimpinan",
  "MGMP dibubarkan/bergabung",
  "Lainnya",
];

function UserWorkspace() {
  const now = new Date();
  const [year, setYear] = useState(now.getFullYear());
  const [month, setMonth] = useState(now.getMonth() + 1);
  const [data, setData] = useState<Workspace | null>(null);
  const [schools, setSchools] = useState<{ id: number; name: string }[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState("");
  const [showActivity, setShowActivity] = useState(false);
  const [showNoAgenda, setShowNoAgenda] = useState(false);
  const [editingId, setEditingId] = useState<number | null>(null);
  const [activity, setActivity] = useState(emptyActivity);
  const [activityStep, setActivityStep] = useState(1);
  const [attendanceMode, setAttendanceMode] = useState<"ALL" | "ABSENT">("ALL");
  const [attendanceSearch, setAttendanceSearch] = useState("");
  const [attendance, setAttendance] = useState<Record<number, Attendance>>({});
  const [memberTab, setMemberTab] = useState<"ACTIVE" | "INACTIVE">("ACTIVE");
  const [showMember, setShowMember] = useState(false);
  const [memberDraft, setMemberDraft] = useState({ fullName: "", identityNumber: "", schoolId: null as number | null, schoolName: "", position: "ANGGOTA", email: "", whatsapp: "", status: "AKTIF" });
  const [importSummary, setImportSummary] = useState<{ importId: number; total: number; success: number; rejected: number } | null>(null);
  const [reason, setReason] = useState("");
  const load = useCallback(
    () =>
      apiFetch(`/api/mgmp/workspace?year=${year}&month=${month}`)
        .then(readAPI<Workspace>)
        .then(setData)
        .catch((e) => setError(e.message)),
    [year, month]
  );
  useEffect(() => {
    void load();
  }, [load]);
  useEffect(() => {
    void apiFetch("/api/public/mgmp/references").then(readAPI<{ schools: { id: number; name: string }[] }>).then((refs) => setSchools(refs.schools)).catch(() => undefined);
  }, []);
  const saveActivity = async () => {
    setBusy(true);
    setError("");
    try {
      const endpoint = editingId
        ? `/api/mgmp/activities/${editingId}`
        : "/api/mgmp/activities";
      await readAPI(
        await apiFetch(endpoint, {
          method: editingId ? "PUT" : "POST",
          body: JSON.stringify({ ...activity, attendance: Object.values(attendance) }),
        })
      );
      setShowActivity(false);
      setEditingId(null);
      setActivity(emptyActivity);
      setActivityStep(1);
      setAttendance({});
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menyimpan.");
    } finally {
      setBusy(false);
    }
  };
  const startEdit = (item: Activity) => {
    setEditingId(item.id);
    setActivity({
      title: item.title,
      activityDate: item.activityDate.slice(0, 10),
      venue: item.venue,
      description: item.description ?? "",
      attendanceUrl: item.attendanceUrl,
      documentationUrl: item.documentationUrl,
      notes: item.notes ?? "",
    });
    const savedAttendance = item.attendance?.length
      ? item.attendance
      : (data?.members.filter((member) => member.status === "AKTIF").map((member) => ({ memberId: member.id, fullName: member.fullName, schoolName: member.schoolName, status: "PRESENT" as const, absenceType: "" as const, absenceNote: "" })) ?? []);
    setAttendance(Object.fromEntries(savedAttendance.map((row) => [row.memberId, row])));
    setAttendanceMode(savedAttendance.some((row) => row.status === "ABSENT") ? "ABSENT" : "ALL");
    setActivityStep(1);
    setShowActivity(true);
  };
  const openNewActivity = () => {
    const active = data?.members.filter((member) => member.status === "AKTIF") ?? [];
    setAttendance(Object.fromEntries(active.map((member) => [member.id, { memberId: member.id, fullName: member.fullName, schoolName: member.schoolName, status: "PRESENT" as const, absenceType: "", absenceNote: "" }])));
    setAttendanceMode("ALL");
    setActivityStep(1);
    setShowActivity(true);
  };
  const activeMembers = data?.members.filter((member) => member.status === "AKTIF") ?? [];
  const absentRows = Object.values(attendance).filter((row) => row.status === "ABSENT");
  const excusedCount = absentRows.filter((row) => row.absenceType === "EXCUSED").length;
  const unexcusedCount = absentRows.filter((row) => row.absenceType === "UNEXCUSED").length;
  const presentCount = Object.values(attendance).filter((row) => row.status === "PRESENT").length;
  const saveNoAgenda = async () => {
    setBusy(true);
    try {
      await readAPI(
        await apiFetch("/api/mgmp/no-agenda", {
          method: "PUT",
          body: JSON.stringify({ year, month, reason }),
        })
      );
      setShowNoAgenda(false);
      setReason("");
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menyimpan.");
    } finally {
      setBusy(false);
    }
  };
  const saveMember = async () => {
    setBusy(true);
    setError("");
    try {
      await readAPI(await apiFetch("/api/mgmp/members", { method: "POST", body: JSON.stringify(memberDraft) }));
      setShowMember(false);
      setMemberDraft({ fullName: "", identityNumber: "", schoolId: null, schoolName: "", position: "ANGGOTA", email: "", whatsapp: "", status: "AKTIF" });
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menambah anggota.");
    } finally { setBusy(false); }
  };
  const importMembers = async (file: File) => {
    setBusy(true);
    setError("");
    try {
      const rows = (await readXlsxFile(file)).slice(1).filter((row) => row.some(Boolean));
      const members = rows.map((row) => ({
        fullName: String(row[0] ?? "").trim(), identityNumber: String(row[1] ?? "").trim(), schoolName: String(row[2] ?? "").trim(),
        position: String(row[3] ?? "ANGGOTA").trim(), email: String(row[4] ?? "").trim(), whatsapp: String(row[5] ?? "").trim(), status: String(row[6] ?? "AKTIF").trim(),
      }));
      const result = await readAPI<{ importId: number; total: number; success: number; rejected: number }>(await apiFetch("/api/mgmp/members/import", { method: "POST", body: JSON.stringify({ members }) }));
      setImportSummary(result);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Import anggota gagal.");
    } finally { setBusy(false); }
  };
  const acknowledgeNotification = async (id: number) => {
    await readAPI(await apiFetch(`/api/mgmp/notifications/${id}/acknowledge`, { method: "POST" }));
    await load();
  };
  if (!data)
    return (
      <div className="p-8 text-sm text-slate-500">
        {error || "Memuat workspace MGMP..."}
      </div>
    );
  const summaryCards = [
    {
      label: "Kegiatan bulan ini",
      value: data.summary.monthCount,
      Icon: CalendarCheck,
    },
    {
      label: "Total tahun berjalan",
      value: data.summary.yearCount,
      Icon: ClipboardList,
    },
    {
      label: "Aktivitas terakhir",
      value: data.summary.lastActivity
        ? new Date(data.summary.lastActivity).toLocaleDateString("id-ID")
        : "—",
      Icon: RotateCcw,
    },
    {
      label: "Status laporan",
      value: data.summary.monthStatus.replaceAll("_", " "),
      Icon: Power,
    },
  ];
  return (
    <div className="min-h-full px-4 py-6 md:px-6">
      <PageTitle
        title={`${data.group.name} SMK ${data.group.region}`}
        description={`${data.group.subject} · Workspace bersama Ketua dan Sekretaris/Wakil`}
        actions={
          <div className="flex flex-wrap gap-2">
            <button
              className={secondaryButtonClass}
              disabled={data.group.status !== "ACTIVE"}
              onClick={() => setShowNoAgenda(true)}
            >
              Tidak Ada Agenda
            </button>
            <button
              className={buttonClass}
              disabled={data.group.status !== "ACTIVE"}
              onClick={openNewActivity}
            >
              <Plus className="h-4 w-4" />
              Tambah Kegiatan
            </button>
          </div>
        }
      />
      {data.group.status !== "ACTIVE" ? (
        <div className="mb-5 rounded-xl border border-red-200 bg-red-50 p-4 text-sm text-red-700">
          {data.group.organizationType} sedang nonaktif. Histori tetap dapat dilihat, tetapi input dan
          edit laporan dinonaktifkan.
        </div>
      ) : null}
      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {summaryCards.map(({ label, value, Icon }) => (
          <Panel key={label}>
            <div className="flex items-center justify-between">
              <div>
                <p className="text-xs text-slate-500">{label}</p>
                <p className="mt-1 text-xl font-bold text-slate-900">
                  {String(value)}
                </p>
              </div>
              <Icon className="h-6 w-6 text-blue-600" />
            </div>
          </Panel>
        ))}
      </div>
      <Panel className="mt-5">
        <div className="flex flex-wrap items-center justify-between gap-3">
          <h2 className="font-bold">
            Kegiatan {monthNames[month - 1]} {year}
          </h2>
          <div className="flex gap-2">
            <select
              className={inputClass}
              value={month}
              onChange={(e) => setMonth(Number(e.target.value))}
            >
              {monthNames.map((m, i) => (
                <option key={m} value={i + 1}>
                  {m}
                </option>
              ))}
            </select>
            <input
              type="number"
              className={`${inputClass} w-28`}
              value={year}
              onChange={(e) => setYear(Number(e.target.value))}
            />
          </div>
        </div>
        <div className="mt-4 space-y-3">
          {data.activities.length ? (
            data.activities.map((a) => (
              <article
                key={a.id}
                className="rounded-xl border border-slate-200 p-4"
              >
                <div className="flex flex-col justify-between gap-2 md:flex-row">
                  <div>
                    <p className="text-xs font-semibold text-blue-700">
                      {new Date(a.activityDate).toLocaleDateString("id-ID", {
                        dateStyle: "long",
                      })}
                    </p>
                    <h3 className="font-bold">{a.title}</h3>
                    <p className="text-sm text-slate-600">
                      {a.venue} · {a.participantCount} peserta
                    </p>
                  </div>
                  <div className="flex flex-wrap gap-2">
                    <button
                      className={secondaryButtonClass}
                      disabled={data.group.status !== "ACTIVE"}
                      onClick={() => startEdit(a)}
                    >
                      Edit
                    </button>
                    <a
                      className={secondaryButtonClass}
                      target="_blank"
                      href={a.attendanceUrl}
                    >
                      Absensi <ExternalLink className="h-3 w-3" />
                    </a>
                    <a
                      className={secondaryButtonClass}
                      target="_blank"
                      href={a.documentationUrl}
                    >
                      Dokumentasi <ExternalLink className="h-3 w-3" />
                    </a>
                  </div>
                </div>
                {a.description ? (
                  <p className="mt-3 text-sm text-slate-600">{a.description}</p>
                ) : null}
                <p className="mt-2 text-xs text-slate-400">
                  Dibuat oleh {a.createdBy || "pengurus"} —{" "}
                  {(a.createdByPosition || "pengurus").replaceAll("_", " ")} ·
                  Terakhir diubah oleh {a.updatedBy || "pengurus"} —{" "}
                  {(a.updatedByPosition || "pengurus").replaceAll("_", " ")}
                </p>
              </article>
            ))
          ) : (
            <p className="rounded-xl bg-slate-50 p-8 text-center text-sm text-slate-500">
              Belum ada kegiatan pada bulan ini.
            </p>
          )}
        </div>
      </Panel>
      <Panel className="mt-5">
        <details>
          <summary className="cursor-pointer font-bold">
            Kelola Anggota {data.group.organizationType} ({data.members?.length ?? 0})
          </summary>
          <div className="mt-3 flex flex-wrap gap-2">
            <button className={memberTab === "ACTIVE" ? buttonClass : secondaryButtonClass} onClick={() => setMemberTab("ACTIVE")}>Anggota Aktif</button>
            <button className={memberTab === "INACTIVE" ? buttonClass : secondaryButtonClass} onClick={() => setMemberTab("INACTIVE")}>Dinonaktifkan / Dikeluarkan</button>
            <button className={secondaryButtonClass} onClick={() => setShowMember(true)}><Plus className="h-4 w-4" /> Tambah Manual</button>
            <label className={secondaryButtonClass}>Import Excel<input type="file" accept=".xlsx" className="hidden" onChange={(e) => e.target.files?.[0] && void importMembers(e.target.files[0])} /></label>
            <a className={secondaryButtonClass} href="/api/public/mgmp/template.xlsx">Download Template</a>
          </div>
          {importSummary ? <div className="mt-3 rounded-xl bg-blue-50 p-3 text-sm text-blue-900">{importSummary.total} baris ditemukan · {importSummary.success} berhasil · {importSummary.rejected} ditolak. <a className="font-semibold underline" href={`/api/mgmp/members/imports/${importSummary.importId}/result.xlsx`}>Unduh Hasil Import</a></div> : null}
          <div className="mt-3 overflow-x-auto">
            <table className="w-full min-w-[650px] text-left text-sm">
              <thead>
                <tr className="border-b text-xs text-slate-500">
                  <th className="p-2">Nama</th>
                  <th className="p-2">Jabatan</th>
                  <th className="p-2">Sekolah</th>
                  <th className="p-2">Kontak</th>
                  <th className="p-2">Status</th>
                </tr>
              </thead>
              <tbody>
                {(data.members ?? []).filter((member) => memberTab === "ACTIVE" ? member.status === "AKTIF" : member.status !== "AKTIF").map((member, index) => (
                  <tr
                    key={`${member.email}-${index}`}
                    className="border-b border-slate-100"
                  >
                    <td className="p-2 font-medium">{member.fullName}</td>
                    <td className="p-2">
                      {member.position.replaceAll("_", " ")}
                    </td>
                    <td className="p-2">{member.schoolName}</td>
                    <td className="p-2">
                      {member.email}
                      <br />
                      {member.whatsapp}
                    </td>
                    <td className="p-2">
                      <StatusPill value={member.status} />
                      {member.status !== "AKTIF" ? <p className="mt-1 max-w-xs text-xs text-slate-500">{member.deactivatedAt ? new Date(member.deactivatedAt).toLocaleDateString("id-ID") : ""} · {member.deactivationReason}<br />{member.deactivationNote}</p> : null}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        </details>
      </Panel>
      {error ? (
        <p className="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700">
          {error}
        </p>
      ) : null}
      {showActivity ? (
        <Modal
          wide
          title={`${editingId ? "Edit" : "Tambah"} Kegiatan ${data.group.organizationType} · Tahap ${activityStep}/4`}
          onClose={() => {
            setShowActivity(false);
            setEditingId(null);
            setActivity(emptyActivity);
            setActivityStep(1);
          }}
        >
          <div className="space-y-3">
            {activityStep === 1 ? <>{(
              [
                ["title", "Nama/Judul Kegiatan", "text"],
                ["activityDate", "Tanggal Kegiatan", "date"],
                ["venue", "Tempat", "text"],
              ] as const
            ).map(([key, label, type]) => (
              <label key={key} className="block text-sm font-medium">
                {label}
                <input
                  type={type}
                  className={`${inputClass} mt-1`}
                  value={String(activity[key])}
                  onChange={(e) =>
                    setActivity({ ...activity, [key]: e.target.value })
                  }
                />
              </label>
            ))}
            <label className="block text-sm font-medium">
              Deskripsi/Ringkasan
              <textarea
                className={`${inputClass} mt-1`}
                rows={3}
                value={activity.description}
                onChange={(e) =>
                  setActivity({ ...activity, description: e.target.value })
                }
              />
            </label>
            <label className="block text-sm font-medium">
              Keterangan (opsional)
              <textarea
                className={`${inputClass} mt-1`}
                value={activity.notes}
                onChange={(e) =>
                  setActivity({ ...activity, notes: e.target.value })
                }
              />
            </label>
            <button className={`${buttonClass} w-full`} disabled={!activity.title.trim() || !activity.venue.trim() || !activity.description.trim()} onClick={() => setActivityStep(2)}>Lanjutkan ke Kehadiran</button></> : null}
            {activityStep === 2 ? <>
              <div className="grid gap-2 sm:grid-cols-2">
                <button className={attendanceMode === "ALL" ? buttonClass : secondaryButtonClass} onClick={() => { setAttendanceMode("ALL"); setAttendance(Object.fromEntries(activeMembers.map((member) => [member.id, { memberId: member.id, fullName: member.fullName, schoolName: member.schoolName, status: "PRESENT" as const, absenceType: "", absenceNote: "" }]))); }}>Hadir Semua</button>
                <button className={attendanceMode === "ABSENT" ? buttonClass : secondaryButtonClass} onClick={() => setAttendanceMode("ABSENT")}>Ada yang Tidak Hadir</button>
              </div>
              {attendanceMode === "ABSENT" ? <><div className="relative"><Search className="absolute top-3 left-3 h-4 w-4 text-slate-400" /><input className={`${inputClass} pl-9`} placeholder="Cari nama atau sekolah" value={attendanceSearch} onChange={(e) => setAttendanceSearch(e.target.value)} /></div>
                <div className="max-h-72 space-y-2 overflow-y-auto">{activeMembers.filter((m) => `${m.fullName} ${m.schoolName}`.toLowerCase().includes(attendanceSearch.toLowerCase())).map((member) => <label key={member.id} className="flex items-start gap-3 rounded-xl border p-3"><input type="checkbox" checked={attendance[member.id]?.status === "ABSENT"} onChange={(e) => setAttendance((current) => ({ ...current, [member.id]: { memberId: member.id, fullName: member.fullName, schoolName: member.schoolName, status: e.target.checked ? "ABSENT" : "PRESENT", absenceType: e.target.checked ? "EXCUSED" : "", absenceNote: "" } }))} /><span><b>{member.fullName}</b><br /><span className="text-xs text-slate-500">{member.schoolName}</span></span></label>)}</div></> : <p className="rounded-xl bg-emerald-50 p-4 text-sm text-emerald-800">Semua {activeMembers.length} anggota aktif akan dicatat hadir.</p>}
              <div className="flex gap-2"><button className={secondaryButtonClass} onClick={() => setActivityStep(1)}>Kembali</button><button className={`${buttonClass} flex-1`} onClick={() => setActivityStep(3)}>Lanjutkan</button></div>
            </> : null}
            {activityStep === 3 ? <>
              <h3 className="font-bold">Alasan Ketidakhadiran</h3>
              {absentRows.length ? <div className="max-h-72 space-y-3 overflow-y-auto">{absentRows.map((row) => <div key={row.memberId} className="rounded-xl border p-3"><b>{row.fullName}</b><p className="text-xs text-slate-500">{row.schoolName}</p><select className={`${inputClass} mt-2`} value={row.absenceType ?? ""} onChange={(e) => setAttendance((current) => ({ ...current, [row.memberId]: { ...current[row.memberId], absenceType: e.target.value as Attendance["absenceType"], absenceNote: e.target.value === "UNEXCUSED" ? "" : current[row.memberId].absenceNote } }))}><option value="">Pilih status</option><option value="EXCUSED">Dengan Keterangan</option><option value="UNEXCUSED">Tanpa Keterangan</option></select>{row.absenceType === "EXCUSED" ? <input className={`${inputClass} mt-2`} placeholder="Sakit, izin, tugas dinas..." value={row.absenceNote ?? ""} onChange={(e) => setAttendance((current) => ({ ...current, [row.memberId]: { ...current[row.memberId], absenceNote: e.target.value } }))} /> : null}</div>)}</div> : <p className="rounded-xl bg-emerald-50 p-4 text-sm text-emerald-800">Tidak ada anggota yang tidak hadir.</p>}
              <label className="block text-sm font-medium">Bukti Absensi<input type="url" className={`${inputClass} mt-1`} value={activity.attendanceUrl} onChange={(e) => setActivity({ ...activity, attendanceUrl: e.target.value })} /><span className="text-xs text-slate-500">Masukkan link file PDF/scan daftar hadir sebagai bukti administrasi kegiatan. Pastikan link dapat dibuka oleh Cabang Dinas.</span></label>
              <label className="block text-sm font-medium">Link Dokumentasi<input type="url" className={`${inputClass} mt-1`} value={activity.documentationUrl} onChange={(e) => setActivity({ ...activity, documentationUrl: e.target.value })} /><span className="text-xs text-slate-500">Masukkan link folder Google Drive yang berisi foto/video dokumentasi kegiatan.</span></label>
              <div className="flex gap-2"><button className={secondaryButtonClass} onClick={() => setActivityStep(2)}>Revisi</button><button className={`${buttonClass} flex-1`} disabled={!activity.attendanceUrl.trim() || !activity.documentationUrl.trim() || absentRows.some((row) => !row.absenceType || (row.absenceType === "EXCUSED" && !row.absenceNote?.trim()))} onClick={() => setActivityStep(4)}>Preview</button></div>
            </> : null}
            {activityStep === 4 ? <><div className="rounded-xl bg-slate-50 p-4 text-sm"><h3 className="font-bold">Ringkasan Kehadiran</h3><p className="mt-2">Kegiatan: {activity.title}</p><p>Tanggal: {new Date(activity.activityDate).toLocaleDateString("id-ID")}</p><p>Total anggota aktif: {Object.keys(attendance).length}</p><p>Hadir: {presentCount}</p><p>Tidak hadir dengan keterangan: {excusedCount}</p><p>Tidak hadir tanpa keterangan: {unexcusedCount}</p>{absentRows.length ? <ul className="mt-2 list-disc pl-5">{absentRows.map((row) => <li key={row.memberId}>{row.fullName} — {row.absenceType === "EXCUSED" ? row.absenceNote : "Tanpa keterangan"}</li>)}</ul> : null}<p className="mt-3 break-all">Bukti absensi: {activity.attendanceUrl}</p><p className="break-all">Dokumentasi: {activity.documentationUrl}</p></div><div className="flex gap-2"><button className={secondaryButtonClass} onClick={() => setActivityStep(3)}>Revisi</button><button className={`${buttonClass} flex-1`} disabled={busy} onClick={() => void saveActivity()}>{busy ? "Menyimpan..." : "Simpan Kegiatan"}</button></div></> : null}
          </div>
        </Modal>
      ) : null}
      {showMember ? <Modal title="Tambah Anggota Manual" onClose={() => setShowMember(false)}><div className="space-y-3">{([['fullName','Nama Lengkap'],['identityNumber','NIP/NIK'],['email','Email'],['whatsapp','Nomor WhatsApp']] as const).map(([key,label]) => <label key={key} className="block text-sm font-medium">{label}<input className={`${inputClass} mt-1`} value={String(memberDraft[key] ?? '')} onChange={(e) => setMemberDraft({ ...memberDraft, [key]: e.target.value })} /></label>)}<label className="block text-sm font-medium">Sekolah Asal<select className={`${inputClass} mt-1`} value={memberDraft.schoolId ?? ""} onChange={(e) => { const school = schools.find((item) => item.id === Number(e.target.value)); setMemberDraft({ ...memberDraft, schoolId: school?.id ?? null, schoolName: school?.name ?? "" }); }}><option value="">Pilih referensi SMK</option>{schools.map((school) => <option key={school.id} value={school.id}>{school.name}</option>)}</select></label><select className={inputClass} value={memberDraft.position} onChange={(e) => setMemberDraft({ ...memberDraft, position: e.target.value })}><option value="ANGGOTA">Anggota</option><option value="BENDAHARA">Bendahara</option><option value="LAINNYA">Lainnya</option></select><button className={`${buttonClass} w-full`} disabled={busy} onClick={() => void saveMember()}>Simpan Anggota</button></div></Modal> : null}
      {data.notifications?.[0] ? <Modal title="Perubahan Keanggotaan" onClose={() => void acknowledgeNotification(data.notifications![0].id)}><div className="space-y-3 text-sm"><p><b>{data.notifications[0].memberName}</b> telah dinonaktifkan dari keanggotaan {data.group.organizationType}.</p><p><b>Tanggal:</b><br />{new Date(data.notifications[0].effectiveDate).toLocaleDateString("id-ID", { dateStyle: "long" })}</p><p><b>Alasan:</b><br />{data.notifications[0].reason}</p><p><b>Keterangan:</b><br />{data.notifications[0].note || "-"}</p><button className={`${buttonClass} w-full`} onClick={() => void acknowledgeNotification(data.notifications![0].id)}>Mengerti</button></div></Modal> : null}
      {showNoAgenda ? (
        <Modal
          title={`Tidak Ada Agenda · ${monthNames[month - 1]} ${year}`}
          onClose={() => setShowNoAgenda(false)}
        >
          <p className="mb-3 text-sm text-slate-600">
            Status ini hanya dapat disimpan jika bulan tersebut belum memiliki
            kegiatan.
          </p>
          <textarea
            className={inputClass}
            rows={4}
            placeholder={`Belum terdapat agenda MGMP pada ${monthNames[month - 1]} ${year}...`}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
          <button
            className={`${buttonClass} mt-3 w-full`}
            disabled={busy || !reason.trim()}
            onClick={() => void saveNoAgenda()}
          >
            Simpan Pernyataan
          </button>
        </Modal>
      ) : null}
    </div>
  );
}

function AdminMonitoring() {
  const now = new Date();
  const [year, setYear] = useState(now.getFullYear());
  const [month, setMonth] = useState(now.getMonth() + 1);
  const [status, setStatus] = useState("");
  const [organizationType, setOrganizationType] = useState("");
  const [query, setQuery] = useState("");
  const [data, setData] = useState<Monitoring | null>(null);
  const [detail, setDetail] = useState<GroupDetail | null>(null);
  const [accounts, setAccounts] = useState<
    | {
        userId: number;
        name: string;
        position: string;
        username: string;
        password: string;
      }[]
    | null
  >(null);
  const [action, setAction] = useState<"DEACTIVATE" | "REACTIVATE" | null>(
    null
  );
  const [reason, setReason] = useState("");
  const [reasonPreset, setReasonPreset] = useState("");
  const [effectiveDate, setEffectiveDate] = useState(
    new Date().toISOString().slice(0, 10)
  );
  const [memberAction, setMemberAction] = useState<Member | null>(null);
  const [memberReason, setMemberReason] = useState("");
  const [memberNote, setMemberNote] = useState("");
  const [attendanceFilter, setAttendanceFilter] = useState("");
  const [adminAttendanceSearch, setAdminAttendanceSearch] = useState("");
  const [error, setError] = useState("");
  const load = useCallback(
    () =>
      apiFetch(`/api/mgmp/monitoring?year=${year}&month=${month}&organizationType=${organizationType}`)
        .then(readAPI<Monitoring>)
        .then(setData)
        .catch((e) => setError(e.message)),
    [year, month, organizationType]
  );
  useEffect(() => {
    void load();
  }, [load]);
  const items = useMemo(
    () =>
      data?.items.filter(
        (i) =>
          (!status || i.monthStatus === status || i.groupStatus === status) &&
          (!query ||
            `${i.name} ${i.subject}`
              .toLowerCase()
              .includes(query.toLowerCase()))
      ) ?? [],
    [data, status, query]
  );
  const openDetail = async (id: number) => {
    setError("");
    try {
      setDetail(await readAPI(await apiFetch(`/api/mgmp/groups/${id}`)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat detail.");
    }
  };
  const reveal = async () => {
    if (!detail) return;
    setAccounts(
      await readAPI(await apiFetch(`/api/mgmp/groups/${detail.id}/accounts`))
    );
  };
  const resetPassword = async (userId: number) => {
    if (!detail || !window.confirm("Generate password baru untuk akun ini?"))
      return;
    try {
      const result = await readAPI<{ password: string }>(
        await apiFetch(
          `/api/mgmp/groups/${detail.id}/accounts/${userId}/reset-password`,
          { method: "POST" }
        )
      );
      setAccounts(
        (current) =>
          current?.map((account) =>
            account.userId === userId
              ? { ...account, password: result.password }
              : account
          ) ?? null
      );
    } catch (e) {
      setError(e instanceof Error ? e.message : "Reset password gagal.");
    }
  };
  const setGroupStatus = async () => {
    if (!detail || !action) return;
    try {
      await readAPI(
        await apiFetch(`/api/mgmp/groups/${detail.id}/status`, {
          method: "POST",
          body: JSON.stringify({
            action,
            reason:
              action === "DEACTIVATE" &&
              reasonPreset &&
              reasonPreset !== "Lainnya"
                ? reasonPreset
                : reason,
            effectiveDate,
          }),
        })
      );
      setAction(null);
      setReason("");
      setReasonPreset("");
      await openDetail(detail.id);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Aksi gagal.");
    }
  };
  const removeMember = async () => {
    if (!detail || !memberAction) return;
    try {
      await readAPI(await apiFetch(`/api/mgmp/groups/${detail.id}/members/${memberAction.id}/status`, { method: "POST", body: JSON.stringify({ action: "REMOVE", reason: memberReason, note: memberNote, effectiveDate }) }));
      setMemberAction(null); setMemberReason(""); setMemberNote("");
      await openDetail(detail.id);
    } catch (e) { setError(e instanceof Error ? e.message : "Aksi anggota gagal."); }
  };
  return (
    <div className="min-h-full px-4 py-6 md:px-6">
      <PageTitle
        title="Monitoring MGMP & JARKOM SMK"
        description="Pantau pelaporan kegiatan bulanan tanpa memberi skor atau ranking."
      />
      <div className="grid gap-3 sm:grid-cols-2 lg:grid-cols-3 xl:grid-cols-6">
        {[
          ["Total Organisasi", "total"],
          ["Aktif", "active"],
          ["Tidak Ada Agenda", "noAgenda"],
          ["Belum Lengkap", "incomplete"],
          ["Belum Melapor", "missing"],
          ["Nonaktif", "inactive"],
        ].map(([label, key]) => (
          <Panel key={key}>
            <p className="text-xs text-slate-500">{label}</p>
            <p className="mt-1 text-2xl font-black text-slate-900">
              {data?.kpi[key] ?? 0}
            </p>
          </Panel>
        ))}
      </div>
      <Panel className="mt-5">
        <div className="grid gap-2 md:grid-cols-5">
          <input
            className={inputClass}
            placeholder="Cari nama/bidang organisasi"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <select className={inputClass} value={organizationType} onChange={(e) => setOrganizationType(e.target.value)}><option value="">Semua jenis</option><option value="MGMP">MGMP</option><option value="JARKOM">JARKOM</option></select>
          <input
            type="number"
            className={inputClass}
            value={year}
            onChange={(e) => setYear(Number(e.target.value))}
          />
          <select
            className={inputClass}
            value={month}
            onChange={(e) => setMonth(Number(e.target.value))}
          >
            {monthNames.map((m, i) => (
              <option key={m} value={i + 1}>
                {m}
              </option>
            ))}
          </select>
          <select
            className={inputClass}
            value={status}
            onChange={(e) => setStatus(e.target.value)}
          >
            <option value="">Semua status</option>
            {[
              "ADA_KEGIATAN",
              "TIDAK_ADA_AGENDA",
              "BELUM_MELAPOR",
              "LAPORAN_BELUM_LENGKAP",
              "ACTIVE",
              "INACTIVE",
            ].map((s) => (
              <option key={s}>{s}</option>
            ))}
          </select>
        </div>
        <div className="mt-4 overflow-x-auto">
          <table className="w-full min-w-[850px] text-left text-sm">
            <thead className="border-b text-xs text-slate-500">
              <tr>
                {[
                  "Jenis",
                  "Nama Organisasi",
                  "Bidang/Mapel",
                  "Kegiatan bulan",
                  "Total tahun",
                  "Aktivitas terakhir",
                  "Status bulan",
                  "Status MGMP",
                  "Detail",
                ].map((h) => (
                  <th key={h} className="px-3 py-3">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items.map((i) => (
                <tr key={i.id} className="border-b border-slate-100">
                  <td className="px-3 py-3"><StatusPill value={i.organizationType} /></td>
                  <td className="px-3 py-3 font-semibold">{i.name}</td>
                  <td className="px-3 py-3">{i.subject}</td>
                  <td className="px-3 py-3">{i.monthCount}</td>
                  <td className="px-3 py-3">{i.yearCount}</td>
                  <td className="px-3 py-3">
                    {i.lastActivity
                      ? new Date(i.lastActivity).toLocaleDateString("id-ID")
                      : "—"}
                  </td>
                  <td className="px-3 py-3">
                    <StatusPill value={i.monthStatus} />
                    {(i.missingStreak ?? 0) >= (data?.warningThreshold ?? 3) ? (
                      <p className="mt-1 text-xs font-semibold text-amber-700">
                        ⚠️ {i.missingStreak} bulan berturut-turut belum melapor
                      </p>
                    ) : null}
                  </td>
                  <td className="px-3 py-3">
                    <StatusPill value={i.groupStatus} />
                  </td>
                  <td className="px-3 py-3">
                    <button
                      className={secondaryButtonClass}
                      onClick={() => void openDetail(i.id)}
                    >
                      <Eye className="h-4 w-4" />
                      Detail
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
          {!items.length ? (
            <p className="p-8 text-center text-sm text-slate-500">
              Belum ada data yang sesuai filter.
            </p>
          ) : null}
        </div>
      </Panel>
      {error ? (
        <p className="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700">
          {error}
        </p>
      ) : null}
      {detail ? (
        <Modal
          wide
          title={`Detail ${detail.name}`}
          onClose={() => {
            setDetail(null);
            setAccounts(null);
          }}
        >
          <div className="flex flex-wrap items-center justify-between gap-3">
            <div>
              <p className="text-sm text-slate-600">
                {detail.subject} · {detail.region} ·{" "}
                {detail.period || "Periode belum diisi"}
              </p>
              <StatusPill value={detail.status} />
            </div>
            <div className="flex flex-wrap gap-2">
              <button
                className={secondaryButtonClass}
                onClick={() => void reveal()}
              >
                <Users className="h-4 w-4" />
                Kelola / Lihat Akun
              </button>
              {detail.status === "ACTIVE" ? (
                <button
                  className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white"
                  onClick={() => setAction("DEACTIVATE")}
                >
                  <Power className="mr-1 inline h-4 w-4" />
                  Nonaktifkan {detail.organizationType}
                </button>
              ) : (
                <button
                  className={buttonClass}
                  onClick={() => setAction("REACTIVATE")}
                >
                  <RotateCcw className="h-4 w-4" />
                  Aktifkan Kembali
                </button>
              )}
            </div>
          </div>
          <h3 className="mt-5 font-bold">Anggota ({detail.members.length})</h3>
          <div className="mt-2 max-h-48 overflow-auto rounded-xl border">
            <table className="w-full text-sm">
              <tbody>
                {detail.members.map((m) => (
                  <tr className="border-b" key={m.id}>
                    <td className="p-2 font-medium">{m.fullName}</td>
                    <td className="p-2">{m.position.replaceAll("_", " ")}</td>
                    <td className="p-2">{m.schoolName}</td>
                    <td className="p-2">{m.email}</td>
                    <td className="p-2"><StatusPill value={m.status} /></td>
                    <td className="p-2 text-xs">{m.attendanceStats ? `${m.attendanceStats.present}/${m.attendanceStats.activities} hadir` : "—"}</td>
                    <td className="p-2">{m.status === "AKTIF" ? <button className="text-xs font-semibold text-red-700" onClick={() => setMemberAction(m)}>Nonaktifkan / Keluarkan</button> : <button className="text-xs font-semibold text-blue-700" onClick={async () => { await readAPI(await apiFetch(`/api/mgmp/groups/${detail.id}/members/${m.id}/status`, { method: "POST", body: JSON.stringify({ action: "REACTIVATE" }) })); await openDetail(detail.id); }}>Aktifkan kembali</button>}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
          <h3 className="mt-5 font-bold">
            Detail Kegiatan ({detail.activities.length})
          </h3>
          <div className="mt-2 space-y-2">
            {detail.activities.map((a, i) => (
              <div key={a.id} className="rounded-xl bg-slate-50 p-3 text-sm">
                <b>
                  Kegiatan {i + 1}: {a.title}
                </b>
                <p>
                  {new Date(a.activityDate).toLocaleDateString("id-ID", {
                    dateStyle: "long",
                  })}{" "}
                  · {a.venue} · {a.participantCount} peserta
                </p>
                <p className="text-xs text-slate-600">Total anggota saat kegiatan: {a.activeMemberCount ?? a.participantCount} · Hadir: {a.participantCount} · Dengan keterangan: {(a.attendance ?? []).filter((row) => row.absenceType === "EXCUSED").length} · Tanpa keterangan: {(a.attendance ?? []).filter((row) => row.absenceType === "UNEXCUSED").length}</p>
                {(a.attendance ?? []).length ? <details className="mt-2"><summary className="cursor-pointer font-semibold text-blue-700">Lihat Kehadiran</summary><div className="mt-2 grid gap-2 sm:grid-cols-2"><input className={inputClass} placeholder="Cari nama atau sekolah" value={adminAttendanceSearch} onChange={(e) => setAdminAttendanceSearch(e.target.value)} /><select className={inputClass} value={attendanceFilter} onChange={(e) => setAttendanceFilter(e.target.value)}><option value="">Semua</option><option value="PRESENT">Hadir</option><option value="ABSENT">Tidak Hadir</option><option value="EXCUSED">Dengan Keterangan</option><option value="UNEXCUSED">Tanpa Keterangan</option></select></div><div className="mt-2 max-h-48 overflow-auto">{a.attendance!.filter((row) => (!attendanceFilter || row.status === attendanceFilter || row.absenceType === attendanceFilter) && `${row.fullName} ${row.schoolName}`.toLowerCase().includes(adminAttendanceSearch.toLowerCase())).map((row) => <div key={row.memberId} className="grid grid-cols-[1fr_auto] gap-2 border-b py-2 text-xs"><span><b>{row.fullName}</b><br />{row.schoolName}</span><span>{row.status === "PRESENT" ? "Hadir" : row.absenceType === "EXCUSED" ? `Tidak hadir — ${row.absenceNote}` : "Tidak hadir tanpa keterangan"}</span></div>)}</div></details> : null}
                <div className="mt-1 flex gap-3">
                  <a
                    className="text-blue-700"
                    target="_blank"
                    href={a.attendanceUrl}
                  >
                    Link Absensi
                  </a>
                  <a
                    className="text-blue-700"
                    target="_blank"
                    href={a.documentationUrl}
                  >
                    Link Dokumentasi
                  </a>
                </div>
              </div>
            ))}
            {!detail.activities.length ? (
              <p className="text-sm text-slate-500">Belum ada kegiatan.</p>
            ) : null}
          </div>
          <h3 className="mt-5 font-bold">Histori MGMP</h3>
          <div className="mt-2 max-h-52 space-y-2 overflow-auto">
            {(detail.history ?? []).map((item, index) => (
              <div
                key={`${item.createdAt}-${index}`}
                className="rounded-xl border border-slate-200 p-3 text-sm"
              >
                <b>{item.eventType.replaceAll("_", " ")}</b>
                <p className="text-slate-600">
                  {item.actor} ·{" "}
                  {new Date(item.createdAt).toLocaleString("id-ID")}
                </p>
                {item.reason ? <p>Alasan: {item.reason}</p> : null}
              </div>
            ))}
          </div>
        </Modal>
      ) : null}
      {accounts ? (
        <Modal
          title="Akun Pengurus MGMP — Mode Demo"
          onClose={() => setAccounts(null)}
        >
          <p className="mb-3 rounded-lg bg-amber-50 p-3 text-xs text-amber-800">
            Credential hanya dapat dibuka admin berwenang. Integrasi email belum
            aktif.
          </p>
          <div className="space-y-3">
            {accounts.map((a) => (
              <div key={a.userId} className="rounded-xl border p-3 text-sm">
                <b>
                  {a.name} — {a.position.replaceAll("_", " ")}
                </b>
                <p>
                  Username: <code>{a.username}</code>
                </p>
                <p>
                  Password: <code>{a.password}</code>
                </p>
                <button
                  className={`${secondaryButtonClass} mt-2`}
                  onClick={() =>
                    navigator.clipboard.writeText(
                      `${a.username}\n${a.password}`
                    )
                  }
                >
                  Salin
                </button>
                <button
                  className={`${secondaryButtonClass} mt-2 ml-2`}
                  onClick={() => void resetPassword(a.userId)}
                >
                  Reset Password
                </button>
              </div>
            ))}
          </div>
        </Modal>
      ) : null}
      {memberAction ? <Modal title={`Keluarkan Anggota · ${memberAction.fullName}`} onClose={() => setMemberAction(null)}><div className="space-y-3"><label className="block text-sm font-medium">Alasan<select className={`${inputClass} mt-1`} value={memberReason} onChange={(e) => setMemberReason(e.target.value)}><option value="">Pilih alasan</option>{["Tidak aktif","Tidak mengikuti kegiatan","Pindah tugas/sekolah","Mengundurkan diri","Keputusan Cabang Dinas","Lainnya"].map((item) => <option key={item}>{item}</option>)}</select></label><label className="block text-sm font-medium">Keterangan<textarea className={`${inputClass} mt-1`} rows={4} value={memberNote} onChange={(e) => setMemberNote(e.target.value)} /></label><label className="block text-sm font-medium">Tanggal efektif<input type="date" className={`${inputClass} mt-1`} value={effectiveDate} onChange={(e) => setEffectiveDate(e.target.value)} /></label><p className="text-xs text-slate-500">Anggota, histori, akun, kegiatan, dan absensi lama tidak akan dihapus.</p><button className={`${buttonClass} w-full`} disabled={!memberReason || !memberNote.trim() || !effectiveDate} onClick={() => void removeMember()}>Konfirmasi Keputusan</button></div></Modal> : null}
      {action ? (
        <Modal
          title={
            action === "DEACTIVATE"
              ? "Nonaktifkan Organisasi"
              : "Aktifkan Kembali Organisasi"
          }
          onClose={() => setAction(null)}
        >
          {action === "DEACTIVATE" ? (
            <label className="block text-sm font-medium">
              Alasan
              <select
                className={`${inputClass} mt-1`}
                value={reasonPreset}
                onChange={(e) => setReasonPreset(e.target.value)}
              >
                <option value="">Pilih alasan</option>
                {deactivationReasons.map((item) => (
                  <option key={item}>{item}</option>
                ))}
              </select>
            </label>
          ) : null}
          {action === "REACTIVATE" || reasonPreset === "Lainnya" ? (
            <label className="mt-3 block text-sm font-medium">
              Keterangan wajib
              <textarea
                className={`${inputClass} mt-1`}
                rows={4}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </label>
          ) : null}
          {action === "DEACTIVATE" ? (
            <label className="mt-3 block text-sm font-medium">
              Tanggal efektif
              <input
                type="date"
                className={`${inputClass} mt-1`}
                value={effectiveDate}
                onChange={(e) => setEffectiveDate(e.target.value)}
              />
            </label>
          ) : null}
          <p className="mt-3 text-xs text-slate-500">
            Histori, anggota, akun, dan kegiatan lama tidak akan dihapus.
          </p>
          <button
            className={`${buttonClass} mt-4 w-full`}
            disabled={
              action === "DEACTIVATE"
                ? !reasonPreset ||
                  (reasonPreset === "Lainnya" && !reason.trim())
                : !reason.trim()
            }
            onClick={() => void setGroupStatus()}
          >
            Konfirmasi
          </button>
        </Modal>
      ) : null}
    </div>
  );
}

export function MGMPDashboard() {
  const { user, status } = useAuth();
  if (status === "loading")
    return <div className="p-8 text-sm text-slate-500">Memuat...</div>;
  return user?.role === "USER_MGMP" ? <UserWorkspace /> : <AdminMonitoring />;
}
