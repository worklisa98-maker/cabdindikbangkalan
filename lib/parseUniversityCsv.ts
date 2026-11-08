export type UniversityAdmissionRecord = Record<string, string>;

const toCamelCase = (value: string) => {
  const parts = value
    .trim()
    .split(/[^a-zA-Z0-9]+/)
    .filter(Boolean);

  if (parts.length === 0) {
    return "";
  }

  const [first, ...rest] = parts;
  return (
    first.toLowerCase() +
    rest
      .map((part) => part.charAt(0).toUpperCase() + part.slice(1).toLowerCase())
      .join("")
  );
};

const parseSemicolonCsv = (contents: string) => {
  const rows: string[][] = [];
  let row: string[] = [];
  let cell = "";
  let inQuotes = false;

  for (let index = 0; index < contents.length; index += 1) {
    const char = contents[index];

    if (inQuotes) {
      if (char === '"') {
        if (contents[index + 1] === '"') {
          cell += '"';
          index += 1;
        } else {
          inQuotes = false;
        }
      } else {
        cell += char;
      }
      continue;
    }

    if (char === '"') {
      inQuotes = true;
      continue;
    }

    if (char === ";") {
      row.push(cell);
      cell = "";
      continue;
    }

    if (char === "\n" || char === "\r") {
      row.push(cell);
      cell = "";
      rows.push(row);
      row = [];

      if (char === "\r" && contents[index + 1] === "\n") {
        index += 1;
      }
      continue;
    }

    cell += char;
  }

  if (cell.length > 0 || row.length > 0) {
    row.push(cell);
    rows.push(row);
  }

  return rows;
};

const normalizeHeaders = (headers: string[]) => {
  const normalized = headers.map((cell) => cell.trim());
  while (normalized.length > 0 && normalized[normalized.length - 1] === "") {
    normalized.pop();
  }
  return normalized;
};

export const parseUniversityCsv = (
  contents: string
): UniversityAdmissionRecord[] => {
  const rows = parseSemicolonCsv(contents);
  if (rows.length === 0) {
    return [];
  }

  const headerCells = normalizeHeaders(rows[0]);
  const headerKeys = headerCells.map((cell) => (cell ? toCamelCase(cell) : ""));

  const records: UniversityAdmissionRecord[] = [];

  rows.slice(1).forEach((row) => {
    let cells = row.map((cell) => cell.trim());
    if (cells.every((cell) => cell === "")) {
      return;
    }

    if (cells.length > headerCells.length) {
      cells = cells.slice(0, headerCells.length);
    }

    if (cells.length < headerCells.length) {
      cells = cells.concat(
        new Array(headerCells.length - cells.length).fill("")
      );
    }

    const record: UniversityAdmissionRecord = {};
    headerKeys.forEach((key, index) => {
      if (!key) {
        return;
      }
      record[key] = cells[index] ?? "";
    });

    records.push(record);
  });

  return records;
};
