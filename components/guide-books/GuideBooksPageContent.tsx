"use client";

import NextImage from "next/image";

import { useMemo, useState } from "react";
import { useQuery } from "@tanstack/react-query";
import {
  ArrowDown,
  ArrowUp,
  BookOpen,
  Eye,
  ImagePlus,
  Plus,
  RefreshCw,
  Trash2,
  X,
} from "lucide-react";

import { GuideBookHtmlEditor } from "@/components/guide-books/GuideBookHtmlEditor";
import { GuideBookViewerModal } from "@/components/guide-books/GuideBookViewerModal";
import { Card, CardContent } from "@/components/ui/card";
import { apiFetch } from "@/lib/api/client";
import { useAuth, type AuthUser } from "@/lib/auth/client";
import {
  GUIDE_BOOK_COLUMN_RATIOS,
  GUIDE_BOOK_LAYOUT_TYPES,
  GUIDE_BOOK_PLACEMENTS,
  guideBookColumnRatioOptions,
  guideBookLayoutOptions,
  guideBookLayoutUsesColumnRatio,
  guideBookLayoutUsesImage1,
  guideBookLayoutUsesImage2,
  guideBookLayoutUsesText,
  guideBookPlacementLabels,
  guideBookPlacementOptions,
  type GuideBookColumnRatio,
  type GuideBookItem,
  type GuideBookLayoutType,
  type GuideBookSlideItem,
} from "@/lib/guide-books";

type GuideBookRole = "ALL" | AuthUser["role"] | "ADMIN_CABDIN" | "VALIDATOR";
type ApiResponse<T> = { data: T };

type GuideBookDraft = {
  id: number | null;
  role: GuideBookRole;
  title: string;
  contentHtml: string;
  sortOrder: number;
  isActive: boolean;
  placementKey: string;
  isPublic: boolean;
  slides: GuideBookSlideItem[];
};

const roleLabels: Record<GuideBookRole, string> = {
  ALL: "Semua Role",
  GASPOL_VOTER: "Pemilih GASPOL",
  SCHOOL_ADMIN: "Admin Sekolah",
  USER_SEKOLAH_GASPOL: "User Sekolah GASPOL",
  USER_MGMP: "User MGMP",
  ADMIN: "Admin Cabdin",
  ADMIN_CABDIN: "Admin Cabdin",
  ADMIN_MASTER: "Admin Master",
  PENGAWAS: "Pengawas",
  VALIDATOR: "Validator",
};

const readApiMessage = (value: unknown) =>
  value && typeof value === "object" && "message" in value
    ? typeof (value as { message?: unknown }).message === "string"
      ? (value as { message: string }).message
      : ""
    : "";

const formatDateTime = (value?: string) => {
  if (!value) return "-";
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? value : date.toLocaleString("id-ID");
};

const emptySlide = (
  sortOrder: number,
  layoutType: GuideBookLayoutType = GUIDE_BOOK_LAYOUT_TYPES.FULL_TEXT
): GuideBookSlideItem => ({
  id: Date.now() + Math.floor(Math.random() * 1000),
  layoutType,
  textHtml: "",
  image1Url: "",
  image2Url: "",
  columnRatio: GUIDE_BOOK_COLUMN_RATIOS.EQUAL,
  sortOrder,
});

const resetSlideOrders = (slides: GuideBookSlideItem[]) =>
  slides.map((slide, index) => ({ ...slide, sortOrder: (index + 1) * 10 }));

const emptyDraft = (): GuideBookDraft => ({
  id: null,
  role: "ALL",
  title: "",
  contentHtml: "<p></p>",
  sortOrder: 0,
  isActive: true,
  placementKey: GUIDE_BOOK_PLACEMENTS.NONE,
  isPublic: false,
  slides: [],
});

const fetchGuideBooks = async (): Promise<GuideBookItem[]> => {
  const response = await apiFetch("/api/guide-books");
  if (!response.ok) throw new Error("Gagal memuat panduan.");
  const payload = (await response.json()) as ApiResponse<GuideBookItem[]>;
  return payload.data ?? [];
};

