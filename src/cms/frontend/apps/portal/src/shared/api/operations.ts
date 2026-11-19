// Lookup of generated operation metadata by method + concrete path (path templates compiled once).
import { operations } from './operation-meta.gen';
import type { Gateway, OperationMeta } from './types';

/** API_DESIGN §10 / Clarification 2026-11-16: same prefixes as scripts/gen-op-meta.mjs. */
export const INTERNAL_PREFIXES = ['/admin/', '/sis/', '/game-admin/', '/game-media'] as const;

export function gatewayForPath(path: string): Gateway {
  return INTERNAL_PREFIXES.some((p) => path.startsWith(p) || `${path}/`.startsWith(p)) ? 'internal' : 'public';
}

interface Compiled {
  meta: OperationMeta;
  re: RegExp;
  params: number;
}

const escape = (s: string) => s.replace(/[.*+?^${}()|[\]\\]/g, '\\$&');

export function compile(list: readonly OperationMeta[]): Compiled[] {
  return list
    .map((meta) => {
      const parts = meta.path.split(/(\{[^}]+\})/);
      const params = parts.filter((p) => p.startsWith('{')).length;
      const re = new RegExp(`^${parts.map((p) => (p.startsWith('{') ? '[^/:]+' : escape(p))).join('')}$`);
      return { meta, re, params };
    })
    .sort((a, b) => a.params - b.params); // literal segments win over parameters
}

let table: Compiled[] | null = null;

export function findOperation(method: string, path: string, list: readonly OperationMeta[] = operations): OperationMeta | undefined {
  const t = list === operations ? (table ??= compile(operations)) : compile(list);
  const m = method.toUpperCase();
  return t.find((c) => c.meta.method === m && c.re.test(path))?.meta;
}

export function findOperationById(operationId: string): OperationMeta | undefined {
  return operations.find((o) => o.operationId === operationId);
}
