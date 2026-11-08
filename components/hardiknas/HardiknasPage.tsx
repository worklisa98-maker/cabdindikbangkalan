"use client";

import NextImage from "next/image";

import Link from "next/link";
import { useEffect, useMemo, useRef, useState } from "react";
import { useQuery, useQueryClient } from "@tanstack/react-query";
import {
  AlertTriangle,
  BookOpen,
  Download,
  Eye,
  ImageIcon,
  Loader2,
  Plus,
  Trash2,
  Trophy,
} from "lucide-react";

import { DataTable } from "@/components/data-table";
import { EmptyState } from "@/components/empty-state";
import { GuideBookViewerModal } from "@/components/guide-books/GuideBookViewerModal";
import { PageHeader } from "@/components/page-header";
import { TypeaheadInput } from "@/components/typeahead-input";
import { Card, CardContent } from "@/components/ui/card";
import { TableCell, TableRow } from "@/components/ui/table";
import { apiFetch } from "@/lib/api/client";
import { useAuth } from "@/lib/auth/client";
import {
  isPengawasRole,
  isSchoolAdminRole,
  normalizeAppRole,
} from "@/lib/auth/role-scope";
import { useDebouncedValue } from "@/lib/hooks/use-debounced-value";
import { GUIDE_BOOK_PLACEMENTS, type GuideBookItem } from "@/lib/guide-books";
import {
  getErrorMessageFromResponse,
  getFilenameFromContentDisposition,
} from "@/lib/http";
import { cn } from "@/lib/utils";

type HardiknasGalleryItem = {
  id: number;
  judul: string;
  nama: string;
  rombel: string;
  sekolah: string;
  tanggalInput: string;
  thumbnailUrl: string;
  imageUrl: string;
  canDelete: boolean;
  submitterType?: string;
  mediaAvailable?: boolean;
  mediaStatusLabel?: string;
};

type HardiknasTableRow = {
  id: number;
  judulPoster: string;
  namaSiswa: string;
  rombel: string;
  sekolah: string;
  tanggalInput: string;
  canDelete: boolean;
  submitterType?: string;
  mediaAvailable?: boolean;
  mediaStatusLabel?: string;
};

type HardiknasWinnerItem = {
  rank: 1 | 2 | 3;
  entryId: number;
  judul: string;
  nama: string;
  rombel: string;
  sekolah: string;
  tanggalInput: string;
  thumbnailUrl: string;
  imageUrl: string;
  mediaAvailable?: boolean;
  mediaStatusLabel?: string;
};

type HardiknasWinnersResponse = {
  published: boolean;
  items: HardiknasWinnerItem[];
};

type HardiknasSaveWinnersPayload = {
  published: boolean;
  winners: Array<{ rank: 1 | 2 | 3; entryId: number }>;
};

type HardiknasWinnerRank = 1 | 2 | 3;

type WinnerSelectedEntryIdsByRank = Record<HardiknasWinnerRank, number>;

type HardiknasListResponse = {
  items: HardiknasGalleryItem[];
  rows: HardiknasTableRow[];
  total: number;
  theme: string;
  cutoffAtLabel: string;
  isCutoffPassed: boolean;
  canCreate: boolean;
  canDelete: boolean;
};

type HardiknasTableResponse = {
  rows: HardiknasTableRow[];
  total: number;
};

type HardiknasSummaryResponse = {
  jumlahKarya: number;
  jumlahSekolahBerpartisipasi: number;
  jumlahSekolahBelumBerpartisipasi: number;
  rataRataKaryaPerSekolah: number;
  totalSekolahDalamScope: number;
};

type HardiknasSchoolSuggestion = {
  id: number;
  namaSekolah: string;
  npsn: string;
  hasSubmission: boolean;
  submissionCount: number;
};

type ApiEnvelope<T> = {
  data: T;
  error?: string;
  message?: string;
};

type FormState = {
  school_name: string;
  school_id: number | null;
  nisn: string;
  nama_siswa: string;
  rombel: string;
  link_surat_pernyataan_orisinalitas: string;
  judul_poster: string;
  hasil_karya: File | null;
};

type HardiknasTableFilters = {
  schoolKeyword: string;
  posterTitle: string;
  studentName: string;
};

const defaultListResponse: HardiknasListResponse = {
  items: [],
  rows: [],
  total: 0,
  theme: "Kartini Masa Kini : Berdaya dan Berkarya menuju Indonesia emas 2045",
  cutoffAtLabel: "24 April 2026 12:00 WIB",
  isCutoffPassed: false,
  canCreate: false,
  canDelete: false,
};

const defaultTableResponse: HardiknasTableResponse = {
  rows: [],
  total: 0,
};

const defaultSummaryResponse: HardiknasSummaryResponse = {
  jumlahKarya: 0,
  jumlahSekolahBerpartisipasi: 0,
  jumlahSekolahBelumBerpartisipasi: 0,
  rataRataKaryaPerSekolah: 0,
  totalSekolahDalamScope: 0,
};

const defaultWinnersResponse: HardiknasWinnersResponse = {
  published: false,
  items: [],
};

const galleryBatchSize = 9;
const tablePageSize = 10;
const emptySchoolMessage = "Belum ada karya yang didaftarkan.";
const emptyReadOnlyMessage = "Belum ada karya poster yang ditampilkan.";
const hardiknasMissingPosterMessage =
  "File poster tidak tersedia di storage. Peserta perlu mengunggah ulang.";
const hardiknasMissingPosterLabel = "File poster tidak tersedia";
const hardiknasDisplayTimezone = "Asia/Jakarta";

const buildHardiknasEndpoint = (isPublic: boolean, suffix = "") =>
  `${isPublic ? "/public" : ""}/lomba-hardiknas${suffix}`;

const formatDateTime = (value: string) => {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return "-";
  }
  return new Intl.DateTimeFormat("id-ID", {
    timeZone: hardiknasDisplayTimezone,
    day: "2-digit",
    month: "short",
    year: "numeric",
    hour: "2-digit",
    minute: "2-digit",
    hour12: false,
  }).format(date);
};

const formatNumber = (value: number) =>
  new Intl.NumberFormat("id-ID").format(value);

const formatAverage = (value: number) =>
  new Intl.NumberFormat("id-ID", {
    minimumFractionDigits: value % 1 === 0 ? 0 : 1,
    maximumFractionDigits: 1,
  }).format(value);

const formatWinnerOptionMeta = (item: HardiknasGalleryItem): string => {
  const rombel = item.rombel.trim();
  return rombel
    ? `${item.nama} — ${rombel} — ${item.sekolah}`
    : `${item.nama} — ${item.sekolah}`;
};

const matchesWinnerSearch = (
  item: HardiknasGalleryItem,
  query: string
): boolean => {
  const normalizedQuery = query.trim().toLowerCase();
  if (!normalizedQuery) {
    return true;
  }

  return [item.judul, item.nama, item.sekolah, item.rombel].some((value) =>
    value.toLowerCase().includes(normalizedQuery)
  );
};

const getWinnerSelectedItem = (
  items: HardiknasGalleryItem[],
  selectedEntryIdsByRank: WinnerSelectedEntryIdsByRank,
  rank: HardiknasWinnerRank
): HardiknasGalleryItem | undefined =>
  items.find((item) => item.id === selectedEntryIdsByRank[rank]);

const isWinnerEntrySelectedByOtherRank = (
  selectedEntryIdsByRank: WinnerSelectedEntryIdsByRank,
  entryId: number,
  rank: HardiknasWinnerRank
): boolean =>
  ([1, 2, 3] as const).some(
    (candidateRank) =>
      candidateRank !== rank && selectedEntryIdsByRank[candidateRank] === entryId
  );

type WinnerEntryPickerProps = {
  rank: HardiknasWinnerRank;
  items: HardiknasGalleryItem[];
  selectedEntryId: number;
  selectedEntryIdsByRank: WinnerSelectedEntryIdsByRank;
  disabled: boolean;
  onSelect(entryId: number): void;
  onClear(): void;
};