const fetchAdminGuideBooks = async (params: {
  role: string;
  includeInactive: boolean;
}): Promise<GuideBookItem[]> => {
  const query = new URLSearchParams();
  if (params.role && params.role !== "ALL_ROLES")
    query.set("role", params.role);
  if (params.includeInactive) query.set("includeInactive", "true");
  const suffix = query.toString();
  const response = await apiFetch(
    suffix ? `/api/admin/guide-books?${suffix}` : "/api/admin/guide-books"
  );
  if (!response.ok) throw new Error("Gagal memuat CMS panduan.");
  const payload = (await response.json()) as ApiResponse<GuideBookItem[]>;
  return payload.data ?? [];
};

const saveGuideBook = async (draft: GuideBookDraft) => {
  const response = await apiFetch(
    draft.id ? `/api/admin/guide-books/${draft.id}` : "/api/admin/guide-books",
    {
      method: draft.id ? "PUT" : "POST",
      body: JSON.stringify({
        role: draft.role,
        title: draft.title,
        contentHtml: draft.contentHtml,
        sortOrder: draft.sortOrder,
        isActive: draft.isActive,
        placementKey: draft.placementKey,
        isPublic: draft.isPublic,
        slides: draft.slides.map((slide) => ({
          layoutType: slide.layoutType,
          textHtml: slide.textHtml,
          image1Url: slide.image1Url,
          image2Url: slide.image2Url,
          columnRatio: slide.columnRatio,
          sortOrder: slide.sortOrder,
        })),
      }),
    }
  );
  const body = (await response.json().catch(() => null)) as
    | ApiResponse<GuideBookItem>
    | { message?: string }
    | null;
  if (!response.ok) {
    throw new Error(readApiMessage(body) || "Gagal menyimpan panduan.");
  }
  return (body as ApiResponse<GuideBookItem> | null)?.data ?? null;
};

const deleteGuideBook = async (id: number) => {
  const response = await apiFetch(`/api/admin/guide-books/${id}`, {
    method: "DELETE",
  });
  if (!response.ok) throw new Error("Gagal menghapus panduan.");
};

const uploadGuideBookImage = async (file: File) => {
  const formData = new FormData();
  formData.append("file", file);
  const response = await apiFetch("/api/admin/guide-books/images", {
    method: "POST",
    body: formData,
  });
  const body = (await response.json().catch(() => null)) as
    | ApiResponse<{ url: string }>
    | { message?: string }
    | null;
  if (!response.ok) {
    throw new Error(readApiMessage(body) || "Upload gambar gagal.");
  }
  const url = (body as ApiResponse<{ url: string }> | null)?.data?.url;
  if (!url) throw new Error("Upload gambar gagal.");
  return url;
};

function SlideImageField({
  label,
  value,
  onChange,
}: {
  label: string;
  value: string;
  onChange: (value: string) => void;
}) {
  const [isUploading, setIsUploading] = useState(false);
  const [error, setError] = useState("");

  const handleFile = async (event: React.ChangeEvent<HTMLInputElement>) => {
    const file = event.target.files?.[0];
    event.target.value = "";
    if (!file) return;
    setIsUploading(true);
    setError("");
    try {
      onChange(await uploadGuideBookImage(file));
    } catch (uploadError) {
      setError(
        uploadError instanceof Error
          ? uploadError.message
          : "Upload gambar gagal."
      );
    } finally {
      setIsUploading(false);
    }
  };

  return (
    <div className="space-y-2 rounded-xl border border-zinc-200 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="text-sm font-medium text-zinc-800">{label}</span>
        <label className="inline-flex cursor-pointer items-center gap-2 rounded-lg border border-slate-300 bg-white px-3 py-2 text-xs font-semibold text-zinc-700 hover:bg-slate-50">
          <ImagePlus className="h-4 w-4" />
          {isUploading ? "Upload..." : value ? "Ganti" : "Upload"}
          <input
            type="file"
            accept="image/png,image/jpeg,image/webp"
            className="hidden"
            onChange={handleFile}
          />
        </label>
      </div>
      {value ? (
        <>
          <div className="flex h-40 items-center justify-center rounded-xl border border-zinc-200 bg-zinc-50 p-2">
            <NextImage
              unoptimized
              width={1200}
              height={900}
              src={value}
              alt={label}
              className="h-full w-full object-contain"
            />
          </div>
          <button
            type="button"
            onClick={() => onChange("")}
            className="text-xs font-semibold text-red-600"
          >
            Hapus gambar
          </button>
        </>
      ) : (
        <div className="flex h-24 items-center justify-center rounded-xl border border-dashed border-zinc-200 bg-zinc-50 text-sm text-zinc-400">
          Slot gambar kosong
        </div>
      )}
      {error ? <p className="text-xs text-red-600">{error}</p> : null}
    </div>
  );
}

