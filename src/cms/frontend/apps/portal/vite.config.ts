/// <reference types="vitest/config" />
// Vite config of the Admin Portal (F050). Port: LOCAL_DEVELOPMENT §3.2 (web 5173). No environment URL here:
// the app reads /config.json at runtime (AGENTS R3). Game imports fail the build (F050-AC-07).
import { defineConfig, loadEnv, type Plugin } from 'vite';
import react from '@vitejs/plugin-react';
import fs from 'node:fs';
import path from 'node:path';
import { forbidGameImports } from '../../tooling/forbid-game-imports.mjs';
import { DEV_INTERNAL_ENV, DEV_INTERNAL_PREFIX, devInternalProxy } from './scripts/dev-internal-proxy.mjs';

const REPO_ROOT = path.resolve(__dirname, '../../../../..');
const WORKSPACES = ['game-ops', 'content-studio', 'school-lms', 'platform-admin', 'live-ops'];

/**
 * Dev only: serve public/config.json if present, else public/config.example.json (local defaults). Without a proxy
 * target for the internal gateway, requests to its same-origin prefix are dropped like an unreachable gateway, so
 * the portal shows its VPN notice instead of a 404.
 */
function devRuntimeConfig(internalProxy: boolean): Plugin {
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
      if (!internalProxy) {
        server.config.logger.warn(`${DEV_INTERNAL_ENV} is not set: internal workspaces stay behind the VPN notice`);
        server.middlewares.use(DEV_INTERNAL_PREFIX, (req) => req.socket.destroy());
      }
    },
  };
}

export default defineConfig(({ mode }) => {
  // Dev only: same-origin prefix to the internal gateway, which serves no CORS (F004-AC-21); target from
  // EW_PORTAL_DEV_INTERNAL_GATEWAY (shell or apps/portal/.env.local), LOCAL_DEVELOPMENT §6.4.
  const internalProxy = devInternalProxy({ ...loadEnv(mode, __dirname, 'EW_'), ...process.env });
  return {
    plugins: [forbidGameImports() as Plugin, react(), devRuntimeConfig(internalProxy !== null)],
    server: {
      port: 5173,
      strictPort: true,
      fs: { allow: [path.resolve(__dirname, '../..'), path.join(REPO_ROOT, 'design/fonts')] },
      proxy: internalProxy ?? undefined,
    },
    build: {
      manifest: true,
      sourcemap: true,
      rollupOptions: {
        output: {
          // one named chunk per workspace (F050-AC-02)
          // MathJax 4 (+ its fonts loaded by range) in its own chunk shared by the Content Studio workspace and the font
          // ranges, so the workspace stays its own dynamic entry and nothing of it reaches the entry chunk (F053)
          manualChunks: (id) => {
            const p = id.split('\\').join('/');
            return /\/(@mathjax|mhchemparser|speech-rule-engine)\//.test(p) && !p.includes('/svg/dynamic/') ? 'vendor-mathjax' : undefined;
          },
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
  };
});
