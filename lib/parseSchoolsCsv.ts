export type School = {
  namaSatuanPendidikan: string;
  bentukSekolah: string;
  statusSekolah: string;
  kabupatenKota: string;
  tmtAkreditasi: string;
  akreditasi: string;
  guru: number;
  tendik: number;
};

const headerToKey: Record<string, keyof School> = {
  "Nama Satuan Pendidikan": "namaSatuanPendidikan",
  "Bentuk Sekolah": "bentukSekolah",
  "Status Sekolah": "statusSekolah",
  "Kabupaten/Kota": "kabupatenKota",
  "TMT Akreditasi": "tmtAkreditasi",
  Akreditasi: "akreditasi",
  Guru: "guru",
  Tendik: "tendik",
};

const numericKeys = new Set<keyof School>(["guru", "tendik"]);
const isNumericKey = (key: keyof School): key is "guru" | "tendik" =>
  numericKeys.has(key);

const parseNumber = (value: string) => {
  const normalized = value.trim();
  if (!normalized) {
    return 0;
  }

  const parsed = Number(normalized);
  return Number.isFinite(parsed) ? parsed : 0;
};

export const parseSchoolsCsv = (contents: string): School[] => {
  const lines = contents
    .split(/\r?\n/)
    .map((line) => line.trim())
    .filter((line) => line.length > 0);

  if (lines.length === 0) {
    return [];
  }

  const headerCells = lines[0].split(";").map((cell) => cell.trim());
  const headerKeys = headerCells.map((cell) => headerToKey[cell] ?? null);

  return lines.slice(1).map((line) => {
    const cells = line.split(";").map((cell) => cell.trim());

    const record: School = {
      namaSatuanPendidikan: "",
      bentukSekolah: "",
      statusSekolah: "",
      kabupatenKota: "",
      tmtAkreditasi: "",
      akreditasi: "",
      guru: 0,
      tendik: 0,
    };

    headerKeys.forEach((key, index) => {
      if (!key) {
        return;
      }

      const value = cells[index] ?? "";
      if (isNumericKey(key)) {
        record[key] = parseNumber(value);
      } else {
        record[key] = value;
      }
    });

    return record;
  });
};
