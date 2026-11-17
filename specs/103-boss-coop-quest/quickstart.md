# Quickstart: F103 — Boss battle nhiều phase và quest co-op (shared progress, bước `interdependent`)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F076, F072, F074, F101 (và F100, F090); module learning đã thêm `game` vào `x-allowed-callers` của question plan (delta R1).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a` (A, leader), `hs.binh.3a` (B) cùng party; `hs.dung.7a` (tenant khác); `ad.truong.a` (nhân sự).
- Dữ liệu mẫu (seed module game, không phải nội dung thật): boss lớp 3 hai phase (`collective_count`, `min_contribution`), `max_phase_retries` nhỏ; quest party lớp 3 `shared_steps` gồm `go_to` → `activity` → bước `interdependent` 2 role; quest `collective_count`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | A `createBossEncounter` `mode = solo` boss lớp 3; thử boss lớp 4; thử `mode = party` | `201 lobby`; `403 CHALLENGE_NOT_ELIGIBLE`; `422` | F103-AC-01 |
| 2 | Party A+B mở danh sách hoạt động (F101) | Boss/quest lớp 3 đủ điều kiện, lý do dạng mã | F103-AC-02 |
| 3 | A start boss trong lobby (F101), gửi lại cùng key | Một encounter `party`, `external_ref` giống nhau | F103-AC-03 |
| 4 | A, B gửi `boss.encounter.ready` | `boss.phase.started`; mỗi người nhận `boss.question.assigned` riêng, không có đáp án | F103-AC-04 |
| 5 | B trả lời đúng (gửi 2 lần); A trả lời sai | Một lần tính cho B; learning record B `mode = challenge`; A chỉ thấy hiệu ứng, B không biết A sai | F103-AC-05, F103-AC-09 |
| 6 | Gửi câu cho phase cũ; `hs.dung.7a` gửi `boss.answer.submit` | `BOSS_PHASE_CLOSED`; `ROOM_NOT_MEMBER` | F103-AC-06 |
| 7 | Đủ `collective_target` | `boss.phase_completed`, năng lượng giảm, phase 2 | F103-AC-07 |
| 8 | Để hết giờ phase 2 (Clock giả) | Retry với câu mới, phase 1 giữ; vượt `max_phase_retries` → `ended` | F103-AC-08 |
| 9 | Hạ boss, A không đạt `min_contribution` | `boss.encounter_completed`: B `eligible`, A không; thưởng một lần khi replay event | F103-AC-10 |
| 10 | B tắt mạng, đăng nhập thiết bị khác trong grace | B nhận lại trạng thái, đóng góp giữ | F103-AC-11 |
| 11 | A `leaveBossEncounter`; encounter tạo nhưng không ai ready quá grace | A rời, đóng góp giữ; encounter `abandoned` | F103-AC-12 |
| 12 | Start quest party (F101) | Hai `quest_progress` cùng `quest_version_id`, `party_id`; một phiên chung | F103-AC-13 |
| 13 | A hoàn thành `go_to`; B làm `activity` không đạt rồi đạt | B được tính `go_to`; `activity` của B chỉ xong khi đạt | F103-AC-14, F103-AC-16 |
| 14 | Quest `collective_count`: A 15 câu đúng, B 3 | Tổng đạt nhưng B chưa đủ `min_contribution` → B chưa được tính | F103-AC-15 |
| 15 | Bước `interdependent`: A `getQuestStepClues`; A chia sẻ; B chia sẻ thẻ của A | A chỉ thấy thẻ của mình; B nhận `quest.clue.shared`; `403` | F103-AC-20, F103-AC-21, F103-AC-22 |
| 16 | Mỗi người làm câu cuối; B ngắt quá grace trước đó | 2 attempt riêng; role của B chuyển cho A | F103-AC-23, F103-AC-24 |
| 17 | Hoàn thành quest; giải tán party ở lần chạy khác | `quest.completed` mỗi người + một `quest.party_completed`; giải tán → `failed`, áp `reset_step` | F103-AC-17, F103-AC-19 |
| 18 | Quest có bước `boss` sau khi hạ boss (kịch bản 9) | Bước `boss` xong cho B, chưa xong cho A | F103-AC-18 |
| 19 | `ad.truong.a` gọi `getBossEncounter`; fixture module `chat` import facade `startPartyActivity`; `hs.dung.7a` đọc encounter của A; thiếu `Idempotency-Key` | `403 GAME_ACCESS_STUDENT_ONLY`; `make arch-check` fail; `404`; `428` | F103-AC-25, F103-AC-26, F103-AC-27, F103-AC-28 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-contract            # provider game.internal ↔ competition; consumer learning.internal
make test-e2e                 # boss party 2 client headless
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 3 và 5, truy vấn learning record theo học sinh (kịch bản 5, 13, 16), bản ghi thưởng không trùng (kịch bản 9).
