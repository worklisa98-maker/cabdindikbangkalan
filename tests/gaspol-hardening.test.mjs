import test from "node:test";
import assert from "node:assert/strict";
import { normalizeGaspolReportURL } from "../lib/gaspol-report-url.ts";
test("report URLs accept Drive, Docs, HTTPS and omitted scheme", () => {
  for (const value of [
    "drive.google.com/file/d/1/view",
    "https://docs.google.com/document/d/1/edit",
    "https://example.com/a",
    "http://example.org/proof",
  ])
    assert.ok(normalizeGaspolReportURL(value), value);
  assert.equal(
    normalizeGaspolReportURL("drive.google.com/file/d/1/view"),
    "https://drive.google.com/file/d/1/view"
  );
});
test("optional empty evidence and multiple independent URLs", () => {
  assert.equal(normalizeGaspolReportURL("  "), "");
  const rows =
    "docs.google.com/document/d/1\n\nhttps://drive.google.com/file/d/2"
      .split(/\r?\n/)
      .map((v) => v.trim())
      .filter(Boolean)
      .map(normalizeGaspolReportURL);
  assert.equal(rows.length, 2);
  assert.ok(rows.every(Boolean));
});
test("unsafe protocols, credentials and whitespace remain rejected", () => {
  for (const value of [
    "javascript:alert(1)",
    "data:text/html,hello",
    "ftp://example.com/a",
    "https://u:p@example.com/a",
    "https://example.com/a b",
  ])
    assert.equal(normalizeGaspolReportURL(value), "", value);
});
