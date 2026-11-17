# Quickstart: F122 — Portal an toàn chat: kiểm duyệt (E), danh mục và công tắc (D), chính sách chat trường (C)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F051, F050, F116, F120, F121.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`; `cd src/cms/frontend && pnpm install && pnpm gen:api && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản: `sm.antoan`, `sa.admin` (bật OTP seed để thử step-up), `ad.truong.a`, `ad.truong.b`, `ad.donvi`, `qlcm.truong.a`, `cm.noidung`; học sinh `hs.an.3a`, `hs.binh.3a`, `hs.em.4a` để sinh case/club.
- Dữ liệu: 2 case (p1 TH Demo A, p2 `pgd-demo`) tạo bằng kịch bản F121; danh mục placeholder seed module chat.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `sm.antoan` mở `/live-ops/moderation/cases`, lọc p1, 2 tenant | Danh sách có cột Tenant, SLA; chỉ thấy menu Kiểm duyệt | F122-AC-01, F122-AC-24 |
| 2 | Mở case p1 | Panel bằng chứng (gốc, revision), lịch sử; ô tìm chỉ trong case | F122-AC-02 |
| 3 | Nhận case, ẩn tin, áp `restrict_text` có lý do, đóng | Network: `Idempotency-Key`, `If-Match`, `X-Tenant-Id` = tenant case | F122-AC-03 |
| 4 | Mở cùng case ở tab khác, thao tác với ETag cũ; áp `account_lock` bằng `sm.antoan` | Thông báo theo `code` + `trace_id`, tải lại; `403` hiển thị "cần trưởng ca" | F122-AC-04 |
| 5 | "Gửi thông báo trường" cho TH Demo A; mở case `hs.tudo` | Chỉ chọn học sinh trường đó; case tenant chung không có nút | F122-AC-05 |
| 6 | Mở "Thông báo nhà trường", lọc `overdue` | Danh sách đúng trạng thái | F122-AC-06 |
| 7 | `sa.admin` mở `/platform-admin/chat-safety/catalog`, thêm term placeholder, xóa mềm một term | Validate schema; xóa có xác nhận | F122-AC-07 |
| 8 | Thử chuỗi; publish có `change_summary` | Quyết định + `masked_text`; version mới có chỉ số hồi quy | F122-AC-08 |
| 9 | `sm.antoan` mở danh mục; kiểm `localStorage` và log console | Chỉ đọc; không có nội dung term trong storage/log | F122-AC-09 |
| 10 | `sa.admin` mở Công tắc, lọc TH Demo A | Dòng Platform + tenant, 6 công tắc; bước cấp trên tắt bị khóa | F122-AC-10 |
| 11 | Tắt `spam` của TH Demo A không lý do, rồi có lý do + `expires_at` 2 giờ | Bị chặn; xác nhận rủi ro; yêu cầu được gửi | F122-AC-11 |
| 12 | Mở Lịch sử | Before/after, lý do, actor | F122-AC-12 |
| 13 | `ad.truong.a`, `sm.antoan`, `sa.admin` (lọc TH Demo A) mở bất kỳ màn | Banner đỏ "Đang tắt kiểm duyệt: spam — đến …"; tua quá hạn → mất ≤ 60 s | F122-AC-13 |
| 14 | `ad.truong.b` mở portal | Không banner; banner của tenant A không có lý do | F122-AC-14 |
| 15 | `sm.antoan` chọn `pgd-demo`, thêm term bổ sung, publish với mã yêu cầu | Thành công; thiếu mã yêu cầu bị chặn | F122-AC-15 |
| 16 | Bật/tắt category `allow_tenant_toggle=true` và `false` | Được / công tắc khóa | F122-AC-16 |
| 17 | `ad.donvi` mở `/school-lms/safety/categories` | Chỉ đọc, không nội dung term | F122-AC-17 |
| 18 | `ad.truong.a` mở `/school-lms/safety/notifications` | Thấy thông báo trường mình, không có nội dung chat, badge số | F122-AC-18 |
| 19 | Xác nhận đã liên hệ; xác nhận lần 2 | Lưu; "Thông báo đã được xác nhận" | F122-AC-19 |
| 20 | Mở bước 18 trên viewport 390 px | Dạng thẻ + màn chi tiết | F122-AC-20 |
| 21 | `ad.truong.a` mở Chat policy, tắt DM, thu hẹp giờ, lưu có lý do | `200`; giá trị kế thừa hiển thị nhãn | F122-AC-21 |
| 22 | Thử nới rate vượt platform; tìm control `match` | Thông báo `CHAT_POLICY_RELAX_NOT_ALLOWED` tại control; `match`/`team`/`party` khóa "Luôn bật"; không có control theo khối | F122-AC-22 |
| 23 | `ad.truong.a` duyệt club `pending_approval` của `hs.em.4a` (trường C — dùng School Admin tương ứng) | `active`; không có màn chat club | F122-AC-23 |
| 24 | `ad.truong.a`, `qlcm.truong.a`, `cm.noidung` mở `/live-ops/moderation/cases`, `/platform-admin/chat-safety/switches` | Trang 403, không tải chunk | F122-AC-24 |
| 25 | Token không MFA; tắt gateway nội bộ | Step-up rồi quay lại; "Cần kết nối mạng quản trị (VPN)" | F122-AC-25 |
| 26 | Dán chuỗi dài vào lý do/tóm tắt/ghi chú; ngắt mạng giữa lúc xác nhận rồi thử lại | Bị giới hạn có bộ đếm; cùng `Idempotency-Key` | F122-AC-26 |
| 27 | `sa.admin` gửi yêu cầu tắt `spam`; `sa.admin` mở danh sách chờ; `sa.admin2` phê duyệt | "Chờ phê duyệt"; người đề xuất không có nút duyệt; sau duyệt công tắc hiệu lực; PII/grooming "Luôn bật" | F122-AC-27 |
| 28 | `ad.truong.a` chọn preset `quick_message_emote_only` cho trường có lý do | Không có lựa chọn theo khối/lớp; lưu thành công | F122-AC-28 |
| 29 | `sa.admin` tắt mặc định platform `menu.live_ops.tenant_terms`; `sm.antoan` tải lại portal; `sa.admin` thử tắt `menu.live_ops.moderation_cases` | Menu "Từ bổ sung / nhóm từ của tenant" biến mất, `createTenantSensitiveTerm` → `403 FEATURE_DISABLED`; tắt hàng đợi case → `422 FEATURE_NOT_DISABLEABLE`, hàng đợi case vẫn hiện | F122-AC-29 |

## 3. Lệnh test tự động

```bash
cd src/cms/frontend
pnpm --filter ops-ui test -- src/chat-safety
pnpm --filter portal test -- src/workspaces/live-ops/moderation src/workspaces/platform-admin/chat-safety src/workspaces/school-lms/safety src/shell/banners
pnpm --filter portal exec playwright test tests/e2e/chat-safety
make web-lint
make web-test
```

## 4. Bằng chứng cần nộp

- Kết quả Vitest/Playwright; network log kịch bản 3, 13, 24; ảnh màn hình sáng/tối kịch bản 1, 7, 10, 18, 21; `trace_id` kịch bản 3, 11, 19.
