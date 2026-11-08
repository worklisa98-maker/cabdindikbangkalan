"use client";
import { useEffect, useState } from "react";
import { apiFetch } from "@/lib/api/client";
export const gaspolField =
  "w-full rounded-xl border border-slate-300 bg-white px-3 py-2 text-sm";
export const gaspolButton =
  "rounded-xl bg-emerald-700 px-4 py-2 text-sm font-semibold text-white disabled:opacity-50";
export async function gaspolRequest<T>(
  path: string,
  body?: unknown,
  method = "POST"
): Promise<T> {
  const response = await apiFetch(
    path,
    body === undefined ? undefined : { method, body: JSON.stringify(body) }
  );
  const payload = await response.json();
  if (!response.ok)
    throw new Error(payload.message ?? payload.error ?? "Permintaan gagal");
  return payload.data as T;
}
type Indicator = {
  key: string;
  label: string;
  weight: number;
  guidance: string;
};
type Rubric = {
  id: number;
  year: number;
  stage: string;
  version: number;
  title: string;
  guidance: string;
  provisional: boolean;
  indicators: Indicator[];
  status: string;
};
const temporary =
  "RUBRIK SEMENTARA — MENUNGGU PERSETUJUAN KOORDINATOR GASPOL BANG";
function initial(stage: string): Indicator[] {
  const labels =
    stage === "INNOVATION"
      ? [
          "Kebaruan dan kreativitas inovasi",
          "Penerapan inovasi di sekolah",
          "Dampak dan manfaat inovasi",
          "Keberlanjutan dan potensi pengembangan",
        ]
      : [
          "Identifikasi permasalahan sekolah",
          "Upaya dan tindakan perbaikan",
          "Hasil nyata dari perbaikan",
          "Kelengkapan bukti pendukung",
        ];
  const weights = stage === "INNOVATION" ? [25, 25, 30, 20] : [20, 30, 30, 20];
  return labels.map((label, i) => ({
    key: `indicator_${i + 1}`,
    label,
    weight: weights[i],
    guidance: "",
  }));
}
export function GaspolRubricManager({ year }: { year: number }) {
  const [versions, setVersions] = useState<Rubric[]>([]);
  const [stage, setStage] = useState("NOT_FAILED_NOMINATION");
  const [title, setTitle] = useState("Nominasi Tidak Tergagal");
  const [guidance, setGuidance] = useState("");
  const [items, setItems] = useState(initial(stage));
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let live = true;
    gaspolRequest<Rubric[]>(`/gaspol/admin/rubrics?year=${year}`)
      .then((v) => {
        if (live) setVersions(v);
      })
      .catch((e) => {
        if (live) setMessage(e.message);
      });
    return () => {
      live = false;
    };
  }, [year]);
  const run = async (work: () => Promise<unknown>) => {
    setBusy(true);
    try {
      await work();
      setVersions(await gaspolRequest(`/gaspol/admin/rubrics?year=${year}`));
      setMessage("Perubahan tersimpan.");
    } catch (e) {
      setMessage((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className="space-y-4 rounded-2xl border bg-white p-5">
      <h3 className="font-bold">Rubrik dinamis dan versi</h3>
      <p className="text-sm text-amber-800">{temporary}</p>
      <p className="text-sm text-slate-600">
        Simpan membuat versi draft baru. Aktivasi berlaku untuk sekolah yang
        belum memulai penilaian.
      </p>
      <select
        aria-label="Kategori rubrik"
        className={gaspolField}
        value={stage}
        onChange={(e) => {
          setStage(e.target.value);
          setItems(initial(e.target.value));
          setTitle(
            e.target.value === "INNOVATION"
              ? "Ter-Inovatif"
              : "Nominasi Tidak Tergagal"
          );
        }}
      >
        <option value="NOT_FAILED_NOMINATION">Nominasi Tidak Tergagal</option>
        <option value="INNOVATION">Ter-Inovatif</option>
      </select>
      <label className="block text-sm">
        Nama rubrik
        <input
          className={gaspolField}
          value={title}
          onChange={(e) => setTitle(e.target.value)}
        />
      </label>
      <label className="block text-sm">
        Panduan umum
        <textarea
          className={gaspolField}
          value={guidance}
          onChange={(e) => setGuidance(e.target.value)}
        />
      </label>
      {items.map((item, i) => (
        <div key={item.key} className="space-y-2 rounded-xl border p-3">
          <label className="block text-sm">
            Indikator {i + 1}
            <input
              className={gaspolField}
              value={item.label}
              onChange={(e) =>
                setItems(
                  items.map((v, n) =>
                    n === i ? { ...v, label: e.target.value } : v
                  )
                )
              }
            />
          </label>
          <label className="block text-sm">
            Bobot (%)
            <input
              className={gaspolField}
              type="number"
              min="0"
              max="100"
              step="0.01"
              value={item.weight}
              onChange={(e) =>
                setItems(
                  items.map((v, n) =>
                    n === i ? { ...v, weight: Number(e.target.value) } : v
                  )
                )
              }
            />
          </label>
          <label className="block text-sm">
            Panduan indikator
            <textarea
              className={gaspolField}
              value={item.guidance}
              onChange={(e) =>
                setItems(
                  items.map((v, n) =>
                    n === i ? { ...v, guidance: e.target.value } : v
                  )
                )
              }
            />
          </label>
          <div className="flex flex-wrap gap-2">
            <button
              type="button"
              disabled={i === 0}
              onClick={() => {
                const copy = [...items];
                [copy[i - 1], copy[i]] = [copy[i], copy[i - 1]];
                setItems(copy);
              }}
            >
              Naik
            </button>
            <button
              type="button"
              disabled={i === items.length - 1}
              onClick={() => {
                const copy = [...items];
                [copy[i + 1], copy[i]] = [copy[i], copy[i + 1]];
                setItems(copy);
              }}
            >
              Turun
            </button>
            <button
              type="button"
              onClick={() => setItems(items.filter((_, n) => n !== i))}
            >
              Hapus indikator
            </button>
          </div>
        </div>
      ))}
      <p>Total bobot: {items.reduce((n, v) => n + v.weight, 0)}%</p>
      <div className="flex flex-wrap gap-3">
        <button
          className={gaspolButton}
          onClick={() =>
            setItems([
              ...items,
              {
                key: crypto.randomUUID(),
                label: "Indikator baru",
                weight: 0,
                guidance: "",
              },
            ])
          }
        >
          Tambah indikator
        </button>
        <button
          className={gaspolButton}
          disabled={busy}
          onClick={() =>
            void run(() =>
              gaspolRequest("/gaspol/admin/rubrics", {
                year,
                stage,
                title,
                guidance,
                provisional: true,
                indicators: items,
              })
            )
          }
        >
          Simpan versi draft baru
        </button>
      </div>
      <p role="status">{message}</p>
      <div className="space-y-2">
        {versions.map((r) => (
          <div
            key={r.id}
            className="flex flex-wrap items-center gap-3 rounded-xl border p-3"
          >
            <span>
              {r.title} · v{r.version} · {r.status}
            </span>
            <button
              onClick={() => {
                setStage(r.stage);
                setTitle(r.title);
                setGuidance(r.guidance);
                setItems(r.indicators);
              }}
            >
              Salin untuk versi baru
            </button>
            {r.status === "DRAFT" && (
              <button
                className={gaspolButton}
                disabled={busy}
                onClick={() =>
                  void run(() =>
                    gaspolRequest(`/gaspol/admin/rubrics/${r.id}/activate`, {})
                  )
                }
              >
                Aktifkan secara eksplisit
              </button>
            )}
          </div>
        ))}
      </div>
    </section>
  );
}
export function GaspolRubricAssessment({
  year,
  schoolId,
  stage,
  editable,
  onSaved,
}: {
  year: number;
  schoolId: number;
  stage: string;
  editable: boolean;
  onSaved: () => Promise<void>;
}) {
  const [rubric, setRubric] = useState<Rubric>();
  const [scores, setScores] = useState<Record<string, string>>({});
  const [notes, setNotes] = useState<Record<string, string>>({});
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    if (!editable) return;
    let live = true;
    gaspolRequest<Rubric>(
      `/gaspol/assessment-rubric?year=${year}&schoolId=${schoolId}&stage=${stage}`
    )
      .then((r) => {
        if (live) setRubric(r);
      })
      .catch((e) => {
        if (live) setMessage(e.message);
      });
    return () => {
      live = false;
    };
  }, [year, schoolId, stage, editable]);
  if (!editable)
    return (
      <p className="mt-3 text-sm">
        Penilaian telah dikirim; perubahan mengikuti pengembalian resmi.
      </p>
    );
  const total =
    rubric?.indicators.reduce(
      (sum, i) => sum + (Number(scores[i.key] || 0) * i.weight) / 100,
      0
    ) ?? 0;
  return (
    <div className="mt-4 space-y-3">
      {rubric && (
        <>
          <p className="font-semibold">
            {rubric.title} · versi {rubric.version}
          </p>
          {rubric.provisional && (
            <p className="text-sm text-amber-800">{temporary}</p>
          )}
          <p>{rubric.guidance}</p>
          {rubric.indicators.map((i) => (
            <div
              key={i.key}
              className="grid gap-2 rounded-xl border p-3 sm:grid-cols-2"
            >
              <div className="sm:col-span-2">
                {i.label} ({i.weight}%)
                <p className="text-sm text-slate-500">{i.guidance}</p>
              </div>
              <label>
                Nilai 0–100
                <input
                  type="number"
                  min="0"
                  max="100"
                  step="0.01"
                  className={gaspolField}
                  value={scores[i.key] ?? ""}
                  onChange={(e) =>
                    setScores({ ...scores, [i.key]: e.target.value })
                  }
                />
              </label>
              <label>
                Catatan
                <textarea
                  className={gaspolField}
                  value={notes[i.key] ?? ""}
                  onChange={(e) =>
                    setNotes({ ...notes, [i.key]: e.target.value })
                  }
                />
              </label>
            </div>
          ))}
          <p>Skor otomatis: {total.toFixed(2)}</p>
          <button
            className={gaspolButton}
            disabled={busy}
            onClick={async () => {
              setBusy(true);
              try {
                if (
                  rubric.indicators.some(
                    (i) =>
                      scores[i.key] === undefined || scores[i.key].trim() === ""
                  )
                )
                  throw new Error("Isi semua nilai indikator.");
                await gaspolRequest("/gaspol/assessments", {
                  year,
                  schoolId,
                  stage,
                  rubricId: rubric.id,
                  rubric: Object.fromEntries(
                    rubric.indicators.map((i) => [i.key, Number(scores[i.key])])
                  ),
                  indicatorNotes: Object.fromEntries(
                    rubric.indicators.map((i) => [i.key, notes[i.key] ?? ""])
                  ),
                  score: 0,
                  notes: rubric.indicators
                    .map((i) => notes[i.key] ?? "")
                    .filter(Boolean)
                    .join("; "),
                });
                setMessage("Terkirim ke Koordinator.");
                await onSaved();
              } catch (e) {
                setMessage((e as Error).message);
              } finally {
                setBusy(false);
              }
            }}
          >
            Kirim penilaian
          </button>
        </>
      )}
      <p role="status">{message}</p>
    </div>
  );
}
type Innovation = {
  year: number;
  schoolId: number;
  status: string;
  title: string;
  background: string;
  problem: string;
  description: string;
  implementation: string;
  impact: string;
  sustainability: string;
  documentationUrl: string;
  evidenceUrls: string[];
  history?: { action: string; actor: string; at: string }[];
};
const innovationFields = {
  title: "Judul inovasi",
  background: "Latar belakang",
  problem: "Permasalahan yang diselesaikan",
  description: "Uraian inovasi",
  implementation: "Pelaksanaan / penerapan",
  impact: "Dampak dan manfaat",
  sustainability: "Keberlanjutan",
  documentationUrl: "Link dokumentasi",
};
export function GaspolInnovationForm({
  year,
  schoolId,
  school = false,
}: {
  year: number;
  schoolId: number;
  school?: boolean;
}) {
  const [value, setValue] = useState<Innovation>();
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let live = true;
    gaspolRequest<Innovation>(
      `/gaspol/innovation-submission?year=${year}&schoolId=${schoolId}`
    )
      .then((v) => {
        if (live) setValue(v);
      })
      .catch((e) => {
        if (live) setMessage(e.message);
      });
    return () => {
      live = false;
    };
  }, [year, schoolId]);
  const editable =
    school &&
    value &&
    (value.status === "DRAFT" || value.status === "RETURNED");
  return (
    <section className="space-y-3 rounded-2xl border p-4">
      <h3 className="font-bold">Pengajuan Ter-Inovatif</h3>
      {value && (
        <>
          <p>Status: {value.status}</p>
          {(
            Object.keys(innovationFields) as (keyof typeof innovationFields)[]
          ).map((key) => (
            <label className="block text-sm" key={key}>
              {innovationFields[key]}
              <textarea
                className={gaspolField}
                disabled={!editable}
                value={value[key] ?? ""}
                onChange={(e) => setValue({ ...value, [key]: e.target.value })}
              />
            </label>
          ))}
          <label className="block text-sm">
            Bukti pendukung (satu tautan per baris)
            <textarea
              className={gaspolField}
              disabled={!editable}
              value={(value.evidenceUrls ?? []).join("\n")}
              onChange={(e) =>
                setValue({ ...value, evidenceUrls: e.target.value.split("\n") })
              }
            />
          </label>
          {editable && (
            <div className="flex gap-3">
              {["DRAFT", "SUBMITTED"].map((status) => (
                <button
                  key={status}
                  className={gaspolButton}
                  disabled={busy}
                  onClick={async () => {
                    setBusy(true);
                    try {
                      await gaspolRequest(
                        "/gaspol/innovation-submission",
                        {
                          ...value,
                          year,
                          status,
                          evidenceUrls: (value.evidenceUrls ?? []).filter((v) =>
                            v.trim()
                          ),
                        },
                        "PUT"
                      );
                      setValue(
                        await gaspolRequest<Innovation>(
                          `/gaspol/innovation-submission?year=${year}&schoolId=${schoolId}`
                        )
                      );
                      setMessage("Pengajuan tersimpan.");
                    } catch (e) {
                      setMessage((e as Error).message);
                    } finally {
                      setBusy(false);
                    }
                  }}
                >
                  {status === "DRAFT"
                    ? "Simpan draft"
                    : "Kirim dan kunci pengajuan"}
                </button>
              ))}
            </div>
          )}
        </>
      )}
      {value?.history && (
        <details>
          <summary>Histori pengajuan</summary>
          {value.history.map((e, i) => (
            <p key={i}>
              {e.actor} - {e.action} - {new Date(e.at).toLocaleString("id-ID")}
            </p>
          ))}
        </details>
      )}
      <p role="status">{message}</p>
    </section>
  );
}

