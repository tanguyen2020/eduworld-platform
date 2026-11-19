// Format: intermediate JSON of the game contexts (design/build/godot/tokens.json, gitignored).
// Input of the Godot writers below and handy for debugging / F060 tooling.
import { byPath } from '../../lib/names.mjs';

export function formatGodotJson(contexts, version) {
  const out = { version, contexts: {} };
  for (const [name, map] of Object.entries(contexts)) {
    const tokens = {};
    for (const p of [...map.keys()].sort(byPath)) tokens[p] = { type: map.get(p).type, value: map.get(p).value };
    out.contexts[name] = tokens;
  }
  return JSON.stringify(out, null, 2) + '\n';
}
