// Display formatting of "Tải bộ cài" (F134 T035): human-readable file size in the UI locale (binary units, one decimal).
// Presentation only — the size itself is `size_bytes` of the signed index returned by the server.

const UNITS = [
  ['gigabyte', 1024 ** 3],
  ['megabyte', 1024 ** 2],
  ['kilobyte', 1024],
] as const;

export function formatSize(bytes: number, locale: string): string {
  for (const [unit, size] of UNITS) {
    if (bytes >= size) return new Intl.NumberFormat(locale, { style: 'unit', unit, maximumFractionDigits: 1 }).format(bytes / size);
  }
  return new Intl.NumberFormat(locale, { style: 'unit', unit: 'byte' }).format(bytes);
}
