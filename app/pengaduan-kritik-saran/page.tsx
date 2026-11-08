"use client";

import { useSearchParams } from "next/navigation";
import Script from "next/script";
import Image from "next/image";
import Link from "next/link";
import {
  CheckCircle2,
  Clipboard,
  MessageSquareWarning,
  Search,
  Settings,
} from "lucide-react";
import { FormEvent, useEffect, useRef, useState } from "react";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";

type Category = { id: number; name: string; reportType: string };
type Report = {
  reportId: string;
  reportType: string;
  title: string;
  status: string;
  createdAt: string;
};
type School = { id: number; name: string; npsn: string };
type FieldErrors = Record<string, string>;
type AdminReport = Report & {
  reporter: string;
  confidentiality: string;
  targetType: string;
  targetName: string;
  assigneeName?: string | null;
  clarificationDeadline?: string | null;
  needsAction?: boolean;
};
type PopupContent = { title: string; imageUrl: string; altText: string };
type SubmissionResult = { reportId: string; pin: string };
type TimelineItem = {
  status: string;
  message?: string | null;
  createdAt?: string;
  internal?: boolean;
};
type Clarification = {
  round: number;
  request: string;
  deadlineAt: string;
  response?: string | null;
  canRespond?: boolean;
};
type TrackedReport = Report & {
  timeline?: TimelineItem[];
  targetName?: string;
  officialResponse?: string | null;
  closedReason?: string | null;
  clarification?: Clarification | null;
  attachments?: { id: number; name: string; mimeType: string }[];
  hasHiddenFollowUpEvidence?: boolean;
};
type AdminAttachment = {
  id: number;
  name: string;
  url: string;
  mimeType: string;
  sizeBytes: number;
  createdAt: string;
  kind: string;
  publicToReporter: boolean;
};
type AdminDetail = {
  reportId: string;
  reportType: string;
  category?: string | null;
  title: string;
  status: string;
  confidentiality: string;
  reporter?: string | null;
  targetType: string;
  targetName?: string | null;
  schoolName?: string | null;
  otherTargetName?: string | null;
  reporterWhatsapp?: string | null;
  reporterEmail?: string | null;
  eventDate?: string | null;
  eventLocation?: string | null;
  relatedParties?: string | null;
  details: string;
  impact?: string | null;
  reason?: string | null;
  expectedResolution?: string | null;
  expectedBenefit?: string | null;
  attachments?: AdminAttachment[];
  assignedTo?: number | null;
  assigneeName?: string | null;
  officialResponse?: string | null;
  closedReason?: string | null;
  clarificationDeadline?: string | null;
  clarifications?: Clarification[];
  timeline?: TimelineItem[];
};
type Assignee = { id: number; name: string; institution: string };

const statusLabel: Record<string, string> = {
  RECEIVED: "Laporan diterima",
  VERIFICATION: "Proses verifikasi",
  FOLLOW_UP: "Proses tindak lanjut",
  RESPONDED: "Tanggapan diberikan",
  COMPLETED: "Selesai",
  NEEDS_INFO: "Butuh informasi tambahan",
  CLARIFICATION_SUBMITTED: "Klarifikasi Dikirim – Menunggu Verifikasi Petugas",
  REJECTED: "Ditolak",
  REJECTED_SPAM: "Ditolak",
  OUT_OF_SCOPE: "Di luar kewenangan",
  REOPENED: "Dibuka kembali",
};

const inputClass =
  "mt-1 w-full rounded-xl border p-3 outline-none transition focus:border-blue-500 focus:ring-2 focus:ring-blue-100";
const errorClass =
  "border-red-500 bg-red-50 focus:border-red-500 focus:ring-red-100";