export function GuideBooksPageContent() {
  const { user } = useAuth();
  const isAdminMaster = user?.role === "ADMIN_MASTER";
  const [isManagerOpen, setIsManagerOpen] = useState(false);
  const [adminRoleFilter, setAdminRoleFilter] = useState("ALL_ROLES");
  const [includeInactive, setIncludeInactive] = useState(false);
  const [draft, setDraft] = useState<GuideBookDraft>(emptyDraft);
  const [formMessage, setFormMessage] = useState("");
  const [formError, setFormError] = useState("");
  const [isSaving, setIsSaving] = useState(false);
  const [isDeleting, setIsDeleting] = useState(false);
  const [viewerGuide, setViewerGuide] = useState<GuideBookItem | null>(null);

  const viewerQuery = useQuery({
    queryKey: ["guide-books-view"],
    queryFn: fetchGuideBooks,
  });

  const adminQuery = useQuery({
    queryKey: ["guide-books-admin", adminRoleFilter, includeInactive],
    queryFn: () =>
      fetchAdminGuideBooks({ role: adminRoleFilter, includeInactive }),
    enabled: isAdminMaster && isManagerOpen,
  });

  const viewerGuides = viewerQuery.data ?? [];
  const adminGuides = useMemo(() => adminQuery.data ?? [], [adminQuery.data]);
  const selectedAdminGuide = useMemo(
    () => adminGuides.find((item) => item.id === draft.id) ?? null,
    [adminGuides, draft.id]
  );
  const isSlideMode =
    draft.placementKey === GUIDE_BOOK_PLACEMENTS.LOMBA_POSTER_KARTINI;

  const startCreate = () => {
    setDraft(emptyDraft());
    setFormMessage("");
    setFormError("");
  };

  const startEdit = (item: GuideBookItem) => {
    setDraft({
      id: item.id,
      role: item.role as GuideBookRole,
      title: item.title,
      contentHtml: item.contentHtml || "<p></p>",
      sortOrder: item.sortOrder ?? 0,
      isActive: item.isActive ?? true,
      placementKey: item.placementKey ?? GUIDE_BOOK_PLACEMENTS.NONE,
      isPublic: item.isPublic ?? false,
      slides: resetSlideOrders(item.slides ?? []),
    });
    setFormMessage("");
    setFormError("");
  };

  const updateSlide = (
    index: number,
    updater: (slide: GuideBookSlideItem) => GuideBookSlideItem
  ) =>
    setDraft((current) => ({
      ...current,
      slides: current.slides.map((slide, slideIndex) =>
        slideIndex === index ? updater(slide) : slide
      ),
    }));

  const moveSlide = (index: number, direction: -1 | 1) =>
    setDraft((current) => {
      const slides = [...current.slides];
      const nextIndex = index + direction;
      if (nextIndex < 0 || nextIndex >= slides.length) return current;
      const [slide] = slides.splice(index, 1);
      slides.splice(nextIndex, 0, slide);
      return { ...current, slides: resetSlideOrders(slides) };
    });

  const removeSlide = (index: number) =>
    setDraft((current) => ({
      ...current,
      slides: resetSlideOrders(
        current.slides.filter((_, slideIndex) => slideIndex !== index)
      ),
    }));

  const save = async () => {
    setFormMessage("");
    setFormError("");
    if (!draft.title.trim()) {
      setFormError("Judul wajib diisi.");
      return;
    }
    if (isSlideMode && draft.slides.length === 0) {
      setFormError("Juknis placement Kartini minimal memiliki 1 slide.");
      return;
    }
    if (!isSlideMode && !draft.contentHtml.trim()) {
      setFormError("Konten wajib diisi.");
      return;
    }
    setIsSaving(true);
    try {
      const saved = await saveGuideBook(draft);
      setFormMessage("Panduan berhasil disimpan.");
      await Promise.all([viewerQuery.refetch(), adminQuery.refetch()]);
      if (saved) startEdit(saved);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Gagal menyimpan.");
    } finally {
      setIsSaving(false);
    }
  };

  const remove = async () => {
    if (draft.id == null || !window.confirm("Hapus panduan ini?")) return;
    setIsDeleting(true);
    setFormMessage("");
    setFormError("");
    try {
      await deleteGuideBook(draft.id);
      setFormMessage("Panduan berhasil dihapus.");
      setDraft(emptyDraft());
      await Promise.all([viewerQuery.refetch(), adminQuery.refetch()]);
    } catch (error) {
      setFormError(error instanceof Error ? error.message : "Gagal menghapus.");
    } finally {
      setIsDeleting(false);
    }
  };

  return (
    <section className="space-y-6">
      <header className="flex flex-col gap-3 rounded-xl border border-zinc-200 bg-white p-6 shadow-sm md:flex-row md:items-center md:justify-between">
        <div className="space-y-2">
          <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
            Panduan Pengguna
          </p>
          <h1 className="flex items-center gap-2 text-2xl font-semibold text-zinc-900">
            <BookOpen className="h-6 w-6" />
            Guide Books
          </h1>
          <p className="text-sm text-zinc-600">
            Panduan ditampilkan sesuai role akun yang sedang login.
          </p>
        </div>
        {isAdminMaster ? (
          <button
            type="button"
            onClick={() => {
              setIsManagerOpen(true);
              startCreate();
            }}
            className="inline-flex items-center gap-2 rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white"
          >
            <Plus className="h-4 w-4" />
            Kelola Panduan
          </button>
        ) : null}
      </header>

      {viewerQuery.isLoading ? (
        <Card>
          <CardContent className="text-sm text-zinc-500">
            Memuat panduan...
          </CardContent>
        </Card>
      ) : viewerQuery.isError ? (
        <Card>
          <CardContent className="space-y-3 text-sm">
            <p className="text-red-600">Gagal memuat panduan.</p>
            <button
              type="button"
              onClick={() => void viewerQuery.refetch()}
              className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-3 py-2 font-medium text-zinc-700"
            >
              <RefreshCw className="h-4 w-4" />
              Muat ulang
            </button>
          </CardContent>
        </Card>
      ) : viewerGuides.length === 0 ? (
        <Card>
          <CardContent className="text-sm text-zinc-500">
            Belum ada panduan aktif untuk role Anda.
          </CardContent>
        </Card>
      ) : (
        <div className="space-y-4">
          {viewerGuides.map((guide) =>
            guide.slides.length > 0 ? (
              <Card key={guide.id}>
                <CardContent className="flex flex-col gap-4 p-5 md:flex-row md:items-center md:justify-between">
                  <div className="space-y-1">
                    <p className="text-xs font-semibold tracking-[0.12em] text-zinc-500 uppercase">
                      {roleLabels[guide.role as GuideBookRole] ?? guide.role}
                    </p>
                    <h2 className="text-base font-semibold text-zinc-900">
                      {guide.title}
                    </h2>
                    <p className="text-xs text-zinc-500">
                      {guideBookPlacementLabels[guide.placementKey] ??
                        guide.placementKey}{" "}
                      | Diperbarui {formatDateTime(guide.updatedAt)}
                    </p>
                  </div>
                  <button
                    type="button"
                    onClick={() => setViewerGuide(guide)}
                    className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700"
                  >
                    <Eye className="h-4 w-4" />
                    Buka Viewer
                  </button>
                </CardContent>
              </Card>
            ) : (
              <Card key={guide.id} className="py-0">
                <details className="group">
                  <summary className="flex cursor-pointer list-none items-center justify-between gap-3 px-6 py-4">
                    <div className="space-y-1">
                      <p className="text-xs font-semibold tracking-[0.12em] text-zinc-500 uppercase">
                        {roleLabels[guide.role as GuideBookRole] ?? guide.role}
                      </p>
                      <h2 className="text-base font-semibold text-zinc-900">
                        {guide.title}
                      </h2>
                      <p className="text-xs text-zinc-500">
                        Diperbarui {formatDateTime(guide.updatedAt)}
                      </p>
                    </div>
                    <span className="text-xs font-medium text-zinc-500 group-open:hidden">
                      Buka
                    </span>
                    <span className="hidden text-xs font-medium text-zinc-500 group-open:inline">
                      Tutup
                    </span>
                  </summary>
                  <div className="border-t border-zinc-200 px-6 py-4">
                    <div
                      className="max-w-none text-sm leading-6 text-zinc-800 [&_a]:text-blue-700 [&_a]:underline [&_blockquote]:my-3 [&_blockquote]:border-l-4 [&_blockquote]:border-zinc-300 [&_blockquote]:pl-3 [&_code]:rounded [&_code]:bg-zinc-100 [&_code]:px-1 [&_code]:py-0.5 [&_h1]:mt-4 [&_h1]:text-2xl [&_h1]:font-semibold [&_h2]:mt-4 [&_h2]:text-xl [&_h2]:font-semibold [&_h3]:mt-4 [&_h3]:text-lg [&_h3]:font-semibold [&_img]:my-3 [&_img]:h-auto [&_img]:max-w-full [&_li]:my-1 [&_ol]:my-2 [&_ol]:list-decimal [&_ol]:pl-6 [&_p]:my-2 [&_pre]:my-3 [&_pre]:overflow-x-auto [&_pre]:rounded [&_pre]:bg-zinc-900 [&_pre]:p-3 [&_pre]:text-zinc-100 [&_ul]:my-2 [&_ul]:list-disc [&_ul]:pl-6"
                      dangerouslySetInnerHTML={{ __html: guide.contentHtml }}
                    />
                  </div>
                </details>
              </Card>
            )
          )}
        </div>
      )}

      {isAdminMaster && isManagerOpen ? (
        <div className="fixed inset-0 z-50 flex items-stretch justify-end bg-black/30 p-0 sm:p-4">
          <button
            type="button"
            onClick={() => setIsManagerOpen(false)}
            className="hidden flex-1 sm:block"
            aria-label="Tutup overlay"
          />
          <section className="flex h-full w-full max-w-7xl flex-col overflow-hidden rounded-none border border-zinc-200 bg-white shadow-2xl sm:rounded-xl">
            <div className="flex items-center justify-between border-b border-zinc-200 px-4 py-3">
              <div>
                <h2 className="text-lg font-semibold text-zinc-900">
                  Kelola Panduan
                </h2>
                <p className="text-xs text-zinc-500">
                  Placement Kartini memakai slide Juknis terstruktur.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsManagerOpen(false)}
                className="rounded-lg border border-slate-300 bg-white p-2 text-zinc-600"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
            <div className="grid min-h-0 flex-1 grid-cols-1 md:grid-cols-[320px_1fr]">
              <aside className="min-h-0 overflow-y-auto border-b border-zinc-200 p-4 md:border-r md:border-b-0">
                <div className="space-y-3">
                  <div className="flex flex-wrap gap-2">
                    <button
                      type="button"
                      onClick={startCreate}
                      className="inline-flex items-center gap-2 rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-3 py-2 text-sm font-semibold text-white"
                    >
                      <Plus className="h-4 w-4" />
                      Baru
                    </button>
                    <button
                      type="button"
                      onClick={() => void adminQuery.refetch()}
                      className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-3 py-2 text-sm font-medium text-zinc-700"
                    >
                      <RefreshCw className="h-4 w-4" />
                      Refresh
                    </button>
                  </div>
                  <select
                    value={adminRoleFilter}
                    onChange={(event) => setAdminRoleFilter(event.target.value)}
                    className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                  >
                    <option value="ALL_ROLES">Semua</option>
                    <option value="ALL">ALL</option>
                    <option value="SCHOOL_ADMIN">SCHOOL_ADMIN</option>
                    <option value="USER_SEKOLAH_GASPOL">
                      USER_SEKOLAH_GASPOL
                    </option>
                    <option value="ADMIN">ADMIN</option>
                    <option value="ADMIN_CABDIN">ADMIN_CABDIN</option>
                    <option value="ADMIN_MASTER">ADMIN_MASTER</option>
                    <option value="PENGAWAS">PENGAWAS</option>
                    <option value="VALIDATOR">VALIDATOR</option>
                  </select>
                  <label className="flex items-center gap-2 text-sm text-zinc-700">
                    <input
                      type="checkbox"
                      checked={includeInactive}
                      onChange={(event) =>
                        setIncludeInactive(event.target.checked)
                      }
                    />
                    Tampilkan inactive
                  </label>
                  {adminQuery.isLoading ? (
                    <p className="text-sm text-zinc-500">
                      Memuat daftar CMS...
                    </p>
                  ) : adminQuery.isError ? (
                    <p className="text-sm text-red-600">
                      Gagal memuat daftar CMS.
                    </p>
                  ) : adminGuides.length === 0 ? (
                    <p className="text-sm text-zinc-500">
                      Tidak ada data panduan.
                    </p>
                  ) : (
                    <div className="space-y-2">
                      {adminGuides.map((item) => {
                        const isSelected = draft.id === item.id;
                        return (
                          <button
                            key={item.id}
                            type="button"
                            onClick={() => startEdit(item)}
                            className={`w-full rounded-md border px-3 py-2 text-left ${
                              isSelected
                                ? "border-zinc-900 bg-zinc-900 text-white"
                                : "border-zinc-200 bg-white text-zinc-800"
                            }`}
                          >
                            <div className="truncate text-sm font-semibold">
                              {item.title}
                            </div>
                            <div
                              className={`mt-1 space-y-1 text-xs ${
                                isSelected ? "text-white/80" : "text-zinc-500"
                              }`}
                            >
                              <div>
                                {item.role} | sort {item.sortOrder}
                              </div>
                              <div>
                                {guideBookPlacementLabels[item.placementKey] ??
                                  "Tanpa placement"}
                              </div>
                            </div>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              </aside>
              <div className="min-h-0 overflow-y-auto p-4">
                <div className="space-y-4">
                  <div className="grid gap-4 xl:grid-cols-[1fr_180px_220px_120px]">
                    <input
                      value={draft.title}
                      onChange={(event) =>
                        setDraft((prev) => ({
                          ...prev,
                          title: event.target.value,
                        }))
                      }
                      className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                      placeholder="Judul panduan"
                    />
                    <select
                      value={draft.role}
                      onChange={(event) =>
                        setDraft((prev) => ({
                          ...prev,
                          role: event.target.value as GuideBookRole,
                        }))
                      }
                      className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                    >
                      <option value="ALL">ALL</option>
                      <option value="SCHOOL_ADMIN">SCHOOL_ADMIN</option>
                      <option value="USER_SEKOLAH_GASPOL">
                        USER_SEKOLAH_GASPOL
                      </option>
                      <option value="ADMIN">ADMIN</option>
                      <option value="ADMIN_CABDIN">ADMIN_CABDIN</option>
                      <option value="ADMIN_MASTER">ADMIN_MASTER</option>
                      <option value="PENGAWAS">PENGAWAS</option>
                      <option value="VALIDATOR">VALIDATOR</option>
                    </select>
                    <select
                      value={draft.placementKey}
                      onChange={(event) =>
                        setDraft((prev) => ({
                          ...prev,
                          placementKey: event.target.value,
                          slides:
                            event.target.value ===
                              GUIDE_BOOK_PLACEMENTS.LOMBA_POSTER_KARTINI &&
                            prev.slides.length === 0
                              ? [emptySlide(10)]
                              : prev.slides,
                        }))
                      }
                      className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                    >
                      {guideBookPlacementOptions.map((option) => (
                        <option
                          key={option.value || "none"}
                          value={option.value}
                        >
                          {option.label}
                        </option>
                      ))}
                    </select>
                    <input
                      type="number"
                      value={draft.sortOrder}
                      onChange={(event) =>
                        setDraft((prev) => ({
                          ...prev,
                          sortOrder: Number(event.target.value) || 0,
                        }))
                      }
                      className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                    />
                  </div>
                  <div className="flex flex-wrap gap-4">
                    <label className="flex items-center gap-2 text-sm text-zinc-700">
                      <input
                        type="checkbox"
                        checked={draft.isActive}
                        onChange={(event) =>
                          setDraft((prev) => ({
                            ...prev,
                            isActive: event.target.checked,
                          }))
                        }
                      />
                      Aktif
                    </label>
                    <label className="flex items-center gap-2 text-sm text-zinc-700">
                      <input
                        type="checkbox"
                        checked={draft.isPublic}
                        onChange={(event) =>
                          setDraft((prev) => ({
                            ...prev,
                            isPublic: event.target.checked,
                          }))
                        }
                      />
                      Public
                    </label>
                  </div>
                  {isSlideMode ? (
                    <div className="space-y-4 rounded-2xl border border-zinc-200 bg-zinc-50 p-4">
                      <div className="flex items-center justify-between gap-3">
                        <div>
                          <h3 className="text-base font-semibold text-zinc-900">
                            Slides Juknis
                          </h3>
                          <p className="text-sm text-zinc-500">
                            Maksimal 10 slide.
                          </p>
                        </div>
                        <button
                          type="button"
                          onClick={() =>
                            setDraft((current) => ({
                              ...current,
                              slides: [
                                ...current.slides,
                                emptySlide((current.slides.length + 1) * 10),
                              ],
                            }))
                          }
                          disabled={draft.slides.length >= 10}
                          className="inline-flex items-center gap-2 rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-3 py-2 text-sm font-semibold text-white disabled:opacity-60"
                        >
                          <Plus className="h-4 w-4" />
                          Tambah Slide
                        </button>
                      </div>
                      {draft.slides.map((slide, index) => (
                        <Card key={slide.id}>
                          <CardContent className="space-y-4 p-4">
                            <div className="flex items-center justify-between">
                              <p className="text-sm font-semibold text-zinc-900">
                                Slide {index + 1}
                              </p>
                              <div className="flex items-center gap-2">
                                <button
                                  type="button"
                                  onClick={() => moveSlide(index, -1)}
                                  disabled={index === 0}
                                  className="rounded-lg border border-zinc-200 bg-white p-2 disabled:opacity-40"
                                >
                                  <ArrowUp className="h-4 w-4" />
                                </button>
                                <button
                                  type="button"
                                  onClick={() => moveSlide(index, 1)}
                                  disabled={index === draft.slides.length - 1}
                                  className="rounded-lg border border-zinc-200 bg-white p-2 disabled:opacity-40"
                                >
                                  <ArrowDown className="h-4 w-4" />
                                </button>
                                <button
                                  type="button"
                                  onClick={() => removeSlide(index)}
                                  className="rounded-lg border border-red-200 bg-white p-2 text-red-600"
                                >
                                  <Trash2 className="h-4 w-4" />
                                </button>
                              </div>
                            </div>
                            <div className="grid gap-4 md:grid-cols-2">
                              <select
                                value={slide.layoutType}
                                onChange={(event) =>
                                  updateSlide(index, (current) => ({
                                    ...current,
                                    layoutType: event.target
                                      .value as GuideBookLayoutType,
                                    columnRatio: guideBookLayoutUsesColumnRatio(
                                      event.target.value as GuideBookLayoutType
                                    )
                                      ? current.columnRatio
                                      : GUIDE_BOOK_COLUMN_RATIOS.EQUAL,
                                    image1Url: guideBookLayoutUsesImage1(
                                      event.target.value as GuideBookLayoutType
                                    )
                                      ? current.image1Url
                                      : "",
                                    image2Url: guideBookLayoutUsesImage2(
                                      event.target.value as GuideBookLayoutType
                                    )
                                      ? current.image2Url
                                      : "",
                                    textHtml: guideBookLayoutUsesText(
                                      event.target.value as GuideBookLayoutType
                                    )
                                      ? current.textHtml
                                      : "",
                                  }))
                                }
                                className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                              >
                                {guideBookLayoutOptions.map((option) => (
                                  <option
                                    key={option.value}
                                    value={option.value}
                                  >
                                    {option.label}
                                  </option>
                                ))}
                              </select>
                              {guideBookLayoutUsesColumnRatio(
                                slide.layoutType
                              ) ? (
                                <select
                                  value={slide.columnRatio}
                                  onChange={(event) =>
                                    updateSlide(index, (current) => ({
                                      ...current,
                                      columnRatio: event.target
                                        .value as GuideBookColumnRatio,
                                    }))
                                  }
                                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                                >
                                  {guideBookColumnRatioOptions.map((option) => (
                                    <option
                                      key={option.value}
                                      value={option.value}
                                    >
                                      {option.label}
                                    </option>
                                  ))}
                                </select>
                              ) : null}
                            </div>
                            {guideBookLayoutUsesText(slide.layoutType) ? (
                              <GuideBookHtmlEditor
                                value={slide.textHtml}
                                onChange={(textHtml) =>
                                  updateSlide(index, (current) => ({
                                    ...current,
                                    textHtml,
                                  }))
                                }
                                onUploadImage={uploadGuideBookImage}
                                allowImages={false}
                                disabled={isSaving || isDeleting}
                              />
                            ) : null}
                            <div className="grid gap-4 md:grid-cols-2">
                              {guideBookLayoutUsesImage1(slide.layoutType) ? (
                                <SlideImageField
                                  label="Gambar 1"
                                  value={slide.image1Url}
                                  onChange={(image1Url) =>
                                    updateSlide(index, (current) => ({
                                      ...current,
                                      image1Url,
                                    }))
                                  }
                                />
                              ) : null}
                              {guideBookLayoutUsesImage2(slide.layoutType) ? (
                                <SlideImageField
                                  label="Gambar 2"
                                  value={slide.image2Url}
                                  onChange={(image2Url) =>
                                    updateSlide(index, (current) => ({
                                      ...current,
                                      image2Url,
                                    }))
                                  }
                                />
                              ) : null}
                            </div>
                          </CardContent>
                        </Card>
                      ))}
                    </div>
                  ) : (
                    <GuideBookHtmlEditor
                      value={draft.contentHtml}
                      onChange={(contentHtml) =>
                        setDraft((prev) => ({ ...prev, contentHtml }))
                      }
                      onUploadImage={uploadGuideBookImage}
                      disabled={isSaving || isDeleting}
                    />
                  )}
                  <div className="grid gap-2 text-xs text-zinc-500 md:grid-cols-2 xl:grid-cols-4">
                    <p>ID: {draft.id ?? "-"}</p>
                    <p>
                      Updated:{" "}
                      {selectedAdminGuide
                        ? formatDateTime(selectedAdminGuide.updatedAt)
                        : "-"}
                    </p>
                    <p>
                      Created:{" "}
                      {selectedAdminGuide
                        ? formatDateTime(selectedAdminGuide.createdAt)
                        : "-"}
                    </p>
                    <p>
                      Target: {roleLabels[draft.role] ?? draft.role} |{" "}
                      {guideBookPlacementLabels[draft.placementKey] ??
                        "Tanpa placement"}
                    </p>
                  </div>
                  {formError ? (
                    <p className="rounded-md border border-red-200 bg-red-50 px-3 py-2 text-sm text-red-700">
                      {formError}
                    </p>
                  ) : null}
                  {formMessage ? (
                    <p className="rounded-md border border-emerald-200 bg-emerald-50 px-3 py-2 text-sm text-emerald-700">
                      {formMessage}
                    </p>
                  ) : null}
                  <div className="flex flex-wrap gap-2">
                    <button
                      type="button"
                      onClick={() => void save()}
                      disabled={isSaving || isDeleting}
                      className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white disabled:opacity-60"
                    >
                      {isSaving
                        ? "Menyimpan..."
                        : draft.id
                          ? "Update"
                          : "Simpan"}
                    </button>
                    {draft.id ? (
                      <button
                        type="button"
                        onClick={() => void remove()}
                        disabled={isSaving || isDeleting}
                        className="rounded-md border border-red-200 bg-white px-4 py-2 text-sm font-semibold text-red-600 disabled:opacity-60"
                      >
                        {isDeleting ? "Menghapus..." : "Hapus"}
                      </button>
                    ) : null}
                    {draft.slides.length > 0 ? (
                      <button
                        type="button"
                        onClick={() =>
                          setViewerGuide({
                            id: draft.id ?? 0,
                            role: draft.role,
                            title: draft.title || "Preview Juknis",
                            contentHtml: draft.contentHtml,
                            sortOrder: draft.sortOrder,
                            placementKey:
                              draft.placementKey as GuideBookItem["placementKey"],
                            isPublic: draft.isPublic,
                            slides: draft.slides,
                            updatedAt:
                              selectedAdminGuide?.updatedAt ??
                              new Date().toISOString(),
                            isActive: draft.isActive,
                            createdAt: selectedAdminGuide?.createdAt,
                          })
                        }
                        className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700"
                      >
                        Preview Viewer
                      </button>
                    ) : null}
                    <button
                      type="button"
                      onClick={startCreate}
                      disabled={isSaving || isDeleting}
                      className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-medium text-zinc-700"
                    >
                      Reset Form
                    </button>
                  </div>
                </div>
              </div>
            </div>
          </section>
        </div>
      ) : null}

      <GuideBookViewerModal
        guide={viewerGuide}
        open={viewerGuide != null}
        onClose={() => setViewerGuide(null)}
      />
    </section>
  );
}
