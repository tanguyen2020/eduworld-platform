# Tasks: F050 — Khung web: pnpm workspace, `ops-ui`, theme từ token, `apiClient`, Storybook

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 pnpm workspace (`apps/*`, `packages/*`), `tsconfig.base.json` (`strict`), script gốc `lint`, `test`, `gen:api`, `dev`, `storybook` — file: `src/cms/frontend/package.json`
- [ ] T002 [P] Khởi tạo `apps/portal` (Vite, React, React Router, TanStack Query) — file: `src/cms/frontend/apps/portal/vite.config.ts`
- [ ] T003 [P] Khởi tạo `packages/ops-ui` (antd@5, pro-components, export `src/index.ts`) — file: `src/cms/frontend/packages/ops-ui/package.json`
- [ ] T004 [P] Target `make web-lint`, `make web-test`, `make storybook` — file: `Makefile`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T005 Generator `gen-op-meta.mjs`: đọc spec gộp → `operation-meta.gen.ts` (gateway theo prefix, tenantHeader, idempotency, ifMatch); `openapi-typescript` → `schema.gen.d.ts`; script `gen:api` + `gen:check` — file: `src/cms/frontend/apps/portal/scripts/gen-op-meta.mjs`
- [ ] T006 ESLint + stylelint gốc (TS strict, react, i18next) — file: `src/cms/frontend/.eslintrc.cjs`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Khung Admin Portal một ứng dụng, 5 workspace (Priority: P1) 🎯 MVP

**Goal**: Portal rỗng 5 workspace lazy, runtime config, i18n.

**Independent Test**: Như spec.md User Story 1.

### Tests cho User Story 1

- [ ] T007 [P] [US1] Test F050-AC-01, F050-AC-02: build tạo 5 chunk workspace; render `/` không import workspace (Vitest + phân tích manifest Vite) — file: `src/cms/frontend/apps/portal/src/app/router.test.tsx` — phải FAIL trước khi code
- [ ] T008 [P] [US1] Test F050-AC-03: `runtime-config` đọc `/config.json`, fallback `VITE_*`; rule lint phát hiện URL literal — file: `src/cms/frontend/apps/portal/src/app/runtime-config.test.ts`
- [ ] T009 [P] [US1] Test F050-AC-04: khóa `vi` ⊆ `en` và ngược lại; ESLint báo chuỗi JSX literal — file: `src/cms/frontend/apps/portal/src/i18n/keys.test.ts`

### Implementation cho User Story 1

- [ ] T010 [US1] `App.tsx`, `router.tsx` với 5 route `React.lazy` (`school`, `content`, `game-ops`, `live-ops`, `platform`) trang trống có tiêu đề dịch — file: `src/cms/frontend/apps/portal/src/app/router.tsx`
- [ ] T011 [P] [US1] `runtime-config.ts` + `public/config.example.json` — file: `src/cms/frontend/apps/portal/src/app/runtime-config.ts`
- [ ] T012 [P] [US1] i18n (react-i18next, `vi` mặc định, `en`, khóa `error.<CODE>`, `error.generic`) — file: `src/cms/frontend/apps/portal/src/i18n/index.ts`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — `ops-ui` theo Admin/Ops design system, không hardcode giao diện (Priority: P1)

**Goal**: Theme provider, lint hardcode/import game, DomainTag, rule form từ OpenAPI.

**Independent Test**: Như spec.md User Story 2.

### Tests cho User Story 2

- [ ] T013 [P] [US2] Test F050-AC-05: `OpsThemeProvider` áp `lightTheme/darkTheme/compactTheme`, locale `vi_VN`/`en_US`, dayjs locale (TC-UI-005) — file: `src/cms/frontend/packages/ops-ui/src/theme/OpsThemeProvider.test.tsx` — phải FAIL trước khi code
- [ ] T014 [P] [US2] Test F050-AC-06: rule `no-hardcoded-style` + stylelint báo màu/font/size/spacing literal; bỏ qua file sinh (TC-UI-003) — file: `src/cms/frontend/eslint-rules/no-hardcoded-style.test.js`
- [ ] T015 [P] [US2] Test F050-AC-07: rule `no-game-imports` báo import `src/game/`, `design/game-tokens` (TC-UI-010) — file: `src/cms/frontend/eslint-rules/no-game-imports.test.js`
- [ ] T016 [P] [US2] Test F050-AC-08: `DomainTag` dùng `domainColors` + có nhãn chữ (TC-UI-008) — file: `src/cms/frontend/packages/ops-ui/src/components/DomainTag/DomainTag.test.tsx`
- [ ] T017 [P] [US2] Test F050-AC-09: `openapiRules(schema)` sinh rule `required`, `max`, `pattern`; vượt `maxLength` bị chặn (TC-TEN-024 phía client) — file: `src/cms/frontend/packages/ops-ui/src/forms/openapiRules.test.ts`

### Implementation cho User Story 2

- [ ] T018 [US2] `OpsThemeProvider` (theme mode + density từ preference, `ConfigProvider`, `App` của antd) — file: `src/cms/frontend/packages/ops-ui/src/theme/OpsThemeProvider.tsx`
- [ ] T019 [P] [US2] Rule ESLint `no-hardcoded-style`, `no-game-imports`; stylelint `color-no-hex`, `declaration-property-value-disallowed-list` cho font/size/spacing — file: `src/cms/frontend/eslint-rules/no-hardcoded-style.js`
- [ ] T020 [P] [US2] `DomainTag` + story — file: `src/cms/frontend/packages/ops-ui/src/components/DomainTag/DomainTag.tsx`
- [ ] T021 [P] [US2] `openapiRules.ts` + `CharCounter.tsx` — file: `src/cms/frontend/packages/ops-ui/src/forms/openapiRules.ts`