export default function ComplaintPage() {
  const { user } = useAuth();
  const formRef = useRef<HTMLFormElement>(null);
  const searchParams = useSearchParams();
  const [tab, setTab] = useState("PENGADUAN");
  useEffect(() => { if (searchParams.get("tab") === "LACAK") setTab("LACAK"); }, [searchParams]);
  const [categories, setCategories] = useState<Category[]>([]);
  const [mine, setMine] = useState<Report[]>([]);
  const [popup, setPopup] = useState<PopupContent | null | undefined>(
    undefined
  );
  const [result, setResult] = useState<SubmissionResult | null>(null);
  const [track, setTrack] = useState<TrackedReport | null>(null);
  const [trackCredentials, setTrackCredentials] = useState({
    reportId: "",
    pin: "",
  });
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [apiError, setApiError] = useState("");
  const [targetType, setTargetType] = useState("");
  const [otherTargetName, setOtherTargetName] = useState("");
  const [confidentiality, setConfidentiality] = useState("");
  const [schoolQuery, setSchoolQuery] = useState("");
  const [schoolId, setSchoolId] = useState<number | null>(null);
  const [schools, setSchools] = useState<School[]>([]);
  const [schoolOpen, setSchoolOpen] = useState(false);
  const [adminReports, setAdminReports] = useState<AdminReport[]>([]);
  const [adminDetail, setAdminDetail] = useState<AdminDetail | null>(null);
  const [adminLoading, setAdminLoading] = useState(false);
  const [assignees, setAssignees] = useState<Assignee[]>([]);
  const isComplaintAdmin =
    user?.role === "ADMIN_MASTER" ||
    user?.role === "ADMIN_CABDIN" ||
    user?.role === "ADMIN";
  const isAdminMaster = user?.role === "ADMIN_MASTER";

  useEffect(() => {
    apiFetch("/api/public/complaints/categories")
      .then((response) => response.json())
      .then((payload) => setCategories(payload.data || []));
    apiFetch("/api/public/complaints/popup")
      .then((response) => response.json())
      .then((payload) => setPopup(payload.data || null));
  }, []);

  useEffect(() => {
    if (user) {
      apiFetch("/api/complaints/mine")
        .then((response) => response.json())
        .then((payload) => setMine(payload.data || []));
    }
  }, [user]);

  useEffect(() => {
    if (user && confidentiality === "ANONYMOUS") {
      setConfidentiality("");
    }
  }, [confidentiality, user]);

  useEffect(() => {
    if (tab !== "ADMIN" || !isComplaintAdmin) return;
    setAdminLoading(true);
    apiFetch("/api/admin/complaints")
      .then((response) => response.json())
      .then((payload) => setAdminReports(payload.data || []))
      .finally(() => setAdminLoading(false));
  }, [isComplaintAdmin, tab]);

  useEffect(() => {
    if (!isAdminMaster || tab !== "ADMIN") return;
    apiFetch("/api/admin/complaints-assignees")
      .then((response) => response.json())
      .then((payload) => setAssignees(payload.data || []));
  }, [isAdminMaster, tab]);

  useEffect(() => {
    if (targetType !== "SCHOOL" || schoolQuery.trim().length < 2 || schoolId) {
      setSchools([]);
      return;
    }
    const controller = new AbortController();
    const timer = window.setTimeout(() => {
      apiFetch(
        `/api/public/complaints/schools?q=${encodeURIComponent(schoolQuery.trim())}`,
        { signal: controller.signal },
        { skipAuthRedirect: true }
      )
        .then((response) => response.json())
        .then((payload) => {
          setSchools(payload.data || []);
          setSchoolOpen(true);
        })
        .catch(() => undefined);
    }, 250);
    return () => {
      window.clearTimeout(timer);
      controller.abort();
    };
  }, [schoolId, schoolQuery, targetType]);

  function clearError(name: string) {
    setErrors((current) => {
      if (!current[name]) return current;
      const next = { ...current };
      delete next[name];
      return next;
    });
    setApiError("");
  }

  function changeTab(nextTab: string) {
    setTab(nextTab);
    setErrors({});
    setApiError("");
  }

  function changeTarget(nextTarget: string) {
    setTargetType(nextTarget);
    clearError("targetType");
    if (nextTarget !== "SCHOOL") {
      setSchoolId(null);
      setSchoolQuery("");
      setSchools([]);
      clearError("schoolId");
    }
    if (nextTarget !== "OTHER") {
      setOtherTargetName("");
      clearError("otherTargetName");
    }
  }

  function changeConfidentiality(nextConfidentiality: string) {
    setConfidentiality(nextConfidentiality);
    clearError("confidentiality");
    if (!user && nextConfidentiality === "ANONYMOUS" && formRef.current) {
      for (const name of ["reporterName", "whatsapp", "email"]) {
        const input = formRef.current.elements.namedItem(
          name
        ) as HTMLInputElement | null;
        if (input) input.value = "";
        clearError(name);
      }
    }
  }

  function validate(form: HTMLFormElement) {
    const data = new FormData(form);
    const next: FieldErrors = {};
    const required = (name: string, message: string) => {
      if (!String(data.get(name) || "").trim()) next[name] = message;
    };

    required("title", "Judul wajib diisi.");
    required("categoryId", "Kategori wajib dipilih.");
    required("targetType", "Unit/layanan tujuan wajib dipilih.");
    if (targetType === "SCHOOL" && !schoolId) {
      next.schoolId = "Pilih sekolah dari daftar yang tersedia.";
    }
    if (targetType === "OTHER" && !otherTargetName.trim()) {
      next.otherTargetName = "Nama unit/layanan tujuan lainnya wajib diisi.";
    }
    if (tab === "PENGADUAN") {
      required("eventDate", "Tanggal kejadian wajib diisi.");
      required("relatedParties", "Pihak terkait wajib diisi.");
    }
    required(
      "details",
      `${tab === "PENGADUAN" ? "Kronologi" : "Isi utama"} wajib diisi.`
    );
    required(
      tab === "SARAN" ? "reason" : "impact",
      "Dampak/alasan wajib diisi."
    );
    required(
      tab === "SARAN" ? "expectedBenefit" : "expectedResolution",
      "Perbaikan/manfaat yang diharapkan wajib diisi."
    );
    required("confidentiality", "Tingkat kerahasiaan wajib dipilih.");
    if (!user && confidentiality && confidentiality !== "ANONYMOUS") {
      required("reporterName", "Nama pelapor wajib diisi.");
    }
    const whatsapp = String(data.get("whatsapp") || "").trim();
    if (whatsapp && !/^\+?[0-9][0-9\s-]{7,19}$/.test(whatsapp)) {
      next.whatsapp = "Format nomor WhatsApp tidak valid.";
    }
    const email = String(data.get("email") || "").trim();
    if (email && !/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email)) {
      next.email = "Format email tidak valid.";
    }
    const attachments = form.elements.namedItem(
      "attachments"
    ) as HTMLInputElement;
    if (!attachments.files?.length) {
      next.attachments = "Minimal satu lampiran/bukti wajib diunggah.";
    }
    if (data.get("truthConfirmed") !== "true") {
      next.truthConfirmed = "Pernyataan kebenaran wajib dicentang.";
    }
    return next;
  }

  function focusFirstError(nextErrors: FieldErrors) {
    const first = Object.keys(nextErrors)[0];
    window.requestAnimationFrame(() => {
      const element = formRef.current?.querySelector<HTMLElement>(
        `[data-field="${first}"]`
      );
      element?.scrollIntoView({ behavior: "smooth", block: "center" });
      element?.focus({ preventScroll: true });
    });
  }

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const formElement = event.currentTarget;
    const nextErrors = validate(formElement);
    if (Object.keys(nextErrors).length) {
      setErrors(nextErrors);
      setApiError("Periksa kembali kolom yang ditandai.");
      focusFirstError(nextErrors);
      return;
    }

    setBusy(true);
    setApiError("");
    const form = new FormData(formElement);
    form.set("reportType", tab);
    if (targetType === "SCHOOL" && schoolId) {
      form.set("schoolId", String(schoolId));
      form.delete("otherTargetName");
    } else if (targetType === "OTHER") {
      form.delete("schoolId");
      form.set("otherTargetName", otherTargetName.trim());
    } else {
      form.delete("schoolId");
      form.delete("otherTargetName");
    }
    if (tab !== "PENGADUAN") {
      form.delete("eventDate");
      form.delete("eventLocation");
      form.delete("relatedParties");
    }
    if (!user && confidentiality === "ANONYMOUS") {
      form.delete("reporterName");
      form.delete("whatsapp");
      form.delete("email");
    }

    try {
      const response = await apiFetch(
        user ? "/api/complaints" : "/api/public/complaints",
        { method: "POST", body: form },
        { skipAuthRedirect: !user }
      );
      const payload = await response.json();
      if (!response.ok) {
        const responseErrors = payload.fields || {};
        setErrors(responseErrors);
        setApiError(
          payload.error || "Laporan belum dapat dikirim. Periksa kembali isian."
        );
        if (Object.keys(responseErrors).length) focusFirstError(responseErrors);
        return;
      }
      setResult(payload.data);
      formElement.reset();
      setTargetType("");
      setOtherTargetName("");
      setConfidentiality("");
      setSchoolId(null);
      setSchoolQuery("");
      setSchools([]);
      setErrors({});
    } catch {
      setApiError(
        "Laporan belum dapat dikirim. Periksa koneksi lalu coba lagi."
      );
    } finally {
      setBusy(false);
    }
  }

  async function tracking(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    const form = new FormData(event.currentTarget);
    const response = await apiFetch(
      "/api/public/complaints/track",
      {
        method: "POST",
        body: JSON.stringify({
          reportId: form.get("reportId"),
          pin: form.get("pin"),
        }),
      },
      { skipAuthRedirect: true }
    );
    const payload = await response.json();
    if (!response.ok) {
      alert(payload.error);
      return;
    }
    setTrackCredentials({
      reportId: String(form.get("reportId") || ""),
      pin: String(form.get("pin") || ""),
    });
    setTrack(payload.data);
  }

  async function runAdminAction(input: {
    action: string;
    message?: string;
    assignedTo?: number;
    public?: boolean;
    attachments?: File[];
    evidenceMessage?: string;
  }): Promise<boolean> {
    if (!adminDetail || busy) return false;
    setBusy(true);
    const reportPath = encodeURIComponent(adminDetail.reportId);
    let response: Response;
    if (input.action === "RESPOND") {
      const form = new FormData();
      form.set("response", input.message || "");
      form.set("message", input.evidenceMessage || "");
      form.set("public", input.public === false ? "false" : "true");
      input.attachments?.forEach((file) => form.append("attachments", file));
      response = await apiFetch(`/api/admin/complaints/${reportPath}/respond`, {
        method: "POST",
        body: form,
      });
    } else {
      response = await apiFetch(`/api/admin/complaints/${reportPath}/actions`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(input),
      });
    }
    const payload = await response.json();
    if (!response.ok) {
      alert(payload.error || "Tindakan gagal diproses.");
    } else {
      await showAdminDetail(adminDetail.reportId);
      const listResponse = await apiFetch("/api/admin/complaints");
      const listPayload = await listResponse.json();
      setAdminReports(listPayload.data || []);
    }
    setBusy(false);
    return response.ok;
  }

  async function showAdminDetail(reportId: string) {
    setAdminLoading(true);
    try {
      const response = await apiFetch(
        `/api/admin/complaints/${encodeURIComponent(reportId)}`
      );
      const payload = await response.json();
      if (!response.ok) {
        alert(payload.error || "Detail laporan tidak dapat dimuat.");
        return;
      }
      setAdminDetail(payload.data);
    } finally {
      setAdminLoading(false);
    }
  }

  const tabs = [
    "PENGADUAN",
    "KRITIK",
    "SARAN",
    "LACAK",
    ...(user ? ["SAYA"] : []),
    ...(isComplaintAdmin ? ["ADMIN"] : []),
  ];
  const fieldClass = (name: string) =>
    `${inputClass} ${errors[name] ? errorClass : ""}`;

  return (
    <div className="mx-auto w-full max-w-6xl space-y-6">
      <header className="rounded-3xl bg-gradient-to-r from-slate-900 to-blue-900 p-7 text-white">
        <div className="flex items-start justify-between gap-3">
          <div className="flex items-center gap-3">
            <MessageSquareWarning />
            <div>
              <h1 className="text-2xl font-bold">
                Kritik, Saran & Pengaduan Masyarakat
              </h1>
              <p className="mt-1 text-sm text-blue-100">
                Kanal resmi, aman, dan menjaga kerahasiaan pelapor.
              </p>
            </div>
          </div>
          {isAdminMaster && (
            <Link
              href="/pengaduan-kritik-saran/settings"
              aria-label="Kelola informasi popup"
              className="rounded-xl bg-white/10 p-2 transition hover:bg-white/20"
            >
              <Settings className="h-5 w-5" />
            </Link>
          )}
        </div>
      </header>
      <div className="flex flex-wrap gap-2">
        {tabs.map((item) => (
          <button
            key={item}
            type="button"
            onClick={() => changeTab(item)}
            className={`rounded-full px-4 py-2 text-sm font-semibold ${
              tab === item
                ? "bg-blue-700 text-white"
                : "border bg-white text-slate-700"
            }`}
          >
            {item === "SAYA"
              ? "Laporan Saya"
              : item === "ADMIN"
                ? "Kelola Laporan"
                : item === "LACAK"
                  ? "Lacak Laporan"
                  : item[0] + item.slice(1).toLowerCase()}
          </button>
        ))}
      </div>

      {["PENGADUAN", "KRITIK", "SARAN"].includes(tab) && (
        <form
          ref={formRef}
          onSubmit={submit}
          noValidate
          className="grid gap-4 rounded-3xl border bg-white p-6 shadow-sm md:grid-cols-2"
        >
          <Field label="Judul" name="title" error={errors.title}>
            <input
              data-field="title"
              maxLength={200}
              name="title"
              onInput={() => clearError("title")}
              className={fieldClass("title")}
            />
          </Field>
          <Field label="Kategori" name="categoryId" error={errors.categoryId}>
            <select
              data-field="categoryId"
              name="categoryId"
              defaultValue=""
              onChange={() => clearError("categoryId")}
              className={fieldClass("categoryId")}
            >
              <option value="">Pilih kategori</option>
              {categories
                .filter((category) => category.reportType === tab)
                .map((category) => (
                  <option key={category.id} value={category.id}>
                    {category.name}
                  </option>
                ))}
            </select>
          </Field>
          <Field
            label="Unit/layanan tujuan"
            name="targetType"
            error={errors.targetType}
          >
            <select
              data-field="targetType"
              name="targetType"
              value={targetType}
              onChange={(event) => changeTarget(event.target.value)}
              className={fieldClass("targetType")}
            >
              <option value="">Pilih unit/layanan tujuan</option>
              <option value="CABDIN">
                Cabang Dinas Pendidikan Wilayah Kabupaten Bangkalan
              </option>
              <option value="SCHOOL">Satuan Pendidikan</option>
              <option value="SIPRES">SIPRES/aplikasi digital</option>
              <option value="OTHER">Lainnya</option>
            </select>
          </Field>
          {targetType === "SCHOOL" && (
            <Field
              label="Nama sekolah/Satuan Pendidikan"
              name="schoolId"
              error={errors.schoolId}
            >
              <div className="relative">
                <input
                  data-field="schoolId"
                  name="schoolSearch"
                  value={schoolQuery}
                  autoComplete="off"
                  placeholder="Cari nama sekolah atau NPSN"
                  onFocus={() => setSchoolOpen(true)}
                  onChange={(event) => {
                    setSchoolQuery(event.target.value);
                    setSchoolId(null);
                    clearError("schoolId");
                  }}
                  className={fieldClass("schoolId")}
                />
                {schoolOpen && schools.length > 0 && (
                  <div className="absolute z-20 mt-1 max-h-64 w-full overflow-auto rounded-xl border bg-white p-1 shadow-xl">
                    {schools.map((school) => (
                      <button
                        type="button"
                        key={school.id}
                        onMouseDown={(event) => event.preventDefault()}
                        onClick={() => {
                          setSchoolId(school.id);
                          setSchoolQuery(
                            `${school.name}${school.npsn ? ` — ${school.npsn}` : ""}`
                          );
                          setSchoolOpen(false);
                          setSchools([]);
                          clearError("schoolId");
                        }}
                        className="block w-full rounded-lg px-3 py-2 text-left text-sm hover:bg-blue-50"
                      >
                        <span className="block font-medium">{school.name}</span>
                        {school.npsn && (
                          <span className="text-xs text-slate-500">
                            NPSN {school.npsn}
                          </span>
                        )}
                      </button>
                    ))}
                  </div>
                )}
              </div>
            </Field>
          )}
          {targetType === "OTHER" && (
            <Field
              label="Nama unit/layanan tujuan lainnya"
              name="otherTargetName"
              error={errors.otherTargetName}
            >
              <input
                data-field="otherTargetName"
                name="otherTargetName"
                value={otherTargetName}
                maxLength={255}
                onChange={(event) => {
                  setOtherTargetName(event.target.value);
                  clearError("otherTargetName");
                }}
                className={fieldClass("otherTargetName")}
              />
            </Field>
          )}
          {tab === "PENGADUAN" && (
            <>
              <Field
                label="Tanggal kejadian"
                name="eventDate"
                error={errors.eventDate}
              >
                <input
                  data-field="eventDate"
                  type="date"
                  name="eventDate"
                  onInput={() => clearError("eventDate")}
                  className={fieldClass("eventDate")}
                />
              </Field>
              <Field label="Lokasi kejadian (opsional)" name="eventLocation">
                <input
                  name="eventLocation"
                  placeholder="Tempat terjadinya kejadian (boleh tidak diisi)"
                  maxLength={255}
                  className={inputClass}
                />
              </Field>
              <Field
                label="Pihak terkait"
                name="relatedParties"
                error={errors.relatedParties}
                wide
              >
                <input
                  data-field="relatedParties"
                  name="relatedParties"
                  placeholder="Pelaku atau pihak yang terkait"
                  maxLength={500}
                  onInput={() => clearError("relatedParties")}
                  className={fieldClass("relatedParties")}
                />
              </Field>
            </>
          )}
          <Field
            label={
              tab === "SARAN"
                ? "Isi usulan"
                : tab === "KRITIK"
                  ? "Penjelasan dan bagian yang dinilai kurang"
                  : "Kronologi"
            }
            name="details"
            error={errors.details}
            wide
          >
            <textarea
              data-field="details"
              minLength={10}
              maxLength={10000}
              name="details"
              rows={6}
              onInput={() => clearError("details")}
              className={fieldClass("details")}
            />
          </Field>
          <Field
            label="Dampak / alasan"
            name={tab === "SARAN" ? "reason" : "impact"}
            error={errors[tab === "SARAN" ? "reason" : "impact"]}
          >
            <textarea
              data-field={tab === "SARAN" ? "reason" : "impact"}
              name={tab === "SARAN" ? "reason" : "impact"}
              maxLength={5000}
              onInput={() => clearError(tab === "SARAN" ? "reason" : "impact")}
              className={fieldClass(tab === "SARAN" ? "reason" : "impact")}
            />
          </Field>
          <Field
            label="Perbaikan / manfaat yang diharapkan"
            name={tab === "SARAN" ? "expectedBenefit" : "expectedResolution"}
            error={
              errors[tab === "SARAN" ? "expectedBenefit" : "expectedResolution"]
            }
          >
            <textarea
              data-field={
                tab === "SARAN" ? "expectedBenefit" : "expectedResolution"
              }
              name={tab === "SARAN" ? "expectedBenefit" : "expectedResolution"}
              maxLength={5000}
              onInput={() =>
                clearError(
                  tab === "SARAN" ? "expectedBenefit" : "expectedResolution"
                )
              }
              className={fieldClass(
                tab === "SARAN" ? "expectedBenefit" : "expectedResolution"
              )}
            />
          </Field>
          <Field
            label="Kerahasiaan"
            name="confidentiality"
            error={errors.confidentiality}
          >
            <select
              data-field="confidentiality"
              name="confidentiality"
              value={confidentiality}
              onChange={(event) => changeConfidentiality(event.target.value)}
              className={fieldClass("confidentiality")}
            >
              <option value="">Pilih tingkat kerahasiaan</option>
              <option value="OPEN">Terbuka kepada petugas</option>
              <option value="CONFIDENTIAL">Rahasia</option>
              {!user && <option value="ANONYMOUS">Anonim</option>}
            </select>
          </Field>
          {!user && (
            <Field
              label="Nama pelapor"
              name="reporterName"
              error={errors.reporterName}
            >
              <input
                data-field="reporterName"
                name="reporterName"
                maxLength={200}
                disabled={confidentiality === "ANONYMOUS"}
                onInput={() => clearError("reporterName")}
                className={fieldClass("reporterName")}
              />
            </Field>
          )}
          <Field
            label="WhatsApp (opsional)"
            name="whatsapp"
            error={errors.whatsapp}
          >
            <input
              data-field="whatsapp"
              name="whatsapp"
              inputMode="tel"
              placeholder="Contoh: 081234567890"
              maxLength={40}
              disabled={!user && confidentiality === "ANONYMOUS"}
              onInput={() => clearError("whatsapp")}
              className={fieldClass("whatsapp")}
            />
          </Field>
          <Field label="Email (opsional)" name="email" error={errors.email}>
            <input
              data-field="email"
              type="email"
              name="email"
              placeholder="Contoh: nama@email.com"
              maxLength={255}
              disabled={!user && confidentiality === "ANONYMOUS"}
              onInput={() => clearError("email")}
              className={fieldClass("email")}
            />
          </Field>
          <Field
            label="Lampiran/Bukti Pendukung (maks. 3 file, masing-masing 1 MB)"
            name="attachments"
            error={errors.attachments}
            wide
          >
            <input
              data-field="attachments"
              type="file"
              name="attachments"
              multiple
              accept=".jpg,.jpeg,.png,.webp,.pdf"
              onChange={() => clearError("attachments")}
              className={fieldClass("attachments")}
            />
          </Field>
          <div className="md:col-span-2">
            <label
              className={`flex gap-2 rounded-xl border p-3 text-sm ${
                errors.truthConfirmed ? errorClass : "border-transparent"
              }`}
            >
              <input
                data-field="truthConfirmed"
                type="checkbox"
                name="truthConfirmed"
                value="true"
                onChange={() => clearError("truthConfirmed")}
              />
              Saya menyatakan informasi yang disampaikan benar dan dapat
              dipertanggungjawabkan.
            </label>
            <ErrorMessage message={errors.truthConfirmed} />
          </div>
          {!user && process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY && (
            <div className="md:col-span-2">
              <Script
                src="https://challenges.cloudflare.com/turnstile/v0/api.js"
                async
                defer
              />
              <div
                className="cf-turnstile"
                data-sitekey={process.env.NEXT_PUBLIC_TURNSTILE_SITE_KEY}
              />
            </div>
          )}
          {apiError && (
            <p
              role="alert"
              className="rounded-xl border border-red-200 bg-red-50 p-3 text-sm text-red-700 md:col-span-2"
            >
              {apiError}
            </p>
          )}
          <button
            disabled={busy}
            className="rounded-xl bg-blue-700 px-5 py-3 font-semibold text-white disabled:cursor-wait disabled:opacity-70 md:col-span-2"
          >
            {busy ? "Mengirim laporan..." : "Kirim Laporan"}
          </button>
        </form>
      )}

      {tab === "LACAK" && (
        <div className="space-y-4 rounded-3xl border bg-white p-6">
          <form onSubmit={tracking} className="grid gap-3 md:grid-cols-3">
            <input
              required
              name="reportId"
              placeholder="ID Laporan"
              className="rounded-xl border p-3"
            />
            <input
              required
              name="pin"
              type="password"
              placeholder="PIN Pelacakan"
              className="rounded-xl border p-3"
            />
            <button className="rounded-xl bg-blue-700 font-semibold text-white">
              <Search className="mr-2 inline h-4" />
              Lacak
            </button>
          </form>
          {track && (
            <ReportCard report={track} credentials={trackCredentials} />
          )}
        </div>
      )}
      {tab === "SAYA" && (
        <div className="space-y-3">
          {mine.map((report) => (
            <ReportCard key={report.reportId} report={report} />
          ))}
        </div>
      )}
      {tab === "ADMIN" && isComplaintAdmin && (
        <section className="space-y-4 rounded-3xl border bg-white p-6 shadow-sm">
          <div>
            <h2 className="text-xl font-bold">Daftar Laporan Masyarakat</h2>
            <p className="text-sm text-slate-500">
              Pilih laporan untuk melihat identitas, tujuan, isi, dan
              lampirannya.
            </p>
          </div>
          {adminLoading && !adminReports.length && (
            <p className="text-sm text-slate-500">Memuat laporan...</p>
          )}
          {!adminLoading && !adminReports.length && (
            <p className="rounded-xl bg-slate-50 p-4 text-sm text-slate-500">
              Belum ada laporan yang dapat ditampilkan.
            </p>
          )}
          <div className="grid gap-3">
            {adminReports.map((report) => (
              <button
                type="button"
                key={report.reportId}
                onClick={() => showAdminDetail(report.reportId)}
                className="rounded-2xl border p-4 text-left transition hover:border-blue-300 hover:bg-blue-50/40"
              >
                <div className="flex flex-wrap items-start justify-between gap-3">
                  <div>
                    <p className="font-mono text-xs text-blue-700">
                      {report.reportId}
                    </p>
                    <h3 className="font-semibold">{report.title}</h3>
                    <p className="mt-1 text-sm text-slate-600">
                      {report.reporter || "Anonim"} ·{" "}
                      {report.targetName || report.targetType}
                    </p>
                    <p className="mt-1 text-xs text-slate-500">
                      Penanggung jawab:{" "}
                      {report.assigneeName || "Belum ditugaskan"}
                      {report.clarificationDeadline
                        ? ` · Tenggat ${new Date(
                            report.clarificationDeadline
                          ).toLocaleString("id-ID")}`
                        : ""}
                    </p>
                  </div>
                  <span className="rounded-full bg-blue-50 px-3 py-1 text-xs font-semibold text-blue-800">
                    {statusLabel[report.status] || report.status}
                  </span>
                  {report.needsAction && (
                    <span className="rounded-full bg-amber-100 px-3 py-1 text-xs font-semibold text-amber-900">
                      Perlu tindakan
                    </span>
                  )}
                </div>
              </button>
            ))}
          </div>
        </section>
      )}
      {popup !== undefined && popup && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div className="max-w-xl rounded-3xl bg-white p-5 shadow-2xl">
            <h2 className="mb-3 text-xl font-bold">{popup.title}</h2>
            <Image
              src={popup.imageUrl}
              alt={popup.altText}
              width={960}
              height={640}
              unoptimized
              className="max-h-[65vh] w-full rounded-xl object-contain"
              onError={(event) => {
                event.currentTarget.style.display = "none";
              }}
            />
            <p className="mt-2 text-sm text-slate-600">{popup.altText}</p>
            <button
              type="button"
              onClick={() => setPopup(null)}
              className="mt-4 w-full rounded-xl bg-blue-700 py-3 font-semibold text-white"
            >
              Saya Mengerti
            </button>
          </div>
        </div>
      )}
      {result && (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 p-4">
          <div className="max-w-md rounded-3xl bg-white p-7 text-center">
            <CheckCircle2 className="mx-auto h-14 w-14 text-emerald-600" />
            <h2 className="mt-3 text-xl font-bold">Laporan berhasil dikirim</h2>
            <p className="mt-4 rounded-xl bg-slate-100 p-3 font-mono">
              {result.reportId}{" "}
              <button
                type="button"
                onClick={() => navigator.clipboard.writeText(result.reportId)}
              >
                <Clipboard className="inline h-4" />
              </button>
            </p>
            <p className="mt-2 rounded-xl bg-amber-50 p-3 font-mono">
              PIN: {result.pin}{" "}
              <button
                type="button"
                onClick={() => navigator.clipboard.writeText(result.pin)}
              >
                <Clipboard className="inline h-4" />
              </button>
            </p>
            <p className="mt-3 text-sm text-red-700">
              Simpan ID dan PIN sekarang. PIN tidak dapat ditampilkan kembali
              setelah jendela ini ditutup.
            </p>
            <button
              type="button"
              onClick={() => setResult(null)}
              className="mt-5 w-full rounded-xl bg-blue-700 py-3 text-white"
            >
              Tutup
            </button>
          </div>
        </div>
      )}
      {adminDetail && (
        <div className="fixed inset-0 z-50 overflow-y-auto bg-black/60 p-4">
          <div className="mx-auto my-6 max-w-3xl rounded-3xl bg-white p-6 shadow-2xl">
            <div className="flex items-start justify-between gap-4">
              <div>
                <p className="font-mono text-sm text-blue-700">
                  {adminDetail.reportId}
                </p>
                <h2 className="text-xl font-bold">{adminDetail.title}</h2>
                <p className="text-sm text-slate-500">
                  {adminDetail.reportType} ·{" "}
                  {adminDetail.category || "Kategori lama"}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setAdminDetail(null)}
                className="rounded-xl border px-3 py-2 text-sm"
              >
                Tutup
              </button>
            </div>
            <WorkflowTracker
              status={adminDetail.status}
              timeline={adminDetail.timeline}
            />
            <WorkflowActions
              detail={adminDetail}
              isMaster={isAdminMaster}
              assignees={assignees}
              busy={busy}
              onAction={runAdminAction}
            />
            <EvidenceUpload
              detail={adminDetail}
              onUploaded={() => showAdminDetail(adminDetail.reportId)}
            />
            <dl className="mt-5 grid gap-3 rounded-2xl bg-slate-50 p-4 text-sm sm:grid-cols-2">
              <DetailItem
                label="Status"
                value={statusLabel[adminDetail.status] || adminDetail.status}
              />
              <DetailItem
                label="Kerahasiaan"
                value={adminDetail.confidentiality}
              />
              <DetailItem
                label="Pelapor"
                value={adminDetail.reporter || "Anonim"}
              />
              <DetailItem
                label="Tujuan"
                value={adminDetail.targetName || adminDetail.targetType}
              />
              <DetailItem
                label="Penanggung jawab"
                value={adminDetail.assigneeName || "Belum ditugaskan"}
              />
              {adminDetail.schoolName && (
                <DetailItem
                  label="Satuan Pendidikan"
                  value={adminDetail.schoolName}
                />
              )}
              {adminDetail.otherTargetName && (
                <DetailItem
                  label="Unit/layanan lainnya"
                  value={adminDetail.otherTargetName}
                />
              )}
              <DetailItem
                label="WhatsApp"
                value={adminDetail.reporterWhatsapp || "—"}
              />
              <DetailItem
                label="Email"
                value={adminDetail.reporterEmail || "—"}
              />
              {adminDetail.eventDate && (
                <DetailItem
                  label="Tanggal kejadian"
                  value={new Date(adminDetail.eventDate).toLocaleDateString(
                    "id-ID"
                  )}
                />
              )}
              {adminDetail.eventLocation && (
                <DetailItem
                  label="Lokasi kejadian"
                  value={adminDetail.eventLocation}
                />
              )}
              {adminDetail.relatedParties && (
                <DetailItem
                  label="Pihak terkait"
                  value={adminDetail.relatedParties}
                />
              )}
            </dl>
            <div className="mt-5 space-y-4 text-sm">
              <DetailText
                label="Kronologi/isi utama"
                value={adminDetail.details}
              />
              <DetailText
                label="Dampak/alasan"
                value={adminDetail.impact || adminDetail.reason}
              />
              <DetailText
                label="Perbaikan/manfaat yang diharapkan"
                value={
                  adminDetail.expectedResolution || adminDetail.expectedBenefit
                }
              />
              <div>
                <h3 className="font-semibold">Lampiran/bukti</h3>
                {adminDetail.attachments?.length ? (
                  <div className="mt-2 flex flex-wrap gap-2">
                    {adminDetail.attachments.map((attachment) => (
                      <a
                        key={attachment.id}
                        href={attachment.url}
                        target="_blank"
                        rel="noreferrer"
                        className="rounded-xl border border-blue-200 bg-blue-50 px-3 py-2 text-blue-800"
                      >
                        {attachment.name}
                        <span className="ml-2 text-xs opacity-70">
                          {attachment.kind === "FOLLOW_UP"
                            ? attachment.publicToReporter
                              ? "Bukti · publik"
                              : "Bukti · internal"
                            : attachment.kind}
                        </span>
                      </a>
                    ))}
                  </div>
                ) : (
                  <p className="mt-1 text-slate-500">
                    Laporan lama ini tidak memiliki lampiran.
                  </p>
                )}
              </div>
              {adminDetail.officialResponse && (
                <DetailText
                  label="Tanggapan resmi"
                  value={adminDetail.officialResponse}
                />
              )}
              {adminDetail.closedReason && (
                <DetailText
                  label="Alasan penutupan"
                  value={adminDetail.closedReason}
                />
              )}
              {!!adminDetail.clarifications?.length && (
                <div>
                  <h3 className="font-semibold">Putaran klarifikasi</h3>
                  <div className="mt-2 space-y-2">
                    {adminDetail.clarifications.map((item) => (
                      <div key={item.round} className="rounded-xl border p-3">
                        <b>Putaran {item.round}</b>
                        <p className="mt-1">Permintaan: {item.request}</p>
                        <p className="text-xs text-slate-500">
                          Tenggat:{" "}
                          {new Date(item.deadlineAt).toLocaleString("id-ID")}
                        </p>
                        {item.response && (
                          <p className="mt-2">Jawaban: {item.response}</p>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              )}
              {!!adminDetail.timeline?.length && (
                <div>
                  <h3 className="font-semibold">Riwayat aktivitas</h3>
                  <ol className="mt-2 space-y-3 border-l pl-4">
                    {adminDetail.timeline.map((item, index) => (
                      <li key={index}>
                        <b>{statusLabel[item.status] || item.status}</b>
                        {item.internal && (
                          <span className="ml-2 rounded bg-slate-200 px-2 py-0.5 text-xs">
                            Internal
                          </span>
                        )}
                        {item.message && <p>{item.message}</p>}
                        {item.createdAt && (
                          <time className="text-xs text-slate-500">
                            {new Date(item.createdAt).toLocaleString("id-ID")}
                          </time>
                        )}
                      </li>
                    ))}
                  </ol>
                </div>
              )}
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function Field({
  label,
  name,
  error,
  wide = false,
  children,
}: {
  label: string;
  name: string;
  error?: string;
  wide?: boolean;
  children: React.ReactNode;
}) {
  return (
    <div
      data-name={name}
      className={`text-sm font-medium ${wide ? "md:col-span-2" : ""}`}
    >
      <span>{label}</span>
      {children}
      <ErrorMessage message={error} />
    </div>
  );
}

function ErrorMessage({ message }: { message?: string }) {
  return message ? (
    <span className="mt-1 block text-xs font-medium text-red-700">
      {message}
    </span>
  ) : null;
}

function DetailItem({ label, value }: { label: string; value: unknown }) {
  return (
    <div>
      <dt className="text-xs font-medium tracking-wide text-slate-500 uppercase">
        {label}
      </dt>
      <dd className="mt-1 break-words text-slate-800">
        {String(value || "—")}
      </dd>
    </div>
  );
}

function DetailText({ label, value }: { label: string; value: unknown }) {
  return (
    <div>
      <h3 className="font-semibold">{label}</h3>
      <p className="mt-1 rounded-xl border p-3 whitespace-pre-wrap text-slate-700">
        {String(value || "—")}
      </p>
    </div>
  );
}

const workflowSteps = [
  { status: "RECEIVED", label: "Diterima" },
  { status: "VERIFICATION", label: "Verifikasi" },
  { status: "FOLLOW_UP", label: "Tindak Lanjut" },
  { status: "RESPONDED", label: "Tanggapan Diberikan" },
  { status: "COMPLETED", label: "Selesai" },
];

function workflowIndex(status: string) {
  if (status === "REOPENED") return 1;
  if (status === "NEEDS_INFO" || status === "CLARIFICATION_SUBMITTED") return 1;
  if (status === "REJECTED" || status === "OUT_OF_SCOPE") return 1;
  return Math.max(
    0,
    workflowSteps.findIndex((step) => step.status === status)
  );
}

function WorkflowTracker({
  status,
  timeline,
}: {
  status: string;
  timeline?: TimelineItem[];
}) {
  const active = workflowIndex(status);
  return (
    <div className="mt-5">
      <ol className="grid gap-3 sm:grid-cols-5">
        {workflowSteps.map((step, index) => {
          const timestamp = timeline?.findLast(
            (item) => item.status === step.status
          )?.createdAt;
          return (
            <li
              key={step.status}
              className={`relative rounded-xl border p-3 text-sm ${
                index < active
                  ? "border-emerald-200 bg-emerald-50 text-emerald-900"
                  : index === active
                    ? "border-blue-600 bg-blue-700 text-white"
                    : "border-slate-200 bg-slate-50 text-slate-500"
              }`}
            >
              <span className="font-semibold">
                {index + 1}. {step.label}
              </span>
              {timestamp && (
                <time className="mt-1 block text-[11px] opacity-80">
                  {new Date(timestamp).toLocaleString("id-ID")}
                </time>
              )}
            </li>
          );
        })}
      </ol>
      {[
        "NEEDS_INFO",
        "CLARIFICATION_SUBMITTED",
        "REJECTED",
        "OUT_OF_SCOPE",
        "REOPENED",
      ].includes(status) && (
        <p
          className={`mt-3 rounded-xl p-3 text-sm font-medium ${
            status === "REJECTED"
              ? "bg-red-50 text-red-800"
              : status === "OUT_OF_SCOPE"
                ? "bg-orange-50 text-orange-800"
                : "bg-amber-50 text-amber-900"
          }`}
        >
          {statusLabel[status]}
        </p>
      )}
    </div>
  );
}

function WorkflowActions({
  detail,
  isMaster,
  assignees,
  busy,
  onAction,
}: {
  detail: AdminDetail;
  isMaster: boolean;
  assignees: Assignee[];
  busy: boolean;
  onAction: (input: {
    action: string;
    message?: string;
    assignedTo?: number;
    public?: boolean;
    attachments?: File[];
    evidenceMessage?: string;
  }) => Promise<boolean>;
}) {
  const [action, setAction] = useState("");
  const [message, setMessage] = useState("");
  const [assignedTo, setAssignedTo] = useState("");
  const [publicNote, setPublicNote] = useState(true);
  const [evidenceFiles, setEvidenceFiles] = useState<File[]>([]);
  const [evidenceMessage, setEvidenceMessage] = useState("");
  const [publicEvidence, setPublicEvidence] = useState(true);
  const closed = ["COMPLETED", "REJECTED", "OUT_OF_SCOPE"].includes(
    detail.status
  );
  const followUpEvidence =
    detail.attachments?.filter((item) => item.kind === "FOLLOW_UP") || [];
  const hasFollowUpImage = followUpEvidence.some((item) =>
    ["image/jpeg", "image/png", "image/webp"].includes(item.mimeType)
  );
  const remainingEvidenceSlots = Math.max(0, 3 - followUpEvidence.length);
  const options = [
    ...(isMaster &&
    ["RECEIVED", "VERIFICATION", "REOPENED"].includes(detail.status)
      ? [
          ["VERIFY_SELF", "Layak – tindak lanjuti sendiri"],
          ["ASSIGN", "Layak – teruskan ke Admin Cabdin"],
          ["OUT_OF_SCOPE", "Di luar kewenangan"],
        ]
      : []),
    ...(!closed
      ? [
          ["NEEDS_INFO", "Minta informasi tambahan"],
          ["PROGRESS", "Tambahkan perkembangan/catatan"],
          ["REJECT", "Tolak laporan"],
        ]
      : []),
    ...(["FOLLOW_UP", "CLARIFICATION_SUBMITTED"].includes(detail.status) &&
    remainingEvidenceSlots > 0
      ? [["RESPOND", "Berikan tanggapan resmi"]]
      : []),
    ...(detail.status === "CLARIFICATION_SUBMITTED"
      ? [["ACCEPT_CLARIFICATION", "Terima klarifikasi dan lanjutkan"]]
      : []),
    ...(detail.status === "RESPONDED" && hasFollowUpImage
      ? [["COMPLETE", "Tetapkan selesai"]]
      : []),
    ...(isMaster && detail.assignedTo && !closed
      ? [
          ["TAKEOVER", "Ambil alih"],
          ["REASSIGN", "Pindahkan penugasan"],
        ]
      : []),
    ...(isMaster && closed ? [["REOPEN", "Buka kembali"]] : []),
  ];
  if (!options.length) return null;
  const requiresAssignee = action === "ASSIGN" || action === "REASSIGN";
  const messageOptional = [
    "VERIFY_SELF",
    "COMPLETE",
    "ACCEPT_CLARIFICATION",
  ].includes(action);
  const respondValid =
    action !== "RESPOND" ||
    (message.trim().length > 0 &&
      evidenceFiles.length >= 1 &&
      evidenceFiles.length <= remainingEvidenceSlots &&
      evidenceFiles.some((file) =>
        ["image/jpeg", "image/png", "image/webp"].includes(file.type)
      ) &&
      evidenceFiles.every((file) => file.size <= 1024 * 1024));
  const buttonLabel: Record<string, string> = {
    PROGRESS: "Simpan perkembangan",
    NEEDS_INFO: "Kirim permintaan klarifikasi",
    RESPOND: "Kirim tanggapan dan bukti",
    COMPLETE: "Tetapkan selesai",
  };
  return (
    <form
      className="mt-5 grid gap-3 rounded-2xl border border-blue-100 bg-blue-50/50 p-4 sm:grid-cols-2"
      onSubmit={(event) => {
        event.preventDefault();
        void onAction({
          action,
          message,
          assignedTo: requiresAssignee ? Number(assignedTo) : undefined,
          public:
            action === "PROGRESS"
              ? publicNote
              : action === "RESPOND"
                ? publicEvidence
                : undefined,
          attachments: action === "RESPOND" ? evidenceFiles : undefined,
          evidenceMessage: action === "RESPOND" ? evidenceMessage : undefined,
        }).then((success) => {
          if (!success) return;
          setAction("");
          setMessage("");
          setEvidenceFiles([]);
          setEvidenceMessage("");
        });
      }}
    >
      <label className="text-sm font-medium">
        Tindakan
        <select
          required
          value={action}
          onChange={(event) => setAction(event.target.value)}
          className="mt-1 w-full rounded-xl border bg-white p-3"
        >
          <option value="">Pilih tindakan</option>
          {options.map(([value, label]) => (
            <option key={value} value={value}>
              {label}
            </option>
          ))}
        </select>
      </label>
      {requiresAssignee && (
        <label className="text-sm font-medium">
          Admin Cabdin
          <select
            required
            value={assignedTo}
            onChange={(event) => setAssignedTo(event.target.value)}
            className="mt-1 w-full rounded-xl border bg-white p-3"
          >
            <option value="">Pilih petugas aktif</option>
            {assignees.map((item) => (
              <option key={item.id} value={item.id}>
                {item.name} {item.institution ? `– ${item.institution}` : ""}
              </option>
            ))}
          </select>
        </label>
      )}
      <label className="text-sm font-medium sm:col-span-2">
        {action === "RESPOND" ? "Tanggapan resmi" : "Catatan/alasan"}
        {messageOptional ? " (opsional)" : ""}
        <textarea
          required={!messageOptional}
          value={message}
          onChange={(event) => setMessage(event.target.value)}
          maxLength={5000}
          className="mt-1 min-h-24 w-full rounded-xl border bg-white p-3"
        />
      </label>
      {action === "RESPOND" && (
        <div className="grid gap-3 sm:col-span-2 sm:grid-cols-2">
          <label className="text-sm font-medium">
            Bukti tindak lanjut (wajib, maks. {remainingEvidenceSlots} file)
            <input
              type="file"
              required
              multiple
              accept=".jpg,.jpeg,.png,.webp,.pdf"
              onChange={(event) =>
                setEvidenceFiles(Array.from(event.target.files || []))
              }
              className="mt-1 w-full rounded-xl border bg-white p-3"
            />
          </label>
          <label className="text-sm font-medium">
            Keterangan bukti (opsional)
            <input
              value={evidenceMessage}
              onChange={(event) => setEvidenceMessage(event.target.value)}
              maxLength={5000}
              className="mt-1 w-full rounded-xl border bg-white p-3"
            />
          </label>
          <p className="text-xs text-slate-600 sm:col-span-2">
            Minimal satu file harus berupa foto/screenshot JPG, PNG, atau WebP.
            PDF hanya dapat menjadi bukti tambahan. Maksimal tiga bukti secara
            total, dengan ukuran maksimal 1 MB per file.
          </p>
          <label className="flex items-center gap-2 text-sm sm:col-span-2">
            <input
              type="checkbox"
              checked={publicEvidence}
              onChange={(event) => setPublicEvidence(event.target.checked)}
            />
            Bukti dapat dilihat pelapor
          </label>
        </div>
      )}
      {action === "PROGRESS" && (
        <label className="flex items-center gap-2 text-sm">
          <input
            type="checkbox"
            checked={publicNote}
            onChange={(event) => setPublicNote(event.target.checked)}
          />
          Dapat dilihat pelapor
        </label>
      )}
      <button
        disabled={busy || !action || !respondValid}
        className="rounded-xl bg-blue-700 p-3 font-semibold text-white disabled:opacity-50 sm:col-span-2"
      >
        {busy ? "Memproses…" : buttonLabel[action] || "Simpan tindakan"}
      </button>
    </form>
  );
}

function EvidenceUpload({
  detail,
  onUploaded,
}: {
  detail: AdminDetail;
  onUploaded: () => Promise<void>;
}) {
  const [uploading, setUploading] = useState(false);
  const evidence =
    detail.attachments?.filter((item) => item.kind === "FOLLOW_UP") || [];
  const hasImage = evidence.some((item) =>
    ["image/jpeg", "image/png", "image/webp"].includes(item.mimeType)
  );
  if (detail.status !== "RESPONDED") return null;
  if (evidence.length >= 3 && hasImage) return null;
  return (
    <form
      className="mt-4 grid gap-3 rounded-2xl border border-amber-200 bg-amber-50/60 p-4 sm:grid-cols-2"
      onSubmit={async (event) => {
        event.preventDefault();
        const form = new FormData(event.currentTarget);
        const files = form
          .getAll("attachments")
          .filter((value): value is File => value instanceof File);
        if (
          files.length < 1 ||
          files.length > 3 - evidence.length ||
          files.some((file) => file.size > 1024 * 1024) ||
          (!hasImage &&
            !files.some((file) =>
              ["image/jpeg", "image/png", "image/webp"].includes(file.type)
            ))
        ) {
          alert(
            `Pilih maksimal ${3 - evidence.length} file (maks. 1 MB per file) dan pastikan tersedia minimal satu foto/screenshot.`
          );
          return;
        }
        setUploading(true);
        const response = await apiFetch(
          `/api/admin/complaints/${encodeURIComponent(detail.reportId)}/evidence`,
          { method: "POST", body: form }
        );
        const payload = await response.json();
        if (!response.ok)
          alert(payload.error || "Bukti tindak lanjut gagal diunggah.");
        else {
          event.currentTarget.reset();
          await onUploaded();
        }
        setUploading(false);
      }}
    >
      {!hasImage && (
        <p className="font-semibold text-amber-900 sm:col-span-2">
          Bukti tindak lanjut belum dilengkapi. Tambahkan minimal satu
          foto/screenshot sebelum laporan dapat diselesaikan.
        </p>
      )}
      <label className="text-sm font-medium">
        {hasImage ? "Tambah bukti tambahan" : "Lengkapi bukti"} (maks.{" "}
        {3 - evidence.length} file)
        <input
          name="attachments"
          type="file"
          required
          multiple
          accept=".jpg,.jpeg,.png,.webp,.pdf"
          className="mt-1 w-full rounded-xl border p-3"
        />
      </label>
      <label className="text-sm font-medium">
        Keterangan
        <input
          name="message"
          maxLength={5000}
          className="mt-1 w-full rounded-xl border p-3"
        />
      </label>
      <label className="flex items-center gap-2 text-sm">
        <input name="public" value="true" type="checkbox" defaultChecked />
        Bukti dapat dilihat pelapor
      </label>
      <button
        disabled={uploading}
        className="rounded-xl border border-blue-700 p-3 font-semibold text-blue-800 disabled:opacity-50"
      >
        {uploading
          ? "Mengunggah…"
          : hasImage
            ? "Tambah bukti tambahan"
            : "Lengkapi bukti"}
      </button>
    </form>
  );
}

function ReportCard({
  report,
  credentials,
}: {
  report: TrackedReport;
  credentials?: { reportId: string; pin: string };
}) {
  const [clarificationSent, setClarificationSent] = useState(false);
  const [sending, setSending] = useState(false);
  return (
    <article className="rounded-2xl border bg-white p-5">
      <div className="flex justify-between gap-4">
        <div>
          <p className="font-mono text-sm text-blue-700">{report.reportId}</p>
          <h3 className="font-semibold">{report.title}</h3>
          <p className="text-sm text-slate-500">{report.reportType}</p>
        </div>
        <span className="h-fit rounded-full bg-blue-50 px-3 py-1 text-xs font-semibold text-blue-800">
          {statusLabel[report.status] || report.status}
        </span>
      </div>
      <WorkflowTracker status={report.status} timeline={report.timeline} />
      {report.targetName && (
        <p className="mt-4 text-sm text-slate-600">
          Tujuan: {report.targetName}
        </p>
      )}
      {report.closedReason && (
        <p className="mt-4 rounded-xl bg-red-50 p-3 text-sm text-red-800">
          {report.closedReason}
        </p>
      )}
      {report.officialResponse && (
        <div className="mt-4 rounded-xl border border-emerald-200 bg-emerald-50 p-4">
          <b>Tanggapan resmi</b>
          <p className="mt-1 whitespace-pre-wrap">{report.officialResponse}</p>
        </div>
      )}
      {report.hasHiddenFollowUpEvidence && (
        <p className="mt-4 rounded-xl bg-slate-100 p-3 text-sm text-slate-700">
          Bukti tindak lanjut telah diunggah, tetapi tidak ditampilkan karena
          memuat informasi internal/pribadi.
        </p>
      )}
      {report.clarification && (
        <div className="mt-4 rounded-xl border border-amber-200 bg-amber-50 p-4 text-sm">
          <b>Permintaan klarifikasi putaran {report.clarification.round}</b>
          <p className="mt-1">{report.clarification.request}</p>
          <p className="mt-1 text-xs text-amber-800">
            Tenggat:{" "}
            {new Date(report.clarification.deadlineAt).toLocaleString("id-ID")}
          </p>
          {report.clarification.response && (
            <p className="mt-2">
              Jawaban Anda: {report.clarification.response}
            </p>
          )}
          {report.clarification.canRespond &&
            credentials &&
            !clarificationSent && (
              <form
                className="mt-4 space-y-3"
                onSubmit={async (event) => {
                  event.preventDefault();
                  setSending(true);
                  const data = new FormData(event.currentTarget);
                  data.set("reportId", credentials.reportId);
                  data.set("pin", credentials.pin);
                  const response = await apiFetch(
                    "/api/public/complaints/clarifications",
                    {
                      method: "POST",
                      body: data,
                    },
                    { skipAuthRedirect: true }
                  );
                  const payload = await response.json();
                  if (!response.ok)
                    alert(payload.error || "Klarifikasi gagal dikirim.");
                  else setClarificationSent(true);
                  setSending(false);
                }}
              >
                <textarea
                  name="clarification"
                  required
                  maxLength={5000}
                  placeholder="Tuliskan klarifikasi Anda"
                  className="min-h-28 w-full rounded-xl border bg-white p-3"
                />
                <input
                  name="attachments"
                  type="file"
                  required
                  multiple
                  accept=".jpg,.jpeg,.png,.webp,.pdf"
                  className="w-full rounded-xl border bg-white p-3"
                />
                <button
                  disabled={sending}
                  className="w-full rounded-xl bg-amber-700 p-3 font-semibold text-white disabled:opacity-50"
                >
                  {sending ? "Mengirim…" : "Kirim klarifikasi dan bukti"}
                </button>
              </form>
            )}
          {clarificationSent && (
            <p className="mt-3 font-medium text-emerald-700">
              Klarifikasi berhasil dikirim dan menunggu verifikasi petugas.
            </p>
          )}
        </div>
      )}
      {!!report.attachments?.length && credentials && (
        <div className="mt-4">
          <b className="text-sm">Lampiran yang dapat dilihat</b>
          <div className="mt-2 flex flex-wrap gap-2">
            {report.attachments.map((attachment) => (
              <button
                key={attachment.id}
                type="button"
                onClick={async () => {
                  const response = await apiFetch(
                    `/api/public/complaints/attachments/${attachment.id}`,
                    {
                      headers: {
                        "X-Complaint-ID": credentials.reportId,
                        "X-Complaint-PIN": credentials.pin,
                      },
                    },
                    { skipAuthRedirect: true }
                  );
                  if (!response.ok) {
                    alert("Lampiran tidak dapat diakses.");
                    return;
                  }
                  const url = URL.createObjectURL(await response.blob());
                  const link = document.createElement("a");
                  link.href = url;
                  link.download = attachment.name;
                  link.click();
                  URL.revokeObjectURL(url);
                }}
                className="rounded-xl border border-blue-200 bg-blue-50 px-3 py-2 text-sm text-blue-800"
              >
                {attachment.name}
              </button>
            ))}
          </div>
        </div>
      )}
      {report.timeline && (
        <ol className="mt-4 border-l pl-4">
          {report.timeline.map((item, index) => (
            <li key={index} className="mb-3 text-sm">
              <b>{statusLabel[item.status] || item.status}</b>
              <p>{item.message}</p>
            </li>
          ))}
        </ol>
      )}
    </article>
  );
}
