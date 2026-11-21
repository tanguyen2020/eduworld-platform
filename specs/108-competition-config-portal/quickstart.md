# Quickstart: F108 — Cấu hình thi đấu `/game-admin/*` và UI workspace A

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F104–F107, F079 (khung workspace A), F051 (portal shell), F050 (`ops-ui`), F004 (gateway nội bộ).
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events`, `pnpm --filter portal dev` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); truy cập workspace A qua gateway nội bộ local.
- Tài khoản demo: `sa.admin` (System Admin, kiêm `competition.config.*`, không MFA — ADR-0094) — dùng thay Competition Admin; `ad.truong.a` (School Admin), `qlcm.truong.a` (Academic Manager), `ad.donvi` (Tenant Admin).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `sa.admin` gọi `listGameConfigs_competition` cho từng loại trong 7 loại | 200, có seed | F108-AC-01 |
| 2 | Tài khoản platform chỉ có `competition.config.season.write` sửa `ranking-rules`; không có `game.content.publish` gọi publish | `403 PERMISSION_DENIED`, audit `authz.denied` | F108-AC-02 |
| 3 | `ad.truong.a` gọi `/api/v1/game-admin/ranking-rules` qua gateway public; gọi thẳng backend | 404 tại APISIX; `403 PERMISSION_DENIED` (không còn kiểm MFA — ADR-0094) | F108-AC-03 |
| 4 | Lưu matchmaking cho phép ghép lớp 3 với lớp 4; ranking-rules tier giảm dần; leaderboard scope `class` metric `xp`; template có `allowed_params` `reward`; bật bonus tốc độ khối 3 | `422 VALIDATION_FAILED` chỉ rõ trường | F108-AC-04 |
| 5 | PATCH version đã publish; tạo nháp từ `based_on_version_id`; publish `effective_from` quá khứ; rollback không lý do / về version chưa từng publish | `409 CONTENT_NOT_EDITABLE`; nháp mới; `422`; `422`/`422 ROLLBACK_TARGET_INVALID` | F108-AC-05 |
| 6 | PATCH không `If-Match`; với `If-Match` cũ | `428 PRECONDITION_REQUIRED`; `412 CONTENT_VERSION_CONFLICT` | F108-AC-06 |
| 7 | Publish 2 lần cùng `Idempotency-Key` | Một release, một `competition_config.published`, audit | F108-AC-07 |
| 8 | Publish mùa mới và leaderboard definition `effective_from` +2 phút (đồng hồ giả) | `seasons`/`leaderboard_definitions` cập nhật đúng thời điểm; trận đang chạy giữ version | F108-AC-08 |
| 9 | `ad.truong.a` mở `/game-ops/competition`; `sa.admin` mở | Trang 403, không tải chunk `game-ops`; thấy menu "Thi đấu" | F108-AC-09 |
| 10 | `sa.admin` dùng UI: tạo nháp ranking-rules, gửi duyệt, duyệt, publish, so sánh version, rollback có lý do | Mọi thao tác thành công, lịch sử release đủ | F108-AC-10 |
| 11 | Nhập tier giảm dần trên UI; ngắt gateway nội bộ | Lỗi trường từ server + `trace_id`; thông báo "Cần kết nối mạng quản trị (VPN)" | F108-AC-11 |
| 12 | `make web-lint`, axe trên màn danh sách/editor | 0 hardcode style; 0 lỗi serious/critical | F108-AC-12 |
| 13 | `qlcm.truong.a`, `ad.donvi` đăng nhập portal; mở `/school-lms/challenges` | Không có menu; trang 403 | F108-AC-13 |
| 14 | `ad.truong.a` mở wizard, chọn template "Giải Toán lớp", lớp 3A, lịch; bấm tạo 2 lần nhanh | Chỉ trường `allowed_params`; chỉ lớp trường mình; 1 Challenge | F108-AC-14 |
| 15 | Mở chi tiết giải: đăng ký hộ 3A, xác nhận, tạo bracket, điều chỉnh nút (thiếu lý do → nút khóa), hủy giải khác | Bracket có tên thật học sinh 3A; mỗi thao tác gọi đúng operationId | F108-AC-15 |
| 16 | `sa.admin` vào workspace C không chọn tenant | Nút tạo bị khóa; chọn `th-demo-a` → gửi `X-Tenant-Id` đúng | F108-AC-16 |
| 17 | `sa.admin` tắt `menu.school_lms.challenges` cho `th-demo-a`; `ad.truong.a` tải lại portal | Menu "Thử thách lớp / giải trường" biến mất; `createChallenge` → `403 FEATURE_DISABLED`; cấu hình thi đấu ở workspace A không bị ảnh hưởng | F108-AC-17 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=competition && make lint MOD=competition
make test MOD=competition && make test-integration MOD=competition
make web-lint && make web-test
pnpm --filter portal exec playwright test e2e/game-ops-competition.spec.ts e2e/school-lms-challenges.spec.ts
```

## 4. Bằng chứng cần nộp

- Số test pass/fail Go (`config`) và web; báo cáo Playwright + axe; ảnh màn danh sách/editor/wizard; trace_id kịch bản 5, 7, 14.
