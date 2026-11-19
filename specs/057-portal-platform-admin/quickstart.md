# Quickstart: F057 — Workspace D — Platform Administration: tenant, gán/clone, thùng rác, danh mục, cấu hình hệ thống

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F045, F051, F010, F016, F050.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md); local gọi public + internal cùng máy).
- Tài khoản: `sa.admin` (MFA tắt ở local; bật OTP seed để thử step-up), `ad.truong.a`, `cm.noidung`, `sm.antoan`.
- Dữ liệu: bộ câu hỏi platform seed; một câu hỏi TH Demo A đã xóa mềm ở workspace B.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `sa.admin` mở Thùng rác; gõ "demo" trong bộ lọc tenant, chọn `th-demo-a`, `pgd-demo` | Tìm theo mã/tên, chọn nhiều; F5 vẫn giữ lựa chọn trong phiên | F057-AC-01 |
| 2 | Xem network của danh sách | Không `X-Tenant-Id`, có `tenant_ids[]`; cột Tenant + loại | F057-AC-02 |
| 3 | Tạo bản ghi tenant (ví dụ khôi phục/import) không chọn tenant đích | Nút gửi bị chặn tới khi chọn một tenant | F057-AC-03 |
| 4 | `ad.truong.a` mở danh sách bất kỳ | Không dropdown, không cột Tenant, không `tenant_ids` | F057-AC-04 |
| 5 | Gán bộ câu hỏi platform cho 2 tenant; xem tab "Tenant được gán" | 2 dòng; `ad.truong.a` thấy bộ câu hỏi chỉ đọc | F057-AC-05 |
| 6 | Gỡ gán `pgd-demo` | Hộp xác nhận nêu lịch sử giữ nguyên; tenant không còn thấy | F057-AC-06 |
| 7 | Wizard clone question set TH Demo A → `pgd-demo` | Job `succeeded`, `id_map` hiển thị; bản sao `draft` | F057-AC-07 |
| 8 | Wizard clone: thử chọn đích = nguồn, tìm loại "học sinh" | Không cho; không có loại dữ liệu học sinh | F057-AC-08 |
| 9 | Thùng rác → khôi phục câu hỏi | Khôi phục thành công sau nhập lý do | F057-AC-09 |
| 10 | Tạo câu hỏi mới cùng mã rồi khôi phục bản cũ | Nút vô hiệu / báo "Đã có bản ghi mới cùng mã"; không có "Xóa vĩnh viễn" | F057-AC-10 |
| 11 | Danh mục → Quốc gia: tắt rồi bật lại `VN` (môi trường local) | Yêu cầu lý do; không có nút xóa | F057-AC-11 |
| 12 | Tỉnh/thành `status=all`: sửa `official_code`; tạo kế thừa | Đơn vị cũ không bị sửa đè; `successor_code` hiển thị | F057-AC-12 |
| 13 | Import CSV trường có 1 dòng mã tỉnh sai cho `pgd-demo` | Dry-run hiển thị lỗi theo dòng; áp dụng khi xác nhận | F057-AC-13 |
| 14 | Cấu hình hệ thống → Dọn học sinh: đặt số ngày thấp, lưu không lý do | Cảnh báo ngưỡng thấp; thiếu lý do bị chặn | F057-AC-14 |
| 15 | Giả lập lần dọn > ngưỡng → Lần dọn chờ xác nhận → Xác nhận | Trạng thái `approved` | F057-AC-15 |
| 16 | Thời hạn chat gameplay = dưới tối thiểu | Ô chặn; API trả `RETENTION_BELOW_MINIMUM` hiển thị theo `code` | F057-AC-16 |
| 16b | Cấu hình hệ thống → Giờ giới nghiêm platform: xem giá trị mặc định; nhập `25:00`; đổi 21:00–06:30 không lý do; rồi có lý do và xác nhận | Nhãn "mặc định"; lỗi tại ô; bị chặn; lưu thành công, audit + `platform_settings.changed`; `getSchoolSettings` của TH Demo A trả `platform_curfew` mới | F057-AC-23 |
| 17 | Tenant: tạo tenant mã trùng (sau delta T004) | `409` hiển thị; tạo mã mới thành công | F057-AC-17 |
| 18 | `ad.truong.a`, `cm.noidung`, `sm.antoan` mở `/portal/platform-admin/trash` | Trang 403; không tải chunk D | F057-AC-18 |
| 19 | Bật OTP cho `sa.admin`, dùng token không MFA; tắt gateway nội bộ | Step-up MFA rồi quay lại URL; "Cần kết nối mạng quản trị (VPN)" | F057-AC-19 |
| 20 | Chặn mạng giữa lúc gán rồi thử lại | Hai request cùng `Idempotency-Key`; một assignment | F057-AC-20 |
| 21 | Dán chuỗi dài vào ô lý do, mã tenant | Bị giới hạn theo `maxLength`, có bộ đếm | F057-AC-21 |
| 22 | `sa.admin` tắt mặc định platform `action.platform_admin.tenant_data.clone`, tải lại portal | Nút "Clone giữa tenant" ẩn; `createTenantClone` → `403 FEATURE_DISABLED`; Tenant và Thùng rác không có công tắc tắt (`422 FEATURE_NOT_DISABLEABLE` nếu gọi API) | F057-AC-22 |

## 3. Lệnh test tự động

```bash
cd src/cms/frontend
pnpm --filter ops-ui test -- src/tenant
pnpm --filter portal test -- src/workspaces/platform-admin src/shared/tenancy
pnpm --filter portal exec playwright test e2e/platform
make web-lint
make web-test
```

