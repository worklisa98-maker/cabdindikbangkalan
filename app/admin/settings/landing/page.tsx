"use client";

import { useEffect, useMemo, useState } from "react";
import Image from "next/image";
import Link from "next/link";
import { useRouter } from "next/navigation";
import { useQuery } from "@tanstack/react-query";
import { Eye, RefreshCcw, Save } from "lucide-react";

import LogoSipres from "@/app/assets/logo-sipres.png";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import {
  defaultLandingSettings,
  type LandingSettings,
} from "@/lib/landing-settings";
import { Card, CardContent } from "@/components/ui/card";

type LandingSettingsApiResponse = {
  data?: {
    settings?: LandingSettings;
  };
};

const fetchLandingSettingsAdmin = async (): Promise<LandingSettings> => {
  const response = await apiFetch("/admin/master/landing-settings");
  if (!response.ok) {
    throw new Error("Failed to fetch landing settings");
  }
  const payload = (await response.json()) as LandingSettingsApiResponse;
  return payload.data?.settings ?? defaultLandingSettings;
};

export default function LandingSettingsPage() {
  const router = useRouter();
  const { user, status } = useAuth();
  const [form, setForm] = useState<LandingSettings>(defaultLandingSettings);
  const [hasInitializedForm, setHasInitializedForm] = useState(false);
  const [isSaving, setIsSaving] = useState(false);
  const [saveMessage, setSaveMessage] = useState("");
  const [saveError, setSaveError] = useState("");
  const [previewBroken, setPreviewBroken] = useState(false);

  const isAdminMaster = user?.role === "ADMIN_MASTER";
  const canAccess = status === "authenticated" && isAdminMaster;

  const query = useQuery({
    queryKey: ["landing-settings-admin"],
    queryFn: fetchLandingSettingsAdmin,
    enabled: canAccess,
  });

  useEffect(() => {
    if (status === "unauthenticated") {
      router.replace("/login");
    }
  }, [router, status]);

  useEffect(() => {
    if (!query.data || hasInitializedForm) return;
    setForm(query.data);
    setHasInitializedForm(true);
  }, [hasInitializedForm, query.data]);

  useEffect(() => {
    setPreviewBroken(false);
  }, [form.hero_image_url]);

  const previewUrl = form.hero_image_url.trim();
  const isDirty = useMemo(() => {
    const baseline = query.data ?? defaultLandingSettings;
    return JSON.stringify(form) !== JSON.stringify(baseline);
  }, [form, query.data]);

  const updateField = <K extends keyof LandingSettings>(
    key: K,
    value: LandingSettings[K]
  ) => {
    setForm((prev) => ({ ...prev, [key]: value }));
    setSaveMessage("");
    setSaveError("");
  };

  const handleSave = async () => {
    setIsSaving(true);
    setSaveMessage("");
    setSaveError("");
    try {
      const response = await apiFetch("/admin/master/landing-settings", {
        method: "PUT",
        body: JSON.stringify({ settings: form }),
      });
      if (!response.ok) {
        throw new Error("save_failed");
      }
      const payload = (await response.json()) as LandingSettingsApiResponse;
      const next = payload.data?.settings ?? form;
      setForm(next);
      setSaveMessage("Konten landing berhasil disimpan.");
      await query.refetch();
    } catch {
      setSaveError("Gagal menyimpan konten landing.");
    } finally {
      setIsSaving(false);
    }
  };

  if (status === "loading") {
    return (
      <div className="rounded-xl border border-dashed border-slate-200 bg-white p-6 text-sm text-slate-500">
        Memuat akses...
      </div>
    );
  }

  if (status === "authenticated" && !isAdminMaster) {
    return (
      <div className="space-y-4">
        <div className="rounded-xl border border-red-200 bg-white p-6 text-sm text-red-600">
          Halaman ini hanya dapat diakses oleh ADMIN_MASTER.
        </div>
        <Link
          href="/"
          className="inline-flex items-center rounded-lg border border-slate-300 px-3 py-2 text-sm font-medium text-slate-700 hover:bg-slate-50"
        >
          Kembali ke Landing
        </Link>
      </div>
    );
  }

  if (!canAccess) {
    return (
      <div className="rounded-xl border border-dashed border-slate-200 bg-white p-6 text-sm text-slate-500">
        Mengalihkan ke login...
      </div>
    );
  }

  return (
    <section className="space-y-6">
      <header className="flex flex-wrap items-start justify-between gap-4">
        <div>
          <p className="text-xs font-semibold tracking-[0.2em] text-slate-500 uppercase">
            ADMIN MASTER
          </p>
          <h1 className="mt-2 text-2xl font-semibold text-slate-950">
            Settings Landing SIPRES
          </h1>
          <p className="mt-2 text-sm text-slate-600">
            Kelola konten hero, visual, dan contact person untuk landing page <code>/</code>.
          </p>
        </div>
        <div className="flex flex-wrap gap-2">
          <Link
            href="/"
            className="inline-flex items-center gap-2 rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50"
          >
            <Eye className="h-4 w-4" />
            Preview Landing
          </Link>
          <button
            type="button"
            onClick={() => {
              setForm(query.data ?? defaultLandingSettings);
              setSaveMessage("");
              setSaveError("");
            }}
            className="inline-flex items-center gap-2 rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:opacity-60"
            disabled={isSaving}
          >
            <RefreshCcw className="h-4 w-4" />
            Reset Form
          </button>
          <button
            type="button"
            onClick={handleSave}
            disabled={isSaving || query.isFetching}
            className="inline-flex items-center gap-2 rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-3 py-2 text-sm font-semibold text-white transition hover:opacity-95 disabled:opacity-60"
          >
            <Save className="h-4 w-4" />
            {isSaving ? "Menyimpan..." : "Simpan"}
          </button>
        </div>
      </header>

      {query.isLoading ? (
        <div className="rounded-xl border border-dashed border-slate-200 bg-white p-6 text-sm text-slate-500">
          Memuat settings landing...
        </div>
      ) : query.isError ? (
        <div className="rounded-xl border border-red-200 bg-white p-6 text-sm text-red-600">
          Gagal memuat settings landing.
        </div>
      ) : (
        <div className="grid gap-6 xl:grid-cols-[1.05fr_0.95fr]">
          <Card className="py-0">
            <CardContent className="space-y-5 p-5">
              <div className="grid gap-2">
                <label className="text-sm font-semibold text-slate-900" htmlFor="hero_title">
                  Hero Title
                </label>
                <textarea
                  id="hero_title"
                  rows={3}
                  value={form.hero_title}
                  onChange={(event) => updateField("hero_title", event.target.value)}
                  className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                />
              </div>

              <div className="grid gap-2">
                <label className="text-sm font-semibold text-slate-900" htmlFor="hero_subtitle">
                  Hero Subtitle
                </label>
                <textarea
                  id="hero_subtitle"
                  rows={4}
                  value={form.hero_subtitle}
                  onChange={(event) => updateField("hero_subtitle", event.target.value)}
                  className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                />
              </div>

              <div className="grid gap-2">
                <label className="text-sm font-semibold text-slate-900" htmlFor="hero_image_url">
                  Hero Image URL / Path
                </label>
                <input
                  id="hero_image_url"
                  type="text"
                  value={form.hero_image_url}
                  onChange={(event) => updateField("hero_image_url", event.target.value)}
                  placeholder="/uploads/hero/sipres-banner.jpg"
                  className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                />
                <div className="flex flex-wrap gap-2">
                  <button
                    type="button"
                    onClick={() => updateField("hero_image_url", "")}
                    className="rounded-lg border border-slate-300 px-3 py-1.5 text-xs font-semibold text-slate-700 hover:bg-slate-50"
                  >
                    Reset Hero ke Fallback
                  </button>
                </div>
              </div>

              <div className="grid gap-2">
                <label className="text-sm font-semibold text-slate-900" htmlFor="hero_image_alt">
                  Hero Image Alt
                </label>
                <input
                  id="hero_image_alt"
                  type="text"
                  value={form.hero_image_alt}
                  onChange={(event) => updateField("hero_image_alt", event.target.value)}
                  className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                />
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div className="grid gap-2">
                  <label className="text-sm font-semibold text-slate-900" htmlFor="contact_whatsapp">
                    WhatsApp
                  </label>
                  <input
                    id="contact_whatsapp"
                    type="text"
                    value={form.contact_whatsapp}
                    onChange={(event) => updateField("contact_whatsapp", event.target.value)}
                    className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  />
                </div>
                <div className="grid gap-2">
                  <label className="text-sm font-semibold text-slate-900" htmlFor="contact_phone">
                    Telepon
                  </label>
                  <input
                    id="contact_phone"
                    type="text"
                    value={form.contact_phone}
                    onChange={(event) => updateField("contact_phone", event.target.value)}
                    className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  />
                </div>
              </div>

              <div className="grid gap-4 sm:grid-cols-2">
                <div className="grid gap-2">
                  <label className="text-sm font-semibold text-slate-900" htmlFor="contact_email">
                    Email
                  </label>
                  <input
                    id="contact_email"
                    type="email"
                    value={form.contact_email}
                    onChange={(event) => updateField("contact_email", event.target.value)}
                    className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  />
                </div>
                <div className="grid gap-2">
                  <label className="text-sm font-semibold text-slate-900" htmlFor="guidebook_url">
                    Guidebook URL (opsional)
                  </label>
                  <input
                    id="guidebook_url"
                    type="text"
                    value={form.guidebook_url}
                    onChange={(event) => updateField("guidebook_url", event.target.value)}
                    placeholder="/guide_books"
                    className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                  />
                </div>
              </div>

              <div className="grid gap-2">
                <label className="text-sm font-semibold text-slate-900" htmlFor="contact_address">
                  Alamat
                </label>
                <textarea
                  id="contact_address"
                  rows={3}
                  value={form.contact_address}
                  onChange={(event) => updateField("contact_address", event.target.value)}
                  className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                />
              </div>

              <div className="grid gap-2">
                <label className="text-sm font-semibold text-slate-900" htmlFor="contact_hours">
                  Jam Layanan
                </label>
                <input
                  id="contact_hours"
                  type="text"
                  value={form.contact_hours}
                  onChange={(event) => updateField("contact_hours", event.target.value)}
                  className="w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm outline-none transition focus:border-slate-500 focus:ring-2 focus:ring-slate-200"
                />
              </div>
            </CardContent>
          </Card>

          <div className="space-y-6">
            <Card className="py-0">
              <CardContent className="space-y-4 p-5">
                <div className="flex items-center gap-3">
                  <Image
                    src={LogoSipres}
                    alt="Logo SIPRES"
                    width={48}
                    height={48}
                    className="h-12 w-12 rounded-xl object-contain"
                  />
                  <div>
                    <p className="text-sm font-semibold text-slate-900">SIPRES</p>
                    <p className="text-xs text-slate-600">
                      Logo hero tetap menggunakan asset internal (non-CMS)
                    </p>
                  </div>
                </div>

                <div className="rounded-2xl border border-slate-200 bg-slate-50 p-4">
                  <p className="text-xs font-semibold tracking-[0.2em] text-slate-500 uppercase">
                    Preview Hero Visual
                  </p>
                  <div className="mt-3 overflow-hidden rounded-xl border border-slate-200 bg-white">
                    {previewUrl && !previewBroken ? (
                      <Image
                        unoptimized
                        width={1200}
                        height={900}
                        src={previewUrl}
                        alt={form.hero_image_alt || "Preview hero image"}
                        className="h-56 w-full object-cover"
                        onError={() => setPreviewBroken(true)}
                      />
                    ) : (
                      <div className="flex h-56 items-center justify-center bg-gradient-to-br from-slate-900 to-sky-900 px-4 text-center text-sm text-white/80">
                        Fallback visual internal akan digunakan di landing jika hero image kosong / tidak valid.
                      </div>
                    )}
                  </div>
                  {previewUrl && previewBroken ? (
                    <p className="mt-2 text-xs text-amber-700">
                      Preview gagal dimuat. Pastikan path/URL internal valid.
                    </p>
                  ) : null}
                </div>
              </CardContent>
            </Card>

            <Card className="py-0">
              <CardContent className="space-y-3 p-5">
                <p className="text-sm font-semibold text-slate-900">Status Simpan</p>
                <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-700">
                  {isDirty ? "Ada perubahan yang belum disimpan." : "Form sinkron dengan data tersimpan."}
                </div>
                {saveMessage ? (
                  <div className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-800">
                    {saveMessage}
                  </div>
                ) : null}
                {saveError ? (
                  <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
                    {saveError}
                  </div>
                ) : null}
              </CardContent>
            </Card>
          </div>
        </div>
      )}
    </section>
  );
}

