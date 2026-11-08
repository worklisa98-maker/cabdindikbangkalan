"use client";

import { useEffect, useMemo, useState } from "react";
import Link from "next/link";
import {
  Check,
  Copy,
  Eye,
  History,
  Settings,
  UserRoundCheck,
  X,
} from "lucide-react";
import { useAuth } from "@/lib/auth/client";
import { apiFetch } from "@/lib/api/client";
import {
  Modal,
  PageTitle,
  Panel,
  PublicMGMPNavigation,
  StatusPill,
  buttonClass,
  inputClass,
  readAPI,
  secondaryButtonClass,
} from "./shared";

type Registration = {
  id: number;
  registrationId: string;
  organizationType: "MGMP" | "JARKOM";
  name: string;
  subject: string;
  region: string;
  period: string;
  status: string;
  delegatedAdminId?: number;
  delegatedAdminName?: string;
  submittedAt: string;
  memberCount: number;
};
type Detail = Registration & {
  revisionNote?: string;
  rejectionReason?: string;
  potentialGroups: {
    id: number;
    name: string;
    subject: string;
    region: string;
    status: string;
  }[];
  members: {
    id: number;
    fullName: string;
    schoolName: string;
    position: string;
    email: string;
    whatsapp: string;
    status: string;
  }[];
};
type Delegate = { id: number; username: string; name: string };
type Credential = {
  userId: number;
  name: string;
  position: string;
  username: string;
  password: string;
};

