# Tasks: F050 — Khung web: pnpm workspace, `ops-ui`, theme từ token, `apiClient`, Storybook

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 pnpm workspace (`apps/*`, `packages/*`), `tsconfig.base.json` (`strict`), script gốc `lint`, `test`, `gen:api`, `dev`, `storybook` — file: `src/cms/frontend/package.json` — **xong 2026-11-19 — `src/cms/frontend/package.json` (pnpm 10.34.9, script `dev`, `build`, `gen:api`, `gen:check`, `lint`, `typecheck`, `test`, `storybook`, `storybook:build`, `test-storybook`, `test:visual`), `pnpm-workspace.yaml`, `tsconfig.base.json` (`strict`), `pnpm-lock.yaml`**
- [x] T002 [P] Khởi tạo `apps/portal` (Vite, React, React Router, TanStack Query) — file: `src/cms/frontend/apps/portal/vite.config.ts` — **xong 2026-11-19 — `apps/portal` (Vite 6.4.5 cổng 5173 `strictPort`, React 18.3.1, React Router 6.30.6, TanStack Query 5.104.5; plugin dev phục vụ `/config.json`)**
- [x] T003 [P] Khởi tạo `packages/ops-ui` (antd@5, pro-components, export `src/index.ts`) — file: `src/cms/frontend/packages/ops-ui/package.json` — **xong 2026-11-19 — `packages/ops-ui` (antd 5.29.3, pro-components 2.8.10 peer, export `src/index.ts`)**
- [x] T004 [P] Target `make web-lint`, `make web-test`, `make storybook` — file: `Makefile` — **xong 2026-11-19 — `make web-install`, `web-lint`, `web-test`, `web-build`, `storybook`, `web-storybook-check`, `web-visual`**

## Phase 2: Foundational (chặn mọi user story)

- [x] T005 Generator `gen-op-meta.mjs`: đọc spec gộp → `operation-meta.gen.ts` (gateway theo prefix, tenantHeader, idempotency, ifMatch); `openapi-typescript` → `schema.gen.d.ts`; script `gen:api` + `gen:check` — file: `src/cms/frontend/apps/portal/scripts/gen-op-meta.mjs` — **xong 2026-11-19 — `apps/portal/scripts/gen-op-meta.mjs` → `operation-meta.gen.ts` (484 operation; 147 nội bộ, 170 idempotency, 47 If-Match) + `schema.gen.d.ts` (openapi-typescript); `gen:check` → "2 file(s) match"**
- [x] T006 ESLint + stylelint gốc (TS strict, react, i18next) — file: `src/cms/frontend/.eslintrc.cjs` — **xong 2026-11-19 — `eslint.config.mjs` (flat config ESLint 9: typescript-eslint, react-hooks, i18next, plugin `ew`) + `stylelint.config.mjs` (Clarification 2026-11-19)**

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Khung Admin Portal một ứng dụng, 5 workspace (Priority: P1) 🎯 MVP

**Goal**: Portal rỗng 5 workspace lazy, runtime config, i18n.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [x] T007 [P] [US1] Test F050-AC-01, F050-AC-02: build tạo 5 chunk workspace; render `/` không import workspace (Vitest + phân tích manifest Vite) — file: `src/cms/frontend/apps/portal/src/app/router.test.tsx` — phải FAIL trước khi code — **xong 2026-11-19 — `src/app/router.test.tsx` 5/5 pass + `scripts/check-chunks.test.mjs` 3/3 pass; `pnpm build` → `check-chunks: 5 workspace chunks, none loaded by the entry`**
- [x] T008 [P] [US1] Test F050-AC-03: `runtime-config` đọc `/config.json`, fallback `VITE_*`; rule lint phát hiện URL literal — file: `src/cms/frontend/apps/portal/src/app/runtime-config.test.ts` — **xong 2026-11-19 — `src/app/runtime-config.test.ts` 8/8 pass**
- [x] T009 [P] [US1] Test F050-AC-04: khóa `vi` ⊆ `en` và ngược lại; ESLint báo chuỗi JSX literal — file: `src/cms/frontend/apps/portal/src/i18n/keys.test.ts` — **xong 2026-11-19 — `src/i18n/keys.test.ts` 7/7 pass**

### Implementation cho User Story 1