## 4. Bằng chứng cần nộp

- Kết quả Vitest/Playwright, network log kịch bản 2, 4, 18 (chunk và header), `trace_id` kịch bản 5, 7, 9, ảnh màn hình (sáng/tối).

### Bằng chứng 2026-11-19 (triển khai, mức integration)

Chạy trong `src/cms/frontend` (tối đa 3 worker), commit `6b7a82d`. Backend là MOCK theo contract (`src/test/mock-backend.ts` + `src/test/platform-backend.ts`); Playwright với backend thật, ảnh màn hình sáng/tối và network log của gateway thật chờ môi trường compose (OQ F057-10).

| Lệnh | Kết quả |
|---|---|
| `pnpm lint` (ESLint + stylelint) · `pnpm typecheck` · `pnpm gen:check` | xanh |
| `node scripts/specs/check-features.mjs` | 93 feature, 0 lỗi, 0 cảnh báo |
| `node scripts/specs/check-portal-catalog.mjs` | 0 lỗi |
| `npx vitest run --maxWorkers=3` (toàn bộ web, trước rebase) | 43 file, 303 test pass |
| `npx vitest run --project portal src/workspaces/platform-admin src/shared/tenancy` | 9 file, 76 test pass (sau rebase: `limits.test.ts` 34 pass, thêm loại `learning_material` của F024) |
| `pnpm build` (+ `check-chunks`) | 5 chunk workspace, chunk `workspace-platform-admin` không do entry tải |

| Kịch bản | Test |
|---|---|
| 1–4 | `TenantListPage.test.tsx` › Bộ lọc tenant (cursor, "đã tắt"); `TrashPage.test.tsx` (không `X-Tenant-Id`, `tenant_ids[]`, cột Tenant); `useCrossTenantScope.test.ts`; `Catalog.test.tsx` › Import (tenant đích bắt buộc); `shell/TenantFilter.test.tsx` (F051) |
| 5–6, 20 | `AssignToTenantsDialog.test.tsx` (gán 2 tenant, gỡ gán có thông điệp lịch sử giữ nguyên, thử lại giữ `Idempotency-Key`, lỗi có `trace_id`) |
| 7–8 | `CloneWizard.test.tsx` (`succeeded` + `id_map`, `partially_failed` không tự chạy lại, không loại dữ liệu học sinh, đích ≠ nguồn); bước xem trước chờ `previewTenantClone` (OQ F057-1) |
| 9–10 | `TrashPage.test.tsx` (lý do 3–1000 + bộ đếm, `X-Tenant-Id` của bản ghi, `restorable=false`/`409`, không "Xóa vĩnh viễn") |
| 11–13 | `Catalog.test.tsx` (quốc gia có lý do, sửa gửi `If-Match` = `version` của dòng — OQ F057-3; tỉnh `status=all`, kế thừa; danh mục trường `tenant_ids[]` nhiều tenant + cột Tenant + lọc `status` — OQ F057-4; import dry-run → lỗi theo dòng → xác nhận áp dụng) |
| 14–16b | `SystemSettings.test.tsx` (cảnh báo < 60 ngày, `default_days=null`, duyệt lần dọn, "Hủy lần dọn" có lý do + thử lại giữ `Idempotency-Key` — OQ F057-2, `RETENTION_BELOW_MINIMUM`, giới nghiêm "Mặc định", "hôm sau", HH:MM, xác nhận, `If-Match`, `422`/`412` theo code) |
| 17, 21 | `TenantListPage.test.tsx` (lọc `q`/`tenant_type`, `maxlength` + bộ đếm, `409`, tắt có lý do + `If-Match`); `limits.test.ts` (mọi giới hạn = OpenAPI) |
| 18–19, 22 | `workspaceGuard.test.tsx` (403 + không tải chunk D cho `ad.truong.a`/`cm.noidung`/`sm.antoan`, MFA step-up giữ URL, VPN, tab ẩn khi action tắt, `403 FEATURE_DISABLED` tải lại quyền) |

### Bằng chứng 2026-11-20 (portal bật màn chờ backend — OQ F057-2/3/4, F059-1/2/3/5)

Chạy trong `src/cms/frontend` (tối đa 2 worker), sau rebase lên `origin/main` (`7e0a59a`). Backend là MOCK theo contract mới (`src/test/platform-backend.ts`, `src/test/platform-access-backend.ts`); backend thật chờ compose (OQ F057-10, F059-9).

| Lệnh | Kết quả |
|---|---|
| `pnpm lint` · `pnpm typecheck` · `pnpm gen:check` | xanh |
| `pnpm build` (+ `check-chunks`) | 5 chunk workspace, không chunk nào do entry tải |
| `npx vitest run --maxWorkers=2` (toàn bộ web) | 77 file, 575 test: 573 pass, 2 lỗi → sửa: `groupPermissions.test.ts` (kỳ vọng cũ `role.created`, đổi theo `action_prefix`) và `TrashPage.test.tsx` (click khi lớp loading còn `pointer-events: none` dưới tải CPU → click thường); chạy lại 2 file: 16/16 pass (thêm 1 test `memberTenantsOf`) |
| Test thay đổi (`SystemSettings`, `TenantListPage`, `Catalog`, `Users`, `AuditLogs`, `IntegrationsGuard`, `limits.test.ts`) | 7 file, 102 test pass (Users 8/8 sau khi tách test ngăn chi tiết) |
| `node scripts/specs/check-features.mjs` · `node scripts/specs/check-portal-catalog.mjs` | 93 feature, 0 lỗi, 0 cảnh báo · 0 lỗi |
