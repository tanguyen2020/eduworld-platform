// Browser capability check of the "Mở bản Web" card (F134 T035, ADR-0101 mục 8): the Web client (Godot Compatibility
// renderer) needs WebGL 2. The check only asks the browser for a `webgl2` context on a detached canvas; it decides
// nothing about the user or the data. Any exception (blocked GPU, privacy mode) counts as "not supported".

export type WebGlSupport = 'supported' | 'unsupported';

export function detectWebGl2(doc: Document = document): WebGlSupport {
  try {
    const canvas = doc.createElement('canvas');
    const ctx = canvas.getContext('webgl2');
    return ctx ? 'supported' : 'unsupported';
  } catch {
    return 'unsupported';
  }
}