export function GaspolAssessmentDetails({ id }: { id: number }) {
  const [data, setData] = useState<{
    rubric?: {
      version?: Rubric;
      scores?: Record<string, number>;
      indicatorNotes?: Record<string, string>;
    };
    notes?: string;
  }>();
  const [error, setError] = useState("");
  return (
    <details
      className="mt-2 rounded-xl border p-3 text-sm"
      onToggle={(e) => {
        if (e.currentTarget.open && !data)
          gaspolRequest<typeof data>(`/gaspol/assessments/${id}/details`)
            .then(setData)
            .catch((e) => setError(e.message));
      }}
    >
      <summary>Rincian indikator dan versi rubrik</summary>
      {data?.rubric?.version && (
        <>
          <p>
            {data.rubric.version.title} - v{data.rubric.version.version}
          </p>
          {data.rubric.version.indicators.map((i) => (
            <div key={i.key} className="mt-2">
              <p>
                {i.label} ({i.weight}%) - Nilai{" "}
                {data.rubric?.scores?.[i.key] ?? "-"}
              </p>
              <p className="whitespace-pre-wrap text-slate-600">
                {data.rubric?.indicatorNotes?.[i.key] ?? ""}
              </p>
            </div>
          ))}
        </>
      )}
      <p>{data?.notes}</p>
      <p role="status">{error}</p>
    </details>
  );
}