function WinnerEntryPicker({
  rank,
  items,
  selectedEntryId,
  selectedEntryIdsByRank,
  disabled,
  onSelect,
  onClear,
}: WinnerEntryPickerProps) {
  const [query, setQuery] = useState("");
  const [isOpen, setIsOpen] = useState(false);
  const blurTimerRef = useRef<number | null>(null);
  const selectedItem = useMemo(
    () =>
      items.find((item) => item.id === selectedEntryId) ??
      getWinnerSelectedItem(items, selectedEntryIdsByRank, rank),
    [items, rank, selectedEntryId, selectedEntryIdsByRank]
  );
  const trimmedQuery = query.trim();
  const visibleItems = useMemo(() => {
    const limit = trimmedQuery ? 15 : 8;
    return items
      .filter((item) => matchesWinnerSearch(item, trimmedQuery))
      .slice(0, limit);
  }, [items, trimmedQuery]);
  const inputId = `hardiknas-winner-entry-picker-${rank}`;

  useEffect(
    () => () => {
      if (blurTimerRef.current) {
        window.clearTimeout(blurTimerRef.current);
      }
    },
    []
  );

  const clearBlurTimer = () => {
    if (blurTimerRef.current) {
      window.clearTimeout(blurTimerRef.current);
      blurTimerRef.current = null;
    }
  };

  const closeWithDelay = () => {
    clearBlurTimer();
    blurTimerRef.current = window.setTimeout(() => {
      setIsOpen(false);
      blurTimerRef.current = null;
    }, 120);
  };

  const handleSelect = (entryId: number) => {
    onSelect(entryId);
    setQuery("");
    setIsOpen(false);
  };

  const handleClear = () => {
    onClear();
    setQuery("");
    setIsOpen(false);
  };

  return (
    <div className="space-y-2">
      <label
        htmlFor={inputId}
        className="block text-sm font-medium text-slate-700"
      >
        Terbaik {rank}
      </label>

      {selectedItem ? (
        <div className="flex flex-col gap-3 rounded-xl border border-slate-200 bg-slate-50 px-3 py-3 sm:flex-row sm:items-start sm:justify-between">
          <div className="min-w-0 space-y-1">
            <p className="break-words text-sm font-semibold text-slate-950">
              {selectedItem.judul}
            </p>
            <div className="space-y-0.5 text-xs text-slate-500">
              <p>{selectedItem.nama}</p>
              {selectedItem.rombel.trim() ? <p>{selectedItem.rombel}</p> : null}
              <p>{selectedItem.sekolah}</p>
            </div>
          </div>
          <button
            type="button"
            onClick={handleClear}
            disabled={disabled}
            className="self-start rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-xs font-semibold text-slate-700 transition hover:bg-slate-100 disabled:cursor-not-allowed disabled:opacity-60"
          >
            Hapus
          </button>
        </div>
      ) : null}

      <div className="relative">
        <input
          id={inputId}
          type="text"
          value={query}
          onChange={(event) => {
            setQuery(event.target.value);
            setIsOpen(true);
          }}
          onFocus={() => {
            clearBlurTimer();
            setIsOpen(true);
          }}
          onBlur={closeWithDelay}
          onKeyDown={(event) => {
            if (event.key === "Escape") {
              setIsOpen(false);
              event.currentTarget.blur();
            }
          }}
          disabled={disabled}
          placeholder="Cari judul, nama siswa, sekolah, atau rombel..."
          className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400 disabled:cursor-not-allowed disabled:bg-slate-100 disabled:text-slate-500"
        />

        {isOpen && !disabled ? (
          <div className="absolute z-[60] mt-2 max-h-72 w-full overflow-y-auto rounded-xl border border-slate-200 bg-white py-1 shadow-lg">
            {visibleItems.length > 0 ? (
              visibleItems.map((item) => {
                const selectedByOtherRank = isWinnerEntrySelectedByOtherRank(
                  selectedEntryIdsByRank,
                  item.id,
                  rank
                );
                return (
                  <button
                    key={item.id}
                    type="button"
                    disabled={selectedByOtherRank}
                    onMouseDown={(event) => event.preventDefault()}
                    onClick={() => {
                      if (!selectedByOtherRank) {
                        handleSelect(item.id);
                      }
                    }}
                    className={cn(
                      "flex w-full items-start justify-between gap-3 px-3 py-2 text-left transition",
                      selectedByOtherRank
                        ? "cursor-not-allowed opacity-55"
                        : "hover:bg-slate-50"
                    )}
                  >
                    <span className="min-w-0">
                      <span className="block break-words text-sm font-semibold text-slate-950">
                        {item.judul}
                      </span>
                      <span className="mt-0.5 block break-words text-xs text-slate-500">
                        {formatWinnerOptionMeta(item)}
                      </span>
                    </span>
                    {selectedByOtherRank ? (
                      <span className="shrink-0 rounded-full bg-amber-100 px-2 py-0.5 text-[11px] font-semibold text-amber-800">
                        Sudah dipilih
                      </span>
                    ) : null}
                  </button>
                );
              })
            ) : (
              <div className="px-3 py-3 text-sm text-slate-500">
                Tidak ada karya yang cocok.
              </div>
            )}
          </div>
        ) : null}
      </div>

      {items.length === 0 ? (
        <p className="text-sm text-slate-500">
          Belum ada karya yang dapat dipilih.
        </p>
      ) : null}
    </div>
  );
}

const buildTableQueryString = (filters: HardiknasTableFilters) => {
  const params = new URLSearchParams();
  if (filters.schoolKeyword.trim()) {
    params.set("school", filters.schoolKeyword.trim());
  }
  if (filters.posterTitle.trim()) {
    params.set("title", filters.posterTitle.trim());
  }
  if (filters.studentName.trim()) {
    params.set("student", filters.studentName.trim());
  }
  return params.toString();
};

const fetchHardiknasList = async (
  isPublic: boolean
): Promise<HardiknasListResponse> => {
  const response = await apiFetch(buildHardiknasEndpoint(isPublic), undefined, {
    skipAuthRedirect: isPublic,
  });
  if (!response.ok) {
    throw new Error("Gagal memuat data Lomba Poster Digital Hari Kartini.");
  }
  const payload = (await response.json()) as ApiEnvelope<HardiknasListResponse>;
  return payload.data ?? defaultListResponse;
};

const fetchHardiknasTable = async (
  isPublic: boolean,
  filters: HardiknasTableFilters
): Promise<HardiknasTableResponse> => {
  const queryString = buildTableQueryString(filters);
  const response = await apiFetch(
    `${buildHardiknasEndpoint(isPublic, "/table")}${
      queryString ? `?${queryString}` : ""
    }`,
    undefined,
    { skipAuthRedirect: isPublic }
  );
  if (!response.ok) {
    throw new Error("Gagal memuat tabel karya poster Hari Kartini.");
  }
  const payload =
    (await response.json()) as ApiEnvelope<HardiknasTableResponse>;
  return payload.data ?? defaultTableResponse;
};

const fetchHardiknasSummary = async (
  isPublic: boolean
): Promise<HardiknasSummaryResponse> => {
  const response = await apiFetch(
    buildHardiknasEndpoint(isPublic, "/summary"),
    undefined,
    { skipAuthRedirect: isPublic }
  );
  if (!response.ok) {
    throw new Error("Gagal memuat rekap Lomba Poster Digital Hari Kartini.");
  }
  const payload =
    (await response.json()) as ApiEnvelope<HardiknasSummaryResponse>;
  return payload.data ?? defaultSummaryResponse;
};

const fetchHardiknasWinners = async (
  isPublic: boolean
): Promise<HardiknasWinnersResponse> => {
  const response = await apiFetch(
    buildHardiknasEndpoint(isPublic, "/winners"),
    undefined,
    { skipAuthRedirect: isPublic }
  );
  if (!response.ok) {
    throw new Error(
      "Gagal memuat pengumuman pemenang Lomba Poster Digital Hari Kartini."
    );
  }
  const payload =
    (await response.json()) as ApiEnvelope<HardiknasWinnersResponse>;
  return payload.data ?? defaultWinnersResponse;
};

const fetchPlacementGuideBook = async (
  isPublic: boolean,
  placementKey: string
): Promise<GuideBookItem | null> => {
  const response = await apiFetch(
    `${isPublic ? "/public" : ""}/guide-books/placement/${placementKey}`,
    undefined,
    { skipAuthRedirect: isPublic }
  );
  if (!response.ok) {
    throw new Error("Gagal memuat Juknis.");
  }
  const payload = (await response.json()) as ApiEnvelope<GuideBookItem | null>;
  return payload.data ?? null;
};

const saveHardiknasWinners = async (
  payload: HardiknasSaveWinnersPayload
): Promise<HardiknasWinnersResponse> => {
  const response = await apiFetch("/lomba-hardiknas/winners", {
    method: "PUT",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(payload),
  });
  if (!response.ok) {
    const errorMessage = await readHardiknasErrorMessage(
      response,
      "Gagal menyimpan pengumuman pemenang."
    );
    throw new Error(errorMessage);
  }
  const result = (await response.json()) as ApiEnvelope<HardiknasWinnersResponse>;
  return result.data ?? defaultWinnersResponse;
};

const fetchHardiknasSchoolSuggestions = async (
  isPublic: boolean,
  keyword: string
): Promise<HardiknasSchoolSuggestion[]> => {
  const params = new URLSearchParams();
  params.set("keyword", keyword);
  params.set("limit", "10");
  const response = await apiFetch(
    `${buildHardiknasEndpoint(isPublic, "/schools")}?${params.toString()}`,
    undefined,
    { skipAuthRedirect: isPublic }
  );
  if (!response.ok) {
    throw new Error("Gagal memuat suggestion sekolah.");
  }
  const payload = (await response.json()) as ApiEnvelope<
    HardiknasSchoolSuggestion[]
  >;
  return payload.data ?? [];
};

