// Minimal types for js-yaml (tests and scripts only).
declare module 'js-yaml' {
  export function load(text: string): unknown;
  const yaml: { load: typeof load };
  export default yaml;
}
