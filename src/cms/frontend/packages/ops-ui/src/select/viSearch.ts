// Client-side search of dropdown options (owner rule 2026-11-22, ADMIN_PORTAL §1.6): case- and Vietnamese
// diacritic-insensitive ("truong" finds "Trường", "DA NANG" finds "Đà Nẵng"). Used as the default `filterOption` of
// SearchSelect; dropdown data is the whole list (portal `fetchAllPages`), so filtering happens in the browser.
import { isValidElement, type ReactNode } from 'react';

/** Folds a string for search: NFD, strips combining marks, đ/Đ → d, lower case, collapses spaces. */
export function foldVi(s: string): string {
  return s
    .normalize('NFD')
    .replace(/[̀-ͯ]/g, '')
    .replace(/[đĐ]/g, 'd')
    .toLowerCase()
    .replace(/\s+/g, ' ')
    .trim();
}

/** Plain text of a React node (option labels may be elements). */
export function nodeText(node: ReactNode): string {
  if (node === null || node === undefined || typeof node === 'boolean') return '';
  if (typeof node === 'string' || typeof node === 'number') return String(node);
  if (Array.isArray(node)) return node.map(nodeText).join(' ');
  if (isValidElement<{ children?: ReactNode }>(node)) return nodeText(node.props.children);
  return '';
}

type SearchableOption = { label?: ReactNode; title?: unknown; searchText?: unknown; children?: ReactNode; value?: unknown };

/** Searchable text of an option: `searchText`, then label, title, children, value. */
export function optionText(option: SearchableOption | undefined): string {
  if (!option) return '';
  if (typeof option.searchText === 'string') return option.searchText;
  const parts = [nodeText(option.label), typeof option.title === 'string' ? option.title : '', nodeText(option.children)];
  const text = parts.filter(Boolean).join(' ');
  return text || (option.value === undefined || option.value === null ? '' : String(option.value));
}

/** `filterOption` of antd Select: every word of the input must appear in the folded option text. */
export function viFilterOption<O extends object>(input: string, option?: O): boolean {
  const words = foldVi(input).split(' ').filter(Boolean);
  if (!words.length) return true;
  const text = foldVi(optionText(option as SearchableOption | undefined));
  return words.every((w) => text.includes(w));
}