- [x] T010 [US1] `App.tsx`, `router.tsx` với 5 route `React.lazy` (`school`, `content`, `game-ops`, `live-ops`, `platform`) trang trống có tiêu đề dịch — file: `src/cms/frontend/apps/portal/src/app/router.tsx` — **xong 2026-11-19 — `app/router.tsx` (route `lazy` cho `/school`, `/content`, `/game-ops`, `/live-ops`, `/platform`), `App.tsx`, `Shell.tsx`, `Home.tsx`, `WorkspacePlaceholder.tsx`, `workspaces/*/index.tsx`**
- [x] T011 [P] [US1] `runtime-config.ts` + `public/config.example.json` — file: `src/cms/frontend/apps/portal/src/app/runtime-config.ts` — **xong 2026-11-19 — `app/runtime-config.ts` + `public/config.example.json`**
- [x] T012 [P] [US1] i18n (react-i18next, `vi` mặc định, `en`, khóa `error.<CODE>`, `error.generic`) — file: `src/cms/frontend/apps/portal/src/i18n/index.ts` — **xong 2026-11-19 — `i18n/index.ts`, `vi.json`, `en.json` (`error.<CODE>` cho 21 ProblemCode + `VPN_REQUIRED`, `NETWORK_ERROR`, `error.generic`)**

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — `ops-ui` theo Admin/Ops design system, không hardcode giao diện (Priority: P1)

**Goal**: Theme provider, lint hardcode/import game, DomainTag, rule form từ OpenAPI.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [x] T013 [P] [US2] Test F050-AC-05: `OpsThemeProvider` áp `lightTheme/darkTheme/compactTheme`, locale `vi_VN`/`en_US`, dayjs locale (TC-UI-005) — file: `src/cms/frontend/packages/ops-ui/src/theme/OpsThemeProvider.test.tsx` — phải FAIL trước khi code — **xong 2026-11-19 — `theme/OpsThemeProvider.test.tsx` 6/6 pass**
- [x] T014 [P] [US2] Test F050-AC-06: rule `no-hardcoded-style` + stylelint báo màu/font/size/spacing literal; bỏ qua file sinh (TC-UI-003) — file: `src/cms/frontend/eslint-rules/no-hardcoded-style.test.js` — **xong 2026-11-19 — `eslint-rules/no-hardcoded-style.test.js` (RuleTester + ESLint config + stylelint) pass**
- [x] T015 [P] [US2] Test F050-AC-07: rule `no-game-imports` báo import `src/game/`, `design/game-tokens` (TC-UI-010) — file: `src/cms/frontend/eslint-rules/no-game-imports.test.js` — **xong 2026-11-19 — `eslint-rules/no-game-imports.test.js` (rule + plugin Vite) pass; build thử với import `ui_tokens.gd` → `error during build ... forbidden in the Admin Portal`**
- [x] T016 [P] [US2] Test F050-AC-08: `DomainTag` dùng `domainColors` + có nhãn chữ (TC-UI-008) — file: `src/cms/frontend/packages/ops-ui/src/components/DomainTag/DomainTag.test.tsx` — **xong 2026-11-19 — `DomainTag.test.tsx` 4/4 pass**
- [x] T017 [P] [US2] Test F050-AC-09: `openapiRules(schema)` sinh rule `required`, `max`, `pattern`; vượt `maxLength` bị chặn (TC-TEN-024 phía client) — file: `src/cms/frontend/packages/ops-ui/src/forms/openapiRules.test.ts` — **xong 2026-11-19 — `forms/openapiRules.test.tsx` 3/3 pass**

### Implementation cho User Story 2

- [x] T018 [US2] `OpsThemeProvider` (theme mode + density từ preference, `ConfigProvider`, `App` của antd) — file: `src/cms/frontend/packages/ops-ui/src/theme/OpsThemeProvider.tsx` — **xong 2026-11-19 — `theme/OpsThemeProvider.tsx` (+ `base.css`, `fonts.css` tự host Be Vietnam Pro)**
- [x] T019 [P] [US2] Rule ESLint `no-hardcoded-style`, `no-game-imports`; stylelint `color-no-hex`, `declaration-property-value-disallowed-list` cho font/size/spacing — file: `src/cms/frontend/eslint-rules/no-hardcoded-style.js` — **xong 2026-11-19 — `eslint-rules/{no-hardcoded-style,no-game-imports,no-env-url-literal,index}.js`, `tooling/forbid-game-imports.mjs`, `stylelint.config.mjs`**
- [x] T020 [P] [US2] `DomainTag` + story — file: `src/cms/frontend/packages/ops-ui/src/components/DomainTag/DomainTag.tsx` — **xong 2026-11-19 — `components/DomainTag/DomainTag.tsx` + `DomainTag.stories.tsx`**
- [x] T021 [P] [US2] `openapiRules.ts` + `CharCounter.tsx` — file: `src/cms/frontend/packages/ops-ui/src/forms/openapiRules.ts` — **xong 2026-11-19 — `forms/openapiRules.ts`, `forms/CharCounter.tsx` (`openapiInputProps`) + story**

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — `apiClient` sinh từ OpenAPI, xử lý header và lỗi chuẩn (Priority: P1)

