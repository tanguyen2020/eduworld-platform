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
