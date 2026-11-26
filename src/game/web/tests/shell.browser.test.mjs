// F133 T026 (part 2, real browsers — F133-AC-16, TC-SIM-005 reference): the exported Web build (make godot-export-web
// → src/game/build/web) in Chromium and WebKit through Playwright:
//   - Chromium: the shell checks pass, the single-threaded engine and pack download, no AudioContext exists before
//     the first click, and after "Chơi" the engine boots (Godot prints its banner) without a page error;
//   - Chromium with WebGL disabled: the notice page, and index.wasm is never requested;
//   - WebKit (Safari engine, not officially supported): never blocked when it has WebGL 2 — banner instead.
//   - Chromium as a phone held upright (390×844, F113-AC-06/07): the rotate prompt covers the loading view; turned
//     sideways the "Chơi" button shows, the engine boots, and turning upright again raises no page error (the game
//     shows its own prompt, OrientationGuard).
// Firefox is not installed with the repository's Playwright browsers; Gecko is covered by the jsdom UA tests.
// Skips (with the reason) when the export or Playwright is missing. Playwright comes from the web workspace
// (src/cms/frontend, pnpm install) or from EW_PLAYWRIGHT_FROM=<path to a package.json that depends on it>.
//   make godot-export-web && node --test src/game/web/tests/
import { test, before, after } from 'node:test';
import assert from 'node:assert/strict';
import { existsSync } from 'node:fs';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { serve } from '../../tools/web/serve_web.mjs';

const BUILD = fileURLToPath(new URL('../../build/web/', import.meta.url));
const FROM = process.env.EW_PLAYWRIGHT_FROM ?? fileURLToPath(new URL('../../../cms/frontend/package.json', import.meta.url));

let pw = null;
let skip = '';
try {
  pw = createRequire(FROM)('@playwright/test');
} catch {
  skip = `Playwright not installed (${FROM}); run pnpm install in src/cms/frontend`;
}
if (!skip && !existsSync(`${BUILD}index.html`)) skip = 'no Web export; run make godot-export-web';

let server;
let base;
before(async () => {
  if (skip) return;
  server = await serve(BUILD, { port: 0 });
  base = `http://localhost:${server.address().port}/`;
});
after(() => server?.close());

const SWIFTSHADER = ['--enable-unsafe-swiftshader', '--use-angle=swiftshader'];
// Counts AudioContext creations so the test can prove audio waits for the first interaction.
const COUNT_AUDIO = () => {
  window.__ewAudioContexts = 0;
  for (const name of ['AudioContext', 'webkitAudioContext']) {
    const Original = window[name];
    if (!Original) continue;
    window[name] = class extends Original {
      constructor(...args) {
        super(...args);
        window.__ewAudioContexts += 1;
      }
    };
  }
};

async function launch(type, args = []) {
  try {
    return await pw[type].launch({ args });
  } catch (e) {
    return { skip: `${type} browser not installed (${String(e.message).split('\n')[0]})` };
  }
}

test('Chromium: engine loads single-threaded, audio waits for the first click, then the game boots', { skip: skip || false, timeout: 240000 }, async (t) => {
  const browser = await launch('chromium', SWIFTSHADER);
  if (browser.skip) return t.skip(browser.skip);
  try {
    const page = await browser.newPage();
    const errors = [];
    const consoleLines = [];
    page.on('pageerror', (e) => errors.push(String(e)));
    page.on('console', (m) => consoleLines.push(m.text()));
    await page.addInitScript(COUNT_AUDIO);
    await page.goto(base);
    await page.waitForSelector('#ew-shell[data-state="ready"]', { timeout: 180000 });
    // Google Chrome / Edge get no banner; the Playwright headless shell reports Chromium brands, so it is shown there.
    const official = await page.evaluate(() => (navigator.userAgentData?.brands ?? []).some((b) => ['Google Chrome', 'Microsoft Edge'].includes(b.brand)));
    assert.equal(await page.isHidden('#ew-banner'), official, 'banner only for browsers other than Chrome/Edge');
    assert.equal(await page.evaluate(() => window.EduWorldShell.threads), false);
    assert.equal(await page.evaluate(() => typeof SharedArrayBuffer === 'undefined' || !crossOriginIsolated), true,
      'runs without cross-origin isolation (no COOP/COEP)');
    assert.equal(await page.evaluate(() => window.__ewAudioContexts), 0, 'no AudioContext before a user gesture');
    await page.click('#ew-play');
    await page.waitForSelector('#ew-shell', { state: 'hidden', timeout: 60000 });
    await page.waitForFunction(() => document.getElementById('canvas').width > 0, null, { timeout: 30000 });
    await page.waitForTimeout(3000);
    assert.ok(consoleLines.some((l) => /Godot Engine v4\.7/.test(l)), `engine banner missing in console:\n${consoleLines.slice(0, 10).join('\n')}`);
    assert.deepEqual(errors, []);
  } finally {
    await browser.close();
  }
});