**Goal**: Client có type, middleware header, Problem, refresh, retry.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [x] T022 [P] [US3] Test F050-AC-10: `gen:check` phát hiện metadata/type lệch spec; metadata đúng cho `getMyPermissions` (optional), `listMyTenants` (none), operation có `IdempotencyKey`, `IfMatch` — file: `src/cms/frontend/apps/portal/scripts/gen-op-meta.test.mjs` — phải FAIL trước khi code — **xong 2026-11-19 — `scripts/gen-op-meta.test.mjs` 6/6 pass**
- [x] T023 [P] [US3] Test F050-AC-11: header `Authorization`, `X-Tenant-Id` theo required/optional/none, `Accept-Language` (MSW) — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/tenant.test.ts` — **xong 2026-11-19 — `middleware/tenant.test.ts` 8/8 pass (MSW)**
- [x] T024 [P] [US3] Test F050-AC-12: `Idempotency-Key` sinh dạng UUIDv7 (kiểm version nibble) và giữ khi retry; `If-Match` từ ETag — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/idempotency.test.ts` — **xong 2026-11-19 — `middleware/idempotency.test.ts` 6/6 pass**
- [x] T025 [P] [US3] Test F050-AC-13: chọn base URL nội bộ cho `/admin/`, `/sis/`, `/game-admin/`, `/game-media`; lỗi kết nối → `vpn_required` — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/gateway.test.ts` — **xong 2026-11-19 — `middleware/gateway.test.ts` 5/5 pass**
- [x] T026 [P] [US3] Test F050-AC-14: ánh xạ `code` → `error.<CODE>`, `VALIDATION_FAILED` → `fieldErrors`, mã lạ → `error.generic`, giữ `traceId` — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/problem.test.ts` — **xong 2026-11-19 — `middleware/problem.test.ts` 4/4 pass**
- [x] T027 [P] [US3] Test F050-AC-15: `TOKEN_EXPIRED` refresh một lần; `TENANT_MISMATCH`, `MFA_REQUIRED` phát sự kiện; `429` chờ `Retry-After` — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/auth.test.ts` — **xong 2026-11-19 — `middleware/auth.test.ts` 8/8 pass**
- [x] T028 [P] [US3] Test F050-AC-16: retry chỉ GET/có key với 502/503/504/lỗi mạng, không retry 4xx — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/retry.test.ts` — **xong 2026-11-19 — `middleware/retry.test.ts` 11/11 pass**
- [x] T029 [P] [US3] Test F050-AC-17: sau bộ test, `localStorage`/`sessionStorage`/`document.cookie` không có token; spy console không thấy `Authorization` — file: `src/cms/frontend/apps/portal/src/shared/api/client.security.test.ts` — **xong 2026-11-19 — `client.security.test.ts` 1/1 pass**

### Implementation cho User Story 3

- [x] T030 [US3] Interface `TokenProvider { getAccessToken(): Promise<string|null>; refresh(): Promise<boolean> }`, `TenantContext` (tenant làm việc hoặc null) — file: `src/cms/frontend/apps/portal/src/shared/auth/TokenProvider.ts` — **xong 2026-11-19 — `shared/auth/TokenProvider.ts`, `shared/auth/TenantContext.tsx`**
- [x] T031 [US3] `createApiClient` (openapi-fetch) + middleware `language`, `auth`, `tenant`, `idempotency`, `ifMatch`, `gateway`, `retry`, `problem`; `events.ts` — file: `src/cms/frontend/apps/portal/src/shared/api/client.ts` — **xong 2026-11-19 — `shared/api/client.ts` + `middleware/{language,auth,tenant,idempotency,ifmatch,gateway,retry,problem}.ts`, `events.ts`, `ids.ts`, `operations.ts`, `types.ts`**

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Storybook, trợ năng và visual regression (Priority: P2)