const mapHardiknasErrorMessage = (value: string) => {
  switch (value.trim().toUpperCase()) {
    case "CREATE_CUTOFF_PASSED":
      return "Pendaftaran sudah ditutup pada 24 April 2026 pukul 12.00 WIB.";
    case "DELETE_CUTOFF_PASSED":
      return "Penghapusan data sudah ditutup pada 24 April 2026 pukul 12.00 WIB.";
    case "INVALID_PAYLOAD":
      return "Mohon lengkapi semua data wajib.";
    case "INVALID_SCHOOL":
      return "Sekolah tidak terdata. Silakan pilih sekolah dari daftar.";
    case "INVALID_GOOGLE_DRIVE_LINK":
      return "Link surat pernyataan harus berupa link Google Drive/Docs yang valid.";
    case "INVALID_IMAGE_TYPE":
      return "File poster harus JPG atau PNG.";
    case "FILE_TOO_LARGE":
      return "Ukuran file maksimal 5 MB.";
    case "FILE_NOT_AVAILABLE":
      return hardiknasMissingPosterMessage;
    case "ACTION_NOT_ALLOWED":
    case "FORBIDDEN":
    case "UNAUTHORIZED":
      return "Aksi tidak diizinkan.";
    default:
      return value;
  }
};

const readHardiknasErrorMessage = async (
  response: Response,
  fallback: string
) => {
  const raw = await getErrorMessageFromResponse(response, fallback);
  return mapHardiknasErrorMessage(raw || fallback);
};

const createHardiknasEntry = async (isPublic: boolean, form: FormState) => {
  const payload = new FormData();
  if (isPublic && form.school_id) {
    payload.append("school_id", String(form.school_id));
  }
  payload.append("nisn", form.nisn);
  payload.append("nama_siswa", form.nama_siswa);
  payload.append("rombel", form.rombel);
  payload.append(
    "link_surat_pernyataan_orisinalitas",
    form.link_surat_pernyataan_orisinalitas
  );
  payload.append("judul_poster", form.judul_poster);
  if (form.hasil_karya) {
    payload.append("hasil_karya", form.hasil_karya);
  }

  const response = await apiFetch(
    isPublic ? "/public/lomba-hardiknas" : "/lomba-hardiknas",
    {
      method: "POST",
      body: payload,
    },
    { skipAuthRedirect: isPublic }
  );
  if (!response.ok) {
    const errorMessage = await readHardiknasErrorMessage(
      response,
      "Gagal menyimpan karya poster."
    );
    throw new Error(errorMessage);
  }
  return response.json();
};

const deleteHardiknasEntry = async (id: number) => {
  const response = await apiFetch(`/lomba-hardiknas/${id}`, {
    method: "DELETE",
  });
  if (!response.ok) {
    const errorMessage = await readHardiknasErrorMessage(
      response,
      "Gagal menghapus karya."
    );
    throw new Error(errorMessage);
  }
  return response.json();
};

const buildDefaultFormState = (): FormState => ({
  school_name: "",
  school_id: null,
  nisn: "",
  nama_siswa: "",
  rombel: "",
  link_surat_pernyataan_orisinalitas: "",
  judul_poster: "",
  hasil_karya: null,
});

function StatCard({
  label,
  value,
  helper,
}: {
  label: string;
  value: string;
  helper?: string;
}) {
  return (
    <Card className="bg-white">
      <CardContent className="space-y-2 p-5">
        <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
          {label}
        </p>
        <p className="text-2xl font-semibold text-slate-950">{value}</p>
        {helper ? <p className="text-sm text-slate-500">{helper}</p> : null}
      </CardContent>
    </Card>
  );
}

function SectionHeading({
  eyebrow,
  title,
  description,
}: {
  eyebrow: string;
  title: string;
  description?: string;
}) {
  return (
    <div className="space-y-1">
      <p className="text-xs font-semibold tracking-[0.18em] text-slate-500 uppercase">
        {eyebrow}
      </p>
      <h2 className="text-xl font-semibold text-slate-950">{title}</h2>
      {description ? (
        <p className="text-sm text-slate-500">{description}</p>
      ) : null}
    </div>
  );
}

