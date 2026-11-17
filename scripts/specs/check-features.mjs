#!/usr/bin/env node
// Kiểm tra feature spec theo ADR-0082 (chuẩn GitHub Spec Kit + EARS).
// Chạy: node scripts/specs/check-features.mjs [--verbose]
// Feature = thư mục specs/NNN-slug/ gồm spec.md, plan.md, data-model.md, contracts/README.md,
// quickstart.md, checklists/requirements.md, tasks.md (research.md tùy chọn).
// Kiểm: tiêu đề + mục bắt buộc theo .specify/templates; mã FR/US/NFR có trong PRD; mã TC có trong
// TEST_STRATEGY; ADR tồn tại; tiêu chí F<NNN>-AC-* theo EARS ("PHẢI"); task đúng định dạng;
// Status=Ready thì không còn [NEEDS CLARIFICATION]; feature có trong danh mục CODE_PLAN;
// không tham chiếu nội dung deferred; mọi FR Release thuộc ít nhất một feature (cảnh báo).
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..', '..');
const verbose = process.argv.includes('--verbose');
const specsDir = path.join(root, 'specs');
const read = (p) => fs.readFileSync(path.join(root, p), 'utf8');

const prd = read('docs/00-overview/PRODUCT_REQUIREMENTS.md');
const tests = read('docs/12-project/TEST_STRATEGY.md');
const catalog = read('docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md');
const adrFiles = fs.readdirSync(path.join(root, 'docs', 'adr'));

const idsIn = (text, re) => new Set([...text.matchAll(re)].map((m) => m[0]));
const prdIds = idsIn(prd, /\b(?:FR|US|NFR|AC)-[A-Z0-9]+-\d{3}\b/g);
const tcIds = idsIn(tests, /\bTC-[A-Z0-9]+(?:-[A-Z0-9]+)*-\d{2,3}\b/g);
for (const m of tests.matchAll(/\b(TC-[A-Z0-9-]+?)-(\d{2,3})\s*(?:…|\.\.\.|–)\s*(\d{2,3})\b/g)) {
  const [, pre, a, b] = m; const w = a.length;
  for (let i = parseInt(a, 10); i <= parseInt(b, 10); i++) tcIds.add(`${pre}-${String(i).padStart(w, '0')}`);
}

const releaseFr = new Set();
for (const line of prd.split('\n')) {
  const m = line.match(/^\|\s*(FR-[A-Z0-9]+-\d{3})\s*\|/);
  if (!m || /Removed|Deferred/i.test(line)) continue;
  // Cùng định nghĩa với scripts/docs/build-rtm.mjs: FR thuộc Release khi cột phase (áp chót) là P1–P6.
  const cells = line.split('|').map((c) => c.trim());
  if (!/^P\d/.test(cells[cells.length - 2] || '')) continue;
  releaseFr.add(m[1]);
}

const FILES = {
  'spec.md': { title: (f) => `# Feature Specification: ${f} — `, heads: ['## User Scenarios & Testing', '## Requirements', '### Functional Requirements', '## Success Criteria', '## Ngoài phạm vi'] },
  'plan.md': { title: (f) => `# Implementation Plan: ${f} — `, heads: ['## Summary', '## Technical Context', '## Constitution Check', '## Project Structure', '## Phase 1: Design & Contracts'] },
  'data-model.md': { title: (f) => `# Data Model: ${f} — `, heads: ['## 1. Entity'] },
  'contracts/README.md': { title: (f) => `# Contracts: ${f} — `, heads: ['## 1. API', '## 6. Delta cần thực hiện'] },
  'quickstart.md': { title: (f) => `# Quickstart: ${f} — `, heads: ['## 2. Kịch bản kiểm tra'] },
  'checklists/requirements.md': { title: (f) => `# Specification Quality Checklist: ${f} — `, heads: ['## Requirement Completeness'] },
  'tasks.md': { title: (f) => `# Tasks: ${f} — `, heads: ['## Phase 1: Setup', '## Phase 2: Foundational', '## Dependencies & Execution Order', '## Definition of Done'] },
};
const CONSTITUTION_ARTICLES = ['I ', 'II ', 'III ', 'IV ', 'V ', 'VI ', 'VII ', 'VIII ', 'IX '];

let errors = 0; let warnings = 0;
const err = (f, msg) => { errors++; console.error(`ERROR ${f}: ${msg}`); };
const warn = (f, msg) => { warnings++; if (verbose) console.warn(`WARN  ${f}: ${msg}`); };

const dirs = fs.readdirSync(specsDir, { withFileTypes: true })
  .filter((d) => d.isDirectory() && /^\d{3}-[a-z0-9-]+$/.test(d.name)).map((d) => d.name).sort();
const covered = new Set();

