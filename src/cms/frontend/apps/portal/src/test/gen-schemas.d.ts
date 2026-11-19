// Types of the schema copy script used by tests (scripts/gen-schemas.mjs, F053 T010).
declare module '*/gen-schemas.mjs' {
  export function expected(): { name: string; content: string }[];
  export const SCHEMAS: Record<string, string>;
}
