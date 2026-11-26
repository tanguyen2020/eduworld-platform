# Quickstart: F082 — Game Studio — quản lý sticker/GIF và dịch nội dung game (`/game-admin/translations*`)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F078, F079, F118 (và F053 `BilingualField`, F051); delta T004–T007 đã có backend.
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make migrate`, `make seed`, `make up-app` (gồm `core-api` nạp module `game`, `chat` và `worker-jobs`); `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm dev`.
- Tài khoản: `sa.admin` (system_admin, chỉ mật khẩu — không MFA, ADR-0094); tài khoản seed `game_designer`/`liveops`/`content_reviewer` [Confirmed theo F079: `gd.thietke`, `lo.vanhanh`, `cm.duyet`]; `ad.truong.a`, `sm.antoan` cho kiểm chặn; học sinh `hs.an.3a` để xem sticker trong game.
- Dữ liệu: seed quest/NPC dialogue có đủ `vi`/`en`, một quest thiếu `en`; 2 tệp webp mẫu, 1 tệp vượt giới hạn, 1 GIF mẫu; reward rule đã publish để làm `unlock_rule`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `gd.thietke` mở Tài nguyên chat → Sticker pack | Danh sách pack, cột trạng thái/version; `ad.truong.a` không có menu | F082-AC-01 |
| 2 | Tạo pack, chọn `unlock_rule` = reward rule từ bộ chọn; bỏ trống alt text `en` | Gửi duyệt bị khóa, liệt kê trường thiếu | F082-AC-02 |
| 3 | Upload 2 webp + 1 tệp quá lớn; ngắt mạng giữa chừng rồi thử lại | Hai tệp `ready`; tệp lớn báo `413 MEDIA_TOO_LARGE` tại tệp; thử lại cùng `Idempotency-Key` | F082-AC-03 |
| 4 | Gửi duyệt; `gd.thietke` mở lại (không có nút Duyệt); `cm.duyet` duyệt; `lo.vanhanh` publish; rollback có lý do | Version publish chỉ đọc; release mới trong lịch sử | F082-AC-04 |
| 5 | Upload 3 GIF, gắn tag vi/en, lọc theo tag | Lưới preview tĩnh, bấm để phát | F082-AC-05 |
| 6 | Người upload thử duyệt bằng API trực tiếp; tìm ô nhập URL GIF ngoài | Server từ chối, UI hiển thị `code`; không có ô URL ngoài | F082-AC-06 |
| 7 | Xem trước pack ở `vi`/`en` | Alt text đổi theo ngôn ngữ; ảnh từ URL ký module chat | F082-AC-07 |
| 8 | Mở quest thiếu `en`, lưu nháp | `translation_status = partial`, `missing_fields` | F082-AC-08 |
| 9 | Lọc Quest Builder "Cần dịch", "Bản dịch lỗi thời" | Đúng item, huy hiệu Thiếu/Lỗi thời | F082-AC-09 |
| 10 | Điền `en`, lưu; sửa câu `vi` đã dịch, lưu | `complete` → `outdated`; drawer diff bản gốc | F082-AC-10 |
| 11 | Gửi duyệt khi còn thiếu; gửi khi chỉ lỗi thời | Bị khóa (server `422 TRANSLATION_INCOMPLETE` nếu gọi thẳng); cảnh báo rồi cho gửi | F082-AC-11 |
| 12 | Xuất XLIFF 120 item "chỉ trường thiếu" | Job `succeeded`, tải file có khóa `<config_type>/<id>/<version>/<pointer>`, link có hạn | F082-AC-12, F082-AC-15 |
| 13 | Sửa `target`, nhập `dry_run` rồi nhập thật | Dry run không tạo version; nhập thật tạo Draft mới, version publish không đổi | F082-AC-13 |
| 14 | Nhập file có khóa sai, khóa version cũ, chuỗi quá dài; file XML hỏng | Báo cáo từng dòng, job `succeeded`; file hỏng → `failed` | F082-AC-14 |
| 15 | Gọi API dịch bằng token client game; `gd.thietke` đọc job của `sa.admin` | `403 PORTAL_ACCESS_STAFF_ONLY` (không còn kiểm MFA — ADR-0094); `404` | F082-AC-16 |
| 16 | Đưa `event_id` sự kiện trường của `th-demo-a` vào `content_ids`; gọi route qua gateway public | Báo lỗi như không tồn tại; 404 tại APISIX | F082-AC-17 |
| 17 | `ad.truong.a`, `sm.antoan` mở `/game-ops/translations` | 403, network log không tải chunk `game-ops` | F082-AC-18 |
| 18 | `content_ids` 501 phần tử; tag 65 ký tự | `422 VALIDATION_FAILED` chỉ rõ field; bộ đếm ký tự trong UI | F082-AC-19 |
| 19 | `sa.admin` tắt mặc định platform `menu.game_ops.translations`; `gd.thietke` tải lại portal | Menu "Dịch nội dung game" biến mất; `exportGameTranslations` → `403 FEATURE_DISABLED`; Sticker/GIF vẫn dùng được | F082-AC-20 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make migrate-lint MOD=game
make test-contract
make web-lint && make web-test
pnpm --filter portal exec playwright test tests/e2e/game-ops/chat-assets tests/e2e/game-ops/translations
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), trace_id của bước 12–13, file XLIFF mẫu (đã ẩn nội dung nếu cần), ảnh màn hình sticker editor/thư viện GIF/trang dịch (sáng/tối), báo cáo axe.

## 5. Bằng chứng triển khai (2026-11-26)

- Kịch bản 1–20 được phủ bằng test tự động (bảng AC → test trong [tasks.md](tasks.md)); chạy tay trên stack compose (`make up` + portal) chưa làm — máy triển khai không dựng stack đầy đủ (giống F079, T037 còn mở).
- Lệnh đã chạy: `go test` unit module game/chat/identity, `go test -tags integration` (testcontainers) các test F082 và toàn bộ suite của module game/chat, Vitest các file `game-ops/__tests__` và ops-ui mới; số liệu trong báo cáo hoàn thành của CODE_PLAN.
