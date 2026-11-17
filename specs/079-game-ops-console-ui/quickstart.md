# Quickstart: F079 — Workspace A — Game Operations Console UI (world, NPC, quest builder, reward, item, event, season)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F051, F050, F077, F078 (và F074 cho progression).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản: `sa.admin` (MFA tắt ở local; bật OTP seed để thử step-up), tài khoản seed `game_designer`/`liveops`/`content_reviewer` [Proposed: `gd.thietke`, `lo.vanhanh`, `cm.duyet`]; `ad.truong.a`, `qlcm.truong.a`, `sm.antoan` cho kiểm chặn.
- Dữ liệu: seed module game (8 vùng, NPC mẫu, client manifest hiện hành).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `gd.thietke` mở `/game-ops/reward-rules` | Danh sách, menu chỉ loại có quyền, cột thumbnail/trạng thái | F079-AC-01 |
| 2 | Tạo rule, sửa; mở cùng bản ở tab khác và sửa | Autosave `If-Match`; tab cũ dừng autosave, báo xung đột | F079-AC-02 |
| 3 | Gửi duyệt; `cm.duyet` duyệt; `lo.vanhanh` publish hẹn giờ | Nút duyệt ẩn với tác giả; `Idempotency-Key` có trong request | F079-AC-03 |
| 4 | Mở tab Phiên bản, diff, rollback có lý do | Version cũ chỉ đọc; release mới trong lịch sử | F079-AC-04 |
| 5 | Quest tham chiếu `scene_ref` không có trong manifest | Lỗi đúng field, không gửi duyệt được | F079-AC-05 |
| 6 | Tạo quest 3 bước kéo-thả; hội thoại NPC graph 4 node | Lưu theo schema; preview đúng thứ tự | F079-AC-06 |
| 7 | Bộ chọn area/NPC | Chỉ giá trị từ manifest/nội dung đã publish | F079-AC-07 |
| 8 | Tạo `shop-items`, tìm trường giá tiền thật | Chỉ giá Xu | F079-AC-08 |
| 9 | Mở `/game-ops/progression`, sửa `exponent` | Biểu đồ/bảng ngưỡng từ server | F079-AC-09 |
| 10 | Bấm Mô phỏng 25 phút/ngày, 5 ngày/tuần, 8 tuần | Gọi `simulateGameConfig`, hiển thị level theo tuần | F079-AC-10 |
| 11 | Đặt ngưỡng giảm; publish thiếu `effective_from` | Lỗi field; hộp xác nhận nêu không giảm level | F079-AC-11 |
| 12 | `lo.vanhanh` chạy Event Builder "Tuần lễ Toán học" | Đủ 8 bước, form vi/en song song | F079-AC-12 |
| 13 | Bỏ bản `en`; đặt lễ hội 3 ngày | `TRANSLATION_INCOMPLETE`, lỗi độ dài; cảnh báo mốc > 3 phiên/tuần không chặn | F079-AC-13 |
| 14 | Mở Lịch sự kiện | Sự kiện Scheduled hiện đúng ngày ICT, cảnh báo chồng chéo | F079-AC-14 |
| 15 | Season track: tìm control `paid_track`; template với `allowed_params` | Không có control trả phí; lưu template | F079-AC-15 |
| 16 | Publish item mới, xem danh sách; render lỗi giả lập | Thumbnail đúng version; placeholder + nhãn | F079-AC-16 |
| 17 | Bấm Preview quest nháp | `GamePreviewFrame` tải Godot Web trong iframe | F079-AC-17 |
| 18 | `ad.truong.a`, `qlcm.truong.a`, `sm.antoan` mở `/game-ops/quests` | Trang 403, không tải chunk | F079-AC-18 |
| 19 | Token không MFA; tắt gateway nội bộ | Step-up quay lại URL; "Cần kết nối mạng quản trị (VPN)" | F079-AC-19 |
| 20 | Dán chuỗi dài vào tiêu đề; ngắt mạng lúc publish rồi thử lại | Giới hạn + bộ đếm; cùng `Idempotency-Key` | F079-AC-20 |
| 21 | `sa.admin` tắt mặc định platform `menu.game_ops.shop`; `lo.vanhanh` tải lại portal | Menu "Shop · Tiền tệ" biến mất; `listGameConfigs?config_type=shop-items` → `403 FEATURE_DISABLED`; `config_type=items` vẫn chạy | F079-AC-21 |

## 3. Lệnh test tự động

```bash
cd src/cms/frontend
pnpm --filter ops-ui test -- src/content-editor src/game
pnpm --filter portal test -- src/workspaces/game-ops
pnpm --filter portal exec playwright test tests/e2e/game-ops
make web-lint
make web-test
```

## 4. Bằng chứng cần nộp

- Kết quả Vitest/Playwright; network log kịch bản 2, 3, 18, 20; ảnh màn hình sáng/tối quest builder, Event Builder, progression; `trace_id` kịch bản 3, 4, 12.
