# Quickstart: F120 — Channel bền vững (world, area, team, DM), club, chính sách theo cấp, tìm kiếm, retention

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F115, F116, F117, F118, F095, F010, F015.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); seed module chat có chat policy mặc định platform.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (TH Demo A, khối 3), `hs.dung.7a` (THCS Demo B, khối 7), `hs.em.4a` (TH Demo C, khối 4) (`pgd-demo`), `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`, `sa.admin`.
- Công cụ: `nats` CLI để phát event mẫu trong `src/backend/modules/chat/tests/fixtures/events/` (dữ liệu giả, không từ nhạy cảm thật — R15).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Chạy `make seed` rồi `hs.an.3a` gọi `listMyChannels`; thử tạo/gửi channel `class`/`study_group` | Không có channel `class`/`study_group`/khối; loại đó → `422` | F120-AC-01, F120-AC-02, F120-AC-03 (Removed — kiểm âm ADR-0095) |
| 2 | `hs.chi.3b` được nhận vào club "CLB Toán" của TH Demo A, đọc lịch sử; rời club rồi đọc lại; được nhận lại | Chỉ thấy tin từ `joined_at`; sau khi rời `404`; vào lại chỉ thấy tin từ `joined_at` mới | F120-AC-04 |
| 3 | Gửi lại cùng `Idempotency-Key` khi tham gia club; phát `user.blocked` hai lần | Một dòng thành viên, một lần `frozen` | F120-AC-04 |
| 4 | `ad.truong.b` (nhân sự) và `hs.dung.7a` gọi `listChannelMessages` của channel "CLB Toán" | `404`; test repository bỏ tenant → 0 dòng | F120-AC-05 |
| 5 | `ad.truong.a` `putChatPolicy` cấp `school` thu hẹp giờ chat, có lý do | `200`, version tăng, audit before/after; học sinh nhắn ngoài giờ → `CHAT_NOT_ALLOWED` | F120-AC-06, F120-AC-10 |
| 6 | Gửi rule rate cao hơn platform; rule có `grade_band`; rule `party.enabled=false`; `putChatPolicy` `scope_type = class` | `422` lần lượt `CHAT_POLICY_RELAX_NOT_ALLOWED`, `VALIDATION_FAILED`, `CHAT_POLICY_ALWAYS_ON`, `VALIDATION_FAILED` | F120-AC-06…09 |
| 7 | `ad.truong.a` `putChatPolicy` cho trường THCS Demo B; cấp `platform` | `404`/`403`, audit `authz.denied` | F120-AC-11 |
| 8 | `hs.an.3a` gửi World; `hs.binh.3a`, `hs.dung.7a` lắng nghe WS | Chỉ `hs.binh.3a` nhận | F120-AC-12 |
| 9 | Gửi Area với `room_id` khác instance đang đứng | Bị từ chối | F120-AC-13 |
| 10 | `hs.an.3a` DM `hs.chi.3b` khi chưa là bạn; kết bạn rồi gửi; `hs.chi.3b` block | `CHAT_NOT_ALLOWED` → gửi được → channel `frozen` | F120-AC-14, F120-AC-15 |
| 11 | Quét OpenAPI/AsyncAPI và `GET` channel | Không có endpoint voice; `capabilities.voice=false` | F120-AC-16 |
| 12 | Đặt `student_club_creation = requires_approval` cho TH Demo C; `hs.em.4a` `createClub`; mời trước khi duyệt; nhân sự tạo club kèm `add_whole_class` | `pending_approval`; `409 CLUB_NOT_APPROVED`; `club_type=study_group` → `422`; `add_whole_class` → `422` | F120-AC-17 |
| 13 | School Admin trường đó `approveClub` | `active`, có `chat_channel_id` (team cố định) | F120-AC-18 |
| 14 | Thêm thành viên vượt giới hạn; mời `hs.dung.7a` khác trường khi tenant chưa bật | `CLUB_FULL`/`CLUB_LIMIT_REACHED`; `SOCIAL_SCOPE_VIOLATION` | F120-AC-19 |
| 15 | Thành viên rời club; archive club | Rời channel; channel `read_only` | F120-AC-20 |
| 16 | Tạo goal `activities_completed` = 3; phát 3 `activity.graded` (1 trùng `event_id`) | Tiến độ = 2 sau trùng, = 3 sau event thứ 3, phát `club.goal_completed` | F120-AC-21 |
| 17 | `hs.an.3a` tìm "nhan vat" | Khớp tin "nhân vật" của channel mình; không tin thu hồi/bị chặn/World | F120-AC-22, F120-AC-23 |
| 18 | Seed tin channel club và DM quá hạn (1 tin thuộc case mở) → chạy job `persistent-chat-retention` | Nội dung ẩn danh hóa, 0 dòng xóa; tin thuộc case có trong `moderation_evidence` | F120-AC-24, F120-AC-25 |
| 19 | `ad.truong.a` chọn preset `quick_message_emote_only` cho TH Demo A; thử gắn preset cho riêng khối 3 | Lưu + audit, hiệu lực ≤ 60 s; theo khối → `422` | F120-AC-26 |
| 20 | `ad.truong.a` đặt `allowed_hours` channel `direct` 06:00–22:00 | `422 CHAT_POLICY_RELAX_NOT_ALLOWED` | F120-AC-27 |
| 21 | Học sinh tenant `common` mở DM, gửi `world`, gửi `text` ở `area`; `sa.admin` bật lại DM có `reason` | `CHAT_NOT_ALLOWED`; `CHAT_KIND_NOT_ALLOWED`; sau khi bật lại DM mở được, có audit | F120-AC-28 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=chat
make lint MOD=chat
make test MOD=chat
make test-integration MOD=chat
make test-contract
make migrate-lint MOD=chat
```

## 4. Bằng chứng cần nộp

- Số test pass/fail, `trace_id` kịch bản 1, 5, 13, 18; log audit `chat_policy.updated`; kết quả `persistent_retention_runs`.
