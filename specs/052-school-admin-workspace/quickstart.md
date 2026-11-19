# Quickstart: F052 — Workspace C — quản trị trường: trường/lớp/học sinh, SIS, người dùng và phân quyền, cài đặt, audit

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F011, F015, F016, F051 (và F010, F012 của chúng); delta contract T004–T007 đã có backend.
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make up-app`; `cd src/cms/frontend && pnpm gen:api && pnpm dev`.
- Tài khoản demo: `ad.truong.a` (school_admin TH Demo A), `ad.truong.b` (school_admin THCS Demo B), `ad.donvi` (tenant_admin `pgd-demo`), `qlcm.truong.a` (academic_manager), `sa.admin` (system_admin); học sinh `hs.an.3a`, `hs.sapbidon`.
- File mẫu: CSV theo [EduWorld SIS Contract](../sis/eduworld-sis-contract.v1.yaml), một dòng trùng username với học sinh tenant khác.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.donvi` mở Trường, lọc `pending_review` | Danh sách trường `pgd-demo` theo trạng thái | F052-AC-01 |
| 2 | Sửa tên tiếng Anh, chọn tỉnh; thử chọn tỉnh `inactive` | Chỉ hiện tỉnh `active`; lưu kèm `If-Match`; trường SIS chỉ đọc | F052-AC-02 |
| 3 | Duyệt trường `pending_review` chưa có tỉnh | Nút Duyệt khóa đến khi chọn tỉnh; sau duyệt → `active` | F052-AC-03 |
| 4 | `ad.truong.a` mở lớp 3A | Sĩ số, học sinh nhãn "SIS", không có nút sửa/gán lớp | F052-AC-04 |
| 5 | Mở "Học sinh chưa có lớp" | Danh sách `missing_class` + hướng dẫn, không nút gán | F052-AC-05 |
| 6 | Mở hoạt động đăng nhập `hs.an.3a` | Có vào/ra, nền tảng, lý do; không IP/`ip_hash`/mã thiết bị | F052-AC-06 |
| 7 | `ad.truong.b` dán URL lớp 3A của TH Demo A | "Không tìm thấy" | F052-AC-07 |
| 8 | `ad.truong.a` tải CSV mẫu; bấm Tải hai lần | Một import duy nhất (cùng Idempotency-Key); tiến trình tới `completed_with_issues` | F052-AC-08, F052-AC-09, F052-AC-11 |
| 9 | Mở "Xung đột đồng bộ", xử lý `duplicate_identifier` có lý do | Issue chuyển `resolved`; audit có bản ghi | F052-AC-10 |
| 10 | `ad.donvi` mở Người dùng | Học sinh chỉ đọc nhãn "SIS", không gán role | F052-AC-12 |
| 11 | Thêm nhân sự với email `AD.A@thdemo.edu.vn` | `409 EMAIL_TAKEN` tại trường email + gợi ý gán role cho tài khoản sẵn có | F052-AC-13 |
| 12 | Tạo role tùy biến; gọi API trực tiếp thêm `game.quest.write` | Picker không có permission platform; API → lỗi tại permission | F052-AC-14 |
| 13 | Mở role `school_admin` | Chỉ đọc + "Nhân bản thành role tùy biến"; xóa role đang dùng → gợi ý thu hồi | F052-AC-15 |
| 14 | Gán `academic_manager` scope Môn Toán cho `qlcm.truong.a`; `ad.truong.a` thử gán ở trường B | Không có phạm vi Lớp; scope trường khác → `403 PERMISSION_DENIED` | F052-AC-16 |
| 15 | Thu hồi UserRole vừa gán, mở Nhật ký phân quyền | Thông báo hiệu lực theo `principal_cache_ttl`; dòng `user_role.*` có trước/sau, lý do | F052-AC-17, F052-AC-18 |
| 16 | `ad.donvi` đặt số ngày dọn thấp; đặt thời hạn chat gameplay dưới tối thiểu | Cảnh báo; `422 RETENTION_BELOW_MINIMUM` tại trường | F052-AC-19, F052-AC-20 |
| 17 | Mở "Học sinh sắp bị dọn", loại trừ `hs.sapbidon` có lý do | Học sinh có nhãn loại trừ | F052-AC-21 |
| 18 | `ad.truong.a` tắt bảng xếp hạng lớp ở Cài đặt trường | Lưu thành công kèm `If-Match` | F052-AC-22 |
| 18b | `ad.truong.a` mở Cài đặt trường → khối "Giờ giới nghiêm" | Thấy giờ platform (chỉ đọc), khung trường trống, giờ hiệu lực = giờ platform; không có lựa chọn theo khối | F052-AC-29 |
| 18c | `ad.truong.a` nhập khung 22:00–06:30 (hẹp hơn platform); sau đó 21:00–07:00 có lý do; gửi thẳng API khung hẹp bằng curl | Lỗi tại ô, nút Lưu khóa; lưu thành công, giờ hiệu lực 21:00–07:00; API trả `422 VALIDATION_FAILED` trỏ `curfew_window.start` | F052-AC-30 |
| 18d | `qlcm.truong.a` mở Cài đặt; `ad.truong.b` gọi `getSchoolSettings` của TH Demo A | Không có menu/ô sửa; `404` | F052-AC-29 |
| 19 | `ad.donvi` lọc Nhật ký theo action, 7 ngày | Danh sách có `trace_id`, không có mục moderation | F052-AC-23 |
| 20 | `sa.admin` mở workspace C chưa chọn tenant | Yêu cầu chọn đúng một tenant | F052-AC-24 |
| 21 | `qlcm.truong.a` mở Người dùng/Role | Không có nút ghi; gọi thẳng API ghi → thông điệp không có quyền | F052-AC-25 |
| 22 | Dán chuỗi dài hơn giới hạn vào Username, Lý do | Bị chặn, bộ đếm ký tự | F052-AC-26 |
| 23 | Thu nhỏ < 768 px | Danh sách dạng thẻ; form role/import → "cần màn hình lớn hơn" | F052-AC-27 |
| 24 | `sa.admin` tắt `menu.school_lms.audit_logs` cho `th-demo-a`; `ad.truong.a` tải lại portal và mở URL Nhật ký | Menu "Nhật ký" biến mất; URL trực tiếp → trang "Tính năng đang tạm tắt"; API `listTenantAuditLogs` → `403 FEATURE_DISABLED`; mục "Người dùng & phân quyền" không có công tắc tắt | F052-AC-28 |

