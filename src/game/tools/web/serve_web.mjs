// Local preview server of the Web export (F133, LOCAL_DEVELOPMENT §3.5): serves a folder (default
// src/game/build/web) on http://localhost:8060, the origin allowed by the local gateway CORS of the game routes
// (deploy/apisix/routes/env/local.env EW_CORS_GAME_ORIGINS) and by realtime (EW_CORS_ALLOWED_ORIGINS).
// Single-threaded export: no COOP/COEP header is needed (PERFORMANCE_BUDGET §14). Preview only — never a CDN.
//   node src/game/tools/web/serve_web.mjs [dir] [--port=8060]
import { createServer } from 'node:http';
import { createReadStream, statSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const DEFAULT_PORT = 8060;

const TYPES = {
  '.html': 'text/html; charset=utf-8',
  '.js': 'text/javascript; charset=utf-8',
  '.mjs': 'text/javascript; charset=utf-8',
  '.css': 'text/css; charset=utf-8',
  '.json': 'application/json',
  '.wasm': 'application/wasm',
  '.pck': 'application/octet-stream',
  '.png': 'image/png',
  '.svg': 'image/svg+xml',
  '.ico': 'image/x-icon',
};

/** Starts the server on 127.0.0.1; resolves with the http.Server once it listens (port 0 = any free port). */
export function serve(dir, { port = DEFAULT_PORT, host = '127.0.0.1' } = {}) {
  const root = path.resolve(dir);
  const server = createServer((req, res) => {
    if (req.method !== 'GET' && req.method !== 'HEAD') {
      res.writeHead(405, { Allow: 'GET, HEAD' }).end();
      return;
    }
    let rel;
    try {
      rel = decodeURIComponent(new URL(req.url, 'http://localhost').pathname);
    } catch {
      res.writeHead(400).end();
      return;
    }
    if (rel.endsWith('/')) rel += 'index.html';
    const file = path.resolve(root, `.${rel}`);
    if (file !== root && !file.startsWith(root + path.sep)) {
      res.writeHead(403).end();
      return;
    }
    let st;
    try {
      st = statSync(file);
    } catch {
      res.writeHead(404).end();
      return;
    }
    if (!st.isFile()) {
      res.writeHead(404).end();
      return;
    }
    res.writeHead(200, {
      'Content-Type': TYPES[path.extname(file).toLowerCase()] ?? 'application/octet-stream',
      'Content-Length': st.size,
      'Cache-Control': 'no-store',
    });
    if (req.method === 'HEAD') {
      res.end();
      return;
    }
    createReadStream(file).pipe(res);
  });
  return new Promise((resolve, reject) => {
    server.once('error', reject);
    server.listen(port, host, () => resolve(server));
  });
}

if (process.argv[1] && pathToFileURL(process.argv[1]).href === import.meta.url) {
  const here = path.dirname(fileURLToPath(import.meta.url));
  const args = process.argv.slice(2);
  const dir = args.find((a) => !a.startsWith('--')) ?? path.join(here, '..', '..', 'build', 'web');
  const port = Number(args.find((a) => a.startsWith('--port='))?.slice(7) ?? DEFAULT_PORT);
  const server = await serve(dir, { port });
  console.log(`serve_web: ${path.resolve(dir)} on http://localhost:${server.address().port}/ (Ctrl+C to stop)`);
}
