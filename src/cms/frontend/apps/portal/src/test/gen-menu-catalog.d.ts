// Types of the catalog generator script used by tests (scripts/gen-menu-catalog.mjs, F051 T054).
declare module '*/gen-menu-catalog.mjs' {
  export function generate(): string;
  export function buildCatalogSource(catalog: unknown): string;
}
