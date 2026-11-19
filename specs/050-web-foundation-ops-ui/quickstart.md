# Quickstart: F050 — Khung web: pnpm workspace, `ops-ui`, theme từ token, `apiClient`, Storybook

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F005 (F001 cho CI; F004 để thử gateway nội bộ thật)
- Lệnh: `cd src/cms/frontend && pnpm install`, `pnpm gen:api`, `pnpm dev`, `pnpm storybook` ([LOCAL_DEVELOPMENT §6.4](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); `make web-lint`, `make web-test`, `make storybook`.
- Tài khoản demo: không cần ở F050 (token giả qua `TokenProvider` thử trong test). Khi F051 xong, dùng `ad.truong.a`, `qlcm.truong.a`, `sa.admin` để thử end-to-end.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `pnpm install && pnpm dev` | Portal khung ở `localhost:5173` | F050-AC-01 |
| 2 | `pnpm build`; xem `dist/assets`; mở `/` với DevTools Network | 5 chunk workspace; `/` không tải chunk workspace | F050-AC-02 |
| 3 | Thêm `fetch("http://localhost:9080/…")` vào code; `make web-lint` | Fail | F050-AC-03 |
| 4 | Thêm `<p>Xin chào</p>`; xóa một khóa ở `en.json` | Lint/test fail | F050-AC-04 |
| 5 | Mở Storybook, đổi light/dark/compact | Theme theo token, locale vi | F050-AC-05 |
| 6 | Thêm `color: #fff` vào `.tsx` và `font-size: 14px` vào CSS | `make web-lint` fail | F050-AC-06 |
| 7 | Import file từ `src/game/` (vd. `../../../../game/` từ `src/cms/frontend/apps/portal`) | Build fail | F050-AC-07 |
| 8 | Story `DomainTag` (môn, độ hiếm, trạng thái) | Màu từ shared-tokens + nhãn chữ | F050-AC-08 |
| 9 | Form thử với schema có `maxLength` | Bộ đếm, chặn vượt, lỗi tại trường | F050-AC-09 |
| 10 | Sửa spec (thêm operation) không chạy `gen:api`; CI | job `gen-check` của `web.yml` fail | F050-AC-10 |
| 11 | Unit test: gọi `getMyPermissions` (optional), `listMyTenants` (none), một operation `required` | Header đúng theo metadata | F050-AC-11 |
| 12 | Gọi operation có `IdempotencyKey` hai lần (retry) | Cùng key; operation `IfMatch` gửi ETag | F050-AC-12 |
| 13 | Gọi `/admin/…` khi gateway nội bộ tắt | Đúng base URL nội bộ; thông báo VPN | F050-AC-13 |
| 14 | Mock trả `422 VALIDATION_FAILED` có `errors[]`; mã lạ | Lỗi tại trường; `error.generic` + `trace_id` | F050-AC-14 |
| 15 | Mock `401 TOKEN_EXPIRED` → refresh ok/lỗi; `403 TENANT_MISMATCH`; `403 MFA_REQUIRED`; `429` | Lặp lại 1 lần / `session_expired`; sự kiện tương ứng; chờ `Retry-After` | F050-AC-15 |
| 16 | Mock 503 cho GET, POST không key, POST có key; 400 | Retry chỉ GET + POST có key; 400 không retry | F050-AC-16 |
| 17 | Quét `localStorage`, `sessionStorage`, cookie, console sau test | Không token, không Authorization | F050-AC-17 |
| 18 | `pnpm --filter ops-ui storybook:build` | Thành công; mỗi component có 3 story mode | F050-AC-18 |
| 19 | `pnpm --filter ops-ui test-storybook` | 0 lỗi axe serious/critical | F050-AC-19 |
| 20 | `pnpm test:visual` | Snapshot light/dark/compact/zoom 200% khớp baseline | F050-AC-20 |
| 21 | PR chỉ sửa `design/admin-tokens/seed.json` | Workflow `web.yml` chạy (gọi từ `design.yml`) | F050-AC-21 |

## 3. Lệnh test tự động

```bash
cd src/cms/frontend
pnpm lint && pnpm test            # Vitest (apiClient, i18n, openapiRules)
pnpm --filter ops-ui storybook:build && pnpm --filter ops-ui test-storybook
pnpm test:visual                  # Playwright trên Storybook tĩnh
```

## 4. Bằng chứng cần nộp

- Số test pass/fail, báo cáo axe, ảnh baseline visual, danh sách chunk build (kịch bản 2).

## Bằng chứng (2026-11-19, local — Windows, Node 20.19, pnpm 10.34.9, Chromium headless shell của Playwright 1.64.1)

| Tiêu chí | Bằng chứng tự động | Kết quả |
|---|---|---|
| F050-AC-01 | pnpm workspace `apps/portal` + `packages/ops-ui`, TS `strict` (`pnpm typecheck` 0 lỗi); Vite cổng 5173 `strictPort` | OK |
| F050-AC-02 | `src/app/router.test.tsx` (render `/` không gọi loader workspace nào, `/school` chỉ gọi `school-lms`), `scripts/check-chunks.test.mjs`; `pnpm build` → 5 chunk `workspace-<id>-*.js`, không chunk nào trong closure tĩnh của entry | 5/5, 3/3 pass; build OK |
| F050-AC-03 | `src/app/runtime-config.test.ts` (`/config.json`, fallback `VITE_*` chỉ ở dev, URL literal → `ew/no-env-url-literal`) | 8/8 pass |
| F050-AC-04 | `src/i18n/keys.test.ts` (khóa vi = en ở 2 namespace, `<p>Xin chào</p>` và `placeholder="…"` → `i18next/no-literal-string`, mọi ProblemCode có thông điệp riêng hoặc `error.generic`) | 7/7 pass (cảnh báo: 142/163 mã chưa có thông điệp riêng, hiển thị `error.generic` + mã) |
| F050-AC-05 | `theme/OpsThemeProvider.test.tsx` (TC-UI-005: light/dark/compact, dark+compact, `vi_VN`/`en_US` + dayjs, không CSS `.ant-*`) | 6/6 pass |
| F050-AC-06 | `eslint-rules/no-hardcoded-style.test.js` (TC-UI-003: RuleTester 15 ca, config dự án, stylelint, bỏ qua file sinh) | pass |
| F050-AC-07 | `eslint-rules/no-game-imports.test.js` (TC-UI-010) + build thử có `import '../../../../game/scripts/core/ui_tokens.gd'` → `error during build … forbidden in the Admin Portal` | pass; build fail như yêu cầu |
| F050-AC-08 | `DomainTag.test.tsx` (TC-UI-008: màu `domainColors` theo mode, nhãn vi/en, mã lạ không chấm màu) | 4/4 pass |
| F050-AC-09 | `forms/openapiRules.test.tsx` (TC-TEN-024 phía client: rule, gõ quá `maxLength` bị chặn, bộ đếm `5/5`, lỗi tại trường) | 3/3 pass |
| F050-AC-10 | `scripts/gen-op-meta.test.mjs` (484 operation, `getMyPermissions` optional, `listMyTenants` none, `revokeAllSessions` idempotency, `updateCountry` If-Match, drift); `pnpm gen:check` | 6/6 pass; "2 file(s) match" |
| F050-AC-11 | `middleware/tenant.test.ts` (TC-TEN-011a, MSW) | 8/8 pass |
| F050-AC-12 | `middleware/idempotency.test.ts` (UUIDv7 version/variant, cùng key qua 503→503→204 và khi người dùng thử lại cùng ý định, If-Match từ ETag) | 6/6 pass |
| F050-AC-13 | `middleware/gateway.test.ts` (TC-CMS-WS-03: prefix, mọi operation sinh khớp quy tắc, gateway nội bộ chết → `vpn_required` + `VPN_REQUIRED`) | 5/5 pass |
| F050-AC-14 | `middleware/problem.test.ts` (map theo `code` không theo `title`, `errors[]` → trường, mã lạ → `error.generic`, `trace_id`) | 4/4 pass |
| F050-AC-15 | `middleware/auth.test.ts` (TC-CMS-WS-02: refresh đúng 1 lần kể cả song song, `session_expired`, `tenant_mismatch`, `mfa_required`, 429 chờ `Retry-After`) | 8/8 pass |
| F050-AC-16 | `middleware/retry.test.ts` | 11/11 pass |
| F050-AC-17 | `client.security.test.ts` (storage/cookie/console sạch token, Authorization, mật khẩu) | 1/1 pass |
| F050-AC-18 | `.storybook/check-modes.mjs` (3 component × Light/Dark/Compact) + `check-modes.test.mjs`; `pnpm storybook:build` thành công | 3/3 pass; build OK |
| F050-AC-19 | `node tooling/storybook-a11y.mjs` (test-runner + axe WCAG 2.1 A/AA, fail serious/critical) — lượt đầu 9/9 fail vì `colorTextSecondary` của Ant Design 3,36:1 → sửa token (tokens@0.2.0) | 9/9 pass |
| F050-AC-20 | `pnpm test:visual` (TC-UI-004: 9 story × zoom 100%/200%, không cuộn ngang ở 200%); sửa DomainTag → 3 ảnh 200% fail tới khi duyệt baseline | 19/19 pass, 18 baseline `win32` |
| F050-AC-21 | `scripts/ci/test-web-rules.sh` (path filter, job lint/gen:check/test/build/Storybook/axe/visual, `workflow_call`); `check-workflows.mjs` | 3/3 pass; 0 lỗi |

Tổng: `pnpm lint` 0 lỗi, `pnpm typecheck` 0 lỗi, `pnpm test` 18 file / 117 test pass (1 worker, 147 s), `pnpm gen:check` khớp, `pnpm build` OK (entry 948 kB chưa tách vendor — cảnh báo kích thước của Vite). `pnpm dev` (kịch bản 1) và DevTools Network (kịch bản 2) không chạy tay trong môi trường này; thay bằng test route + kiểm manifest build. Gọi API thật qua gateway (F004) chờ F051. Baseline ảnh Linux do CI tạo (`--update-snapshots=missing`, artifact `web-visual`) cần duyệt rồi commit.
