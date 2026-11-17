# Quickstart: F120 — Channel bền vững (world, area, class, study group, team, DM), club, chính sách theo cấp, tìm kiếm, retention

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F115, F116, F117, F118, F095, F040, F010, F011, F015.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); seed module chat có chat policy mặc định platform.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (TH Demo A), `hs.dung.7a`, `hs.em.4a` (`pgd-demo`), `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a`, `sa.admin`.
- Công cụ: `nats` CLI để phát event mẫu trong `src/backend/modules/chat/tests/fixtures/events/` (dữ liệu giả, không từ nhạy cảm thật — R15).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Chạy `make seed` (SIS sync lớp 3A, 3B) rồi `hs.an.3a` gọi `listMyChannels` | Có channel `class` của 3A | F120-AC-01 |
| 2 | Phát `enrollment.changed` chuyển `hs.chi.3b` sang 3A; `hs.chi.3b` gọi lịch sử channel 3B | `404`; thấy channel 3A từ thời điểm vào | F120-AC-02, F120-AC-04 |
| 3 | `ad.truong.a` tạo nhóm học tập 5 học sinh (F040) | Channel `study_group` có 5 thành viên; xóa nhóm → `archived` | F120-AC-03 |
| 4 | `ad.truong.b` (nhân sự) và `hs.dung.7a` gọi `listChannelMessages` của channel 3A | `404`; test repository bỏ tenant → 0 dòng | F120-AC-05 |
| 5 | `ad.truong.a` `putChatPolicy` cấp `school` thu hẹp giờ chat, có lý do | `200`, version tăng, audit before/after; học sinh nhắn ngoài giờ → `CHAT_NOT_ALLOWED` | F120-AC-06, F120-AC-10 |
| 6 | Gửi rule rate cao hơn platform; rule có `grade_band`; rule `party.enabled=false` | `422` lần lượt `CHAT_POLICY_RELAX_NOT_ALLOWED`, `VALIDATION_FAILED`, `CHAT_POLICY_ALWAYS_ON` | F120-AC-07, F120-AC-08, F120-AC-09 |
| 7 | `ad.truong.a` `putChatPolicy` cho trường THCS Demo B; cấp `platform` | `404`/`403`, audit `authz.denied` | F120-AC-11 |
| 8 | `hs.an.3a` gửi World; `hs.binh.3a`, `hs.dung.7a` lắng nghe WS | Chỉ `hs.binh.3a` nhận | F120-AC-12 |
| 9 | Gửi Area với `room_id` khác instance đang đứng | Bị từ chối | F120-AC-13 |
| 10 | `hs.an.3a` DM `hs.chi.3b` khi chưa là bạn; kết bạn rồi gửi; `hs.chi.3b` block | `CHAT_NOT_ALLOWED` → gửi được → channel `frozen` | F120-AC-14, F120-AC-15 |
| 11 | Quét OpenAPI/AsyncAPI và `GET` channel | Không có endpoint voice; `capabilities.voice=false` | F120-AC-16 |
| 12 | Đặt `student_club_creation = requires_approval` cho TH Demo C; `hs.em.4a` `createClub`; mời trước khi duyệt | `pending_approval`; `409 CLUB_NOT_APPROVED`; `club_type=study_group` → `422` | F120-AC-17 |
| 13 | School Admin trường đó `approveClub` | `active`, có `chat_channel_id` (team cố định) | F120-AC-18 |
| 14 | Thêm thành viên vượt giới hạn; mời `hs.dung.7a` khác trường khi tenant chưa bật | `CLUB_FULL`/`CLUB_LIMIT_REACHED`; `SOCIAL_SCOPE_VIOLATION` | F120-AC-19 |
| 15 | Thành viên rời club; archive club | Rời channel; channel `read_only` | F120-AC-20 |
| 16 | Tạo goal `activities_completed` = 3; phát 3 `activity.graded` (1 trùng `event_id`) | Tiến độ = 2 sau trùng, = 3 sau event thứ 3, phát `club.goal_completed` | F120-AC-21 |
| 17 | `hs.an.3a` tìm "nhan vat" | Khớp tin "nhân vật" của channel mình; không tin thu hồi/bị chặn/World | F120-AC-22, F120-AC-23 |
| 18 | Seed tin channel lớp quá hạn (1 tin thuộc case mở) → chạy job `persistent-chat-retention` | Nội dung ẩn danh hóa, 0 dòng xóa; tin thuộc case có trong `moderation_evidence` | F120-AC-24, F120-AC-25 |
| 19 | `ad.truong.a` chọn preset `quick_message_emote_only` cho TH Demo A; thử gắn preset cho riêng khối 3 | Lưu + audit, hiệu lực ≤ 60 s; theo khối → `422` | F120-AC-26 |
| 20 | `ad.truong.a` đặt `allowed_hours` channel `class` 06:00–22:00 | `422 CHAT_POLICY_RELAX_NOT_ALLOWED` | F120-AC-27 |
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