## 3. Lệnh test tự động

```bash
make web-lint
make web-test
cd src/cms/frontend && pnpm --filter portal test:e2e -- --grep @F052
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), báo cáo axe; ảnh màn kịch bản 6, 11, 14; `trace_id` của kịch bản 8 và 9.

## 5. Bằng chứng (2026-11-20)

Chạy tự động trong worktree `f052-school-admin` (Windows, 2 worker Vitest), commit `3771c3f`. Backend = **MOCK** (`src/test/school-lms-mock.ts`, gắn nhãn R17) theo `specs/openapi/eduworld-api.v1.yaml`; kịch bản với backend local thật, ảnh màn và `trace_id` thật chờ OQ F051-9 và backend ba operation mới (Câu hỏi mở F052-1).

| Lệnh | Kết quả |
|---|---|
| `npx vitest run --maxWorkers=2` (toàn bộ `src/cms/frontend`, trước rebase cuối) | 53 file, 391 test pass (1 test F051 `catalogMenus` sửa điều kiện chờ thông báo/trang tắt — thứ tự hiển thị đổi khi màn Lớp có thật) |
| `vitest … apps/portal/src/workspaces/school-lms` | 10 file, 87 test pass (sau tách test gán/thu hồi UserRole) |
| `vitest … i18n, school-lms/i18n, common, students, catalogMenus` (sau rebase cuối) | 5 file, 45 test pass |
| `pnpm lint` (ESLint + stylelint) · `pnpm typecheck` · `pnpm build` (check-chunks) | xanh; chunk `workspace-school-lms` riêng |
| `pnpm gen:check` | 2 file sinh khớp spec gộp; menu-catalog khớp |
| `node scripts/specs/check-features.mjs` · `check-cross-features.mjs` · `check-portal-catalog.mjs` · `check-maxlength.mjs` · `check-problem-codes.mjs` · `check-enums.mjs` · `validate-schemas.mjs` · `split-public.mjs --check` · `sync-platform-ops.mjs --check` | 0 lỗi |
| `go run ./tools/routematrix check` (GOMAXPROCS=2) | 535 operation, 0 problem (ma trận sinh lại có 3 operation mới) |
| `go test ./pkg/httpx/featuregate/ ./pkg/httpx/` | ok |
| `node scripts/docs/check-links.mjs` | 0 link hỏng |

Kịch bản §2 ↔ test: 1–3 `schools.test.tsx`; 4–7 `classes.test.tsx`, `students.test.tsx`; 8–9 `sis.test.tsx`, `sisFlow.test.ts`; 10–15 `access.test.tsx`; 16–18d `settings.test.tsx`; 19, 24 `audit.test.tsx`; 20 `schools.test.tsx`; 21 `sis.test.tsx`/`access.test.tsx`/`settings.test.tsx` (người chỉ đọc); 22 `fieldSchemas.test.ts`, `access.test.tsx`; 23 `classes.test.tsx`, `sis.test.tsx`. Chưa chạy: Playwright `@F052`, axe toàn trang (SC-005), visual baseline của `school-lms.stories.tsx` (`make web-visual` cần duyệt ảnh).
