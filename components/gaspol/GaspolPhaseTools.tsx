"use client";
import { GaspolDeterminations } from "./GaspolDeterminations";
import { useEffect, useState } from "react";
import {
  GaspolRubricManager,
  GaspolAssessmentDetails,
  gaspolRequest,
  gaspolButton,
  gaspolField,
} from "./GaspolRubrics";
import { GaspolAccountSelect, GaspolSchoolSelect } from "./GaspolSelectors";
import { GaspolVotingPanel } from "./GaspolVoting";
type Like = {
  id: number;
  schoolId: number;
  schoolName: string;
  likeCount: number;
  status: string;
  reason: string;
  evidenceUrl: string;
  contentUrl: string;
  capturedAt: string;
  reviewNotes: string;
  actorUserId: number;
};
type Overview = {
  likeSchools: { schoolId: number; schoolName: string }[];
  cutoffAt?: string;
  assessments?: {
    id: number;
    schoolId: number;
    schoolName: string;
    stage: string;
    assessor: string;
    rawScore: number;
    finalScore?: number;
    status: string;
    eligible: boolean;
    rubric: unknown;
    notes: string;
  }[];
  audit?: { actor: string; action: string; at: string; payload: unknown }[];
};
function Likes({ year, master }: { year: number; master: boolean }) {
  const [rows, setRows] = useState<Like[]>([]);
  const [overview, setOverview] = useState<Overview>();
  const [message, setMessage] = useState("");
  const [school, setSchool] = useState("");
  const [user, setUser] = useState("");
  const [count, setCount] = useState(0);
  const [url, setUrl] = useState("");
  const [evidence, setEvidence] = useState("");
  const [reason, setReason] = useState("");
  const [at, setAt] = useState("");
  const [busy, setBusy] = useState(false);
  useEffect(() => {
    let live = true;
    Promise.all([
      gaspolRequest<Like[]>(`/gaspol/likes?year=${year}`),
      gaspolRequest<Overview>(`/gaspol/phase-overview?year=${year}`),
    ])
      .then(([r, o]) => {
        if (live) {
          setRows(r);
          setOverview(o);
          if (o.cutoffAt) setAt(new Date(o.cutoffAt).toISOString());
        }
      })
      .catch((e) => {
        if (live) setMessage(e.message);
      });
    return () => {
      live = false;
    };
  }, [year]);
  const run = async (action: string, extra: Record<string, unknown>) => {
    setBusy(true);
    try {
      await gaspolRequest("/gaspol/likes", { year, action, ...extra });
      setRows(await gaspolRequest(`/gaspol/likes?year=${year}`));
      setMessage("Perubahan tersimpan.");
    } catch (e) {
      setMessage((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className="space-y-4 rounded-2xl border bg-white p-5">
      <h3 className="font-bold">Delegasi Verifikasi Like Konten</h3>
      <p className="text-sm">
        Cut-off bersama:{" "}
        {overview?.cutoffAt
          ? new Date(overview.cutoffAt).toLocaleString("id-ID")
          : "Belum ditetapkan"}
        . Hanya data terverifikasi yang masuk skor final.
      </p>
      {master ? (
        <GaspolSchoolSelect
          year={year}
          value={school ? school.split(",").map(Number) : []}
          onChange={(ids) => setSchool(ids.join(","))}
        />
      ) : (
        <label className="block text-sm">
          Sekolah tugas
          <select
            className={gaspolField}
            value={school}
            onChange={(e) => setSchool(e.target.value)}
          >
            <option value="">Pilih sekolah</option>
            {overview?.likeSchools.map((s) => (
              <option key={s.schoolId} value={s.schoolId}>
                {s.schoolName}
              </option>
            ))}
          </select>
        </label>
      )}
      {master && (
        <>
          <GaspolAccountSelect
            year={year}
            label="Petugas verifikasi like"
            value={user ? [Number(user)] : []}
            onChange={(ids) => setUser(ids[0] ? String(ids[0]) : "")}
          />
          <button
            className={gaspolButton}
            disabled={busy}
            onClick={() =>
              void run("ASSIGN", {
                schoolIds: school.split(",").map(Number),
                userId: Number(user),
                active: true,
              })
            }
          >
            Tugaskan petugas
          </button>
          <button
            className={gaspolButton}
            disabled={busy}
            onClick={() =>
              void run("ASSIGN", {
                schoolIds: school.split(",").map(Number),
                userId: Number(user),
                active: false,
              })
            }
          >
            Cabut tugas
          </button>
        </>
      )}
      <div className="grid gap-3 sm:grid-cols-2">
        <label>
          Link konten resmi
          <input
            className={gaspolField}
            value={url}
            onChange={(e) => setUrl(e.target.value)}
          />
        </label>
        <label>
          Jumlah like
          <input
            className={gaspolField}
            type="number"
            min="0"
            value={count}
            onChange={(e) => setCount(Number(e.target.value))}
          />
        </label>
        <label>
          Waktu pengambilan (ISO cut-off bersama)
          <input
            className={gaspolField}
            value={at}
            onChange={(e) => setAt(e.target.value)}
          />
        </label>
        <label>
          Link screenshot bukti
          <input
            className={gaspolField}
            value={evidence}
            onChange={(e) => setEvidence(e.target.value)}
          />
        </label>
        <label className="sm:col-span-2">
          Keterangan / alasan koreksi
          <textarea
            className={gaspolField}
            value={reason}
            onChange={(e) => setReason(e.target.value)}
          />
        </label>
      </div>
      <div className="flex gap-3">
        {["DRAFT", "SUBMIT"].map((action) => (
          <button
            className={gaspolButton}
            disabled={busy || !school || school.includes(",")}
            key={action}
            onClick={() =>
              void run(action, {
                schoolId: Number(school),
                likeCount: count,
                contentUrl: url,
                evidenceUrl: evidence,
                reason,
                capturedAt: at,
              })
            }
          >
            {action === "DRAFT" ? "Simpan draft" : "Ajukan verifikasi"}
          </button>
        ))}
      </div>
      {rows.map((r) => (
        <div key={r.id} className="space-y-2 rounded-xl border p-3">
          <p>
            {r.schoolName} · {r.likeCount} like · {r.status} · Petugas #
            {r.actorUserId}
          </p>
          <p>
            {new Date(r.capturedAt).toLocaleString("id-ID")} · {r.reason}
          </p>
          <a
            className="text-emerald-700 underline"
            href={r.evidenceUrl}
            target="_blank"
            rel="noopener noreferrer"
          >
            Bukti screenshot
          </a>
          <p>{r.reviewNotes}</p>
          {master && r.status === "SUBMITTED" && (
            <div className="flex gap-3">
              <button
                className={gaspolButton}
                disabled={busy}
                onClick={() => void run("VERIFY", { id: r.id })}
              >
                Verifikasi
              </button>
              <button
                className={gaspolButton}
                disabled={busy || !reason.trim()}
                onClick={() => void run("RETURN", { id: r.id, reason })}
              >
                Kembalikan dengan alasan koreksi
              </button>
            </div>
          )}
          <button
            onClick={() => {
              setSchool(String(r.schoolId));
              setCount(r.likeCount);
              setUrl(r.contentUrl);
              setEvidence(r.evidenceUrl);
              setReason("");
              setAt(r.capturedAt);
            }}
          >
            Salin untuk koreksi baru
          </button>
        </div>
      ))}
      <p role="status">{message}</p>
    </section>
  );
}
function ResultOverview({ year }: { year: number }) {
  const [data, setData] = useState<Overview>();
  const [error, setError] = useState("");
  useEffect(() => {
    let live = true;
    gaspolRequest<Overview>(`/gaspol/phase-overview?year=${year}`)
      .then((v) => {
        if (live) setData(v);
      })
      .catch((e) => {
        if (live) setError(e.message);
      });
    return () => {
      live = false;
    };
  }, [year]);
  return (
    <section className="space-y-3 rounded-2xl border bg-white p-5">
      <h3 className="font-bold">Rincian penilaian dan audit</h3>
      <p>
        Rincian ini adalah preview. Penetapan resmi mengikuti workflow dan hasil
        terkunci.
      </p>
      {data?.assessments?.map((a) => (
        <details className="rounded-xl border p-3" key={a.id}>
          <summary>
            {a.schoolName} · {a.stage} · {a.rawScore} · {a.status} ·{" "}
            {a.eligible ? "Tervalidasi" : "Belum layak"}
          </summary>
          <p>
            Penilai: {a.assessor} · Nilai final {a.finalScore ?? "—"}
          </p>
          <p>{a.notes}</p>
          <GaspolAssessmentDetails id={a.id} />
        </details>
      ))}
      <h4 className="font-semibold">200 tindakan terakhir</h4>
      {data?.audit?.map((a, i) => (
        <details className="rounded-xl border p-3" key={i}>
          <summary>
            {a.actor} · {a.action} · {new Date(a.at).toLocaleString("id-ID")}
          </summary>
          <pre className="overflow-auto text-xs break-all whitespace-pre-wrap">
            {JSON.stringify(a.payload, null, 2)}
          </pre>
        </details>
      ))}
      <p role="status">{error}</p>
    </section>
  );
}
export function GaspolDelegatedTools({ year }: { year: number }) {
  const [overview, setOverview] = useState<Overview>();
  const [queue, setQueue] = useState<
    {
      id: number;
      sessionId: number;
      voter: string;
      school: string;
      reason: string;
      consentAt: string;
    }[]
  >([]);
  const [message, setMessage] = useState("");
  useEffect(() => {
    let live = true;
    Promise.all([
      gaspolRequest<Overview>(`/gaspol/phase-overview?year=${year}`),
      gaspolRequest<typeof queue>(`/gaspol/voting/assistance?year=${year}`),
    ])
      .then(([o, q]) => {
        if (live) {
          setOverview(o);
          setQueue(q);
        }
      })
      .catch(() => {});
    return () => {
      live = false;
    };
  }, [year]);
  return (
    <>
      {!!overview?.likeSchools.length && <Likes year={year} master={false} />}{" "}
      {queue.length > 0 && (
        <section className="space-y-3 rounded-2xl border bg-white p-5">
          <h3 className="font-bold">
            Bantuan voting dengan persetujuan pemilih
          </h3>
          {queue.map((q) => (
            <div key={q.id} className="rounded-xl border p-3">
              <p>
                {q.voter} telah menyetujui pilihan {q.school} pada{" "}
                {new Date(q.consentAt).toLocaleString("id-ID")}
              </p>
              <p>{q.reason}</p>
              <button
                className={gaspolButton}
                onClick={async () => {
                  try {
                    await gaspolRequest("/gaspol/voting/assistance", {
                      action: "EXECUTE",
                      sessionId: q.sessionId,
                      id: q.id,
                    });
                    setQueue(
                      await gaspolRequest(
                        `/gaspol/voting/assistance?year=${year}`
                      )
                    );
                    setMessage("Suara dicatat sesuai persetujuan pemilih.");
                  } catch (e) {
                    setMessage((e as Error).message);
                  }
                }}
              >
                Catat sesuai persetujuan
              </button>
            </div>
          ))}
          <p role="status">{message}</p>
        </section>
      )}
    </>
  );
}
export function GaspolPhaseTools({
  year,
  master = false,
}: {
  year: number;
  master?: boolean;
}) {
  const [tab, setTab] = useState(master ? "rubric" : "likes");
  const tabs = master
    ? [
        ["determinations", "Penetapan & BA"],
        ["rubric", "Rubrik"],
        ["voting", "Ter-Favorit"],
        ["likes", "Verifikasi Like"],
        ["results", "Rincian & Histori"],
      ]
    : [["likes", "Verifikasi Like"]];
  return (
    <div className="space-y-3">
      <div className="flex flex-wrap gap-2">
        {tabs.map(([key, label]) => (
          <button
            key={key}
            className={
              tab === key ? gaspolButton : "rounded-xl border px-4 py-2 text-sm"
            }
            onClick={() => setTab(key)}
          >
            {label}
          </button>
        ))}
      </div>
      {tab === "determinations" && master && (
        <GaspolDeterminations year={year} />
      )}
      {tab === "rubric" && master && <GaspolRubricManager year={year} />}{" "}
      {tab === "voting" && <GaspolVotingPanel year={year} master={master} />}{" "}
      {tab === "likes" && <Likes year={year} master={master} />}{" "}
      {tab === "results" && master && <ResultOverview year={year} />}
    </div>
  );
}
