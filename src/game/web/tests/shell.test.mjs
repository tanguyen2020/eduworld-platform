// F133 T026 (part 1, jsdom — always runs): logic of the Web shell src/game/web/shell.html for F133-AC-16
// (TC-SIM-005 reference), the Web part of F133-AC-25 (ADR-0086) and AGENTS R32. The real-browser part (Chromium,
// WebKit on the exported build) is shell.browser.test.mjs.
//   node --test src/game/web/tests/      (needs npm ci --prefix scripts for jsdom)
import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { createRequire } from 'node:module';

const require = createRequire(new URL('../../../../scripts/package.json', import.meta.url));
const { JSDOM } = require('jsdom');

const SHELL = readFileSync(new URL('../shell.html', import.meta.url), 'utf8');
const TOKENS = readFileSync(new URL('../shell_tokens.css', import.meta.url), 'utf8');
const CONFIG = { args: [], canvasResizePolicy: 2, executable: 'index', fileSizes: { 'index.pck': 10, 'index.wasm': 20 },
  focusCanvas: true, gdextensionLibs: [] };

const UA = {
  chrome: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36',
  edge: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 Edg/142.0.0.0',
  chromeAndroid: 'Mozilla/5.0 (Linux; Android 13; SM-A145F) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Mobile Safari/537.36',
  firefox: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:145.0) Gecko/20100101 Firefox/145.0',
  safari: 'Mozilla/5.0 (Macintosh; Intel Mac OS X 14_6) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Safari/605.1.15',
  chromeIos: 'Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) CriOS/142.0 Mobile/15E148 Safari/604.1',
  opera: 'Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/142.0.0.0 Safari/537.36 OPR/125.0.0.0',
};

/** The page as the Godot exporter writes it (placeholders substituted). */
function exported() {
  return SHELL.replaceAll('$GODOT_URL', 'index.js').replaceAll('$GODOT_CONFIG', JSON.stringify(CONFIG))
    .replaceAll('$GODOT_THREADS_ENABLED', 'false').replaceAll('$GODOT_PROJECT_NAME', 'EduWorld')
    .replaceAll('$GODOT_HEAD_INCLUDE', '');
}

class FakeMemory {
  constructor(pages) { this.pages = pages; this.calls = []; this.failAbove = Infinity; }
  get buffer() { return { byteLength: this.pages * 65536 }; }
  grow(delta) {
    this.calls.push(delta);
    if (this.pages + delta > this.failAbove) throw new RangeError('cannot reserve');
    const old = this.pages;
    this.pages += delta;
    return old;
  }
}

function page({ ua = UA.chrome, webgl2 = true, secure = true, languages = ['vi-VN'], brands } = {}) {
  const dom = new JSDOM(exported(), {
    runScripts: 'dangerously',
    beforeParse(win) {
      win.EW_SHELL_MANUAL_START = true;
      Object.defineProperty(win.navigator, 'userAgent', { get: () => ua });
      Object.defineProperty(win.navigator, 'languages', { get: () => languages });
      if (brands) Object.defineProperty(win.navigator, 'userAgentData', { get: () => ({ brands }) });
      Object.defineProperty(win, 'isSecureContext', { get: () => secure });
      win.HTMLCanvasElement.prototype.getContext = (type) => (type === 'webgl2' && webgl2 ? {} : null);
      win.WebAssembly = { Memory: class extends FakeMemory {} };
      win.fetch = () => Promise.reject(new Error('no network in tests'));
      win.console.error = () => {};
    },
  });
  const win = dom.window;
  const engine = { created: [], init: [], preload: [], start: [] };
  const loaded = [];
  class FakeEngine {
    constructor(config) { this.config = config; engine.created.push(config); }
    init(exe) { engine.init.push(exe); return Promise.resolve(); }
    preloadFile(file, path) { engine.preload.push([file, path]); return Promise.resolve(); }
    start(override) { engine.start.push(override); return Promise.resolve(); }
  }
  const deps = { loadScript: (src) => { loaded.push(src); win.Engine = FakeEngine; return Promise.resolve(); } };
  const $ = (id) => win.document.getElementById(id);
  return { win, engine, loaded, deps, $ };
}

const tick = () => new Promise((r) => setTimeout(r, 0));

