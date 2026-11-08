"use client";

import { useCallback, useEffect, useState } from "react";
import { GaspolAccountSelect } from "./GaspolSelectors";
import { apiFetch } from "@/lib/api/client";
import { gaspolRequest, gaspolButton, gaspolField } from "./GaspolRubrics";

type Candidate = {
  rank: number;
  schoolId: number;
  schoolNpsn?: string;
  schoolName: string;
  monthlyScore: number;
  fieldStage1Score: number;
  fieldStage2Score: number;
  presentationScore: number;
  nominationScore: number;
  appealScore: number;
  total: number;
  innovation?: number;
  votes?: number;
  likes?: number;
  likeVerified?: boolean;
  favorite?: { internalScore: number; likeScore: number; total: number };
};
type Obligation = {
  schoolId: number;
  schoolName: string;
  assessor: string;
  assessmentId?: number;
  status: string;
  finalScore?: number;
};
type Approval = {
  approvalId?: number;
  approvalStatus: string;
  obligations: Obligation[];
};
type Lock = {
  id: number;
  kind: string;
  revision: number;
  rows: Candidate[];
  lockedAt: string;
  documentId?: number;
  baNumber: string;
  baDate: string;
  uploadedBy: string;
  uploadedAt: string;
};
type Monitor = {
  kasiAssignments?: { userId: number; name: string }[];
  config?: {
    year: number;
    state: string;
    publicFullRankingEnabled: boolean;
    [key: string]: unknown;
  };
  standings?: {
    rank: number;
    schoolId: number;
    schoolName: string;
    badge: string;
    cohort: string;
    scores: { scoreTotal: number };
  }[];
  master: boolean;
  kasi: boolean;
  innovationAssessor: boolean;
  innovation?: Approval;
  locks?: Lock[];
  previews?: Record<
    string,
    { rows: Candidate[]; obligations: Obligation[]; ready: boolean }
  >;
  final?: {
    rows: Candidate[];
    blockers: string[];
    version: string;
    obligations: Record<string, Obligation[]>;
    approvalStatus: string;
    locked: boolean;
    denominator: number;
  };
};
const labels: Record<string, string> = {
  TOP_10: "Top 10 resmi",
  TOP_5: "Top 5",
  BOTTOM_5: "Lima kandidat banding",
  FINAL: "Tiga penerima Ter Gagal",
  AWARDING: "Penghargaan final bersama",
};
async function download(path: string, name: string) {
  const response = await apiFetch(path);
  if (!response.ok) throw new Error("Dokumen tidak dapat diunduh.");
  saveBlob(await response.blob(), name);
}
function saveBlob(blob: Blob, name: string) {
  const url = URL.createObjectURL(blob);
  const a = document.createElement("a");
  a.href = url;
  a.download = name;
  a.click();
  setTimeout(() => URL.revokeObjectURL(url), 1000);
}
function Scores({
  rows,
  final = false,
  nomination = false,
}: {
  rows: Candidate[] | null | undefined;
  final?: boolean;
  nomination?: boolean;
}) {
  return (
    <div className="overflow-x-auto">
      <table className="w-full min-w-[800px] text-left text-sm">
        <thead>
          <tr className="border-b bg-slate-50">
            <th className="p-2">Sekolah</th>
            <th>Bulanan ×20%</th>
            <th>T1 ×40%</th>
            <th>{nomination ? "Nominasi ×30%" : "T2 ×20%"}</th>
            <th>{nomination ? "Banding ×10%" : "Presentasi ×20%"}</th>
            <th>Kumulatif</th>
            {final && (
              <>
                <th>Inovasi</th>
                <th>Suara / skor</th>
                <th>Like / verifikasi / skor</th>
                <th>Favorit 60:40</th>
              </>
            )}
          </tr>
        </thead>
        <tbody>
          {(rows ?? []).map((r) => (
            <tr className="border-b" key={r.schoolId}>
              <td className="p-2 font-medium">{r.schoolName}</td>
              <td>
                {r.monthlyScore.toFixed(2)} /{" "}
                {(r.monthlyScore * 0.2).toFixed(2)}
              </td>
              <td>
                {r.fieldStage1Score.toFixed(2)} /{" "}
                {(r.fieldStage1Score * 0.4).toFixed(2)}
              </td>
              <td>
                {(nomination ? r.nominationScore : r.fieldStage2Score).toFixed(
                  2
                )}{" "}
                /{" "}
                {(nomination
                  ? r.nominationScore * 0.3
                  : r.fieldStage2Score * 0.2
                ).toFixed(2)}
              </td>
              <td>
                {(nomination ? r.appealScore : r.presentationScore).toFixed(2)}{" "}
                /{" "}
                {(nomination
                  ? r.appealScore * 0.1
                  : r.presentationScore * 0.2
                ).toFixed(2)}
              </td>
              <td className="font-bold">{r.total.toFixed(2)}</td>
              {final && (
                <>
                  <td>{r.innovation?.toFixed(2)}</td>
                  <td>
                    {r.votes} / {r.favorite?.internalScore.toFixed(2)}
                  </td>
                  <td>
                    {r.likes} / {r.likeVerified ? "Sah" : "Belum sah"} /{" "}
                    {r.favorite?.likeScore.toFixed(2)}
                  </td>
                  <td>{r.favorite?.total.toFixed(2)}</td>
                </>
              )}
            </tr>
          ))}
        </tbody>
      </table>
    </div>
  );
}
function Progress({
  rows,
  master,
  onReturn,
}: {
  rows: Obligation[] | null | undefined;
  master: boolean;
  onReturn: (id: number) => void;
}) {
  return (
    <details className="rounded-xl border p-3">
      <summary>
        Progres juri:{" "}
        {
          (rows ?? []).filter((r) => r.status === "FINAL" && r.finalScore != null)
            .length
        }
        /{rows?.length ?? 0} kewajiban selesai
      </summary>
      <ul className="mt-3 space-y-2 text-sm">
        {(rows ?? []).map((r, i) => (
          <li
            key={`${r.schoolId}-${i}`}
            className="flex flex-wrap items-center justify-between gap-2"
          >
            <span>
              {r.schoolName} · {r.assessor} · {r.status}
              {r.finalScore != null ? ` · ${r.finalScore}` : ""}
            </span>
            {master && r.status === "FINAL" && r.assessmentId && (
              <button
                className="rounded-lg border px-3 py-1"
                onClick={() => onReturn(r.assessmentId!)}
              >
                Kembalikan Penilaian
              </button>
            )}
          </li>
        ))}
      </ul>
    </details>
  );
}