test('Chromium without WebGL: friendly notice, the engine is never downloaded', { skip: skip || false, timeout: 120000 }, async (t) => {
  const browser = await launch('chromium', ['--disable-webgl', '--disable-webgl2', '--disable-3d-apis']);
  if (browser.skip) return t.skip(browser.skip);
  try {
    const page = await browser.newPage();
    const requested = [];
    page.on('request', (r) => requested.push(new URL(r.url()).pathname));
    await page.goto(base);
    await page.waitForSelector('#ew-shell[data-state="unsupported"]', { timeout: 30000 });
    assert.equal(await page.getAttribute('#ew-shell', 'data-reason'), 'webgl2');
    assert.match(await page.textContent('#ew-message'), /WebGL 2/);
    await page.waitForTimeout(1000);
    assert.ok(!requested.some((p) => p.endsWith('.wasm') || p.endsWith('.pck') || p.endsWith('index.js')), requested.join(', '));
  } finally {
    await browser.close();
  }
});

test('WebKit (not officially supported): banner when WebGL 2 works, notice otherwise — never an error', { skip: skip || false, timeout: 240000 }, async (t) => {
  const browser = await launch('webkit');
  if (browser.skip) return t.skip(browser.skip);
  try {
    const page = await browser.newPage();
    await page.goto(base);
    await page.waitForFunction(() => document.getElementById('ew-shell').dataset.state !== 'checking', null, { timeout: 30000 });
    const state = await page.getAttribute('#ew-shell', 'data-state');
    if (state === 'unsupported') {
      assert.equal(await page.getAttribute('#ew-shell', 'data-reason'), 'webgl2');
      t.diagnostic('WebKit build has no WebGL 2 here: notice page shown');
    } else {
      assert.equal(await page.isVisible('#ew-banner'), true, 'unofficial browser banner');
      assert.ok(['loading', 'ready'].includes(state), state);
      await page.waitForSelector('#ew-shell[data-state="ready"]', { timeout: 180000 });
    }
  } finally {
    await browser.close();
  }
});

test('F113: Chromium phone upright — rotate prompt over the loading view, sideways to play, upright again without error', { skip: skip || false, timeout: 240000 }, async (t) => {
  const browser = await launch('chromium', SWIFTSHADER);
  if (browser.skip) return t.skip(browser.skip);
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 }, hasTouch: true, isMobile: true });
    const errors = [];
    page.on('pageerror', (e) => errors.push(String(e)));
    await page.goto(base);
    await page.waitForSelector('#ew-shell[data-state="ready"]', { timeout: 180000 });
    assert.equal(await page.getAttribute('#ew-shell', 'data-portrait'), 'true');
    assert.equal(await page.isVisible('#ew-rotate'), true, 'rotate prompt shown in portrait');
    assert.match(await page.textContent('#ew-rotate-title'), /Xoay ngang|sideways/);
    await page.setViewportSize({ width: 844, height: 390 });
    await page.waitForSelector('#ew-rotate', { state: 'hidden', timeout: 5000 });
    assert.equal(await page.isVisible('#ew-play'), true, '"Chơi" reachable once sideways');
    await page.click('#ew-play');
    await page.waitForSelector('#ew-shell', { state: 'hidden', timeout: 60000 });
    await page.waitForTimeout(3000);
    await page.setViewportSize({ width: 390, height: 844 });
    await page.waitForTimeout(2000);
    await page.setViewportSize({ width: 844, height: 390 });
    await page.waitForTimeout(1000);
    assert.deepEqual(errors, []);
  } finally {
    await browser.close();
  }
});