**Checkpoint**: User Story 2 chạy và kiểm độc lập được.

## Phase 5: User Story 3 — `apiClient` sinh từ OpenAPI, xử lý header và lỗi chuẩn (Priority: P1)

**Goal**: Client có type, middleware header, Problem, refresh, retry.

**Independent Test**: Như spec.md User Story 3.

### Tests cho User Story 3

- [ ] T022 [P] [US3] Test F050-AC-10: `gen:check` phát hiện metadata/type lệch spec; metadata đúng cho `getMyPermissions` (optional), `listMyTenants` (none), operation có `IdempotencyKey`, `IfMatch` — file: `src/cms/frontend/apps/portal/scripts/gen-op-meta.test.mjs` — phải FAIL trước khi code
- [ ] T023 [P] [US3] Test F050-AC-11: header `Authorization`, `X-Tenant-Id` theo required/optional/none, `Accept-Language` (MSW) — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/tenant.test.ts`
- [ ] T024 [P] [US3] Test F050-AC-12: `Idempotency-Key` sinh dạng UUIDv7 (kiểm version nibble) và giữ khi retry; `If-Match` từ ETag — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/idempotency.test.ts`
- [ ] T025 [P] [US3] Test F050-AC-13: chọn base URL nội bộ cho `/admin/`, `/sis/`, `/game-admin/`, `/game-media`; lỗi kết nối → `vpn_required` — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/gateway.test.ts`
- [ ] T026 [P] [US3] Test F050-AC-14: ánh xạ `code` → `error.<CODE>`, `VALIDATION_FAILED` → `fieldErrors`, mã lạ → `error.generic`, giữ `traceId` — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/problem.test.ts`
- [ ] T027 [P] [US3] Test F050-AC-15: `TOKEN_EXPIRED` refresh một lần; `TENANT_MISMATCH`, `MFA_REQUIRED` phát sự kiện; `429` chờ `Retry-After` — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/auth.test.ts`
- [ ] T028 [P] [US3] Test F050-AC-16: retry chỉ GET/có key với 502/503/504/lỗi mạng, không retry 4xx — file: `src/cms/frontend/apps/portal/src/shared/api/middleware/retry.test.ts`
- [ ] T029 [P] [US3] Test F050-AC-17: sau bộ test, `localStorage`/`sessionStorage`/`document.cookie` không có token; spy console không thấy `Authorization` — file: `src/cms/frontend/apps/portal/src/shared/api/client.security.test.ts`

### Implementation cho User Story 3

- [ ] T030 [US3] Interface `TokenProvider { getAccessToken(): Promise<string|null>; refresh(): Promise<boolean> }`, `TenantContext` (tenant làm việc hoặc null) — file: `src/cms/frontend/apps/portal/src/shared/auth/TokenProvider.ts`
- [ ] T031 [US3] `createApiClient` (openapi-fetch) + middleware `language`, `auth`, `tenant`, `idempotency`, `ifMatch`, `gateway`, `retry`, `problem`; `events.ts` — file: `src/cms/frontend/apps/portal/src/shared/api/client.ts`

**Checkpoint**: User Story 3 chạy và kiểm độc lập được.

## Phase 6: User Story 4 — Storybook, trợ năng và visual regression (Priority: P2)

**Goal**: Storybook 3 mode, axe, Playwright visual, workflow `web.yml`.

**Independent Test**: Như spec.md User Story 4.

### Tests cho User Story 4

- [ ] T032 [P] [US4] Kiểm F050-AC-18: script xác nhận mọi component có story `Light`, `Dark`, `Compact` (TC-UI-005) — file: `src/cms/frontend/packages/ops-ui/.storybook/check-modes.mjs` — phải FAIL trước khi code
- [ ] T033 [P] [US4] Test-runner axe F050-AC-19 (fail serious/critical) (TC-UI-006) — file: `src/cms/frontend/packages/ops-ui/.storybook/test-runner.ts`
- [ ] T034 [P] [US4] Playwright F050-AC-20: snapshot story ở 3 mode + zoom 200% (TC-UI-004) — file: `src/cms/frontend/tests/visual/storybook.spec.ts`
- [ ] T035 [P] [US4] Test F050-AC-21: path filter của workflow `web.yml` — file: `scripts/ci/test-web-rules.sh`

### Implementation cho User Story 4

- [ ] T036 [US4] Cấu hình Storybook 8 (decorator `OpsThemeProvider`, toolbar mode/locale) — file: `src/cms/frontend/packages/ops-ui/.storybook/preview.tsx`
- [ ] T037 [US4] Workflow `web.yml` (action ghim SHA): lint, `gen:check`, test, build portal, build Storybook, test-runner, visual — file: `.github/workflows/web.yml`

**Checkpoint**: User Story 4 chạy và kiểm độc lập được.

## Phase N: Polish & Cross-Cutting

- [ ] T038 [P] Cập nhật LOCAL_DEVELOPMENT §6.4 (`pnpm gen:api` sinh cả metadata, runtime `config.json`) và ADMIN_PORTAL §1.1 (vị trí apiClient) — file: `docs/11-operations/LOCAL_DEVELOPMENT.md`
- [ ] T039 [P] CHANGELOG mục F050 — file: `CHANGELOG.md`
- [ ] T040 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — file: `specs/050-web-foundation-ops-ui/quickstart.md`

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2, US3 (song song) → US4 → Polish.
- T005 trước T022–T031; đầu ra F005 phải có trước T013, T018.
- F051 bắt đầu khi US1 + US3 xong.

## Definition of Done

- [ ] Mọi tiêu chí F050-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