export function HardiknasPage() {
  const queryClient = useQueryClient();
  const { status, user } = useAuth();
  const isHydrating = status === "loading";
  const isPublic = status === "unauthenticated";
  const rawRole = isPublic
    ? "PUBLIC"
    : String(user?.role ?? "PUBLIC")
        .trim()
        .toUpperCase();
  const role = normalizeAppRole(rawRole) || rawRole;
  const isSchoolAdmin = isSchoolAdminRole(role);
  const isAdminDeleteRole = role === "ADMIN" || role === "ADMIN_MASTER";
  const isAdminMaster = role === "ADMIN_MASTER";
  const isReadOnlyRole = isPublic || isPengawasRole(role) || isAdminDeleteRole;
  const isAllowed = isSchoolAdmin || isReadOnlyRole;
  const canUseTableFilters =
    isPublic || isPengawasRole(role) || isAdminDeleteRole;
  const canExport = isAdminDeleteRole;

  const [tablePage, setTablePage] = useState(1);
  const [galleryBatchIndex, setGalleryBatchIndex] = useState(0);
  const [previewItem, setPreviewItem] = useState<HardiknasGalleryItem | null>(
    null
  );
  const [isCreateModalOpen, setIsCreateModalOpen] = useState(false);
  const [isWinnerModalOpen, setIsWinnerModalOpen] = useState(false);
  const [isGuideBookOpen, setIsGuideBookOpen] = useState(false);
  const [formState, setFormState] = useState<FormState>(buildDefaultFormState);
  const [formError, setFormError] = useState("");
  const [successMessage, setSuccessMessage] = useState("");
  const [isSubmitting, setIsSubmitting] = useState(false);
  const [deletingId, setDeletingId] = useState<number | null>(null);
  const [isExporting, setIsExporting] = useState(false);
  const [isSavingWinners, setIsSavingWinners] = useState(false);
  const [exportError, setExportError] = useState("");
  const [winnerSaveError, setWinnerSaveError] = useState("");
  const [winnerDraft, setWinnerDraft] =
    useState<HardiknasSaveWinnersPayload>({
      published: false,
      winners: [],
    });
  const [brokenThumbnailIds, setBrokenThumbnailIds] = useState<
    Record<number, boolean>
  >({});
  const [previewImageFailed, setPreviewImageFailed] = useState(false);
  const [tableFilters, setTableFilters] = useState<HardiknasTableFilters>({
    schoolKeyword: "",
    posterTitle: "",
    studentName: "",
  });

  useEffect(() => {
    setPreviewImageFailed(false);
  }, [previewItem?.id]);

  const debouncedSchoolKeyword = useDebouncedValue(
    tableFilters.schoolKeyword,
    300
  );
  const debouncedPosterTitle = useDebouncedValue(tableFilters.posterTitle, 300);
  const debouncedStudentName = useDebouncedValue(tableFilters.studentName, 300);
  const debouncedSuggestionKeyword = useDebouncedValue(
    tableFilters.schoolKeyword,
    250
  );
  const debouncedCreateSchoolKeyword = useDebouncedValue(
    formState.school_name,
    250
  );

  const listQueryKey = useMemo(
    () => ["hardiknas-list", status, role] as const,
    [role, status]
  );
  const tableQueryKey = useMemo(
    () =>
      [
        "hardiknas-table",
        status,
        role,
        debouncedSchoolKeyword,
        debouncedPosterTitle,
        debouncedStudentName,
      ] as const,
    [
      debouncedPosterTitle,
      debouncedSchoolKeyword,
      debouncedStudentName,
      role,
      status,
    ]
  );
  const summaryQueryKey = useMemo(
    () => ["hardiknas-summary", status, role] as const,
    [role, status]
  );
  const winnersQueryKey = useMemo(
    () => ["hardiknas-winners", status, role] as const,
    [role, status]
  );
  const guideBookQueryKey = useMemo(
    () => ["hardiknas-guide-book", status, role] as const,
    [role, status]
  );
  const schoolSuggestionQueryKey = useMemo(
    () =>
      [
        "hardiknas-school-suggestions",
        "table",
        status,
        role,
        debouncedSuggestionKeyword,
      ] as const,
    [debouncedSuggestionKeyword, role, status]
  );
  const createSchoolSuggestionQueryKey = useMemo(
    () =>
      [
        "hardiknas-school-suggestions",
        "create",
        status,
        debouncedCreateSchoolKeyword,
      ] as const,
    [debouncedCreateSchoolKeyword, status]
  );

  const {
    data: listData = defaultListResponse,
    isLoading: isListLoading,
    isError: isListError,
    refetch: refetchList,
  } = useQuery({
    queryKey: listQueryKey,
    queryFn: () => fetchHardiknasList(isPublic),
    enabled: !isHydrating && isAllowed,
  });

  const {
    data: tableData = defaultTableResponse,
    isLoading: isTableLoading,
    isError: isTableError,
    refetch: refetchTable,
  } = useQuery({
    queryKey: tableQueryKey,
    queryFn: () =>
      fetchHardiknasTable(isPublic, {
        schoolKeyword: debouncedSchoolKeyword,
        posterTitle: debouncedPosterTitle,
        studentName: debouncedStudentName,
      }),
    enabled: !isHydrating && isAllowed,
  });

  const {
    data: summaryData = defaultSummaryResponse,
    isLoading: isSummaryLoading,
    isError: isSummaryError,
    refetch: refetchSummary,
  } = useQuery({
    queryKey: summaryQueryKey,
    queryFn: () => fetchHardiknasSummary(isPublic),
    enabled: !isHydrating && isAllowed && !isSchoolAdmin,
  });

  const {
    data: winnersData = defaultWinnersResponse,
    isLoading: isWinnersLoading,
    isError: isWinnersError,
    refetch: refetchWinners,
  } = useQuery({
    queryKey: winnersQueryKey,
    queryFn: () => fetchHardiknasWinners(isPublic),
    enabled: !isHydrating && isAllowed,
  });

  const { data: guideBookData = null } = useQuery({
    queryKey: guideBookQueryKey,
    queryFn: () =>
      fetchPlacementGuideBook(
        isPublic,
        GUIDE_BOOK_PLACEMENTS.LOMBA_POSTER_KARTINI
      ),
    enabled: !isHydrating && isAllowed,
  });

  const { data: schoolSuggestions = [], isLoading: isSchoolSuggestionLoading } =
    useQuery({
      queryKey: schoolSuggestionQueryKey,
      queryFn: () =>
        fetchHardiknasSchoolSuggestions(isPublic, debouncedSuggestionKeyword),
      enabled:
        !isHydrating &&
        isAllowed &&
        canUseTableFilters &&
        debouncedSuggestionKeyword.trim().length >= 1,
    });
  const {
    data: createSchoolSuggestions = [],
    isLoading: isCreateSchoolSuggestionLoading,
  } = useQuery({
    queryKey: createSchoolSuggestionQueryKey,
    queryFn: () =>
      fetchHardiknasSchoolSuggestions(true, debouncedCreateSchoolKeyword),
    enabled:
      !isHydrating &&
      isPublic &&
      isCreateModalOpen &&
      debouncedCreateSchoolKeyword.trim().length >= 1,
  });

  const galleryBatchCount = Math.max(
    1,
    Math.ceil(listData.items.length / galleryBatchSize)
  );
  const pagedRows = useMemo(() => {
    const start = (tablePage - 1) * tablePageSize;
    return tableData.rows.slice(start, start + tablePageSize);
  }, [tableData.rows, tablePage]);
  const visibleGalleryItems = useMemo(() => {
    const start = galleryBatchIndex * galleryBatchSize;
    return listData.items.slice(start, start + galleryBatchSize);
  }, [galleryBatchIndex, listData.items]);
  const hasActiveTableFilters = useMemo(
    () =>
      Boolean(
        tableFilters.schoolKeyword.trim() ||
        tableFilters.posterTitle.trim() ||
        tableFilters.studentName.trim()
      ),
    [tableFilters]
  );
  const shouldShowSchoolSubmissionMessage =
    canUseTableFilters &&
    tableFilters.schoolKeyword.trim().length > 0 &&
    schoolSuggestions.length > 0 &&
    !schoolSuggestions.some((item) => item.hasSubmission);
  const shouldShowCreateSchoolMissingMessage =
    isPublic &&
    isCreateModalOpen &&
    formState.school_name.trim().length > 0 &&
    !isCreateSchoolSuggestionLoading &&
    createSchoolSuggestions.length === 0;
  const selectedWinnerEntryIds = winnerDraft.winners
    .map((item) => item.entryId)
    .filter((entryId) => entryId > 0);
  const selectedWinnerEntryIdsByRank = useMemo<WinnerSelectedEntryIdsByRank>(
    () => ({
      1: winnerDraft.winners.find((item) => item.rank === 1)?.entryId ?? 0,
      2: winnerDraft.winners.find((item) => item.rank === 2)?.entryId ?? 0,
      3: winnerDraft.winners.find((item) => item.rank === 3)?.entryId ?? 0,
    }),
    [winnerDraft.winners]
  );
  const hasWinnerDuplicate =
    new Set(selectedWinnerEntryIds).size !== selectedWinnerEntryIds.length;
  const hasCompletePublishedWinners =
    ([1, 2, 3] as const).every((rank) =>
      winnerDraft.winners.some((item) => item.rank === rank && item.entryId > 0)
    ) && !hasWinnerDuplicate;
  const isWinnerSaveDisabled =
    isSavingWinners ||
    hasWinnerDuplicate ||
    (winnerDraft.published && !hasCompletePublishedWinners);

  useEffect(() => {
    setGalleryBatchIndex((current) =>
      current >= galleryBatchCount ? 0 : current
    );
  }, [galleryBatchCount]);

  useEffect(() => {
    const totalPages = Math.max(1, Math.ceil(tableData.total / tablePageSize));
    if (tablePage > totalPages) {
      setTablePage(totalPages);
    }
  }, [tableData.total, tablePage]);

  useEffect(() => {
    setTablePage(1);
  }, [
    tableFilters.posterTitle,
    tableFilters.schoolKeyword,
    tableFilters.studentName,
  ]);

  useEffect(() => {
    if (galleryBatchCount <= 1) {
      return;
    }
    const timer = window.setInterval(
      () => {
        setGalleryBatchIndex((current) => (current + 1) % galleryBatchCount);
      },
      5 * 60 * 1000
    );
    return () => window.clearInterval(timer);
  }, [galleryBatchCount]);

  useEffect(() => {
    if (!isWinnerModalOpen) {
      return;
    }
    setWinnerDraft({
      published: winnersData.published,
      winners: winnersData.items
        .filter((item) => item.rank >= 1 && item.rank <= 3)
        .map((item) => ({ rank: item.rank, entryId: item.entryId })),
    });
    setWinnerSaveError("");
  }, [isWinnerModalOpen, winnersData.items, winnersData.published]);

  const closeCreateModal = () => {
    setIsCreateModalOpen(false);
    setFormState(buildDefaultFormState());
    setFormError("");
    setIsSubmitting(false);
  };

  const invalidateHardiknasQueries = async () => {
    await Promise.all([
      queryClient.invalidateQueries({ queryKey: ["hardiknas-list"] }),
      queryClient.invalidateQueries({ queryKey: ["hardiknas-table"] }),
      queryClient.invalidateQueries({ queryKey: ["hardiknas-summary"] }),
      queryClient.invalidateQueries({ queryKey: ["hardiknas-winners"] }),
      queryClient.invalidateQueries({
        queryKey: ["hardiknas-school-suggestions"],
      }),
    ]);
  };

  const mapWinnerToPreviewItem = (
    item: HardiknasWinnerItem
  ): HardiknasGalleryItem => ({
    id: item.entryId,
    judul: item.judul,
    nama: item.nama,
    rombel: item.rombel,
    sekolah: item.sekolah,
    tanggalInput: item.tanggalInput,
    thumbnailUrl: item.thumbnailUrl,
    imageUrl: item.imageUrl,
    canDelete: false,
    mediaAvailable: item.mediaAvailable,
    mediaStatusLabel: item.mediaStatusLabel,
  });

  const updateWinnerDraftSlot = (rank: 1 | 2 | 3, entryId: number) => {
    setWinnerDraft((current) => {
      const remaining = current.winners.filter((item) => item.rank !== rank);
      return {
        ...current,
        winners:
          entryId > 0
            ? [...remaining, { rank, entryId }].sort((a, b) => a.rank - b.rank)
            : remaining,
      };
    });
  };

  const handleSaveWinners = async () => {
    if (isWinnerSaveDisabled) {
      setWinnerSaveError(
        hasWinnerDuplicate
          ? "Satu karya tidak boleh dipilih untuk lebih dari satu Terbaik."
          : "Lengkapi Terbaik 1, Terbaik 2, dan Terbaik 3 sebelum publikasi."
      );
      return;
    }
    if (winnersData.published) {
      const confirmed = window.confirm(
        "Pengumuman pemenang sudah dipublikasikan. Perubahan ini akan langsung terlihat oleh semua user. Lanjutkan?"
      );
      if (!confirmed) {
        return;
      }
    }

    setIsSavingWinners(true);
    setWinnerSaveError("");
    try {
      await saveHardiknasWinners({
        published: winnerDraft.published,
        winners: winnerDraft.winners.filter((item) => item.entryId > 0),
      });
      await Promise.all([
        queryClient.invalidateQueries({ queryKey: ["hardiknas-winners"] }),
        queryClient.invalidateQueries({ queryKey: ["hardiknas-list"] }),
      ]);
      setIsWinnerModalOpen(false);
      setSuccessMessage("Pengumuman pemenang berhasil disimpan.");
    } catch (error) {
      setWinnerSaveError(
        error instanceof Error
          ? error.message
          : "Gagal menyimpan pengumuman pemenang."
      );
    } finally {
      setIsSavingWinners(false);
    }
  };

  const handleCreateSubmit = async () => {
    if (
      isPublic &&
      (formState.school_name.trim().length === 0 || !formState.school_id)
    ) {
      setFormError("Silakan pilih sekolah dari daftar yang tersedia.");
      return;
    }
    if (!formState.hasil_karya) {
      setFormError("File poster wajib diunggah.");
      return;
    }
    setIsSubmitting(true);
    setFormError("");
    try {
      await createHardiknasEntry(isPublic, formState);
      closeCreateModal();
      await invalidateHardiknasQueries();
      setSuccessMessage(
        isPublic
          ? "Karya berhasil dikirim. Data yang dikirim melalui jalur public tidak dapat dihapus secara mandiri."
          : "Karya berhasil dikirim."
      );
    } catch (error) {
      setFormError(
        error instanceof Error ? error.message : "Gagal menyimpan karya."
      );
      setIsSubmitting(false);
    }
  };

  const handleDelete = async (row: HardiknasTableRow) => {
    if (!row.canDelete || deletingId != null) {
      return;
    }
    const confirmed = window.confirm(
      `Hapus karya "${row.judulPoster}" milik ${row.namaSiswa}?`
    );
    if (!confirmed) {
      return;
    }

    setDeletingId(row.id);
    try {
      await deleteHardiknasEntry(row.id);
      await invalidateHardiknasQueries();
    } catch (error) {
      window.alert(
        error instanceof Error ? error.message : "Gagal menghapus karya poster."
      );
    } finally {
      setDeletingId(null);
    }
  };

  const handleExport = async () => {
    setIsExporting(true);
    setExportError("");
    try {
      const queryString = buildTableQueryString(tableFilters);
      const response = await apiFetch(
        `/lomba-hardiknas/export.xlsx${queryString ? `?${queryString}` : ""}`
      );
      if (!response.ok) {
        throw new Error(
          await getErrorMessageFromResponse(
            response,
            "Gagal mengunduh Excel Lomba Poster Digital Hari Kartini."
          )
        );
      }

      const blob = await response.blob();
      const filename = getFilenameFromContentDisposition(
        response.headers.get("content-disposition"),
        "lomba-hardiknas.xlsx"
      );
      const url = URL.createObjectURL(blob);
      const link = document.createElement("a");
      link.href = url;
      link.download = filename;
      document.body.appendChild(link);
      link.click();
      link.remove();
      URL.revokeObjectURL(url);
    } catch (error) {
      setExportError(
        error instanceof Error
          ? error.message
          : "Gagal mengunduh Excel Lomba Poster Digital Hari Kartini."
      );
    } finally {
      setIsExporting(false);
    }
  };

  if (isHydrating) {
    return (
      <div className="space-y-6">
        <PageHeader
          eyebrow="Lomba Hari Kartini"
          title="Lomba Poster Digital Hari Kartini"
          description="Memuat data lomba poster..."
        />
        <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
          {Array.from({ length: 4 }).map((_, index) => (
            <div
              key={`hardiknas-skeleton-${index}`}
              className="h-32 animate-pulse rounded-xl border border-slate-200 bg-slate-100"
            />
          ))}
        </div>
      </div>
    );
  }

  if (!isAllowed) {
    return (
      <EmptyState
        icon={<AlertTriangle className="h-5 w-5" />}
        title="Akses modul tidak tersedia."
        description="Halaman ini hanya tersedia untuk role yang mendapat akses Lomba Poster Digital Hari Kartini."
      />
    );
  }

  const isGalleryEmpty = listData.total === 0;
  const canCreate = listData.canCreate && (isSchoolAdmin || isPublic);
  const galleryInfo =
    listData.total > galleryBatchSize
      ? `Total ${formatNumber(listData.total)} karya terdaftar. Batch ${
          galleryBatchIndex + 1
        } dari ${galleryBatchCount}.`
      : `Total ${formatNumber(listData.total)} karya terdaftar.`;
  const tableEmptyMessage = hasActiveTableFilters
    ? "Belum ada karya yang cocok dengan filter tabel."
    : isSchoolAdmin
      ? emptySchoolMessage
      : emptyReadOnlyMessage;
  const showGuideBookButton = guideBookData != null;
  const headerActions =
    canCreate || showGuideBookButton || isAdminMaster ? (
      <div className="flex flex-wrap items-center gap-2">
        {showGuideBookButton ? (
          <button
            type="button"
            onClick={() => setIsGuideBookOpen(true)}
            className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-zinc-700 transition hover:bg-slate-50"
          >
            <BookOpen className="h-4 w-4" />
            Juknis
          </button>
        ) : null}
        {isAdminMaster ? (
          <button
            type="button"
            onClick={() => setIsWinnerModalOpen(true)}
            className="inline-flex items-center gap-2 rounded-lg border border-amber-300 bg-amber-50 px-4 py-2 text-sm font-semibold text-amber-800 transition hover:bg-amber-100"
          >
            <Trophy className="h-4 w-4" />
            Pemenang
          </button>
        ) : null}
        {canCreate ? (
          <button
            type="button"
            onClick={() => {
              setSuccessMessage("");
              setIsCreateModalOpen(true);
            }}
            disabled={!canCreate}
            className="inline-flex items-center gap-2 rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white shadow-sm transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-50"
          >
            <Plus className="h-4 w-4" />
            Tambahkan
          </button>
        ) : null}
      </div>
    ) : null;

  return (
    <div className="space-y-8">
      <PageHeader
        eyebrow="Lomba Hari Kartini"
        title="Lomba Poster Digital Hari Kartini"
        description={`Tema lomba: ${listData.theme}`}
        actions={headerActions}
      />

      {successMessage ? (
        <div className="rounded-xl border border-emerald-200 bg-emerald-50 px-4 py-3 text-sm text-emerald-900">
          {successMessage}
        </div>
      ) : null}

      <section className="space-y-4">
        <SectionHeading
          eyebrow="Pengumuman"
          title="Pengumuman Pemenang Lomba Poster Digital Hari Kartini"
          description="Berdasarkan Penilaian Juri dan diumumkan secara resmi oleh Cabang Dinas Pendidikan Wilayah Bangkalan. 3 Terbaik ini Akan Mewakili Cabang Dinas Pendidikan Wilayah Kabupaten Bangkalan ke Tingkat Provinsi"
        />
        {isWinnersLoading ? (
          <div className="grid gap-4 md:grid-cols-3">
            {Array.from({ length: 3 }).map((_, index) => (
              <div
                key={`hardiknas-winner-skeleton-${index}`}
                className="h-80 animate-pulse rounded-xl border border-slate-200 bg-slate-100"
              />
            ))}
          </div>
        ) : isWinnersError ? (
          <EmptyState
            title="Gagal memuat pengumuman pemenang."
            description="Silakan coba lagi untuk mengambil data pengumuman pemenang."
            action={
              <button
                type="button"
                onClick={() => refetchWinners()}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700"
              >
                Coba lagi
              </button>
            }
          />
        ) : !winnersData.published ? (
          <EmptyState
            icon={<Trophy className="h-5 w-5" />}
            title="Pengumuman pemenang belum tersedia"
            description="Hasil lomba akan ditampilkan setelah diumumkan secara resmi oleh Cabang Dinas Pendidikan Wilayah Bangkalan."
          />
        ) : (
          <div className="grid gap-4 pt-4 md:grid-cols-3 md:items-start">
            {[1, 2, 3].map((rank) => {
              const item = winnersData.items.find(
                (winner) => winner.rank === rank
              );
              if (!item) {
                return null;
              }
              const isFirstRank = rank === 1;
              const podiumClass =
                rank === 1
                  ? "md:order-2 md:-translate-y-4 ring-2 ring-amber-300"
                  : rank === 2
                    ? "md:order-1 md:translate-y-4"
                    : "md:order-3 md:translate-y-8";
              return (
                <Card
                  key={item.entryId}
                  role={item.mediaAvailable === false ? undefined : "button"}
                  tabIndex={item.mediaAvailable === false ? -1 : 0}
                  onClick={() => {
                    if (item.mediaAvailable === false) {
                      return;
                    }
                    setPreviewItem(mapWinnerToPreviewItem(item));
                  }}
                  onKeyDown={(event) => {
                    if (item.mediaAvailable === false) {
                      return;
                    }
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      setPreviewItem(mapWinnerToPreviewItem(item));
                    }
                  }}
                  className={cn(
                    "overflow-hidden bg-white transition",
                    podiumClass,
                    item.mediaAvailable === false
                      ? "cursor-default"
                      : "cursor-pointer hover:shadow-md"
                  )}
                >
                  <div className="border-b border-slate-100 bg-gradient-to-r from-amber-50 to-white px-4 py-3">
                    <span
                      className={cn(
                        "inline-flex items-center gap-2 rounded-full px-3 py-1 text-xs font-semibold",
                        isFirstRank
                          ? "bg-amber-100 text-amber-900"
                          : "bg-slate-100 text-slate-700"
                      )}
                    >
                      <Trophy className="h-3.5 w-3.5" />
                      Terbaik {rank}
                    </span>
                  </div>
                  <div className="flex h-56 items-center justify-center bg-slate-50 p-3">
                    {item.mediaAvailable === false ||
                    brokenThumbnailIds[item.entryId] ? (
                      <div className="flex h-full w-full items-center justify-center rounded-lg border border-dashed border-slate-300 bg-white px-4 text-center text-sm text-slate-500">
                        {item.mediaStatusLabel || hardiknasMissingPosterLabel}
                      </div>
                    ) : (
                      <NextImage
                        unoptimized
                        width={1200}
                        height={900}
                        src={item.thumbnailUrl}
                        alt={item.judul}
                        className="h-full w-full object-contain"
                        onError={() =>
                          setBrokenThumbnailIds((current) => ({
                            ...current,
                            [item.entryId]: true,
                          }))
                        }
                      />
                    )}
                  </div>
                  <CardContent className="space-y-2 p-4">
                    <p className="line-clamp-2 text-base font-semibold text-slate-950">
                      {item.judul}
                    </p>
                    <div className="space-y-1 text-sm text-slate-500">
                      <p>{item.nama}</p>
                      <p>{item.rombel}</p>
                      <p>{item.sekolah}</p>
                    </div>
                    {item.mediaAvailable === false ? (
                      <div className="inline-flex rounded-full bg-amber-100 px-2.5 py-1 text-xs font-semibold text-amber-800">
                        {item.mediaStatusLabel || hardiknasMissingPosterLabel}
                      </div>
                    ) : null}
                    <div className="flex items-center justify-between pt-1 text-xs text-slate-500">
                      <span>{formatDateTime(item.tanggalInput)}</span>
                      <span
                        className={cn(
                          "inline-flex items-center gap-1 font-semibold",
                          item.mediaAvailable === false
                            ? "text-slate-400"
                            : "text-slate-700"
                        )}
                      >
                        <Eye className="h-3.5 w-3.5" />
                        {item.mediaAvailable === false
                          ? "Preview tidak tersedia"
                          : "Preview"}
                      </span>
                    </div>
                  </CardContent>
                </Card>
              );
            })}
          </div>
        )}
      </section>

      {isSchoolAdmin ? (
        <section className="space-y-4">
          <SectionHeading
            eyebrow="Pendaftaran"
            title="Pendaftaran hasil karya sekolah"
            description="Data yang sudah didaftarkan tidak dapat diedit. Penghapusan hanya tersedia sebelum cutoff."
          />
          <Card className="bg-white">
            <CardContent className="grid gap-4 p-5 md:grid-cols-[1.4fr_0.8fr]">
              <div className="space-y-3">
                <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
                  <p className="font-semibold">Perhatian</p>
                  <p className="mt-1">
                    Setelah karya tersimpan, data tidak dapat diedit. Pastikan
                    NISN, nama, rombel, judul poster, dan file yang diunggah
                    sudah benar.
                  </p>
                </div>
                <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
                  <p className="font-semibold text-slate-900">
                    Cutoff pendaftaran
                  </p>
                  <p className="mt-1">{listData.cutoffAtLabel}</p>
                </div>
              </div>
              <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
                <p className="font-semibold text-slate-900">
                  Status akses sekolah
                </p>
                <p className="mt-1">
                  {listData.isCutoffPassed
                    ? "Periode tambah dan hapus karya sudah ditutup."
                    : "Sekolah masih dapat menambah dan menghapus karya sampai cutoff."}
                </p>
              </div>
            </CardContent>
          </Card>
        </section>
      ) : (
        <section className="space-y-4">
          <SectionHeading
            eyebrow="Rekapan"
            title="Ringkasan partisipasi Lomba Poster Digital Hari Kartini"
            description="Rekap menyesuaikan scope akses role yang sedang login. Public menampilkan agregat umum."
          />
          {isSummaryError ? (
            <EmptyState
              title="Gagal memuat rekapan."
              description="Silakan coba muat ulang ringkasan Lomba Poster Digital Hari Kartini."
              action={
                <button
                  type="button"
                  onClick={() => refetchSummary()}
                  className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700"
                >
                  Coba lagi
                </button>
              }
            />
          ) : (
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <StatCard
                label="Jumlah karya"
                value={
                  isSummaryLoading
                    ? "..."
                    : formatNumber(summaryData.jumlahKarya)
                }
              />
              <StatCard
                label="Sekolah berpartisipasi"
                value={
                  isSummaryLoading
                    ? "..."
                    : formatNumber(summaryData.jumlahSekolahBerpartisipasi)
                }
              />
              <StatCard
                label="Sekolah belum berpartisipasi"
                value={
                  isSummaryLoading
                    ? "..."
                    : formatNumber(summaryData.jumlahSekolahBelumBerpartisipasi)
                }
              />
              <StatCard
                label="Rata-rata karya per sekolah"
                value={
                  isSummaryLoading
                    ? "..."
                    : formatAverage(summaryData.rataRataKaryaPerSekolah)
                }
                helper={
                  isSummaryLoading
                    ? undefined
                    : `Scope ${formatNumber(summaryData.totalSekolahDalamScope)} sekolah`
                }
              />
            </div>
          )}
        </section>
      )}

      <section className="space-y-4">
        <SectionHeading
          eyebrow="Galeri"
          title={
            isSchoolAdmin
              ? "Galeri karya sekolah"
              : "Galeri Karya Poster Digital Hari Kartini"
          }
          description={
            isGalleryEmpty
              ? undefined
              : "Gallery tetap menjadi showcase global. Klik kartu untuk melihat preview penuh."
          }
        />

        {isListLoading ? (
          <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
            {Array.from({ length: 3 }).map((_, index) => (
              <div
                key={`hardiknas-gallery-skeleton-${index}`}
                className="h-80 animate-pulse rounded-xl border border-slate-200 bg-slate-100"
              />
            ))}
          </div>
        ) : isListError ? (
          <EmptyState
            title="Gagal memuat galeri."
            description="Silakan coba lagi untuk mengambil data karya poster."
            action={
              <button
                type="button"
                onClick={() => refetchList()}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700"
              >
                Coba lagi
              </button>
            }
          />
        ) : isGalleryEmpty ? (
          <EmptyState
            icon={<ImageIcon className="h-5 w-5" />}
            title={isSchoolAdmin ? emptySchoolMessage : emptyReadOnlyMessage}
            description={
              isSchoolAdmin
                ? "Unggah karya poster sekolah melalui tombol Tambahkan."
                : "Galeri publik akan tampil otomatis setelah ada karya yang masuk."
            }
          />
        ) : (
          <div className="space-y-4">
            <div className="rounded-xl border border-slate-200 bg-slate-50 px-4 py-3 text-sm text-slate-600">
              {galleryInfo}
            </div>
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3">
              {visibleGalleryItems.map((item) => (
                <Card
                  key={item.id}
                  role={item.mediaAvailable === false ? undefined : "button"}
                  tabIndex={item.mediaAvailable === false ? -1 : 0}
                  onClick={() => {
                    if (item.mediaAvailable === false) {
                      return;
                    }
                    setPreviewItem(item);
                  }}
                  onKeyDown={(event) => {
                    if (item.mediaAvailable === false) {
                      return;
                    }
                    if (event.key === "Enter" || event.key === " ") {
                      event.preventDefault();
                      setPreviewItem(item);
                    }
                  }}
                  className={cn(
                    "overflow-hidden bg-white",
                    item.mediaAvailable === false
                      ? "cursor-default"
                      : "cursor-pointer hover:-translate-y-0.5"
                  )}
                >
                  <div className="flex h-56 items-center justify-center bg-slate-50 p-3">
                    {brokenThumbnailIds[item.id] ? (
                      <div className="flex h-full w-full items-center justify-center rounded-lg border border-dashed border-slate-300 bg-white px-4 text-center text-sm text-slate-500">
                        {hardiknasMissingPosterLabel}
                      </div>
                    ) : (
                      <NextImage
                        unoptimized
                        width={1200}
                        height={900}
                        src={item.thumbnailUrl}
                        alt={item.judul}
                        className="h-full w-full object-contain"
                        onError={() =>
                          setBrokenThumbnailIds((current) => ({
                            ...current,
                            [item.id]: true,
                          }))
                        }
                      />
                    )}
                  </div>
                  <CardContent className="space-y-2 p-4">
                    <p className="line-clamp-2 text-base font-semibold text-slate-950">
                      {item.judul}
                    </p>
                    <div className="space-y-1 text-sm text-slate-500">
                      <p>{item.nama}</p>
                      {!isSchoolAdmin ? <p>{item.sekolah}</p> : null}
                    </div>
                    {item.mediaAvailable === false ? (
                      <div className="inline-flex rounded-full bg-amber-100 px-2.5 py-1 text-xs font-semibold text-amber-800">
                        {item.mediaStatusLabel || hardiknasMissingPosterLabel}
                      </div>
                    ) : null}
                    <div className="flex items-center justify-between pt-1 text-xs text-slate-500">
                      <span>{formatDateTime(item.tanggalInput)}</span>
                      <span
                        className={cn(
                          "inline-flex items-center gap-1 font-semibold",
                          item.mediaAvailable === false
                            ? "text-slate-400"
                            : "text-slate-700"
                        )}
                      >
                        <Eye className="h-3.5 w-3.5" />
                        {item.mediaAvailable === false
                          ? "Preview tidak tersedia"
                          : "Preview"}
                      </span>
                    </div>
                  </CardContent>
                </Card>
              ))}
            </div>
          </div>
        )}
      </section>

      <section className="space-y-4">
        <div className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
          <SectionHeading
            eyebrow="Daftar"
            title={
              isSchoolAdmin
                ? "Daftar karya sekolah"
                : "Daftar Karya Poster Digital Hari Kartini"
            }
            description={
              canUseTableFilters
                ? "Filter hanya berlaku untuk tabel. Gallery di atas tetap global dan tidak ikut terfilter."
                : "Tabel dipakai untuk memeriksa karya yang sudah tersimpan dan menghapus sebelum cutoff."
            }
          />
          {canExport ? (
            <div className="flex flex-col items-start gap-2">
              <button
                type="button"
                onClick={() => void handleExport()}
                disabled={isExporting}
                className="inline-flex items-center gap-2 rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 transition hover:bg-slate-50 disabled:cursor-not-allowed disabled:opacity-60"
              >
                <Download className="h-4 w-4" />
                {isExporting ? "Menyiapkan..." : "Download Excel"}
              </button>
              {exportError ? (
                <p className="text-xs text-red-600">{exportError}</p>
              ) : null}
            </div>
          ) : null}
        </div>

        {canUseTableFilters ? (
          <Card className="bg-white">
            <CardContent className="grid gap-4 p-5 md:grid-cols-3">
              <div className="space-y-2">
                <label className="text-sm font-medium text-slate-700">
                  Nama Sekolah
                </label>
                <TypeaheadInput
                  value={tableFilters.schoolKeyword}
                  onValueChange={(value) =>
                    setTableFilters((current) => ({
                      ...current,
                      schoolKeyword: value,
                    }))
                  }
                  items={schoolSuggestions.map((school) => ({
                    key: school.id,
                    label: school.namaSekolah,
                    value: school.namaSekolah,
                    subLabel: school.hasSubmission
                      ? "sudah mengirimkan karya"
                      : "belum mengirimkan karya",
                  }))}
                  onSelect={(item) =>
                    setTableFilters((current) => ({
                      ...current,
                      schoolKeyword: item.value,
                    }))
                  }
                  placeholder="Ketik nama sekolah..."
                  emptyText="Sekolah tidak ditemukan"
                  isLoading={isSchoolSuggestionLoading}
                  minChars={1}
                  showClear={Boolean(tableFilters.schoolKeyword)}
                  onClear={() =>
                    setTableFilters((current) => ({
                      ...current,
                      schoolKeyword: "",
                    }))
                  }
                />
                {shouldShowSchoolSubmissionMessage ? (
                  <p className="text-xs text-amber-700">
                    Sekolah belum mengirimkan karya
                  </p>
                ) : null}
              </div>

              <label className="space-y-2">
                <span className="text-sm font-medium text-slate-700">
                  Judul Poster
                </span>
                <input
                  value={tableFilters.posterTitle}
                  onChange={(event) =>
                    setTableFilters((current) => ({
                      ...current,
                      posterTitle: event.target.value,
                    }))
                  }
                  placeholder="Cari judul poster..."
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400"
                />
              </label>

              <label className="space-y-2">
                <span className="text-sm font-medium text-slate-700">
                  Nama Siswa
                </span>
                <input
                  value={tableFilters.studentName}
                  onChange={(event) =>
                    setTableFilters((current) => ({
                      ...current,
                      studentName: event.target.value,
                    }))
                  }
                  placeholder="Cari nama siswa..."
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400"
                />
              </label>
            </CardContent>
          </Card>
        ) : null}

        <DataTable
          columns={
            isSchoolAdmin
              ? [
                  { key: "no", label: "No", className: "w-16" },
                  { key: "judul", label: "Judul Poster" },
                  { key: "nama", label: "Nama Siswa" },
                  { key: "rombel", label: "Rombel" },
                  { key: "tanggal", label: "Tanggal Input" },
                  { key: "aksi", label: "Aksi", align: "center" },
                ]
              : [
                  { key: "no", label: "No", className: "w-16" },
                  { key: "judul", label: "Judul Poster" },
                  { key: "nama", label: "Nama Siswa" },
                  { key: "rombel", label: "Rombel" },
                  { key: "sekolah", label: "Sekolah" },
                  { key: "tanggal", label: "Tanggal Input" },
                  { key: "aksi", label: "Aksi", align: "center" },
                ]
          }
          rows={pagedRows}
          isLoading={isTableLoading}
          isError={isTableError}
          emptyMessage={tableEmptyMessage}
          page={tablePage}
          pageSize={tablePageSize}
          total={tableData.total}
          onPageChange={setTablePage}
          renderRow={(row) => {
            const index =
              (tablePage - 1) * tablePageSize + pagedRows.indexOf(row) + 1;
            const previewHref = `/lomba-hardiknas/karya/${row.id}`;
            return (
              <TableRow key={row.id}>
                <TableCell>{index}</TableCell>
                <TableCell className="font-medium text-slate-900">
                  {row.judulPoster}
                </TableCell>
                <TableCell>{row.namaSiswa}</TableCell>
                <TableCell>{row.rombel}</TableCell>
                {!isSchoolAdmin ? <TableCell>{row.sekolah}</TableCell> : null}
                <TableCell>{formatDateTime(row.tanggalInput)}</TableCell>
                <TableCell className="text-center">
                  <div className="flex flex-wrap items-center justify-center gap-2">
                    {row.mediaAvailable === false ? (
                      <span className="inline-flex items-center gap-1 rounded-lg border border-slate-200 bg-slate-100 px-3 py-1.5 text-xs font-semibold text-slate-500">
                        <Eye className="h-3.5 w-3.5" />
                        Preview nonaktif
                      </span>
                    ) : (
                      <Link
                        href={previewHref}
                        target="_blank"
                        rel="noreferrer"
                        className="inline-flex items-center gap-1 rounded-lg border border-blue-200 bg-white px-3 py-1.5 text-xs font-semibold text-blue-700 transition hover:bg-blue-50"
                      >
                        <Eye className="h-3.5 w-3.5" />
                        Preview
                      </Link>
                    )}
                    {row.canDelete && (isSchoolAdmin || isAdminDeleteRole) ? (
                      <button
                        type="button"
                        onClick={() => void handleDelete(row)}
                        disabled={deletingId === row.id}
                        className={cn(
                          "inline-flex items-center gap-1 rounded-lg border px-3 py-1.5 text-xs font-semibold transition",
                          "border-red-200 bg-white text-red-600 hover:bg-red-50"
                        )}
                      >
                        {deletingId === row.id ? (
                          <Loader2 className="h-3.5 w-3.5 animate-spin" />
                        ) : (
                          <Trash2 className="h-3.5 w-3.5" />
                        )}
                        Hapus
                      </button>
                    ) : null}
                    {row.mediaAvailable === false ? (
                      <span className="rounded-full bg-amber-100 px-2.5 py-1 text-[11px] font-semibold text-amber-800">
                        {row.mediaStatusLabel || hardiknasMissingPosterLabel}
                      </span>
                    ) : null}
                  </div>
                </TableCell>
              </TableRow>
            );
          }}
        />
        {isTableError ? (
          <div className="flex justify-end">
            <button
              type="button"
              onClick={() => refetchTable()}
              className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700"
            >
              Coba lagi
            </button>
          </div>
        ) : null}
      </section>

      <GuideBookViewerModal
        guide={guideBookData}
        open={isGuideBookOpen && guideBookData != null}
        onClose={() => setIsGuideBookOpen(false)}
      />

      {isCreateModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/55 px-4 py-6">
          <div className="w-full max-w-3xl rounded-2xl bg-white shadow-xl">
            <div className="border-b border-slate-200 px-5 py-4">
              <h2 className="text-lg font-semibold text-slate-950">
                Pendaftaran karya poster
              </h2>
              <p className="mt-1 text-sm text-slate-500">
                {isPublic
                  ? "Submit public langsung menampilkan karya di galeri. Data yang dikirim lewat jalur public tidak dapat dihapus secara mandiri."
                  : "Submit langsung menyimpan data. Edit tidak tersedia setelah karya terdaftar."}
              </p>
            </div>

            <div className="grid gap-4 px-5 py-5 md:grid-cols-2">
              {isPublic ? (
                <div className="space-y-2 md:col-span-2">
                  <span className="text-sm font-medium text-slate-700">
                    Sekolah
                  </span>
                  <TypeaheadInput
                    value={formState.school_name}
                    onValueChange={(value) =>
                      setFormState((current) => ({
                        ...current,
                        school_name: value,
                        school_id:
                          current.school_name === value
                            ? current.school_id
                            : null,
                      }))
                    }
                    items={createSchoolSuggestions.map((school) => ({
                      key: school.id,
                      label: school.namaSekolah,
                      value: school.namaSekolah,
                      subLabel: school.npsn ? `NPSN ${school.npsn}` : undefined,
                    }))}
                    onSelect={(item) =>
                      setFormState((current) => ({
                        ...current,
                        school_name: item.value,
                        school_id: Number(item.key),
                      }))
                    }
                    placeholder="Cari dan pilih sekolah..."
                    emptyText="Sekolah tidak terdata. Silakan hubungi panitia/Cabdin."
                    isLoading={isCreateSchoolSuggestionLoading}
                    minChars={1}
                    showClear={Boolean(formState.school_name)}
                    onClear={() =>
                      setFormState((current) => ({
                        ...current,
                        school_name: "",
                        school_id: null,
                      }))
                    }
                  />
                  {shouldShowCreateSchoolMissingMessage ? (
                    <p className="text-xs text-amber-700">
                      Sekolah tidak terdata. Silakan hubungi panitia/Cabdin.
                    </p>
                  ) : null}
                </div>
              ) : null}
              <label className="space-y-2">
                <span className="text-sm font-medium text-slate-700">NISN</span>
                <input
                  value={formState.nisn}
                  onChange={(event) =>
                    setFormState((current) => ({
                      ...current,
                      nisn: event.target.value,
                    }))
                  }
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400"
                />
              </label>
              <label className="space-y-2">
                <span className="text-sm font-medium text-slate-700">
                  Nama Siswa
                </span>
                <input
                  value={formState.nama_siswa}
                  onChange={(event) =>
                    setFormState((current) => ({
                      ...current,
                      nama_siswa: event.target.value,
                    }))
                  }
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400"
                />
              </label>
              <label className="space-y-2">
                <span className="text-sm font-medium text-slate-700">
                  Rombel
                </span>
                <input
                  value={formState.rombel}
                  onChange={(event) =>
                    setFormState((current) => ({
                      ...current,
                      rombel: event.target.value,
                    }))
                  }
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400"
                />
              </label>
              <label className="space-y-2">
                <span className="text-sm font-medium text-slate-700">
                  Judul Poster
                </span>
                <input
                  value={formState.judul_poster}
                  onChange={(event) =>
                    setFormState((current) => ({
                      ...current,
                      judul_poster: event.target.value,
                    }))
                  }
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400"
                />
              </label>
              <label className="space-y-2 md:col-span-2">
                <span className="text-sm font-medium text-slate-700">
                  Link surat pernyataan orisinalitas
                </span>
                <input
                  type="url"
                  value={formState.link_surat_pernyataan_orisinalitas}
                  onChange={(event) =>
                    setFormState((current) => ({
                      ...current,
                      link_surat_pernyataan_orisinalitas: event.target.value,
                    }))
                  }
                  placeholder="https://drive.google.com/..."
                  className="w-full rounded-lg border border-slate-200 px-3 py-2 text-sm outline-none focus:border-slate-400"
                />
              </label>
              <label className="space-y-2 md:col-span-2">
                <span className="text-sm font-medium text-slate-700">
                  Hasil Karya (JPG/PNG, max 5 MB)
                </span>
                <input
                  type="file"
                  accept="image/png,image/jpeg"
                  onChange={(event) =>
                    setFormState((current) => ({
                      ...current,
                      hasil_karya: event.target.files?.[0] ?? null,
                    }))
                  }
                  className="block w-full rounded-lg border border-slate-200 px-3 py-2 text-sm text-slate-600 file:mr-3 file:rounded-md file:border-0 file:bg-slate-100 file:px-3 file:py-2 file:text-sm file:font-semibold file:text-slate-700"
                />
              </label>
            </div>

            {formError ? (
              <div className="px-5 pb-1 text-sm text-red-600">{formError}</div>
            ) : null}

            <div className="flex flex-col gap-2 border-t border-slate-200 px-5 py-4 sm:flex-row sm:justify-end">
              <button
                type="button"
                onClick={closeCreateModal}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={() => void handleCreateSubmit()}
                disabled={isSubmitting}
                className="inline-flex items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] px-4 py-2 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isSubmitting ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Plus className="h-4 w-4" />
                )}
                Simpan karya
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {isWinnerModalOpen ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/55 px-4 py-6">
          <div className="w-full max-w-3xl rounded-2xl bg-white shadow-xl">
            <div className="border-b border-slate-200 px-5 py-4">
              <h2 className="text-lg font-semibold text-slate-950">
                Atur Pemenang
              </h2>
              <p className="mt-1 text-sm text-slate-500">
                Pilih pemenang dari karya poster yang sudah masuk. Data yang
                dipublikasikan akan terlihat oleh semua user.
              </p>
            </div>

            <div className="space-y-4 px-5 py-5">
              {listData.items.length === 0 ? (
                <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
                  Belum ada karya yang dapat dipilih.
                </div>
              ) : null}

              {([1, 2, 3] as const).map((rank) => (
                <WinnerEntryPicker
                  key={rank}
                  rank={rank}
                  items={listData.items}
                  selectedEntryId={selectedWinnerEntryIdsByRank[rank]}
                  selectedEntryIdsByRank={selectedWinnerEntryIdsByRank}
                  disabled={listData.items.length === 0 || isSavingWinners}
                  onSelect={(entryId) => updateWinnerDraftSlot(rank, entryId)}
                  onClear={() => updateWinnerDraftSlot(rank, 0)}
                />
              ))}

              {hasWinnerDuplicate ? (
                <div className="rounded-xl border border-red-200 bg-red-50 px-4 py-3 text-sm text-red-700">
                  Satu karya tidak boleh dipilih untuk lebih dari satu Terbaik.
                </div>
              ) : null}

              <label className="flex items-start gap-3 rounded-xl border border-slate-200 bg-slate-50 px-4 py-3">
                <input
                  type="checkbox"
                  checked={winnerDraft.published}
                  onChange={(event) =>
                    setWinnerDraft((current) => ({
                      ...current,
                      published: event.target.checked,
                    }))
                  }
                  disabled={isSavingWinners}
                  className="mt-0.5 h-4 w-4 rounded border-slate-300"
                />
                <span>
                  <span className="block text-sm font-semibold text-slate-900">
                    Publikasikan pengumuman
                  </span>
                  <span className="mt-1 block text-sm text-slate-500">
                    Jika aktif, Terbaik 1, Terbaik 2, dan Terbaik 3 wajib lengkap.
                  </span>
                </span>
              </label>

              {winnerDraft.published && !hasCompletePublishedWinners ? (
                <div className="rounded-xl border border-amber-200 bg-amber-50 px-4 py-3 text-sm text-amber-900">
                  Lengkapi semua slot pemenang sebelum mempublikasikan
                  pengumuman.
                </div>
              ) : null}

              {winnerSaveError ? (
                <div className="text-sm text-red-600">{winnerSaveError}</div>
              ) : null}
            </div>

            <div className="flex flex-col gap-2 border-t border-slate-200 px-5 py-4 sm:flex-row sm:justify-end">
              <button
                type="button"
                onClick={() => {
                  setIsWinnerModalOpen(false);
                  setWinnerSaveError("");
                }}
                disabled={isSavingWinners}
                className="rounded-lg border border-slate-300 bg-white px-4 py-2 text-sm font-semibold text-slate-700 disabled:cursor-not-allowed disabled:opacity-60"
              >
                Batal
              </button>
              <button
                type="button"
                onClick={() => void handleSaveWinners()}
                disabled={isWinnerSaveDisabled}
                className="inline-flex items-center justify-center gap-2 rounded-lg bg-gradient-to-r from-amber-500 to-orange-500 px-4 py-2 text-sm font-semibold text-white disabled:cursor-not-allowed disabled:opacity-60"
              >
                {isSavingWinners ? (
                  <Loader2 className="h-4 w-4 animate-spin" />
                ) : (
                  <Trophy className="h-4 w-4" />
                )}
                Simpan
              </button>
            </div>
          </div>
        </div>
      ) : null}

      {previewItem ? (
        <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/70 px-4 py-6">
          <div className="w-full max-w-5xl rounded-2xl bg-white shadow-xl">
            <div className="flex items-start justify-between gap-4 border-b border-slate-200 px-5 py-4">
              <div>
                <h2 className="text-lg font-semibold text-slate-950">
                  {previewItem.judul}
                </h2>
                <p className="mt-1 text-sm text-slate-500">
                  {previewItem.nama}
                  {isSchoolAdmin ? "" : ` - ${previewItem.sekolah}`}
                </p>
              </div>
              <button
                type="button"
                onClick={() => setPreviewItem(null)}
                className="rounded-lg border border-slate-300 bg-white px-3 py-1.5 text-sm font-semibold text-slate-700"
              >
                Tutup
              </button>
            </div>
            <div className="flex min-h-[60vh] items-center justify-center bg-slate-50 p-5">
              {previewImageFailed ? (
                <div className="flex min-h-[30vh] w-full items-center justify-center rounded-xl border border-dashed border-slate-300 bg-white px-6 text-center text-sm text-slate-500">
                  {hardiknasMissingPosterMessage}
                </div>
              ) : (
                <NextImage
                  unoptimized
                  width={1200}
                  height={900}
                  src={previewItem.imageUrl}
                  alt={previewItem.judul}
                  className="max-h-[78vh] w-full object-contain"
                  onError={() => setPreviewImageFailed(true)}
                />
              )}
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
}
