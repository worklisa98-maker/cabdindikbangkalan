"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { useAuth } from "@/lib/auth/client";
import { apiFetch } from "@/lib/api/client";

import { Card, CardContent } from "@/components/ui/card";
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select";

const formatDate = (value: string) => {
  if (!value) return "-";
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) return value;
  return date.toLocaleString("id-ID");
};

type UploadRecord = {
  id: number;
  type: string;
  year: number;
  status: string;
  errorLog: string | null;
  originalFilename: string;
  createdAt: string;
};

type School = {
  id: number;
  namaSekolah: string;
};

const fetchSchools = async (): Promise<School[]> => {
  const response = await apiFetch("/schools");
  if (!response.ok) {
    throw new Error("Failed to fetch schools");
  }
  const payload = (await response.json()) as { data: School[] };
  return payload.data ?? [];
};

const fetchUploads = async (): Promise<UploadRecord[]> => {
  const response = await apiFetch("/uploads");
  if (!response.ok) {
    throw new Error("Failed to fetch uploads");
  }
  const payload = (await response.json()) as { data: UploadRecord[] };
  return payload.data ?? [];
};

export default function UploadsPage() {
  const router = useRouter();
  const { status, user } = useAuth();
  const [uploads, setUploads] = useState<UploadRecord[]>([]);
  const [isLoading, setIsLoading] = useState(true);
  const [error, setError] = useState("");
  const [message, setMessage] = useState("");
  const [schools, setSchools] = useState<School[]>([]);
  const [selectedSchoolId, setSelectedSchoolId] = useState<number | null>(null);
  const [type, setType] = useState("ACHIEVEMENT");
  const [year, setYear] = useState(String(new Date().getFullYear()));
  const [file, setFile] = useState<File | null>(null);
  const [isSubmitting, setIsSubmitting] = useState(false);

  useEffect(() => {
    if (status !== "authenticated") return;
    // eslint-disable-next-line react-hooks/set-state-in-effect
    setIsLoading(true);
    fetchUploads()
      .then((data) => {
        setUploads(data);
        setError("");
      })
      .catch(() => setError("Gagal memuat riwayat upload."))
      .finally(() => setIsLoading(false));
  }, [status]);

  useEffect(() => {
    if (status !== "authenticated") return;
    fetchSchools()
      .then((data) => {
        setSchools(data);
        if (data.length > 0) {
          const defaultId =
            user?.role === "ADMIN"
              ? data[0].id
              : user?.schoolId ?? data[0].id;
          setSelectedSchoolId(defaultId);
        }
      })
      .catch(() => setSchools([]));
  }, [status, user?.role, user?.schoolId]);

  const handleSubmit = async (event: React.FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setMessage("");
    if (!file) {
      setMessage("File belum dipilih.");
      return;
    }

    setIsSubmitting(true);
    const formData = new FormData();
    formData.append("file", file);
    formData.append("type", type);
    formData.append("year", year);
    if (user?.role === "ADMIN" && selectedSchoolId) {
      formData.append("school_id", String(selectedSchoolId));
    }

    const response = await apiFetch("/uploads", {
      method: "POST",
      body: formData,
    });

    if (!response.ok) {
      setMessage("Upload gagal. Periksa file dan kolom wajib.");
      setIsSubmitting(false);
      return;
    }

    setMessage("Upload berhasil diproses.");
    setIsSubmitting(false);
    setFile(null);
    fetchUploads()
      .then((data) => setUploads(data))
      .catch(() => setUploads([]));
  };

  useEffect(() => {
    if (status === "unauthenticated") {
      router.replace("/login");
    }
  }, [status, router]);

  if (status === "loading") {
    return <p className="text-sm text-zinc-500">Memuat...</p>;
  }

  if (status === "unauthenticated") {
    return null;
  }

  return (
    <section className="space-y-6">
      <header className="space-y-1">
        <p className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
          Upload Excel
        </p>
        <h1 className="text-2xl font-semibold">Riwayat Upload</h1>
      </header>

      <Card>
        <CardContent>
          <form className="mb-6 space-y-4" onSubmit={handleSubmit}>
            <div className="grid gap-4 md:grid-cols-3">
              {user?.role === "ADMIN" && schools.length > 0 ? (
                <div className="space-y-1">
                  <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                    Sekolah
                  </label>
                  <Select
                    value={selectedSchoolId ? String(selectedSchoolId) : ""}
                    onValueChange={(value) =>
                      setSelectedSchoolId(Number(value))
                    }
                  >
                    <SelectTrigger className="w-full">
                      <SelectValue placeholder="Pilih sekolah" />
                    </SelectTrigger>
                    <SelectContent>
                      {schools.map((school) => (
                        <SelectItem key={school.id} value={String(school.id)}>
                          {school.namaSekolah}
                        </SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              ) : null}
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tipe
                </label>
                <Select value={type} onValueChange={setType}>
                  <SelectTrigger className="w-full">
                    <SelectValue placeholder="Pilih tipe" />
                  </SelectTrigger>
                  <SelectContent>
                    <SelectItem value="ACHIEVEMENT">Prestasi</SelectItem>
                    <SelectItem value="ACCEPTANCE">
                      Siswa Perguruan Tinggi
                    </SelectItem>
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1">
                <label className="text-xs font-semibold tracking-[0.2em] text-zinc-500 uppercase">
                  Tahun
                </label>
                <input
                  value={year}
                  onChange={(event) => setYear(event.target.value)}
                  className="w-full rounded-md border border-zinc-200 bg-white px-3 py-2 text-sm"
                />
              </div>
            </div>
            <div className="flex flex-col gap-3 md:flex-row md:items-center md:justify-between">
              <input
                type="file"
                accept=".xlsx"
                onChange={(event) =>
                  setFile(event.target.files?.[0] ?? null)
                }
              />
              <button
                type="submit"
                className="rounded-lg bg-gradient-to-r from-[#0F3D8F] to-[#1E63D5] shadow-sm px-4 py-2 text-sm font-semibold text-white transition hover:opacity-95 disabled:cursor-not-allowed disabled:opacity-70"
                disabled={isSubmitting}
              >
                {isSubmitting ? "Mengunggah..." : "Upload"}
              </button>
            </div>
            {message ? (
              <p className="text-sm text-zinc-600">{message}</p>
            ) : null}
          </form>
          {isLoading ? (
            <p className="text-sm text-zinc-500">Memuat data...</p>
          ) : error ? (
            <p className="text-sm text-red-600">{error}</p>
          ) : uploads.length === 0 ? (
            <p className="text-sm text-zinc-500">Belum ada upload.</p>
          ) : (
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>Tanggal</TableHead>
                  <TableHead>Tipe</TableHead>
                  <TableHead>Tahun</TableHead>
                  <TableHead>Status</TableHead>
                  <TableHead>Error</TableHead>
                  <TableHead>File</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {uploads.map((upload) => (
                  <TableRow key={upload.id}>
                    <TableCell>{formatDate(upload.createdAt)}</TableCell>
                    <TableCell>{upload.type}</TableCell>
                    <TableCell>{upload.year}</TableCell>
                    <TableCell>{upload.status}</TableCell>
                    <TableCell className="max-w-xs truncate">
                      {upload.errorLog ?? "-"}
                    </TableCell>
                    <TableCell className="max-w-xs truncate">
                      {upload.originalFilename}
                    </TableCell>
                  </TableRow>
                ))}
              </TableBody>
            </Table>
          )}
        </CardContent>
      </Card>
    </section>
  );
}