test('F133-AC-16: without WebGL 2 the notice page is shown and the engine is never loaded', async () => {
  const p = page({ webgl2: false });
  const result = await p.win.EduWorldShell.run(p.win, p.deps);
  assert.equal(result, 'unsupported');
  assert.equal(p.$('ew-shell').dataset.state, 'unsupported');
  assert.equal(p.$('ew-shell').dataset.reason, 'webgl2');
  assert.match(p.$('ew-message').textContent, /WebGL 2/);
  assert.match(p.$('ew-message').textContent, /Chrome hoặc Edge/);
  assert.deepEqual(p.loaded, [], 'engine script must not be requested');
  assert.equal(p.$('ew-play').hidden, true);
});

test('F133-AC-16: an insecure context also gets the notice instead of the engine', async () => {
  const p = page({ secure: false });
  assert.equal(await p.win.EduWorldShell.run(p.win, p.deps), 'unsupported');
  assert.equal(p.$('ew-shell').dataset.reason, 'runtime');
  assert.deepEqual(p.loaded, []);
});

test('F133-AC-16 (OQ-04): Firefox/Safari with WebGL 2 are not blocked — banner only, engine loads', async () => {
  for (const ua of [UA.firefox, UA.safari, UA.chromeIos, UA.opera]) {
    const p = page({ ua });
    p.win.EduWorldShell.run(p.win, p.deps);
    await tick();
    await tick();
    assert.equal(p.$('ew-banner').hidden, false, ua);
    assert.match(p.$('ew-banner-text').textContent, /Chrome và Edge/);
    assert.deepEqual(p.loaded, ['index.js'], ua);
    assert.equal(p.$('ew-shell').dataset.state, 'ready', ua);
    p.$('ew-banner-close').click();
    assert.equal(p.$('ew-banner').hidden, true);
  }
});

test('F133-AC-16: Chrome/Edge load the single-threaded engine; it starts only after the first interaction', async () => {
  for (const ua of [UA.chrome, UA.edge, UA.chromeAndroid]) {
    const p = page({ ua });
    const done = p.win.EduWorldShell.run(p.win, p.deps);
    await tick();
    await tick();
    assert.equal(p.$('ew-banner').hidden, true, `${ua}: no banner on a supported browser`);
    assert.equal(p.win.EduWorldShell.threads, false);
    assert.deepEqual(p.engine.init, ['index']);
    assert.deepEqual(p.engine.preload, [['index.pck', 'index.pck']]);
    assert.equal(p.$('ew-shell').dataset.state, 'ready');
    assert.equal(p.$('ew-play').hidden, false);
    // Audio policy: nothing runs (no AudioContext is created) before a user gesture.
    assert.deepEqual(p.engine.start, []);
    p.$('ew-play').click();
    assert.equal(await done, 'started');
    // Objects come from the page realm: compare by value.
    assert.equal(JSON.stringify(p.engine.start), JSON.stringify([{ args: ['--main-pack', 'index.pck'] }]));
    assert.equal(p.$('ew-shell').hidden, true);
  }
});

test('Chromium brands decide when the browser exposes userAgentData', () => {
  const s = page().win.EduWorldShell;
  assert.equal(s.browserSupport({ userAgentData: { brands: [{ brand: 'Google Chrome' }] }, userAgent: UA.firefox }), 'supported');
  assert.equal(s.browserSupport({ userAgentData: { brands: [{ brand: 'Microsoft Edge' }] } }), 'supported');
  assert.equal(s.browserSupport({ userAgentData: { brands: [{ brand: 'Chromium' }, { brand: 'Opera' }] } }), 'unofficial');
});

test('shell strings: Vietnamese by default, English when preferred, same keys in both', () => {
  const s = page().win.EduWorldShell;
  assert.equal(s.pickLocale(['vi-VN', 'en']), 'vi');
  assert.equal(s.pickLocale(['en-US']), 'en');
  assert.equal(s.pickLocale(['fr-FR']), 'vi');
  assert.deepEqual(Object.keys(s.STRINGS.en).sort(), Object.keys(s.STRINGS.vi).sort());
});