for (const dir of dirs) {
  const fid = `F${dir.slice(0, 3)}`;
  if (!new RegExp(`\\|\\s*\\[?${fid}\\b`).test(catalog)) err(dir, 'chưa có trong danh mục feature của CODE_PLAN_CMS_LMS_FIRST.md');
  let status = '';
  for (const [file, rule] of Object.entries(FILES)) {
    const rel = `${dir}/${file}`;
    const p = path.join(specsDir, dir, file);
    if (!fs.existsSync(p)) { err(rel, 'thiếu file'); continue; }
    const text = fs.readFileSync(p, 'utf8');
    if (!text.startsWith(rule.title(fid))) err(rel, `tiêu đề phải bắt đầu "${rule.title(fid)}"`);
    for (const h of rule.heads) if (!text.includes(h)) err(rel, `thiếu mục "${h}"`);
    for (const id of idsIn(text, /\b(?:FR|US|NFR)-[A-Z0-9]+-\d{3}\b/g)) {
      if (!prdIds.has(id)) err(rel, `${id} không có trong PRD`);
      else if (id.startsWith('FR-') && file === 'spec.md') covered.add(id);
    }
    for (const id of idsIn(text, /\bTC-[A-Z0-9]+(?:-[A-Z0-9]+)*-\d{2,3}\b/g)) if (!tcIds.has(id)) err(rel, `${id} không có trong TEST_STRATEGY`);
    for (const id of idsIn(text, /\bADR-\d{4}\b/g)) if (!adrFiles.some((f) => f.startsWith(`${id}-`))) err(rel, `${id} không tồn tại`);
    if (/specs\/_deferred|99-deferred/.test(text)) err(rel, 'tham chiếu nội dung deferred (ADR-0082)');
    if (file === 'spec.md') {
      status = (text.match(/\*\*Status\*\*:\s*([A-Za-z ]+?)\s*(?:·|$)/m) || [])[1] || '';
      if (!['Draft', 'Ready', 'In progress', 'Done'].includes(status)) err(rel, `Status không hợp lệ: "${status}"`);
      if (status !== 'Draft' && /\[NEEDS CLARIFICATION/.test(text)) err(rel, 'Status khác Draft nhưng còn [NEEDS CLARIFICATION]');
      if (!/### User Story 1 .*\(Priority: P[1-3]\)/.test(text)) err(rel, 'thiếu "### User Story 1 — … (Priority: P1)"');
      if (!text.includes('**Independent Test**')) err(rel, 'user story thiếu **Independent Test**');
      const acRows = text.split('\n').filter((l) => new RegExp(`^\\|\\s*${fid}-AC-\\d{2}\\s*\\|`).test(l));
      if (acRows.length === 0) err(rel, 'không có tiêu chí EARS nào');
      const seen = new Set();
      for (const row of acRows) {
        const cells = row.split('|').map((c) => c.trim());
        if (seen.has(cells[1])) err(rel, `${cells[1]} bị trùng`); seen.add(cells[1]);
        if (!/PHẢI/.test(cells[2] || '')) err(rel, `${cells[1]} không theo EARS (thiếu "PHẢI")`);
        if (!/(FR|AC|NFR)-|ADR-|§/.test(cells[3] || '')) warn(rel, `${cells[1]} thiếu cột Nguồn`);
        if (!/TC-|—/.test(cells[4] || '')) warn(rel, `${cells[1]} thiếu cột TC (mã TC hoặc —)`);
      }
    }
    if (file === 'plan.md') {
      const sec = text.split('## Constitution Check')[1]?.split('\n## ')[0] || '';
      for (const a of CONSTITUTION_ARTICLES) if (!new RegExp(`^\\|\\s*${a.trim()}\\s`, 'm').test(sec)) err(rel, `Constitution Check thiếu điều ${a.trim()}`);
    }
    if (file === 'tasks.md') {
      const taskLines = text.split('\n').filter((l) => /^- \[[ x]\] T\d{3}/.test(l));
      if (taskLines.length === 0) err(rel, 'không có task nào');
      for (const l of taskLines) if (!/^- \[[ x]\] T\d{3}( \[P\])?( \[US\d+\])? \S/.test(l)) err(rel, `task sai định dạng: ${l.slice(0, 60)}`);
      for (const ac of idsIn(fs.readFileSync(path.join(specsDir, dir, 'spec.md'), 'utf8'), new RegExp(`\\b${fid}-AC-\\d{2}\\b`, 'g')))
        if (!text.includes(ac)) warn(rel, `${ac} chưa có task test`);
    }
  }
}

const uncovered = [...releaseFr].filter((id) => !covered.has(id));
if (dirs.length === 0) console.warn('WARN  chưa có feature nào trong specs/NNN-*/');
else if (uncovered.length) {
  warnings += uncovered.length;
  console.warn(`WARN  ${uncovered.length} FR Release chưa thuộc feature nào${verbose ? ': ' + uncovered.join(', ') : ' (--verbose để xem)'}`);
}
console.log(`check-features: ${dirs.length} feature, ${[...releaseFr].filter((id) => covered.has(id)).length}/${releaseFr.size} FR Release được phủ, ${errors} lỗi, ${warnings} cảnh báo`);
process.exit(errors ? 1 : 0);
