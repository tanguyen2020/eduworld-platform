// Types for forbid-game-imports.mjs (used from vite/vitest TS configs).
export function isGameImport(source: string, importer?: string): boolean;
export function forbidGameImports(): {
  name: string;
  enforce: 'pre';
  resolveId(this: { error(message: string): never }, source: string, importer?: string): null;
};