export function MGMPRegistrations() {
  const { user } = useAuth();
  const [items, setItems] = useState<Registration[]>([]);
  const [delegates, setDelegates] = useState<Delegate[]>([]);
  const [detail, setDetail] = useState<Detail | null>(null);
  const [delegateId, setDelegateId] = useState("");
  const [existingGroupId, setExistingGroupId] = useState("");
  const [decision, setDecision] = useState<
    "APPROVE" | "REVISION" | "REJECT" | null
  >(null);
  const [reason, setReason] = useState("");
  const [credentials, setCredentials] = useState<Credential[] | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [organizationType, setOrganizationType] = useState("");
  const load = () =>
    apiFetch("/api/mgmp/registrations")
      .then(readAPI<Registration[]>)
      .then(setItems)
      .catch((e) => setError(e.message));
  useEffect(() => {
    void load();
    if (user?.role === "ADMIN_MASTER")
      void apiFetch("/api/mgmp/delegates")
        .then(readAPI<Delegate[]>)
        .then(setDelegates);
  }, [user?.role]);
  const open = async (id: number) => {
    try {
      setDetail(await readAPI(await apiFetch(`/api/mgmp/registrations/${id}`)));
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal memuat detail.");
    }
  };
  const delegate = async () => {
    if (!detail || !delegateId) return;
    setBusy(true);
    try {
      await readAPI(
        await apiFetch(`/api/mgmp/registrations/${detail.id}/delegate`, {
          method: "PUT",
          body: JSON.stringify({ adminId: Number(delegateId) }),
        })
      );
      setDetail(null);
      await load();
    } catch (e) {
      setError(e instanceof Error ? e.message : "Delegasi gagal.");
    } finally {
      setBusy(false);
    }
  };
  const decide = async () => {
    if (!detail || !decision) return;
    setBusy(true);
    try {
      const result = await readAPI<{ credentials?: Credential[] }>(
        await apiFetch(`/api/mgmp/registrations/${detail.id}/decision`, {
          method: "POST",
          body: JSON.stringify({
            action: decision,
            reason,
            existingGroupId: existingGroupId ? Number(existingGroupId) : null,
          }),
        })
      );
      setDecision(null);
      setReason("");
      setExistingGroupId("");
      setDetail(null);
      if (result.credentials) setCredentials(result.credentials);
      await load();
    } catch (e) {
      const raw = e instanceof Error ? e.message : "";
      setError(
        raw === "FORBIDDEN"
          ? "Anda tidak memiliki akses untuk memproses pendaftaran ini."
          : raw.includes("sudah diproses") || raw === "CONFLICT"
            ? "Pendaftaran ini sudah diproses dan keputusan tersebut tidak dapat diulang."
            : decision === "REVISION"
              ? "Gagal meminta perbaikan pendaftaran. Silakan coba kembali."
              : decision === "REJECT"
                ? "Gagal menolak pendaftaran. Silakan coba kembali."
                : "Gagal menyetujui pendaftaran. Silakan coba kembali."
      );
    } finally {
      setBusy(false);
    }
  };
  return (
    <div className="min-h-full px-4 py-6 md:px-6">
      <PageTitle
        title="Pendaftaran / Verval MGMP & JARKOM"
        description="Admin Master mendelegasikan pendaftaran; Admin Cabdin melakukan verifikasi dan keputusan."
      />
      <Panel>
        <div className="mb-4 flex gap-2">
          {["", "MGMP", "JARKOM"].map((type) => (
            <button
              key={type || "ALL"}
              className={
                organizationType === type ? buttonClass : secondaryButtonClass
              }
              onClick={() => setOrganizationType(type)}
            >
              {type || "Semua"}
            </button>
          ))}
        </div>
        <div className="overflow-x-auto">
          <table className="w-full min-w-[760px] text-left text-sm">
            <thead className="border-b text-xs text-slate-500">
              <tr>
                {[
                  "ID Registrasi",
                  "Jenis",
                  "Nama Organisasi",
                  "Bidang",
                  "Anggota",
                  "Delegasi",
                  "Status",
                  "Aksi",
                ].map((h) => (
                  <th key={h} className="p-3">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {items
                .filter(
                  (item) =>
                    !organizationType ||
                    item.organizationType === organizationType
                )
                .map((item) => (
                  <tr key={item.id} className="border-b border-slate-100">
                    <td className="p-3 font-mono text-xs">
                      {item.registrationId}
                    </td>
                    <td className="p-3">
                      <StatusPill value={item.organizationType} />
                    </td>
                    <td className="p-3 font-semibold">{item.name}</td>
                    <td className="p-3">{item.subject}</td>
                    <td className="p-3">{item.memberCount}</td>
                    <td className="p-3">
                      {item.delegatedAdminName || "Belum didelegasikan"}
                    </td>
                    <td className="p-3">
                      <StatusPill value={item.status} />
                    </td>
                    <td className="p-3">
                      <button
                        className={secondaryButtonClass}
                        onClick={() => void open(item.id)}
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
            <p className="p-10 text-center text-sm text-slate-500">
              Belum ada pendaftaran dalam scope Anda.
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
          title={`${detail.registrationId} · ${detail.name}`}
          onClose={() => setDetail(null)}
        >
          <div className="grid gap-3 md:grid-cols-3">
            <div>
              <p className="text-xs text-slate-500">Bidang/Mapel</p>
              <b>{detail.subject}</b>
            </div>
            <div>
              <p className="text-xs text-slate-500">Wilayah</p>
              <b>{detail.region}</b>
            </div>
            <div>
              <p className="text-xs text-slate-500">Status</p>
              <StatusPill value={detail.status} />
            </div>
          </div>
          <h3 className="mt-5 font-bold">Ketua & Sekretaris/Wakil</h3>
          <div className="mt-2 grid gap-3 md:grid-cols-2">
            {detail.members
              .filter((m) => ["KETUA", "SEKRETARIS_WAKIL"].includes(m.position))
              .map((m) => (
                <div key={m.id} className="rounded-xl bg-blue-50 p-3 text-sm">
                  <b>{m.position.replaceAll("_", " ")}</b>
                  <p>{m.fullName}</p>
                  <p>{m.schoolName}</p>
                  <p>
                    {m.email} · {m.whatsapp}
                  </p>
                </div>
              ))}
          </div>
          <details className="mt-4 rounded-xl border p-3">
            <summary className="cursor-pointer font-semibold">
              Semua anggota ({detail.members.length})
            </summary>
            <div className="mt-2 max-h-52 overflow-auto">
              {detail.members.map((m) => (
                <div key={m.id} className="border-t py-2 text-sm">
                  <b>{m.fullName}</b> · {m.position.replaceAll("_", " ")} ·{" "}
                  {m.schoolName}
                </div>
              ))}
            </div>
          </details>
          {detail.status === "REVISION_REQUIRED" && detail.revisionNote ? (
            <div className="mt-4 rounded-xl border border-orange-200 bg-orange-50 p-3 text-sm text-orange-900">
              <b>Catatan perbaikan</b>
              <p className="mt-1 whitespace-pre-wrap">{detail.revisionNote}</p>
            </div>
          ) : null}
          {user?.role === "ADMIN_MASTER" &&
          ["PENDING", "REVISION_REQUIRED"].includes(detail.status) ? (
            <div className="mt-5 rounded-xl border border-blue-200 bg-blue-50 p-4">
              <label className="text-sm font-semibold">
                Delegasikan ke Admin Cabdin
                <select
                  className={`${inputClass} mt-2`}
                  value={delegateId}
                  onChange={(e) => setDelegateId(e.target.value)}
                >
                  <option value="">Pilih admin</option>
                  {delegates.map((d) => (
                    <option key={d.id} value={d.id}>
                      {d.name} ({d.username})
                    </option>
                  ))}
                </select>
              </label>
              <button
                className={`${buttonClass} mt-3`}
                disabled={!delegateId || busy}
                onClick={() => void delegate()}
              >
                <UserRoundCheck className="h-4 w-4" />
                Simpan Delegasi
              </button>
            </div>
          ) : null}
          {user?.role !== "ADMIN_MASTER" &&
          detail.delegatedAdminId === user?.id &&
          ["PENDING", "REVISION_REQUIRED"].includes(detail.status) ? (
            <div className="mt-5 flex flex-wrap justify-end gap-2">
              {detail.status === "PENDING" ? (
                <button
                  className={secondaryButtonClass}
                  onClick={() => setDecision("REVISION")}
                >
                  Minta Perbaikan
                </button>
              ) : null}
              <button
                className="rounded-lg bg-red-600 px-4 py-2 text-sm font-semibold text-white"
                onClick={() => setDecision("REJECT")}
              >
                <X className="mr-1 inline h-4 w-4" />
                Tolak
              </button>
              {detail.status === "PENDING" ? (
                <button
                  className={buttonClass}
                  onClick={() => setDecision("APPROVE")}
                >
                  <Check className="h-4 w-4" />
                  Terima
                </button>
              ) : null}
            </div>
          ) : null}
        </Modal>
      ) : null}
      {decision ? (
        <Modal
          title={
            decision === "APPROVE" ? "Setujui Pendaftaran" : "Catatan Keputusan"
          }
          onClose={() => setDecision(null)}
        >
          {decision === "APPROVE" ? (
            <div>
              <p className="text-sm text-slate-600">
                Sistem akan membuat entitas organisasi aktif dan dua akun
                pengurus dari data Ketua serta Sekretaris/Wakil.
              </p>
              {detail?.potentialGroups?.length ? (
                <label className="mt-4 block text-sm font-medium">
                  Potensi duplikat ditemukan
                  <select
                    className={`${inputClass} mt-1`}
                    value={existingGroupId}
                    onChange={(e) => setExistingGroupId(e.target.value)}
                  >
                    <option value="">Buat organisasi baru</option>
                    {detail.potentialGroups.map((group) => (
                      <option key={group.id} value={group.id}>
                        Gabungkan ke: {group.name} · {group.subject} (
                        {group.status})
                      </option>
                    ))}
                  </select>
                  <span className="text-xs text-slate-500">
                    Admin menentukan apakah pendaftaran digabung atau
                    benar-benar entitas baru.
                  </span>
                </label>
              ) : null}
            </div>
          ) : (
            <label className="block text-sm font-medium">
              Alasan/keterangan wajib
              <textarea
                className={`${inputClass} mt-2`}
                rows={4}
                value={reason}
                onChange={(e) => setReason(e.target.value)}
              />
            </label>
          )}
          <button
            className={`${buttonClass} mt-4 w-full`}
            disabled={busy || (decision !== "APPROVE" && !reason.trim())}
            onClick={() => void decide()}
          >
            {busy ? "Memproses..." : "Konfirmasi"}
          </button>
        </Modal>
      ) : null}
      {credentials ? (
        <Modal
          title="Pendaftaran organisasi berhasil disetujui"
          onClose={() => setCredentials(null)}
        >
          <p className="mb-3 rounded-lg bg-amber-50 p-3 text-xs text-amber-800">
            Simpan credential untuk demo. Credential dapat dibuka kembali dari
            Detail organisasi → Kelola / Lihat Akun.
          </p>
          <div className="space-y-3">
            {credentials.map((c) => (
              <div key={c.userId} className="rounded-xl border p-3 text-sm">
                <b>
                  {c.name} — {c.position.replaceAll("_", " ")}
                </b>
                <p>
                  Username: <code>{c.username}</code>
                </p>
                <p>
                  Password: <code>{c.password}</code>
                </p>
                <button
                  className={`${secondaryButtonClass} mt-2`}
                  onClick={() =>
                    navigator.clipboard.writeText(
                      `${c.username}\n${c.password}`
                    )
                  }
                >
                  <Copy className="h-4 w-4" />
                  Salin
                </button>
              </div>
            ))}
          </div>
        </Modal>
      ) : null}
    </div>
  );
}

type HistoryItem = {
  id: number;
  eventType: string;
  mgmpName: string;
  actor: string;
  oldStatus?: string;
  newStatus?: string;
  reason?: string;
  details: unknown;
  createdAt: string;
};
export function MGMPHistory() {
  const [items, setItems] = useState<HistoryItem[]>([]);
  const [query, setQuery] = useState("");
  const [type, setType] = useState("");
  const [error, setError] = useState("");
  useEffect(() => {
    void apiFetch("/api/mgmp/history")
      .then(readAPI<HistoryItem[]>)
      .then(setItems)
      .catch((e) => setError(e.message));
  }, []);
  const shown = useMemo(
    () =>
      items.filter(
        (i) =>
          (!query ||
            `${i.mgmpName} ${i.actor}`
              .toLowerCase()
              .includes(query.toLowerCase())) &&
          (!type || i.eventType.includes(type))
      ),
    [items, query, type]
  );
  return (
    <div className="min-h-full px-4 py-6 md:px-6">
      <PageTitle
        title="Histori MGMP & JARKOM"
        description="Jejak kegiatan dan tindakan administratif penting disimpan secara append-only."
      />
      <Panel>
        <div className="grid gap-2 md:grid-cols-3">
          <input
            className={inputClass}
            placeholder="Cari organisasi atau pelaksana"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
          <select
            className={inputClass}
            value={type}
            onChange={(e) => setType(e.target.value)}
          >
            <option value="">Semua tipe aktivitas</option>
            <option value="REGISTRATION">Registrasi / Verval</option>
            <option value="ACTIVITY">Kegiatan</option>
            <option value="GROUP_">Aktivasi organisasi</option>
            <option value="ACCOUNT">Akun</option>
          </select>
        </div>
        <div className="mt-4 space-y-2">
          {shown.map((i) => (
            <div
              key={i.id}
              className="flex gap-3 rounded-xl border border-slate-200 p-3"
            >
              <History className="mt-0.5 h-5 w-5 shrink-0 text-blue-600" />
              <div className="min-w-0">
                <div className="flex flex-wrap items-center gap-2">
                  <b className="text-sm">{i.eventType.replaceAll("_", " ")}</b>
                  {i.newStatus ? <StatusPill value={i.newStatus} /> : null}
                </div>
                <p className="text-sm text-slate-600">
                  {i.mgmpName} · oleh {i.actor}
                </p>
                {i.reason ? (
                  <p className="mt-1 text-sm text-slate-700">
                    Alasan: {i.reason}
                  </p>
                ) : null}
                <p className="mt-1 text-xs text-slate-400">
                  {new Date(i.createdAt).toLocaleString("id-ID")}
                </p>
              </div>
            </div>
          ))}
          {!shown.length ? (
            <p className="p-8 text-center text-sm text-slate-500">
              Belum ada histori.
            </p>
          ) : null}
        </div>
      </Panel>
      {error ? <p className="mt-4 text-red-700">{error}</p> : null}
    </div>
  );
}

type SettingsData = {
  active_period?: number;
  registration_enabled?: boolean;
  missing_report_warning_months?: number;
  positions?: string[];
  deactivation_reasons?: string[];
};
export function MGMPSettings() {
  const [data, setData] = useState<SettingsData>({});
  const [message, setMessage] = useState("");
  const [error, setError] = useState("");
  useEffect(() => {
    void apiFetch("/api/mgmp/settings")
      .then(readAPI<SettingsData>)
      .then(setData)
      .catch((e) => setError(e.message));
  }, []);
  const save = async () => {
    try {
      await readAPI(
        await apiFetch("/api/mgmp/settings", {
          method: "PUT",
          body: JSON.stringify(data),
        })
      );
      setMessage("Settings MGMP & JARKOM berhasil disimpan.");
    } catch (e) {
      setError(e instanceof Error ? e.message : "Gagal menyimpan.");
    }
  };
  return (
    <div className="mx-auto min-h-full max-w-4xl px-4 py-6 md:px-6">
      <PageTitle
        title="Settings MGMP & JARKOM SMK"
        description="Konfigurasi administratif modul. Hanya Admin Master yang dapat mengubahnya."
      />
      <Panel>
        <div className="mb-4 flex items-center gap-2">
          <Settings className="h-5 w-5 text-blue-600" />
          <h2 className="font-bold">Pengaturan Umum</h2>
        </div>
        <div className="grid gap-4 md:grid-cols-2">
          <label className="text-sm font-medium">
            Tahun/periode aktif
            <input
              type="number"
              className={`${inputClass} mt-1`}
              value={data.active_period ?? new Date().getFullYear()}
              onChange={(e) =>
                setData({ ...data, active_period: Number(e.target.value) })
              }
            />
          </label>
          <label className="text-sm font-medium">
            Indikator belum melapor beruntun (bulan)
            <input
              type="number"
              min={1}
              className={`${inputClass} mt-1`}
              value={data.missing_report_warning_months ?? 3}
              onChange={(e) =>
                setData({
                  ...data,
                  missing_report_warning_months: Number(e.target.value),
                })
              }
            />
          </label>
          <label className="flex items-center gap-3 rounded-xl border p-4 text-sm font-medium">
            <input
              type="checkbox"
              checked={data.registration_enabled ?? true}
              onChange={(e) =>
                setData({ ...data, registration_enabled: e.target.checked })
              }
            />
            Aktifkan registrasi publik
          </label>
        </div>
        <div className="mt-5 rounded-xl border border-dashed border-slate-300 p-4">
          <b className="text-sm">Email & Notifikasi</b>
          <p className="mt-1 text-sm text-slate-500">
            Disiapkan sebagai tab lanjutan. SMTP dan pengiriman email belum
            termasuk MVP.
          </p>
        </div>
        {message ? (
          <p className="mt-4 rounded-lg bg-emerald-50 p-3 text-sm text-emerald-700">
            {message}
          </p>
        ) : null}
        {error ? (
          <p className="mt-4 rounded-lg bg-red-50 p-3 text-sm text-red-700">
            {error}
          </p>
        ) : null}
        <button className={`${buttonClass} mt-5`} onClick={() => void save()}>
          Simpan Settings
        </button>
      </Panel>
    </div>
  );
}

export function MGMPStatusCheck() {
  type RevisionMember = {
    id: number;
    fullName: string;
    identityNumber: string;
    schoolId: number | null;
    schoolName: string;
    position: string;
    email: string;
    whatsapp: string;
    status: string;
  };
  type StatusResult = {
    registrationId: string;
    organizationType: "MGMP" | "JARKOM";
    name: string;
    subject: string;
    region: string;
    managementPeriod?: string;
    status: string;
    statusLabel: string;
    note?: string;
    submittedAt: string;
    timeline: {
      event: string;
      label: string;
      note?: string;
      createdAt: string;
    }[];
    credentials: {
      name: string;
      position: string;
      username: string;
      password: string;
    }[];
    members: RevisionMember[];
  };
  const [code, setCode] = useState("");
  const [result, setResult] = useState<StatusResult | null>(null);
  const [refs, setRefs] = useState<{ schools: { id: number; name: string }[] }>(
    { schools: [] }
  );
  const [revisionDraft, setRevisionDraft] = useState<StatusResult | null>(null);
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [copied, setCopied] = useState("");
  useEffect(() => {
    void apiFetch("/api/public/mgmp/references")
      .then(readAPI<{ schools: { id: number; name: string }[] }>)
      .then(setRefs)
      .catch(() => undefined);
  }, []);
  const check = async () => {
    setError("");
    setResult(null);
    setCopied("");
    setBusy(true);
    try {
      const loaded = await readAPI<StatusResult>(
        await apiFetch(
          `/api/public/mgmp/registrations/${encodeURIComponent(code)}`
        )
      );
      setResult(loaded);
      setRevisionDraft(loaded.status === "REVISION_REQUIRED" ? loaded : null);
    } catch (requestError) {
      setError(
        requestError instanceof Error
          ? requestError.message
          : "ID registrasi tidak ditemukan."
      );
    } finally {
      setBusy(false);
    }
  };
  const updateRevisionMember = (
    index: number,
    patch: Partial<RevisionMember>
  ) =>
    setRevisionDraft((current) =>
      current
        ? {
            ...current,
            members: current.members.map((member, memberIndex) =>
              memberIndex === index ? { ...member, ...patch } : member
            ),
          }
        : current
    );
  const resubmit = async () => {
    if (!revisionDraft) return;
    setBusy(true);
    setError("");
    try {
      await readAPI(
        await apiFetch(
          `/api/public/mgmp/registrations/${encodeURIComponent(revisionDraft.registrationId)}/resubmit`,
          {
            method: "PUT",
            body: JSON.stringify({
              organizationType: revisionDraft.organizationType,
              name: revisionDraft.name,
              subject: revisionDraft.subject,
              region: revisionDraft.region,
              managementPeriod: revisionDraft.managementPeriod ?? "",
              members: revisionDraft.members,
            }),
          }
        )
      );
      setCode(revisionDraft.registrationId);
      setRevisionDraft(null);
      const refreshed = await readAPI<StatusResult>(
        await apiFetch(
          `/api/public/mgmp/registrations/${encodeURIComponent(revisionDraft.registrationId)}`
        )
      );
      setResult(refreshed);
    } catch (requestError) {
      const raw = requestError instanceof Error ? requestError.message : "";
      setError(
        raw.includes("tidak sedang menunggu") || raw === "CONFLICT"
          ? "Pendaftaran ini sudah diproses dan tidak dapat dikirim ulang."
          : raw ||
              "Gagal mengirim ulang perbaikan pendaftaran. Silakan coba kembali."
      );
    } finally {
      setBusy(false);
    }
  };
  const copy = async (value: string, label: string) => {
    try {
      await navigator.clipboard.writeText(value);
      setCopied(`${label} disalin`);
      window.setTimeout(() => setCopied(""), 2500);
    } catch {
      setCopied("Gagal menyalin. Silakan salin secara manual.");
    }
  };
  const formatDate = (value: string) =>
    new Intl.DateTimeFormat("id-ID", {
      dateStyle: "long",
      timeStyle: "short",
      timeZone: "Asia/Jakarta",
    }).format(new Date(value));
  const positionLabel = (position: string) =>
    position === "KETUA" ? "Ketua" : "Sekretaris/Wakil";
  const summary = result
    ? [
        ["ID Registrasi", result.registrationId],
        ["Jenis Organisasi", result.organizationType],
        [`Nama ${result.organizationType}`, result.name],
        ["Bidang / Mapel", result.subject],
        ["Wilayah", result.region],
        ...(result.managementPeriod
          ? [["Periode kepengurusan", result.managementPeriod]]
          : []),
        ["Tanggal pendaftaran", formatDate(result.submittedAt)],
      ]
    : [];
  return (
    <div className="mx-auto min-h-full max-w-4xl px-4 py-6 md:px-6">
      <PublicMGMPNavigation active="status" />
      <PageTitle
        title="Cek Pendaftaran MGMP & JARKOM"
        description="Lacak proses pendaftaran dengan ID registrasi yang diperoleh setelah pengajuan."
      />
      <Panel>
        <form
          className="flex flex-col gap-2 sm:flex-row"
          onSubmit={(event) => {
            event.preventDefault();
            void check();
          }}
        >
          <input
            className={inputClass}
            placeholder="MGMP-2026-0001 atau JARKOM-2026-0001"
            aria-label="ID registrasi MGMP atau JARKOM"
            value={code}
            onChange={(e) => setCode(e.target.value.toUpperCase())}
          />
          <button
            type="submit"
            className={buttonClass}
            disabled={!code.trim() || busy}
          >
            {busy ? "Memeriksa..." : "Cek Pendaftaran"}
          </button>
        </form>
        {error ? (
          <p
            role="alert"
            className="mt-4 rounded-lg border border-red-200 bg-red-50 p-3 text-sm text-red-700"
          >
            {error}
          </p>
        ) : null}
      </Panel>

      {result ? (
        <div className="mt-5 space-y-5" aria-live="polite">
          <Panel>
            <div className="flex flex-col justify-between gap-3 sm:flex-row sm:items-start">
              <div>
                <p className="text-xs font-bold tracking-wider text-slate-500 uppercase">
                  Ringkasan Pendaftaran
                </p>
                <h2 className="mt-1 text-xl font-bold text-slate-900">
                  {result.name}
                </h2>
              </div>
              <div className="flex flex-col items-start sm:items-end">
                <StatusPill value={result.status} />
              </div>
            </div>
            <dl className="mt-5 grid gap-x-8 gap-y-4 border-t border-slate-100 pt-5 sm:grid-cols-2">
              {summary.map(([label, value]) => (
                <div key={label}>
                  <dt className="text-xs font-semibold text-slate-500 uppercase">
                    {label}
                  </dt>
                  <dd className="mt-1 text-sm font-medium text-slate-900">
                    {value}
                  </dd>
                </div>
              ))}
            </dl>
          </Panel>

          <Panel>
            <h2 className="font-bold text-slate-900">Timeline Proses</h2>
            {result.timeline.length ? (
              <ol className="mt-5 space-y-0">
                {result.timeline.map((item, index) => (
                  <li
                    key={`${item.event}-${item.createdAt}-${index}`}
                    className="relative grid grid-cols-[1.25rem_1fr] gap-3 pb-6 last:pb-0"
                  >
                    {index < result.timeline.length - 1 ? (
                      <span className="absolute top-5 bottom-0 left-[0.5625rem] w-px bg-blue-200" />
                    ) : null}
                    <span className="relative mt-1 h-5 w-5 rounded-full border-4 border-blue-100 bg-blue-700" />
                    <div>
                      <p className="font-semibold text-slate-900">
                        {item.label}
                      </p>
                      {item.note ? (
                        <p className="mt-1 text-sm whitespace-pre-wrap text-slate-600">
                          {item.note}
                        </p>
                      ) : null}
                      <time className="text-sm text-slate-500">
                        {formatDate(item.createdAt)} WIB
                      </time>
                    </div>
                  </li>
                ))}
              </ol>
            ) : (
              <p className="mt-3 text-sm text-slate-500">
                Riwayat proses belum tersedia.
              </p>
            )}
          </Panel>

          {result.status === "REVISION_REQUIRED" ? (
            <section className="rounded-2xl border border-orange-200 bg-orange-50 p-5 shadow-sm">
              <h2 className="font-bold text-orange-900">
                Pendaftaran Memerlukan Perbaikan
              </h2>
              <p className="mt-3 text-sm font-semibold text-orange-900">
                Catatan Verifikator:
              </p>
              <p className="mt-1 text-sm whitespace-pre-wrap text-orange-800">
                {result.note || "Tidak ada catatan tambahan."}
              </p>
              {revisionDraft ? (
                <div className="mt-5 space-y-4 border-t border-orange-200 pt-4">
                  <h3 className="font-bold text-slate-900">
                    Perbaiki Data Pendaftaran
                  </h3>
                  <div className="grid gap-3 sm:grid-cols-2">
                    {(
                      ["name", "subject", "region", "managementPeriod"] as const
                    ).map((key) => (
                      <label
                        key={key}
                        className="text-sm font-medium text-slate-700"
                      >
                        {
                          {
                            name: "Nama organisasi",
                            subject: "Bidang / Mapel",
                            region: "Wilayah",
                            managementPeriod: "Periode kepengurusan",
                          }[key]
                        }
                        <input
                          className={`${inputClass} mt-1`}
                          value={revisionDraft[key] ?? ""}
                          onChange={(event) =>
                            setRevisionDraft({
                              ...revisionDraft,
                              [key]: event.target.value,
                            })
                          }
                        />
                      </label>
                    ))}
                  </div>
                  <div className="space-y-3">
                    {revisionDraft.members.map((member, index) => (
                      <div
                        key={member.id}
                        className="rounded-xl border border-orange-200 bg-white p-3"
                      >
                        <div className="grid gap-2 md:grid-cols-2">
                          <input
                            className={inputClass}
                            value={member.fullName}
                            aria-label={`Nama anggota ${index + 1}`}
                            onChange={(event) =>
                              updateRevisionMember(index, {
                                fullName: event.target.value,
                              })
                            }
                          />
                          <input
                            className={inputClass}
                            value={member.identityNumber}
                            placeholder="NIP/NIK (opsional)"
                            onChange={(event) =>
                              updateRevisionMember(index, {
                                identityNumber: event.target.value,
                              })
                            }
                          />
                          <select
                            className={inputClass}
                            value={member.schoolId ?? ""}
                            onChange={(event) => {
                              const school = refs.schools.find(
                                (item) => item.id === Number(event.target.value)
                              );
                              updateRevisionMember(index, {
                                schoolId: school?.id ?? null,
                                schoolName: school?.name ?? "",
                              });
                            }}
                          >
                            <option value="">Pilih sekolah</option>
                            {refs.schools.map((school) => (
                              <option key={school.id} value={school.id}>
                                {school.name}
                              </option>
                            ))}
                          </select>
                          <select
                            className={inputClass}
                            value={member.position}
                            onChange={(event) =>
                              updateRevisionMember(index, {
                                position: event.target.value,
                              })
                            }
                          >
                            <option value="KETUA">Ketua</option>
                            <option value="SEKRETARIS_WAKIL">
                              Sekretaris/Wakil
                            </option>
                            <option value="BENDAHARA">Bendahara</option>
                            <option value="ANGGOTA">Anggota</option>
                            <option value="LAINNYA">Lainnya</option>
                          </select>
                          <input
                            className={inputClass}
                            type="email"
                            value={member.email}
                            placeholder="Email"
                            onChange={(event) =>
                              updateRevisionMember(index, {
                                email: event.target.value,
                              })
                            }
                          />
                          <input
                            className={inputClass}
                            value={member.whatsapp}
                            placeholder="WhatsApp"
                            onChange={(event) =>
                              updateRevisionMember(index, {
                                whatsapp: event.target.value,
                              })
                            }
                          />
                          <select
                            className={inputClass}
                            value={member.status}
                            onChange={(event) =>
                              updateRevisionMember(index, {
                                status: event.target.value,
                              })
                            }
                          >
                            <option value="AKTIF">Aktif</option>
                            <option value="NONAKTIF">Nonaktif</option>
                          </select>
                        </div>
                      </div>
                    ))}
                  </div>
                  <button
                    type="button"
                    className={buttonClass}
                    disabled={busy}
                    onClick={() => void resubmit()}
                  >
                    {busy ? "Mengirim ulang..." : "Kirim Ulang Perbaikan"}
                  </button>
                </div>
              ) : null}
            </section>
          ) : null}

          {result.status === "REJECTED" ? (
            <section className="rounded-2xl border border-red-200 bg-red-50 p-5 shadow-sm">
              <h2 className="font-bold text-red-900">Pendaftaran Ditolak</h2>
              <p className="mt-3 text-sm font-semibold text-red-900">Alasan:</p>
              <p className="mt-1 text-sm whitespace-pre-wrap text-red-800">
                {result.note || "Tidak ada alasan tambahan."}
              </p>
            </section>
          ) : null}

          {result.status === "APPROVED" ? (
            <Panel className="border-emerald-200">
              <div className="rounded-xl bg-emerald-50 p-4 text-emerald-900">
                <h2 className="font-bold">Pendaftaran Anda telah disetujui.</h2>
                <p className="mt-1 text-sm">
                  Silakan gunakan akun Ketua atau Sekretaris/Wakil untuk login
                  ke workspace organisasi.
                </p>
              </div>
              <h3 className="mt-5 font-bold text-slate-900">Akun Organisasi</h3>
              {result.credentials.length ? (
                <div className="mt-3 grid gap-4 md:grid-cols-2">
                  {result.credentials.map((credential) => (
                    <section
                      key={`${credential.position}-${credential.username}`}
                      className="rounded-xl border border-slate-200 p-4"
                    >
                      <h4 className="font-bold text-slate-900">
                        Akun {positionLabel(credential.position)}
                      </h4>
                      <dl className="mt-3 space-y-3 text-sm">
                        <div>
                          <dt className="text-slate-500">Nama</dt>
                          <dd className="font-semibold text-slate-900">
                            {credential.name}
                          </dd>
                        </div>
                        <div>
                          <dt className="text-slate-500">Jabatan</dt>
                          <dd className="font-semibold text-slate-900">
                            {positionLabel(credential.position)}
                          </dd>
                        </div>
                        {(
                          [
                            ["Username", credential.username],
                            ["Password", credential.password],
                          ] as const
                        ).map(([label, value]) => (
                          <div key={label}>
                            <dt className="text-slate-500">{label}</dt>
                            <dd className="mt-1 flex items-center justify-between gap-2 rounded-lg bg-slate-50 px-3 py-2 font-mono font-semibold text-slate-900">
                              <span className="min-w-0 break-all">{value}</span>
                              <button
                                type="button"
                                className="inline-flex shrink-0 items-center gap-1 rounded-md border border-slate-200 bg-white px-2 py-1 font-sans text-xs font-semibold text-slate-700 hover:bg-slate-100"
                                onClick={() => void copy(value, label)}
                                aria-label={`Salin ${label.toLowerCase()} akun ${positionLabel(credential.position)}`}
                              >
                                <Copy className="h-3.5 w-3.5" />
                                Copy
                              </button>
                            </dd>
                          </div>
                        ))}
                      </dl>
                    </section>
                  ))}
                </div>
              ) : (
                <p className="mt-3 rounded-lg bg-amber-50 p-3 text-sm text-amber-800">
                  Informasi akun belum tersedia. Silakan hubungi administrator.
                </p>
              )}
              {copied ? (
                <p
                  role="status"
                  className="mt-4 text-sm font-semibold text-emerald-700"
                >
                  {copied}
                </p>
              ) : null}
              <Link href="/login" className={`${buttonClass} mt-5`}>
                Login
              </Link>
            </Panel>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}
