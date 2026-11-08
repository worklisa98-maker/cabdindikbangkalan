"use client";
import { useEffect, useState } from "react";
import Image from "next/image";
import { GaspolAccountSelect } from "./GaspolSelectors";
import QRCode from "qrcode";
import { gaspolRequest, gaspolButton, gaspolField } from "./GaspolRubrics";
type Candidate = {
  schoolId: number;
  schoolName: string;
  npsn: string;
  shortCode: string;
};
type Voting = {
  id: number;
  year: number;
  status: string;
  candidates: Candidate[];
  voted: boolean;
  canVote: boolean;
  registered?: number;
  present?: number;
  votedCount?: number;
  notVoted?: number;
  participation?: number;
  denominator?: number;
  roster?: {
    userId: number;
    username: string;
    present: boolean;
    active: boolean;
    voted: boolean;
  }[];
  results?: Record<string, number>;
  readyToEstablish?: boolean;
  preview?: {
    schoolId: number;
    likes?: number;
    verified: boolean;
    eligible: boolean;
    score: { internalScore: number; likeScore: number; total: number };
  }[];
};
function QRCard({
  candidate,
  session,
  year,
}: {
  candidate: Candidate;
  session: number;
  year: number;
}) {
  const [qr, setQR] = useState("");
  useEffect(() => {
    let live = true;
    const url = new URL("/gaspol/voting", window.location.origin);
    url.searchParams.set("year", String(year));
    url.searchParams.set("schoolId", String(candidate.schoolId));
    url.searchParams.set("sessionId", String(session));
    QRCode.toDataURL(url.href, { width: 360, margin: 2 }).then((v) => {
      if (live) setQR(v);
    });
    return () => {
      live = false;
    };
  }, [candidate.schoolId, session, year]);
  return (
    <div className="space-y-2 rounded-xl border p-3">
      <h4 className="font-semibold">{candidate.schoolName}</h4>
      <p className="text-sm">
        NPSN {candidate.npsn} · {year} · Ter-Favorit
      </p>
      {qr && (
        <Image
          src={qr}
          alt={`QR voting ${candidate.schoolName}`}
          width={240}
          height={240}
          unoptimized
        />
      )}
      <p className="font-mono break-all">Kode: {candidate.shortCode}</p>
      <div className="flex flex-wrap gap-3">
        {qr && (
          <a
            className={gaspolButton}
            href={qr}
            download={`QR-Ter-Favorit-${year}-${candidate.schoolId}.png`}
          >
            Download QR
          </a>
        )}
        <button
          className={gaspolButton}
          disabled={!qr}
          onClick={() => {
            const w = window.open("", "_blank");
            if (!w) return;
            w.document.title = `QR ${candidate.schoolName}`;
            const title = w.document.createElement("h1");
            title.textContent = candidate.schoolName;
            const info = w.document.createElement("p");
            info.textContent = `NPSN ${candidate.npsn} · ${year} · Ter-Favorit · Kode ${candidate.shortCode}`;
            const img = w.document.createElement("img");
            img.src = qr;
            img.width = 360;
            img.onload = () => w.print();
            w.document.body.append(title, info, img);
          }}
        >
          Cetak QR
        </button>
      </div>
    </div>
  );
}
export function GaspolVotingPanel({
  year,
  master = false,
  school = false,
  initialSchoolId = 0,
  initialSessionId = 0,
}: {
  year: number;
  master?: boolean;
  school?: boolean;
  initialSchoolId?: number;
  initialSessionId?: number;
}) {
  const [session, setSession] = useState<Voting>();
  const [message, setMessage] = useState("");
  const [busy, setBusy] = useState(false);
  const [selected, setSelected] = useState(initialSchoolId);
  const [code, setCode] = useState("");
  const [assistReason, setAssistReason] = useState("");
  const [tieSchool, setTieSchool] = useState(0);
  const [operator, setOperator] = useState("");
  const [confirm, setConfirm] = useState(false);
  const [capacity, setCapacity] = useState(33);
  const [closes, setCloses] = useState("");
  const [cutoff, setCutoff] = useState("");
  const [users, setUsers] = useState("");
  const [accounts, setAccounts] = useState("");
  const [credentials, setCredentials] = useState<
    { username: string; initialPassword: string }[]
  >([]);
  useEffect(() => {
    let live = true;
    gaspolRequest<Voting>(`/gaspol/voting?year=${year}`)
      .then((v) => {
        if (initialSessionId && v.id !== initialSessionId)
          throw new Error(
            "QR berasal dari sesi lain. Gunakan QR resmi sesi berjalan."
          );
        if (live && v.id) setSession(v);
      })
      .catch((e) => {
        if (live) setMessage(e.message);
      });
    return () => {
      live = false;
    };
  }, [year, initialSessionId]);
  const reload = async () =>
    setSession(await gaspolRequest<Voting>(`/gaspol/voting?year=${year}`));
  const command = async (
    action: string,
    extra: Record<string, unknown> = {}
  ) => {
    setBusy(true);
    try {
      const out = await gaspolRequest<{
        credentials?: { username: string; initialPassword: string }[];
      }>("/gaspol/admin/voting", {
        action,
        year,
        sessionId: session?.id,
        ...extra,
      });
      if (out.credentials) setCredentials(out.credentials);
      await reload();
      setMessage("Perubahan tersimpan.");
    } catch (e) {
      setMessage((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  const vote = async () => {
    setBusy(true);
    try {
      await gaspolRequest("/gaspol/voting/vote", {
        sessionId: session?.id,
        schoolId: selected,
        shortCode: code,
      });
      await reload();
      setConfirm(false);
      setMessage("Suara Anda berhasil dicatat.");
    } catch (e) {
      setMessage((e as Error).message);
    } finally {
      setBusy(false);
    }
  };
  return (
    <section className="space-y-4 rounded-2xl border bg-white p-5">
      <h3 className="font-bold">Ter-Favorit — voting internal {year}</h3>
      {!session && master && (
        <div className="grid gap-3 sm:grid-cols-2">
          <label>
            Kapasitas pemilih
            <input
              className={gaspolField}
              type="number"
              min="1"
              value={capacity}
              onChange={(e) => setCapacity(Number(e.target.value))}
            />
          </label>
          <label>
            Batas voting
            <input
              className={gaspolField}
              type="datetime-local"
              value={closes}
              onChange={(e) => setCloses(e.target.value)}
            />
          </label>
          <label>
            Cut-off like bersama
            <input
              className={gaspolField}
              type="datetime-local"
              value={cutoff}
              onChange={(e) => setCutoff(e.target.value)}
            />
          </label>
          <button
            className={gaspolButton}
            disabled={busy || !closes || !cutoff}
            onClick={() =>
              void command("CREATE", {
                capacity,
                closesAt: new Date(closes).toISOString(),
                cutoffAt: new Date(cutoff).toISOString(),
              })
            }
          >
            Buat sesi draft dengan Top 5 resmi
          </button>
        </div>
      )}
      {session && (
        <>
          <p>
            Status: <strong>{session.status}</strong>
          </p>
          {master && (
            <>
              <p>
                Terdaftar {session.registered} · Hadir {session.present} · Sudah
                memilih {session.votedCount} · Belum {session.notVoted} ·
                Partisipasi {session.participation?.toFixed(1)}%
              </p>
              {session.status === "DRAFT" && (
                <div className="space-y-3">
                  <GaspolAccountSelect
                    year={year}
                    label="Akun pemilih existing"
                    multiple
                    value={users ? users.split(",").map(Number) : []}
                    onChange={(ids) => setUsers(ids.join(","))}
                  />
                  <button
                    className={gaspolButton}
                    disabled={busy || !users.trim()}
                    onClick={() =>
                      void command("VOTERS", {
                        userIds: users.split(",").map(Number),
                        active: true,
                        present: false,
                      })
                    }
                  >
                    Tambahkan akun existing
                  </button>
                  <label className="block text-sm">
                    Akun baru massal — satu username,nama per baris
                    <textarea
                      className={gaspolField}
                      value={accounts}
                      onChange={(e) => setAccounts(e.target.value)}
                    />
                  </label>
                  <button
                    className={gaspolButton}
                    disabled={busy || !accounts.trim()}
                    onClick={() =>
                      void command("ACCOUNTS", {
                        accounts: accounts
                          .split("\n")
                          .filter((v) => v.trim())
                          .map((line) => {
                            const [username, ...name] = line.split(",");
                            return {
                              username: username.trim(),
                              name: name.join(",").trim(),
                            };
                          }),
                        active: true,
                        present: false,
                      })
                    }
                  >
                    Buat / gunakan kembali akun pemilih
                  </button>
                  <GaspolAccountSelect
                    year={year}
                    label="Operator bantuan"
                    value={operator ? [Number(operator)] : []}
                    onChange={(ids) =>
                      setOperator(ids[0] ? String(ids[0]) : "")
                    }
                  />
                  <button
                    className={gaspolButton}
                    disabled={busy || !operator}
                    onClick={async () => {
                      try {
                        await gaspolRequest("/gaspol/voting/assistance", {
                          action: "ASSIGN",
                          sessionId: session.id,
                          userId: Number(operator),
                          active: true,
                        });
                        setMessage("Operator ditugaskan.");
                      } catch (e) {
                        setMessage((e as Error).message);
                      }
                    }}
                  >
                    Delegasikan operator
                  </button>
                  <button
                    className={gaspolButton}
                    disabled={busy}
                    onClick={() => void command("OPEN")}
                  >
                    Sahkan daftar hadir dan buka voting
                  </button>
                </div>
              )}
              {session.status === "OPEN" && (
                <button
                  className={gaspolButton}
                  disabled={busy}
                  onClick={() => void command("CLOSE")}
                >
                  Tutup voting
                </button>
              )}
              {credentials.length > 0 && (
                <div className="rounded-xl border border-amber-300 p-3">
                  <p>
                    Kredensial awal ditampilkan sekali pada respons ini.
                    Sampaikan langsung melalui kanal privat; tidak disimpan di
                    histori.
                  </p>
                  {credentials.map((c) => (
                    <p key={c.username} className="font-mono break-all">
                      {c.username}: {c.initialPassword}
                    </p>
                  ))}
                  <button onClick={() => setCredentials([])}>
                    Hapus dari layar
                  </button>
                </div>
              )}
              <div className="space-y-2">
                {session.roster?.map((r) => (
                  <div
                    key={r.userId}
                    className="flex flex-wrap items-center gap-3 rounded-xl border p-2"
                  >
                    <span>
                      {r.username} (ID {r.userId}) ·{" "}
                      {r.voted ? "Sudah memilih" : "Belum memilih"}
                    </span>
                    <label>
                      <input
                        type="checkbox"
                        checked={r.present}
                        disabled={busy || session.status !== "DRAFT"}
                        onChange={(e) =>
                          void command("VOTERS", {
                            userIds: [r.userId],
                            present: e.target.checked,
                            active: r.active,
                          })
                        }
                      />{" "}
                      Hadir
                    </label>
                    <label>
                      <input
                        type="checkbox"
                        checked={r.active}
                        disabled={busy || session.status !== "DRAFT"}
                        onChange={(e) =>
                          void command("VOTERS", {
                            userIds: [r.userId],
                            present: r.present,
                            active: e.target.checked,
                          })
                        }
                      />{" "}
                      Akses aktif
                    </label>
                  </div>
                ))}
              </div>
            </>
          )}
          {!master && !school && (
            <>
              {session.voted ? (
                <p className="rounded-xl bg-emerald-50 p-3">
                  Suara Anda telah tercatat. Satu akun hanya dapat memilih satu
                  kali dalam sesi ini.
                </p>
              ) : (
                <>
                  <label className="block text-sm">
                    Sekolah pilihan
                    <select
                      className={gaspolField}
                      value={selected}
                      disabled={!session.canVote}
                      onChange={(e) => {
                        setSelected(Number(e.target.value));
                        setCode("");
                        setConfirm(false);
                      }}
                    >
                      <option value="0">Pilih sekolah / gunakan kode</option>
                      {session.candidates.map((c) => (
                        <option key={c.schoolId} value={c.schoolId}>
                          {c.schoolName} · {c.npsn}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="block text-sm">
                    Fallback kode pendek
                    <input
                      className={gaspolField}
                      value={code}
                      disabled={!session.canVote}
                      onChange={(e) => {
                        setCode(e.target.value);
                        setSelected(0);
                        setConfirm(false);
                      }}
                    />
                  </label>
                  <button
                    className={gaspolButton}
                    disabled={busy || !session.canVote || (!selected && !code)}
                    onClick={() => {
                      const target = session.candidates.find((c) =>
                        selected
                          ? c.schoolId === selected
                          : c.shortCode === code.trim().toUpperCase()
                      );
                      if (!target) {
                        setMessage("Kode sekolah tidak ditemukan.");
                        return;
                      }
                      setSelected(target.schoolId);
                      setConfirm(true);
                    }}
                  >
                    Tinjau pilihan
                  </button>
                  {confirm && (
                    <div className="rounded-xl border border-amber-300 p-4">
                      <p>
                        Konfirmasi suara untuk{" "}
                        <strong>
                          {
                            session.candidates.find(
                              (c) => c.schoolId === selected
                            )?.schoolName
                          }
                        </strong>
                        . Pilihan tidak dapat diubah.
                      </p>
                      <button
                        className={gaspolButton}
                        disabled={busy}
                        onClick={() => void vote()}
                      >
                        Konfirmasi suara
                      </button>
                      <button
                        className="ml-4"
                        onClick={() => setConfirm(false)}
                      >
                        Batal
                      </button>
                      <label className="mt-3 block text-sm">
                        Bantuan perangkat - alasan
                        <textarea
                          className={gaspolField}
                          value={assistReason}
                          onChange={(e) => setAssistReason(e.target.value)}
                        />
                      </label>
                      <button
                        className={gaspolButton}
                        disabled={busy || !assistReason.trim()}
                        onClick={async () => {
                          setBusy(true);
                          try {
                            await gaspolRequest("/gaspol/voting/assistance", {
                              action: "REQUEST",
                              sessionId: session.id,
                              schoolId: selected,
                              reason: assistReason,
                            });
                            setMessage(
                              "Persetujuan tercatat untuk sekolah ini. Operator yang ditugaskan dapat membantu mencatat suara."
                            );
                            setConfirm(false);
                          } catch (e) {
                            setMessage((e as Error).message);
                          } finally {
                            setBusy(false);
                          }
                        }}
                      >
                        Setujui bantuan operator untuk pilihan ini
                      </button>
                    </div>
                  )}
                </>
              )}
            </>
          )}
          {(master || school) && (
            <div className="grid gap-4 md:grid-cols-2">
              {session.candidates.map((c) => (
                <QRCard
                  key={c.schoolId}
                  candidate={c}
                  session={session.id}
                  year={year}
                />
              ))}
            </div>
          )}
          {master && session.results && (
            <div>
              <p className="font-semibold">Hasil setelah penutupan (preview)</p>
              {session.candidates.map((c) => (
                <p key={c.schoolId}>
                  {c.schoolName}: {session.results?.[String(c.schoolId)] ?? 0}{" "}
                  suara
                </p>
              ))}
              {session.preview?.map((r) => (
                <p key={r.schoolId} className="text-sm">
                  {
                    session.candidates.find((c) => c.schoolId === r.schoolId)
                      ?.schoolName
                  }
                  : voting {r.score.internalScore}, like {r.score.likeScore},
                  gabungan {r.score.total} / 100. Like {r.likes ?? "belum ada"}{" "}
                  ({r.verified ? "terverifikasi" : "belum terverifikasi"}).{" "}
                  {r.eligible ? "Sumber siap" : "Belum layak ditetapkan"}.
                </p>
              ))}
              <p className="text-sm">
                {session.readyToEstablish
                  ? "Seluruh bukti siap untuk penetapan."
                  : "Penetapan menunggu sesi ditutup, cut-off tercapai, dan seluruh bukti like sah."}
              </p>
              <label className="block text-sm">
                Pilihan jika hasil seri penuh (sesuai keputusan resmi)
                <select
                  className={gaspolField}
                  value={tieSchool}
                  onChange={(e) => setTieSchool(Number(e.target.value))}
                >
                  <option value={0}>Tidak ada keputusan seri</option>
                  {session.candidates.map((c) => (
                    <option key={c.schoolId} value={c.schoolId}>
                      {c.schoolName}
                    </option>
                  ))}
                </select>
              </label>
              <p className="text-sm text-slate-600">
                Penetapan Ter Favorit dilakukan bersama Juara 1 dan Ter Inovatif
                melalui tab Penetapan & BA.
              </p>
            </div>
          )}
        </>
      )}
      <p role="status" className="text-sm">
        {message}
      </p>
    </section>
  );
}
