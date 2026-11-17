# Quickstart: F100 — Party: tạo/tham gia/mời/kick/chuyển leader/giải tán, một party active, giới hạn

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F095 (bạn bè/block), F090 (gateway giao `ew:rt:user`), F013/F014 (token game, game session); F116 cho kịch bản kiểm duyệt tên (trước đó chạy với module chat dừng để kiểm fallback).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events`; kiểm projection bằng `make seed-check` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (th-demo-a/3A), `hs.chi.3b` (th-demo-a/3B), `hs.dung.7a` (pgd-demo), `ad.truong.a`, `sm.antoan`.
- Seed quan hệ: `hs.an.3a` ↔ `hs.binh.3a` và `hs.an.3a` ↔ `hs.chi.3b` là bạn bè (F095 hoặc seed module chat). Token game qua `gameLogin`; mở WebSocket (F090) cho từng học sinh để quan sát message.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` `createParty` (`max_members = 2`, key K1); gửi lại K1 | `201`, A leader, `party.created` trong outbox; lần 2 cùng response | F100-AC-01, F100-AC-03 |
| 2 | A `createParty` lần nữa (key K2) | `409 PARTY_ALREADY_IN_PARTY` | F100-AC-02 |
| 3 | A `createParty` với `name` 41 ký tự / `max_members = 9` | `422 VALIDATION_FAILED` chỉ rõ trường | F100-AC-04 |
| 4 | A `updatePartySettings` đặt tên vi phạm danh mục placeholder; dừng module chat rồi đặt tên khác | `422` không kèm từ khớp; lần 2 lưu tên mặc định | F100-AC-05, F100-AC-21 |
| 5 | A mời `hs.binh.3a` | `201 pending`; B nhận `party.invitation.received` ≤ 1 s | F100-AC-07 |
| 6 | A mời `hs.chi.3b` (khác lớp, lớp 3) | `403 PARTY_POLICY_VIOLATION`, có audit | F100-AC-09 |
| 7 | B block A (F095) rồi A mời lại B | `403 PARTY_INVITE_NOT_ALLOWED` (không nói block); B bỏ block | F100-AC-08, F100-AC-25 |
| 8 | A gửi 11 lời mời trong 1 phút (script) | Lời mời thứ 11 → `429 PARTY_INVITE_RATE_LIMITED`, audit | F100-AC-10 |
| 9 | B `listMyInvitations`, `acceptPartyInvitation` (gửi 2 lần) | Chỉ lời mời của B; 2 thành viên, không trùng; A nhận `party.member.joined` | F100-AC-15, F100-AC-11 |
| 10 | Mời giả lập thứ 3 khi đủ 2 người; accept lời mời quá hạn (đặt Clock) | `409 PARTY_FULL`; `410 PARTY_INVITATION_EXPIRED` | F100-AC-12, F100-AC-14 |
| 11 | A đổi `visibility = class`; bạn cùng lớp `joinParty`; đổi lại `private`, người không có lời mời `joinParty` | Vào được; `403 PARTY_POLICY_VIOLATION` | F100-AC-16 |
| 12 | A `transferPartyLeader` cho B (B offline rồi online) | Offline → `409 PARTY_MEMBER_NOT_ONLINE`; online → B leader, audit | F100-AC-19 |
| 13 | B kick A; B mời lại A ngay | A rời (`kicked`); lời mời → `429`/`403` trong `party_kick_reinvite_block` | F100-AC-18 |
| 14 | B `disbandParty` (gọi 2 lần) | `disbanded`, lời mời pending `expired`, một `party.disbanded` | F100-AC-20, F100-AC-22 |
| 15 | `hs.dung.7a` `getParty` party của A; `ad.truong.a`, `sm.antoan` gọi `getMyParty` | `404`; `403 GAME_ACCESS_STUDENT_ONLY` | F100-AC-06, F100-AC-23, F100-AC-24 |

## 3. Lệnh test tự động

```bash
make test MOD=competition
make test-integration MOD=competition
make test-contract            # competition ↔ chat moderateDisplayText
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 1 và 9, truy vấn chứng minh không có học sinh có hai membership active, ảnh audit log kịch bản 6/8/13/14.