test('PERFORMANCE_BUDGET §14: the heap reserves 512 MiB at the first growth and never exceeds 1 GiB', () => {
  const s = page().win.EduWorldShell;
  assert.deepEqual({ ...s.HEAP_POLICY }, { initialMiB: 512, maxMiB: 1024 });
  const wasm = { Memory: class extends FakeMemory {} };
  const policy = s.installHeapPolicy(wasm, s.HEAP_POLICY);
  assert.deepEqual({ ...policy }, { maxPages: 16384, initialPages: 8192 });
  assert.equal(s.installHeapPolicy(wasm, s.HEAP_POLICY), policy, 'installing twice keeps one wrapper');
  const mem = new wasm.Memory(256); // 16 MiB
  mem.grow(16);
  assert.equal(mem.pages, 8192, 'first growth reserves the initial 512 MiB');
  mem.grow(4096);
  assert.equal(mem.pages, 12288);
  assert.throws(() => mem.grow(8192), /heap budget exceeded \(1024 MiB\)/, 'growth past 1 GiB is refused');
  assert.equal(mem.pages, 12288);
  // A device that cannot reserve 512 MiB still grows by the requested amount.
  const small = new wasm.Memory(256);
  small.failAbove = 1024;
  small.grow(16);
  assert.equal(small.pages, 272);
});

test('ADR-0086 (F133-AC-25, Web): the shell never touches browser storage', () => {
  assert.doesNotMatch(SHELL, /localStorage\.|sessionStorage\.|indexedDB\.|document\.cookie|openDatabase/);
});

test('AGENTS R32: shell styles use only game tokens; every variable used is generated', () => {
  const css = /<style>([\s\S]*?)<\/style>/.exec(SHELL)[1];
  assert.doesNotMatch(css, /#[0-9a-fA-F]{3,8}\b|\brgba?\(|\bhsla?\(|\d+(\.\d+)?(px|em|rem)\b/, 'no literal color or size');
  const used = new Set([...css.matchAll(/var\((--ew-[a-z0-9-]+)\)/g)].map((m) => m[1]));
  assert.ok(used.size > 10);
  for (const name of used) assert.match(TOKENS, new RegExp(`^\\s*${name}:`, 'm'), `${name} missing in shell_tokens.css`);
  assert.match(TOKENS, /GENERATED by design\/.*game-tokens/);
  assert.doesNotMatch(TOKENS, /--ew-antd-/, 'no admin token in the game shell');
});

test('F096-AC-02: the loading view is EduWorld — logo, Mây, rounded bar, friendly line — and never a Godot logo', async () => {
  const p = page();
  p.win.EduWorldShell.run(p.win, p.deps);
  assert.equal(p.$('ew-title').textContent, 'EduWorld');
  assert.ok(p.win.document.querySelector('.ew-logo svg .ew-spark-fill'), 'Knowledge Spark mark');
  assert.ok(p.win.document.querySelectorAll('.ew-mascot .ew-puff circle').length >= 6, 'Mây');
  assert.equal(p.$('ew-progress').getAttribute('role'), 'progressbar');
  await tick();
  assert.equal(p.$('ew-shell').dataset.state, 'ready');
  const markup = SHELL.replace(/<!--[\s\S]*?-->/g, '');
  assert.doesNotMatch(markup, /godot[-_ ]?logo|GODOT_SPLASH|<img/i, 'no Godot logo, splash image or bitmap in the shell');
  const vi = page({ languages: ['vi-VN'] }).win.EduWorldShell.STRINGS;
  assert.equal(vi.vi.loading, 'Đang chuẩn bị thế giới cho em…');
  assert.equal(vi.en.loading, 'Getting your world ready…');
});

test('F096-AC-02: the bar shows the downloaded share, and moves gently while the size is unknown', () => {
  const p = page();
  const bar = p.$('ew-progress');
  p.win.EduWorldShell.setProgress(bar, 30, 120);
  assert.equal(bar.dataset.indeterminate, 'false');
  assert.equal(bar.getAttribute('aria-valuenow'), '25');
  assert.equal(p.$('ew-progress-fill').style.width, '25%');
  p.win.EduWorldShell.setProgress(bar, 0, 0);
  assert.equal(bar.dataset.indeterminate, 'true');
  assert.equal(bar.getAttribute('aria-valuenow'), null);
  assert.equal(p.$('ew-progress-fill').style.width, '');
});

test('F096-AC-02: the notice page (no WebGL 2) keeps the logo and Mây', async () => {
  const p = page({ webgl2: false });
  await p.win.EduWorldShell.run(p.win, p.deps);
  assert.equal(p.$('ew-shell').hidden, false);
  assert.equal(p.$('ew-title').textContent, 'EduWorld');
  assert.ok(p.win.document.querySelector('#ew-shell .ew-mascot'));
});
