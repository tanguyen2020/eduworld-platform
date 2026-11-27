// Path-filter expectations for pull requests (F001 T031, F001-AC-12, F001-AC-24, DEPLOYMENT §10.2).
// Evaluates `on.pull_request.paths` of every workflow and .github/module-filters.yml like GitHub/paths-filter.
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { workflowsTriggered, moduleMatrix, globToRegExp } from './path-filters.mjs';

const ALL = ['chat', 'competition', 'game', 'identity', 'learning'];

const cases = [
  { files: ['src/backend/modules/learning/internal/domain/doc.go'], workflows: ['go', 'security'], modules: ['learning'] },
  { files: ['src/backend/modules/learning/README.md'], workflows: ['go', 'security'], modules: ['learning'] },
  { files: ['src/backend/modules/chat/migrations/20270101000000_x.sql'], workflows: ['go', 'security'], modules: ['chat'] },
  { files: ['src/backend/pkg/health/registry.go'], workflows: ['go', 'security'], modules: ALL },
  { files: ['src/backend/cmd/core-api/main.go'], workflows: ['go', 'security'], modules: ALL },
  { files: ['src/backend/go.mod'], workflows: ['go', 'security'], modules: ALL },
  { files: ['deploy/postgres/init/02-roles.sh'], workflows: ['deploy-config', 'go', 'security'], modules: ALL },
  { files: ['deploy/compose/docker-compose.yml'], workflows: ['deploy-config', 'security'], modules: [] },
  // F028: the media-tools stage is the Pandoc of the learning Word import tests (go.yml step "Pandoc 3.6.4").
  { files: ['deploy/docker/worker.Dockerfile'], workflows: ['deploy-config', 'go', 'security'], modules: ['learning'] },
  { files: ['docs/README.md'], workflows: ['docs', 'security'], modules: [] },
  { files: ['README.md'], workflows: ['docs', 'security'], modules: [] },
  { files: ['specs/001-monorepo-compose-ci-skeleton/spec.md'], workflows: ['docs', 'godot', 'security', 'specs'], modules: [] },
  { files: ['specs/openapi/modules/identity.internal.v1.yaml'], workflows: ['go', 'godot', 'security', 'specs', 'web'], modules: ['identity'] },
  { files: ['specs/events/payloads/identity/tenant.created.v1.schema.json'], workflows: ['go', 'godot', 'security', 'specs'], modules: ALL },
  { files: ['.github/workflows/go.yml'], workflows: ['go', 'security', 'specs'], modules: ALL },
  { files: ['src/cms/frontend/apps/portal/src/main.tsx'], workflows: ['security', 'web'], modules: [] },
  { files: ['src/game/project.godot'], workflows: ['godot', 'security'], modules: [] },
  { files: ['design/shared-tokens/brand.json'], workflows: ['design', 'godot', 'security', 'web'], modules: [] },
  { files: ['design/admin-tokens/seed.json'], workflows: ['design', 'security', 'web'], modules: [] },
  { files: ['design/game-tokens/semantic.json'], workflows: ['design', 'godot', 'security'], modules: [] },
  // F133 T006: release scripts, the school APK kit and the CDN rules of the Web client start godot.yml.
  { files: ['scripts/godot/export.sh'], workflows: ['godot', 'security'], modules: [] },
  { files: ['deploy/installer/school-kit/build-kit.sh'], workflows: ['deploy-config', 'godot', 'security'], modules: [] },
  { files: ['deploy/cdn/play/headers.yaml'], workflows: ['deploy-config', 'godot', 'security'], modules: [] },
  { files: ['src/backend/modules/learning/x.go', 'src/backend/modules/game/y.go'], workflows: ['go', 'security'], modules: ['game', 'learning'] },
  // F132 T015: the load tooling triggers the perf lane (unit tests + k6 smoke); a perf report alone only docs.
  { files: ['src/backend/tools/loadbot/cmd/loadbot/main.go'], workflows: ['go', 'perf', 'security'], modules: ALL },
  { files: ['tests/load/k6/ccu_10k.js'], workflows: ['perf', 'security'], modules: [] },
  { files: ['docs/12-project/perf-reports/README.md'], workflows: ['docs', 'security'], modules: [] },
];

for (const c of cases) {
  test(`${c.files.join(', ')} → ${c.workflows.join('+')} [${c.modules.join(',')}]`, () => {
    assert.deepEqual(workflowsTriggered(c.files), c.workflows);
    assert.deepEqual(moduleMatrix(c.files), c.modules);
  });
}

test('no workflow but security runs on a docs-only PR (F001-AC-24)', () => {
  assert.deepEqual(workflowsTriggered(['docs/12-project/ROADMAP.md']), ['docs', 'security']);
});

test('glob semantics follow GitHub', () => {
  assert.ok(globToRegExp('src/backend/modules/**').test('src/backend/modules/a/b/c.go'));
  assert.ok(globToRegExp('*.md').test('README.md'));
  assert.ok(!globToRegExp('*.md').test('docs/README.md'));
  assert.ok(globToRegExp('specs/**/*.md').test('specs/001-x/spec.md'));
  assert.ok(globToRegExp('specs/openapi/modules/identity.*').test('specs/openapi/modules/identity.internal.v1.yaml'));
});