export function GaspolDeterminations({ year }: { year: number }) {
  const [data, setData] = useState<Monitor>();
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);
  const [kasiAccount, setKasiAccount] = useState<number[]>([]);
  const [standingsPreview, setStandingsPreview] = useState<
    NonNullable<Monitor["standings"]>
  >([]);
  const [standingsVersion, setStandingsVersion] = useState("");
  const [standingsDecision, setStandingsDecision] = useState("");
  const [standingsConfirmed, setStandingsConfirmed] = useState(false);
  const [reopenID, setReopenID] = useState<number>();
  const [kind, setKind] = useState("");
  const [step, setStep] = useState(1);
  const [file, setFile] = useState<File>();
  const [previewURL, setPreviewURL] = useState("");
  const [baNumber, setBANumber] = useState("");
  const [baDate, setBADate] = useState("");
  const [confirmed, setConfirmed] = useState(false);
  const [reason, setReason] = useState("");
  const [winners, setWinners] = useState<Record<string, number>>({});
  const [tieSchool, setTieSchool] = useState(0);
  const [order, setOrder] = useState("");
  const [material, setMaterial] = useState<{
    id: number;
    version: string;
    sourceVersion: string;
    decision: string;
  }>();
  const [returnID, setReturnID] = useState<number>();
  const [returnReason, setReturnReason] = useState("");
  const reload = useCallback(async () => {
    const response = await apiFetch(
      `/gaspol/internal/determinations?year=${year}`,
      undefined,
      { suppressErrorStatuses: [403] }
    );
    if (response.status === 403) return;
    const json = await response.json();
    if (!response.ok)
      throw new Error(json.message ?? json.error ?? "Gagal memuat penetapan");
    const monitor: Monitor = json.data;
    setData({
      ...monitor,
      final: monitor.final
        ? {
            ...monitor.final,
            rows: monitor.final.rows ?? [],
            blockers: monitor.final.blockers ?? [],
            obligations: monitor.final.obligations ?? {},
          }
        : undefined,
    });
  }, [year]);
  useEffect(() => {
    setData(undefined);
    setMaterial(undefined);
    void reload().catch((e) => setError(e.message));
  }, [reload]);
  useEffect(() => {
    if (!file) {
      setPreviewURL("");
      return;
    }
    const url = URL.createObjectURL(file);
    setPreviewURL(url);
    return () => URL.revokeObjectURL(url);
  }, [file]);
  const run = async (action: () => Promise<unknown>) => {
    setBusy(true);
    setError("");
    try {
      await action();
      await reload();
    } catch (e) {
      setError((e as Error).message);
      await reload().catch(() => undefined);
    } finally {
      setBusy(false);
    }
  };
  if (!data)
    return error ? (
      <p role="alert" className="text-red-700">
        {error}
      </p>
    ) : null;
  const final = data.final;
  const current = data.previews?.[kind];
  const locks = data.locks ?? [];
  const approval = data.innovation;
  const winnerPreview = (award: string) => {
    const rows = [...(final?.rows ?? [])];
    const score = (r: Candidate) =>
      award === "BEST"
        ? r.total
        : award === "INNOVATIVE"
          ? (r.innovation ?? 0)
          : (r.favorite?.total ?? 0);
    rows.sort(
      (a, b) =>
        score(b) - score(a) ||
        (award === "FAVORITE"
          ? (b.favorite?.internalScore ?? 0) - (a.favorite?.internalScore ?? 0)
          : 0)
    );
    if (!rows.length) return "Belum tersedia";
    const tied = rows.filter(
      (r) =>
        score(r) === score(rows[0]) &&
        (award !== "FAVORITE" ||
          r.favorite?.internalScore === rows[0].favorite?.internalScore)
    );
    if (tied.length === 1) return rows[0].schoolName;
    return reason.trim()
      ? (tied.find((r) => r.schoolId === winners[award])?.schoolName ??
          "Menunggu musyawarah")
      : "Menunggu musyawarah";
  };
  const selectionRows = () => {
    if (kind === "AWARDING") return final?.rows ?? [];
    const rows = [...(current?.rows ?? [])];
    const ids = order.split(",").map(Number).filter(Boolean);
    if (!ids.length && tieSchool) ids.push(tieSchool);
    const preference = (id: number) => {
      const index = ids.indexOf(id);
      return index < 0 ? Number.MAX_SAFE_INTEGER : index;
    };
    const bottom = kind !== "TOP_5";
    rows.sort(
      (a, b) =>
        (bottom ? a.total - b.total : b.total - a.total) ||
        preference(a.schoolId) - preference(b.schoolId)
    );
    return rows.slice(0, kind === "FINAL" ? 3 : 5);
  };
  const decision = JSON.stringify({ winners, reason });
  const stale =
    !!material &&
    (material.sourceVersion !== final?.version ||
      material.decision !== decision);
  const start = (value: string) => {
    setKind(value);
    setStep(1);
    setFile(undefined);
    setConfirmed(false);
    setBANumber("");
    setBADate("");
    setTieSchool(0);
    setError("");
  };
  const exportMaterial = async () => {
    const result = await gaspolRequest<{
      id: number;
      version: string;
      payload: unknown;
    }>("/gaspol/admin/final-determination", {
      year,
      action: "export",
      winners,
      reason,
    });
    setMaterial({ ...result, sourceVersion: final!.version, decision });
    saveBlob(
      new Blob([JSON.stringify(result, null, 2)], { type: "application/json" }),
      `GASPOL-${year}-bahan-BA-${result.id}.json`
    );
    const cell = (v: unknown) => `"${String(v ?? "").replaceAll('"', '""')}"`;
    const header = [
      "Posisi",
      "ID Sekolah",
      "NPSN",
      "Sekolah",
      "Bulanan",
      "Bobot Bulanan",
      "T1",
      "Bobot T1",
      "T2",
      "Bobot T2",
      "Presentasi",
      "Bobot Presentasi",
      "Total",
      "Inovasi",
      "Suara",
      "Skor Internal",
      "Like",
      "Terverifikasi",
      "Skor Like",
      "Skor Favorit",
      "Versi",
      "Keputusan Musyawarah",
    ];
    const csv = [
      header,
      ...final!.rows.map((r) => [
        r.rank,
        r.schoolId,
        r.schoolNpsn,
        r.schoolName,
        r.monthlyScore,
        0.2,
        r.fieldStage1Score,
        0.4,
        r.fieldStage2Score,
        0.2,
        r.presentationScore,
        0.2,
        r.total,
        r.innovation,
        r.votes,
        r.favorite?.internalScore,
        r.likes,
        r.likeVerified,
        r.favorite?.likeScore,
        r.favorite?.total,
        result.version,
        reason,
      ]),
    ]
      .map((row) => row.map(cell).join(","))
      .join("\r\n");
    saveBlob(
      new Blob(["\uFEFF" + csv], { type: "text/csv;charset=utf-8" }),
      `GASPOL-${year}-bahan-BA-${result.id}.csv`
    );
  };
  const lock = async () => {
    let documentId: number | undefined;
    if (kind !== "BOTTOM_5") {
      if (!file) throw new Error("PDF BA wajib diunggah.");
      const form = new FormData();
      form.append("year", String(year));
      form.append("file", file);
      const response = await apiFetch("/gaspol/admin/ba", {
        method: "POST",
        body: form,
      });
      const json = await response.json();
      if (!response.ok)
        throw new Error(json.message ?? json.error ?? "Upload gagal");
      documentId = json.data.id;
    }
    try {
      if (kind === "AWARDING")
        await gaspolRequest("/gaspol/admin/final-determination", {
          year,
          action: "lock",
          baDocumentId: documentId,
          baNumber,
          baDate,
          winners,
          reason,
          exportId: material?.id,
          version: material?.version,
        });
      else
        await gaspolRequest("/gaspol/admin/result-locks", {
          year,
          lockType: kind,
          baDocumentId: documentId,
          baNumber,
          baDate,
          reason,
          manualTieSchoolId: tieSchool || undefined,
          order: order.split(",").map(Number).filter(Boolean),
        });
      setKind("");
    } catch (e) {
      if (documentId)
        await apiFetch(`/gaspol/admin/ba/${documentId}`, {
          method: "DELETE",
        }).catch(() => undefined);
      throw e;
    }
  };
  return (
    <section className="space-y-5 rounded-2xl border border-slate-200 bg-white p-5">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <h2 className="text-lg font-semibold">
          Monitoring, Penetapan & BA {year}
        </h2>
        <button
          className="rounded-xl border px-4 py-2"
          disabled={busy}
          onClick={() => void run(reload)}
        >
          Segarkan data
        </button>
      </div>
      {error && (
        <p role="alert" className="text-sm whitespace-pre-wrap text-red-700">
          {error}
        </p>
      )}
      {approval && (
        <>
          <h3 className="font-semibold">
            Ter Inovatif · {approval.approvalStatus || "Belum diajukan"}
          </h3>
          <Progress
            rows={approval.obligations}
            master={false}
            onReturn={() => {}}
          />
          {data.innovationAssessor && (
            <button
              className={gaspolButton}
              disabled={busy}
              onClick={() =>
                void run(() =>
                  gaspolRequest("/gaspol/awards/innovation/propose", { year })
                )
              }
            >
              Ajukan seluruh nilai inovasi kepada Kasi
            </button>
          )}
          {data.kasi &&
            approval.approvalId &&
            approval.approvalStatus === "SUBMITTED" && (
              <div className="space-y-2">
                <label className="block">
                  Catatan Kasi
                  <textarea
                    className={gaspolField}
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                  />
                </label>
                {["APPROVE", "RETURN"].map((value) => (
                  <button
                    key={value}
                    disabled={busy || (value === "RETURN" && !reason.trim())}
                    className={gaspolButton}
                    onClick={() =>
                      void run(() =>
                        gaspolRequest(
                          `/gaspol/awards/innovation/${approval.approvalId}/decision`,
                          { decision: value, notes: reason }
                        )
                      )
                    }
                  >
                    {value === "APPROVE"
                      ? "Setujui nilai terbaru"
                      : "Kembalikan usulan"}
                  </button>
                ))}
              </div>
            )}
        </>
      )}
      {locks.map((l) => (
        <details className="rounded-xl border p-3" key={l.id}>
          <summary>
            {labels[l.kind]} · terkunci · versi {l.revision}
          </summary>
          <p className="my-2 text-sm">
            BA {l.baNumber || "Metadata lama"} · {l.baDate} · Pengunggah{" "}
            {l.uploadedBy} · {l.uploadedAt || l.lockedAt}
          </p>
          <Scores
            rows={l.rows}
            nomination={["BOTTOM_5", "FINAL"].includes(l.kind)}
          />
          {data.master && ["TOP_5", "BOTTOM_5", "FINAL"].includes(l.kind) && (
            <button
              className="mr-2 rounded-xl border px-4 py-2 text-sm"
              disabled={busy}
              onClick={() => {
                setReopenID(l.id);
                setReturnReason("");
              }}
            >
              Revisi penetapan resmi
            </button>
          )}
          {l.documentId && (
            <button
              disabled={busy}
              className={gaspolButton}
              onClick={() =>
                void run(() =>
                  download(
                    `/gaspol/ba/${l.documentId}`,
                    `BA-${l.kind}-${year}.pdf`
                  )
                )
              }
            >
              Preview / Unduh BA resmi
            </button>
          )}
        </details>
      ))}
      {data.master && (
        <details className="space-y-3 rounded-xl border p-3">
          <summary>Penugasan Kasi SMK untuk persetujuan Ter Inovatif</summary>
          <p className="text-sm">
            Aktif:{" "}
            {data.kasiAssignments?.map((a) => a.name).join(", ") ||
              "Belum ditugaskan"}
          </p>
          <GaspolAccountSelect
            year={year}
            value={kasiAccount}
            onChange={setKasiAccount}
            label="Akun Kasi SMK"
          />
          <button
            className={gaspolButton}
            disabled={busy || !kasiAccount.length || !!final?.locked}
            onClick={() =>
              void run(() =>
                gaspolRequest("/gaspol/admin/stage-assignments", {
                  year,
                  stage: "INNOVATION",
                  assignmentRole: "KASI_SMK_APPROVER",
                  assessorUserId: kasiAccount[0],
                  isActive: true,
                })
              )
            }
          >
            Simpan penugasan Kasi
          </button>
        </details>
      )}
      {data.master && data.config && (
        <label className="flex items-center gap-3 rounded-xl border p-3 text-sm">
          <input
            type="checkbox"
            checked={data.config.publicFullRankingEnabled}
            disabled={
              busy ||
              (!data.config.publicFullRankingEnabled &&
                data.standings?.length !== 21)
            }
            onChange={(e) => {
              const enabled = e.target.checked;
              void run(() =>
                gaspolRequest(
                  "/gaspol/admin/competition",
                  { ...data.config, publicFullRankingEnabled: enabled },
                  "PUT"
                )
              );
            }}
          />
          Publikasikan klasemen lengkap dan penghargaan resmi (memerlukan 21
          posisi terkunci)
        </label>
      )}
      {data.standings && data.standings.length > 0 && (
        <details className="rounded-xl border p-3">
          <summary>Klasemen resmi 1�21 � lima zona terkunci</summary>
          <ol className="mt-3 space-y-2 text-sm">
            {data.standings.map((r) => (
              <li key={r.schoolId}>
                {r.rank}. {r.schoolName} � {r.badge} �{" "}
                {r.scores.scoreTotal.toFixed(2)}
              </li>
            ))}
          </ol>
        </details>
      )}
      {data.master && (
        <div className="flex flex-wrap gap-2">
          {["TOP_5", "BOTTOM_5", "FINAL", "AWARDING"].map((k) => (
            <button
              key={k}
              disabled={busy || locks.some((l) => l.kind === k)}
              className={gaspolButton}
              onClick={() => start(k)}
            >
              {locks.some((l) => l.kind === k)
                ? `${labels[k]} terkunci`
                : `Preview & tetapkan ${labels[k]}`}
            </button>
          ))}
        </div>
      )}
      {final && (
        <details className="space-y-3 rounded-xl border p-3">
          <summary>
            Hasil presentasi, inovasi & Ter Favorit · Kasi:{" "}
            {final.approvalStatus || "Belum menyetujui"}
          </summary>
          <p className="text-sm">
            Pemilih internal: {final.denominator}. Skor Favorit = 60% internal +
            40% like, maksimal 100 untuk ≥800 like.
          </p>
          <Scores rows={final.rows} final />
          {Object.entries(final.obligations).map(([stage, rows]) => (
            <Progress
              key={stage}
              rows={rows}
              master={data.master && !final.locked}
              onReturn={setReturnID}
            />
          ))}
          {final.blockers.map((b, i) => (
            <p className="text-sm text-amber-800" key={i}>
              {b}
            </p>
          ))}
        </details>
      )}
      {data.master && final && !final.locked && (
        <div className="space-y-3 rounded-xl border p-3">
          <h3 className="font-semibold">Bahan BA final gabungan</h3>
          <p className="text-sm">
            Preview pemenang: Juara 1 � {winnerPreview("BEST")}; Ter Inovatif �{" "}
            {winnerPreview("INNOVATIVE")}; Ter Favorit �{" "}
            {winnerPreview("FAVORITE")}.
          </p>
          <p className="text-sm">
            Ekspor memuat rincian lima finalis dan versi data. Perubahan hasil
            mewajibkan ekspor ulang.
          </p>
          {["BEST", "INNOVATIVE", "FAVORITE"].map((k) => (
            <label className="block text-sm" key={k}>
              Keputusan musyawarah jika seri: {k}
              <select
                className={gaspolField}
                value={winners[k] ?? 0}
                onChange={(e) =>
                  setWinners({ ...winners, [k]: Number(e.target.value) })
                }
              >
                <option value={0}>
                  Gunakan hasil tertinggi jika tidak seri
                </option>
                {final.rows.map((r) => (
                  <option key={r.schoolId} value={r.schoolId}>
                    {r.schoolName}
                  </option>
                ))}
              </select>
            </label>
          ))}
          <label className="block text-sm">
            Catatan musyawarah / pengesahan Kepala Cabdin
            <textarea
              className={gaspolField}
              value={reason}
              onChange={(e) => setReason(e.target.value)}
            />
          </label>
          <button
            className={gaspolButton}
            disabled={busy}
            onClick={() => void run(exportMaterial)}
          >
            Ekspor bahan BA (CSV + JSON)
          </button>
          {material && (
            <p className={stale ? "text-red-700" : "text-emerald-700"}>
              {stale
                ? "Bahan BA kedaluwarsa. Ekspor ulang dan sesuaikan BA."
                : `Bahan BA versi ${material.version.slice(0, 12)} tersedia.`}
            </p>
          )}
        </div>
      )}
      {data.previews &&
        Object.entries(data.previews).map(([k, p]) => (
          <details className="rounded-xl border p-3" key={k}>
            <summary>
              {labels[k]} ·{" "}
              {p.ready ? "Penugasan selesai" : "Masih ada kewajiban penilaian"}
            </summary>
            <Scores rows={p.rows} nomination={k !== "TOP_5"} />
            <Progress
              rows={p.obligations}
              master={data.master && !locks.some((l) => l.kind === k)}
              onReturn={setReturnID}
            />
          </details>
        ))}
      {!!data.locks && (
        <div className="flex flex-wrap gap-2">
          <button
            className={gaspolButton}
            disabled={busy}
            onClick={() =>
              void run(() =>
                download(
                  `/gaspol/internal/ba-zip?year=${year}`,
                  `BA-GASPOL-${year}.zip`
                )
              )
            }
          >
            Unduh seluruh BA resmi (ZIP)
          </button>
          {data.master && (
            <details className="w-full rounded-xl border p-3">
              <summary>Tetapkan klasemen akhir lima zona</summary>
              <p className="my-2 text-sm">
                Memerlukan penetapan penghargaan bersama dan Ter Gagal. Semua 21
                sekolah harus unik.
              </p>
              <label className="block text-sm">
                Urutan ID sekolah seri sesuai musyawarah (pisahkan koma)
                <input
                  className={gaspolField}
                  value={order}
                  onChange={(e) => setOrder(e.target.value)}
                />
              </label>
              <label className="block text-sm">
                Alasan musyawarah
                <textarea
                  className={gaspolField}
                  value={reason}
                  onChange={(e) => setReason(e.target.value)}
                />
              </label>
              <button
                className={gaspolButton}
                disabled={busy}
                onClick={() =>
                  void run(async () => {
                    const result = await gaspolRequest<{
                      rows: NonNullable<Monitor["standings"]>;
                      version: string;
                    }>("/gaspol/admin/final-determination", {
                      year,
                      action: "standings-preview",
                      order: order.split(",").map(Number).filter(Boolean),
                      reason,
                    });
                    setStandingsPreview(result.rows);
                    setStandingsVersion(result.version);
                    setStandingsDecision(JSON.stringify({ order, reason }));
                    setStandingsConfirmed(false);
                  })
                }
              >
                Preview klasemen lima zona
              </button>
              {standingsPreview.length === 21 && (
                <>
                  <ol className="my-3 space-y-1 text-sm">
                    {standingsPreview.map((r) => (
                      <li key={r.schoolId}>
                        {r.rank}. {r.schoolName} - {r.badge} -{" "}
                        {r.scores.scoreTotal.toFixed(2)}
                      </li>
                    ))}
                  </ol>
                  <label className="my-3 flex gap-2 text-sm">
                    <input
                      type="checkbox"
                      checked={standingsConfirmed}
                      onChange={(e) => setStandingsConfirmed(e.target.checked)}
                    />
                    Saya mengonfirmasi 21 posisi dan keputusan musyawarah di
                    atas.
                  </label>
                </>
              )}
              <button
                className={gaspolButton}
                disabled={
                  busy ||
                  !standingsConfirmed ||
                  standingsPreview.length !== 21 ||
                  standingsDecision !== JSON.stringify({ order, reason })
                }
                onClick={() =>
                  void run(() =>
                    gaspolRequest("/gaspol/admin/final-determination", {
                      year,
                      action: "standings",
                      version: standingsVersion,
                      order: order.split(",").map(Number).filter(Boolean),
                      reason,
                    })
                  )
                }
              >
                Tetapkan klasemen resmi 1–21
              </button>
            </details>
          )}
        </div>
      )}
      {kind && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label={`Penetapan ${labels[kind]}`}
          className="fixed inset-0 z-50 flex items-center justify-center overflow-y-auto bg-black/40 p-4"
        >
          <div className="max-h-[90vh] w-full max-w-6xl space-y-4 overflow-y-auto rounded-2xl bg-white p-5">
            <h3 className="text-lg font-semibold">
              {labels[kind]} · Langkah {step}/3
            </h3>
            {step === 1 && (
              <>
                <Scores
                  rows={
                    kind === "AWARDING"
                      ? (final?.rows ?? [])
                      : (current?.rows ?? [])
                  }
                  final={kind === "AWARDING"}
                  nomination={["BOTTOM_5", "FINAL"].includes(kind)}
                />
                {current && (
                  <>
                    <p className="text-sm">
                      {kind === "TOP_5"
                        ? "Lima sekolah teratas menjadi calon finalis."
                        : kind === "BOTTOM_5"
                          ? "Lima sekolah dengan nilai terendah menjadi kandidat banding."
                          : "Tiga sekolah dengan nilai terendah menjadi penerima setara Ter Gagal."}
                    </p>
                    <Progress
                      rows={current.obligations}
                      master={false}
                      onReturn={() => {}}
                    />
                  </>
                )}
                <label className="block text-sm">
                  Pilihan sekolah jika seri pada batas seleksi
                  <select
                    className={gaspolField}
                    value={tieSchool}
                    onChange={(e) => setTieSchool(Number(e.target.value))}
                  >
                    <option value={0}>Tidak ada keputusan seri</option>
                    {(current?.rows ?? []).map((r) => (
                      <option key={r.schoolId} value={r.schoolId}>
                        {r.schoolName} (ID {r.schoolId})
                      </option>
                    ))}
                  </select>
                </label>
                <label className="block text-sm">
                  Urutan ID sekolah seri sesuai musyawarah (pisahkan koma;
                  sekolah terpilih didahulukan)
                  <input
                    className={gaspolField}
                    value={order}
                    onChange={(e) => setOrder(e.target.value)}
                  />
                </label>
                <label className="block text-sm">
                  Alasan / keputusan musyawarah
                  <textarea
                    className={gaspolField}
                    value={reason}
                    onChange={(e) => setReason(e.target.value)}
                  />
                </label>
              </>
            )}
            {step === 2 &&
              (kind === "BOTTOM_5" ? (
                <p>Penetapan kandidat tidak memerlukan BA.</p>
              ) : (
                <>
                  <label className="block text-sm">
                    Nomor BA
                    <input
                      className={gaspolField}
                      value={baNumber}
                      onChange={(e) => setBANumber(e.target.value)}
                    />
                  </label>
                  <label className="block text-sm">
                    Tanggal BA
                    <input
                      className={gaspolField}
                      type="date"
                      value={baDate}
                      onChange={(e) => setBADate(e.target.value)}
                    />
                  </label>
                  <label className="block text-sm">
                    BA resmi PDF, maksimal 10 MB
                    <input
                      className={gaspolField}
                      type="file"
                      accept="application/pdf,.pdf"
                      onChange={(e) => {
                        const f = e.target.files?.[0];
                        if (
                          f &&
                          (!f.name.toLowerCase().endsWith(".pdf") ||
                            f.size > 10 * 1024 * 1024)
                        ) {
                          setError("Pilih PDF maksimal 10 MB.");
                          return;
                        }
                        setFile(f);
                      }}
                    />
                  </label>
                  {previewURL && (
                    <iframe
                      title="Preview BA"
                      src={previewURL}
                      className="h-72 w-full rounded-xl border"
                    />
                  )}
                </>
              ))}
            {step === 3 && (
              <>
                <p className="text-sm font-semibold">
                  Peserta yang ditetapkan:{" "}
                  {selectionRows()
                    .map((r) => r.schoolName)
                    .join(", ")}
                </p>
                <p>
                  BA: {baNumber || "Tidak diperlukan"} · {baDate}
                </p>
                <Scores
                  rows={selectionRows()}
                  nomination={["BOTTOM_5", "FINAL"].includes(kind)}
                />
                <label className="flex gap-2">
                  <input
                    type="checkbox"
                    checked={confirmed}
                    onChange={(e) => setConfirmed(e.target.checked)}
                  />
                  Saya telah memeriksa nilai, peserta, BA, dan keputusan resmi.
                </label>
              </>
            )}
            {error && (
              <p role="alert" className="text-red-700">
                {error}
              </p>
            )}
            <div className="flex flex-wrap gap-2">
              <button
                className="rounded-xl border px-4 py-2"
                disabled={busy}
                onClick={() => setKind("")}
              >
                Tutup
              </button>
              {step > 1 && (
                <button
                  className={gaspolButton}
                  onClick={() => setStep(step - 1)}
                >
                  Kembali
                </button>
              )}
              {step < 3 ? (
                <button
                  className={gaspolButton}
                  disabled={
                    busy ||
                    (step === 1 &&
                      (kind === "AWARDING"
                        ? !!final?.blockers.length || !material || stale
                        : !current?.ready)) ||
                    (step === 2 &&
                      kind !== "BOTTOM_5" &&
                      (!file || !baNumber.trim() || !baDate))
                  }
                  onClick={() => setStep(step + 1)}
                >
                  Lanjut
                </button>
              ) : (
                <button
                  className={gaspolButton}
                  disabled={
                    busy || !confirmed || (kind === "AWARDING" && stale)
                  }
                  onClick={() => void run(lock)}
                >
                  {busy ? "Memproses…" : "Tetapkan & Kunci"}
                </button>
              )}
            </div>
          </div>
        </div>
      )}
      {reopenID && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Revisi penetapan resmi"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        >
          <div className="w-full max-w-lg space-y-3 rounded-2xl bg-white p-5">
            <h3 className="font-semibold">Revisi penetapan resmi</h3>
            <p className="text-sm">
              Riwayat dan dokumen lama dipertahankan. Revisi ditolak jika ada
              penetapan tahap berikutnya atau klasemen final yang bergantung
              padanya.
            </p>
            <label className="block text-sm">
              Dasar otorisasi dan alasan revisi
              <textarea
                className={gaspolField}
                value={returnReason}
                onChange={(e) => setReturnReason(e.target.value)}
              />
            </label>
            <button
              className={gaspolButton}
              disabled={busy || !returnReason.trim()}
              onClick={() =>
                void run(async () => {
                  await gaspolRequest(
                    `/gaspol/admin/result-locks/${reopenID}/reopen`,
                    { reason: returnReason }
                  );
                  setReopenID(undefined);
                })
              }
            >
              Buka revisi resmi
            </button>
            <button
              className="ml-3 rounded-xl border px-4 py-2"
              onClick={() => setReopenID(undefined)}
            >
              Batal
            </button>
            {error && (
              <p role="alert" className="text-red-700">
                {error}
              </p>
            )}
          </div>
        </div>
      )}
      {returnID && (
        <div
          role="dialog"
          aria-modal="true"
          aria-label="Kembalikan penilaian"
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/40 p-4"
        >
          <div className="w-full max-w-lg space-y-3 rounded-2xl bg-white p-5">
            <h3 className="font-semibold">Kembalikan penilaian #{returnID}</h3>
            <label className="block text-sm">
              Alasan wajib
              <textarea
                className={gaspolField}
                value={returnReason}
                onChange={(e) => setReturnReason(e.target.value)}
              />
            </label>
            <button
              className={gaspolButton}
              disabled={busy || !returnReason.trim()}
              onClick={() =>
                void run(async () => {
                  await gaspolRequest(
                    `/gaspol/stage-assessments/${returnID}/master-return`,
                    { notes: returnReason }
                  );
                  setReturnID(undefined);
                  setReturnReason("");
                })
              }
            >
              Kembalikan kepada juri
            </button>
            <button
              disabled={busy}
              className="ml-3 rounded-xl border px-4 py-2"
              onClick={() => setReturnID(undefined)}
            >
              Batal
            </button>
            {error && <p role="alert">{error}</p>}
          </div>
        </div>
      )}
    </section>
  );
}
