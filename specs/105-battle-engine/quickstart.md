# Quickstart: F105 — Battle engine server-authoritative và 8 chế độ thi đấu

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F104 (matchmaking), F030/F022 (grader), F090 (gateway), F014 (game session); question bank Toán lớp 3 và lớp 7 đã publish, gán cho `th-demo-a`/`pgd-demo`.
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events`, `make run-realtime` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); `EW_BATTLE_ROOM_CHECKPOINT=5s`.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (khối 3, TH Demo A), `hs.dung.7a` (khối 7, `pgd-demo`), `ad.truong.a`, `ad.truong.b`. Trận lớp 7 với 2 người, đội 3v3, Timed Quiz 20 người: bot WS + fixture `testkit`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a`, `hs.binh.3a` ghép `battle_1v1` Toán (F104), quan sát message WS | `battle.countdown.started` → 7 `battle.question.shown` cùng `shown_at`; plan ghim version; không có `answer_key`/`correct_response` trong `question.shown` | F105-AC-01, F105-AC-02 |
| 2 | Bot gửi `battle.answer.submit` hai lần cùng câu; gửi sau `deadline_at`; gửi `question_instance_id` lạ | Lần 2 `ANSWER_ALREADY_SUBMITTED`; `ANSWER_LATE`; từ chối + `integrity_signals` | F105-AC-03 |
| 3 | Kết thúc câu | `battle.answer.evaluated` (đáp án, điểm 100/0, `time_bonus = 0`) và `battle.question.revealed` tới cả hai | F105-AC-04, F105-AC-05 |
| 4 | Fixture trận lớp 7 | Câu đúng có `time_bonus` 0–50 theo giờ server; câu cuối ×2 | F105-AC-04 |
| 5 | Bot gửi thêm trường `score: 999` trong lệnh | `VALIDATION_FAILED`, điểm không đổi | F105-AC-06, F105-AC-27 |
| 6 | Hết trận; test integration gọi `RecordMatchResult` lần hai | 1 dòng `match_results`, 1 event `match.completed`, audit `match_result.recorded`; lần 2 `MATCH_RESULT_ALREADY_RECORDED` | F105-AC-07, F105-AC-08 |
| 7 | Kiểm `assessment.attempt_answers` của hai học sinh | Mỗi câu đã chấm 1 dòng `mode = competition`; câu `late` không điểm | F105-AC-09 |
| 8 | `valkey-cli FLUSHALL` sau trận rồi `getMatch` | Kết quả còn nguyên | F105-AC-10 |
| 9 | Ngắt WS `hs.binh.3a` 10 s rồi nối lại | `battle.participant.status_changed` có `grace_until`; nối lại nhận `battle.state.snapshot` đúng câu | F105-AC-11 |
| 10 | Ngắt 35 s | Trận kết thúc, `hs.binh.3a` `abandoned`, `hs.an.3a` `win`; cả hai ngắt → `cancelled` | F105-AC-12 |
| 11 | Trận đội 3v3 (fixture) ngắt 1 người 50 s; Timed Quiz ngắt 1 người | Đội tiếp tục, người đó `abandoned`; quiz giữ chỗ tới hết giờ | F105-AC-13 |
| 12 | `hs.an.3a` đăng nhập thiết bị thứ hai giữa trận (takeover) | Vào lại cùng room; phiên cũ `replaced_by_other_device` | F105-AC-14 |
| 13 | Kill instance room host khi `running`; lặp lại với checkpoint bị xóa | Room khôi phục, `replay_current_question = true`; `aborted`, không rating | F105-AC-15 |
| 14 | Fixture học sinh bỏ 3 trận trong 24 h | Dòng `competition_restrictions` `abandon_penalty` + audit; F104 ticket → `403 POLICY_DENIED` | F105-AC-16 |
| 15 | Trận `team_battle` lớp 6 (fixture) | Lượt chính xoay vòng, lượt hỗ trợ 30% | F105-AC-17 |
| 16 | Party lớp 2 vào Co-op Challenge | Mục tiêu tập thể; mỗi câu trả lời vào attempt riêng; không thứ hạng | F105-AC-18 |
| 17 | Thử thách khối Timed Quiz khối 4 (fixture, F107) và khối 2 | Khối 4: top 5 + vị trí mình; khối 2: chỉ kết quả của em + `grade_goal`, không `top[]`, không nickname người khác | F105-AC-19 |
| 18 | `startBotMatch` (F104) | Bot nhãn "Robot", `rated = false`, có attempt học tập | F105-AC-20 |
| 19 | Trận giải (F107 fixture) ngắt 2 lần > 60 s | `forfeit`, `rated = false` | F105-AC-21 |
| 20 | Kiểm `multiplayer.rooms`, `session_memberships`; bot gửi lệnh vào room không phải của mình; ticket tenant B vào room tenant A | Bản ghi đầy đủ; `ROOM_NOT_MEMBER`; `PERMISSION_DENIED` | F105-AC-22 |
| 21 | `hs.chi.3b` `getMatch` trận ở bước 1; `ad.truong.a` mở trong portal; `ad.truong.b` mở | `404`; 200 nickname + review; `404` | F105-AC-23 |
| 22 | Bot trả lời 3 câu liên tiếp nhanh hơn `cheat_fast_answer_flag` | `integrity_signals` `fast_answer`, không bonus, kết quả giữ | F105-AC-24 |
| 23 | `ad.truong.a` gọi `listMyMatches` bằng token game; `hs.an.3a` gửi `X-Tenant-Id` khác | `403 GAME_ACCESS_STUDENT_ONLY`; `403 TENANT_MISMATCH` | F105-AC-25 |
| 24 | Kiểm stream `EVT_COMPETITION` | `match.started`, `match.completed` có `results[]` + đội; không có cờ tắt chat | F105-AC-26 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning && make gen-check MOD=competition
make lint MOD=learning && make lint MOD=competition
make test MOD=competition && make test MOD=learning
make test-integration MOD=competition && make test-integration MOD=learning
make test-contract
make migrate-lint MOD=competition && make migrate-lint MOD=learning
```

## 4. Bằng chứng cần nộp

- Số test pass/fail `battle`, `questionplan`; kết quả facade test competition ↔ learning; trace_id kịch bản 1, 6, 13; log xác nhận không chứa câu trả lời; báo cáo TC-PERF-013 khi chạy tải.
