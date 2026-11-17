# Quickstart: F101 — Party Lobby, Start Session, leader/thành viên mất kết nối

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F100, F014, F090 (presence + lệnh WS), F091 (room host `private_instance`); F103 cho kịch bản `quest`/`boss` (trước đó chỉ chạy `explore`).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a` (leader A), `hs.binh.3a` (B), `hs.dung.7a` (tenant khác), `ad.truong.a`.
- Dữ liệu: A và B là bạn, cùng party (F100); seed module game có 1 quest party lớp 3 (`party_size.min = 2`) và 1 boss chỉ `grade_levels = [4]` (dữ liệu mẫu, không phải nội dung thật). Hai client WS headless (hoặc hai instance Godot với `--device-id` khác nhau).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | B gửi `party.member.ready_set {ready: true}` | A nhận `party.member.ready_changed` ≤ 1 s, `all_ready = false` | F101-AC-01 |
| 2 | B gửi 6 lệnh ready trong 1 giây | Lệnh thứ 6 → `system.command.failed` `RATE_LIMITED` | F101-AC-02 |
| 3 | A `listPartyEligibleActivities` | Quest party lớp 3 đủ điều kiện; boss lớp 4 kèm cảnh báo mã `grade_not_allowed`, không lộ khối của B | F101-AC-05 |
| 4 | B `selectPartyActivity`; A chọn với `expected_version` cũ; A chọn đúng version | `403 PARTY_NOT_LEADER`; `409 VERSION_MISMATCH`; thành công, Ready reset, cả hai nhận `party.activity.selected` | F101-AC-03, F101-AC-04 |
| 5 | A `startPartySession` khi B chưa Ready; thiếu `Idempotency-Key` | `409 PARTY_NOT_READY`; `428` | F101-AC-09 |
| 6 | A chọn boss lớp 4, cả hai Ready, start | `409 PARTY_ACTIVITY_NOT_ELIGIBLE` + 2 `student_id` lý do; không có `party_activities` mới | F101-AC-08 |
| 7 | A chọn quest party, cả hai Ready, start (key K1), gửi lại K1 | `201`, cả hai nhận `party.session.started` cùng `room_id`; lần 2 cùng response, một activity | F101-AC-07, F101-AC-10 |
| 8 | Dừng module game rồi start một party khác | `503 UNAVAILABLE` + `Retry-After`; party vẫn `open` | F101-AC-10 |
| 9 | `hs.dung.7a` gửi lệnh vào `room_id` ở bước 7 | `ROOM_NOT_MEMBER` | F101-AC-11 |
| 10 | Ngắt WS của B trong phiên, kết nối lại trong grace | A thấy B `disconnected`; B về đúng `room_id` | F101-AC-14, F101-AC-17 |
| 11 | Ngắt B quá grace (Clock giả) với quest `party_size.min = 2` | Activity kết thúc `timeout`, `party.session.ended`, party về `open` | F101-AC-18, F101-AC-12 |
| 12 | Hoàn thành quest (F103 phát `quest.party_completed`) | `party.activity_completed`; `party.session.ended` không có điểm cá nhân | F101-AC-12, F101-AC-13 |
| 13 | Ở lobby ngắt WS của A quá `party_leader_grace`; lặp lại nhưng A quay lại trong grace | B leader, `party.leader.transferred` `leader_disconnected`, audit; lần 2 A giữ leader | F101-AC-15 |
| 14 | Ngắt cả A và B quá `party_dormant_timeout` (Clock giả) | Party `disbanded`, reason `inactivity` | F101-AC-16 |
| 15 | Mất Valkey rồi `getParty` | Snapshot dựng lại, Ready = false | F101-AC-06 |
| 16 | `ad.truong.a` gọi `startPartySession`; B gửi `party.session.start` với `party_id` tenant khác | `403 GAME_ACCESS_STUDENT_ONLY`; `PERMISSION_DENIED` | F101-AC-19, F101-AC-20 |

## 3. Lệnh test tự động

```bash
make test MOD=competition
make test-integration MOD=competition
make test-contract            # consumer competition → game.internal
make test-e2e                 # lobby + start + disconnect 2 client headless
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 7 (gồm span gọi module game), đo p95 lobby của kịch bản 1, log worker kịch bản 13/14.