**Goal**: Storybook 3 mode, axe, Playwright visual, workflow `web.yml`.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [x] T032 [P] [US4] Kiểm F050-AC-18: script xác nhận mọi component có story `Light`, `Dark`, `Compact` (TC-UI-005) — file: `src/cms/frontend/packages/ops-ui/.storybook/check-modes.mjs` — phải FAIL trước khi code — **xong 2026-11-19 — `.storybook/check-modes.mjs` → "3 component(s), each with Light/Dark/Compact stories"; `check-modes.test.mjs` 3/3 pass**
- [x] T033 [P] [US4] Test-runner axe F050-AC-19 (fail serious/critical) (TC-UI-006) — file: `src/cms/frontend/packages/ops-ui/.storybook/test-runner.ts` — **xong 2026-11-19 — `.storybook/test-runner.ts` + `tooling/storybook-a11y.mjs`: 9/9 story pass (lượt đầu fail 9/9 vì `colorTextSecondary` 3,36:1 → sửa token, tokens@0.2.0)**
- [x] T034 [P] [US4] Playwright F050-AC-20: snapshot story ở 3 mode + zoom 200% (TC-UI-004) — file: `src/cms/frontend/tests/visual/storybook.spec.ts` — **xong 2026-11-19 — `tests/visual/storybook.spec.ts` + `playwright.config.ts`: 19/19 pass (18 ảnh baseline `win32`); đổi DomainTag → 3 ảnh zoom 200% fail cho tới khi duyệt baseline**
- [x] T035 [P] [US4] Test F050-AC-21: path filter của workflow `web.yml` — file: `scripts/ci/test-web-rules.sh` — **xong 2026-11-19 — `scripts/ci/test-web-rules.sh` → `web-rules.test.mjs` 3/3 pass; `path-filters.test.mjs` 22/22**

### Implementation cho User Story 4

- [x] T036 [US4] Cấu hình Storybook 8 (decorator `OpsThemeProvider`, toolbar mode/locale) — file: `src/cms/frontend/packages/ops-ui/.storybook/preview.tsx` — **xong 2026-11-19 — `.storybook/main.ts`, `preview.tsx` (decorator `OpsThemeProvider`, toolbar mode/density/locale, `parameters.opsTheme`)**
- [x] T037 [US4] Workflow `web.yml` (action ghim SHA): lint, `gen:check`, test, build portal, build Storybook, test-runner, visual — file: `.github/workflows/web.yml` — **xong 2026-11-19 — `.github/workflows/web.yml` (job `lint`, `test`, `storybook`; action ghim SHA); `check-workflows` 0 lỗi**

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [x] T038 [P] Cập nhật LOCAL_DEVELOPMENT §6.4 (`pnpm gen:api` sinh cả metadata, runtime `config.json`) và ADMIN_PORTAL §1.1 (vị trí apiClient) — file: `docs/11-operations/LOCAL_DEVELOPMENT.md` — **xong 2026-11-19 — LOCAL_DEVELOPMENT §6.4, §7; ADMIN_PORTAL §1.1**
- [x] T039 [P] CHANGELOG mục F050 — file: `CHANGELOG.md` — **xong 2026-11-19 — CHANGELOG mục F050**
- [x] T040 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/050-web-foundation-ops-ui/quickstart.md` — **xong 2026-11-19 — quickstart.md mục Bằng chứng**

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2, US3 (song song) → US4 → Polish.
- T005 trước T022–T031; đầu ra F005 phải có trước T013, T018.
- F051 bắt đầu khi US1 + US3 xong.

## Definition of Done

- [x] Mọi tiêu chí F050-AC-* có test pass (ghi mã test/TC) — **F050-AC-01…21 có test pass (quickstart §Bằng chứng)**
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh — **không áp dụng Go; web: lint 0 lỗi, typecheck 0 lỗi, `gen:check` khớp, Vitest 117/117, build OK, axe 9/9, visual 19/19**
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — **đạt; MSW/fake TokenProvider chỉ trong test, gắn nhãn MOCK**
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi — **LOCAL_DEVELOPMENT, ADMIN_PORTAL, DESIGN_SYSTEM (không đổi quy tắc), CHANGELOG**
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — **CODE_PLAN: Done**
