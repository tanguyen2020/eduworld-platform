/// <reference types="vitest/config" />
// Vite config of the Admin Portal (F050). Port: LOCAL_DEVELOPMENT §3.2 (web 5173). No environment URL here:
// the app reads /config.json at runtime (AGENTS R3). Game imports fail the build (F050-AC-07).
import { defineConfig, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import fs from 'node:fs';
import path from 'node:path';
import { forbidGameImports } from '../../tooling/forbid-game-imports.mjs';

const REPO_ROOT = path.resolve(__dirname, '../../../../..');
const WORKSPACES = ['game-ops', 'content-studio', 'school-lms', 'platform-admin', 'live-ops'];

/** Dev only: serve public/config.json if present, else public/config.example.json (local defaults). */
function devRuntimeConfig(): Plugin {
  return {
    name: 'eduworld:dev-runtime-config',
    apply: 'serve',
    configureServer(server) {
      server.middlewares.use('/config.json', (_req, res) => {
        const own = path.join(__dirname, 'public/config.json');
        const file = fs.existsSync(own) ? own : path.join(__dirname, 'public/config.example.json');
        res.setHeader('Content-Type', 'application/json');
        res.setHeader('Cache-Control', 'no-store');
        res.end(fs.readFileSync(file));
      });
    },
  };
}

export default defineConfig({
  plugins: [forbidGameImports() as Plugin, react(), devRuntimeConfig()],
  server: {
    port: 5173,
    strictPort: true,
    fs: { allow: [path.resolve(__dirname, '../..'), path.join(REPO_ROOT, 'design/fonts')] },
  },
  build: {
    manifest: true,
    sourcemap: true,
    rollupOptions: {
      output: {
        // one named chunk per workspace (F050-AC-02)
        chunkFileNames: (chunk) => {
          const ws = WORKSPACES.find((w) => chunk.facadeModuleId?.includes(`/workspaces/${w}/`));
          return ws ? `assets/workspace-${ws}-[hash].js` : 'assets/[name]-[hash].js';
        },
      },
    },
  },
  test: {
    name: 'portal',
    // whole-portal tests (router + Ant Design) on a shared, throttled CPU (2 workers)
    testTimeout: 120_000,
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    include: ['src/**/*.test.{ts,tsx}', 'scripts/**/*.test.mjs'],
    css: false,
  },
});
