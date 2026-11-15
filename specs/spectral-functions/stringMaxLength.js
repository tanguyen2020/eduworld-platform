// Spectral custom function — ADR-0041: schema kiểu string phải có maxLength.
// Ngoại lệ giống scripts/specs/check-maxlength.mjs: enum, const, format uuid|date|date-time|time|duration|binary|byte.
const EXEMPT_FORMATS = ['uuid', 'date', 'date-time', 'time', 'duration', 'binary', 'byte'];

export default function stringMaxLength(targetVal) {
  if (!targetVal || typeof targetVal !== 'object' || Array.isArray(targetVal)) return [];
  const t = targetVal.type;
  const isString = t === 'string' || (Array.isArray(t) && t.includes('string'));
  if (!isString) return [];
  if ('enum' in targetVal || 'const' in targetVal) return [];
  if (EXEMPT_FORMATS.includes(targetVal.format)) return [];
  if (typeof targetVal.maxLength === 'number') return [];
  return [{ message: 'Schema kiểu string thiếu maxLength (ADR-0041).' }];
}
