// go.yml helper: turn dorny/paths-filter `changes` (JSON array) into the module matrix.
// Usage: node scripts/ci/module-matrix.mjs '["learning"]'   → prints modules=["learning"]
import { moduleMatrixFromChanges } from './path-filters.mjs';

const changes = JSON.parse(process.argv[2] || '[]');
process.stdout.write(`modules=${JSON.stringify(moduleMatrixFromChanges(changes))}\n`);
