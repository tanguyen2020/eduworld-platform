# Quickstart: F030 — Activity Registry và Attempt API (`quiz`, `interactive_exercise`)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F022, F025, F026 (và F027 cho câu có công thức).
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events`, `make up-app EXCEPT=core-api` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: học sinh `hs.an.3a`, `hs.binh.3a` (`th-demo-a`), `hs.dung.7a` (`pgd-demo`); `cm.noidung` (content_admin); `ad.truong.a` (school_admin, token portal). Token game lấy qua `/api/v1/auth/game/login`.
- Dữ liệu: activity `quiz` 10 câu Toán lớp 3 đã publish và gán `th-demo-a`; activity `quiz` có `time_limit_seconds`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` `startActivityAttempt` (Idempotency-Key) | `201`, `in_progress`, items có `question_version_id`, không có trường đáp án | F030-AC-01, F030-AC-02 |
| 2 | Gọi lại cùng key; gọi start khác key khi attempt còn dở | Cùng `attempt_id` cả hai lần | F030-AC-04, F030-AC-05 |
| 3 | Start không `Idempotency-Key`; start liên tục vượt giới hạn | `428`; `429 RATE_LIMITED` | F030-AC-05 |
| 4 | `hs.dung.7a` start activity của `th-demo-a`; start activity đã unpublish | `404`; `409 CONTENT_NOT_PUBLISHED` | F030-AC-03 |
| 5 | Token portal `ad.truong.a` gọi start; `hs.binh.3a` đọc attempt của `hs.an.3a` | `403 GAME_ACCESS_STUDENT_ONLY`; `404 ATTEMPT_NOT_FOUND` | F030-AC-06 |
| 6 | Trả lời câu 1 đúng kèm `"score": 100` | `is_correct`, `score` theo server; trường client bị bỏ qua | F030-AC-08, F030-AC-09 |
| 7 | Trả lời `response` sai kiểu; `position = 99`; trả lời câu 1 quá số lượt | `422 INVALID_RESPONSE`; `422 VALIDATION_FAILED`; `409 ANSWER_LIMIT_REACHED` | F030-AC-10 |
| 8 | Trả lời 3 câu, `:pause`, đóng game, `getActivityAttempt` | `paused_at`; 3 `saved_responses`, đúng version ghim | F030-AC-16, F030-AC-17 |
| 9 | Trả lời đủ, `:submit` 2 lần cùng key, 1 lần khác key | Response giống nhau; `409 ATTEMPT_ALREADY_SUBMITTED`; outbox `activity.submitted` + `activity.graded`; 4 loại learning event | F030-AC-12, F030-AC-13 |
| 10 | Trả lời sau khi đã nộp | `409 ATTEMPT_NOT_IN_PROGRESS` | F030-AC-11 |
| 11 | `getActivityAttemptReview` | Item có `explanation`, `feedback` phương án đã chọn | F030-AC-15 |
| 12 | `cm.noidung` publish v2 activity; `hs.an.3a` xem lại attempt cũ và làm lại | Attempt cũ giữ v1; attempt mới ghim v2 | F030-AC-14 |
| 13 | Start quiz tính giờ, trả lời 1 câu, chờ quá hạn (clock giả) | `expired` → `graded`, `submit_reason = timeout`; trả lời sau hạn `410`; nộp `422 ATTEMPT_DEADLINE_PASSED` | F030-AC-19 |
| 14 | Để attempt practice quá `attempt_resume_window` (clock giả) | `abandoned`, event `activity.abandoned`; gọi tiếp `410 ATTEMPT_EXPIRED` | F030-AC-18 |
| 15 | `listMyActivityAttempts?status=graded` | Chỉ attempt của mình | F030-AC-07 |
| 16 | `cm.noidung` tạo activity `speaking`; `quiz` có `config` sai | `422 ACTIVITY_TYPE_NOT_SUPPORTED`; `422 ACTIVITY_CONFIG_INVALID` | F030-AC-20, F030-AC-21 |
| 17 | `listActivityTypes` | `quiz`, `interactive_exercise` có `min_client_version`, `schema_url` | F030-AC-22 |
| 18 | `make test MOD=learning` (contract plugin); xóa đăng ký plugin `quiz` và khởi động | Contract test xanh; `/health/ready` báo lỗi | F030-AC-23 |
| 19 | `ad.truong.b` sửa activity của `th-demo-a`; PATCH thiếu `If-Match` | `404`; `428` | F030-AC-24 |
| 20 | Gọi facade `createAttempt` từ module `game` (mode `quest`); thêm fixture import facade từ module `chat`; `make arch-check` | Attempt tạo; `make arch-check` fail ở fixture `chat` | F030-AC-25, F030-AC-26 |
| 21 | `startAdHocAttempt` `selection` topic 10 câu; topic chỉ 5 câu | `201` 10 item; `422 QUESTION_POOL_INSUFFICIENT` | F030-AC-27 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make test MOD=learning
make test-integration MOD=learning
make test-contract          # facade test module game ↔ module learning
make migrate-lint MOD=learning
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo F030-AC-*, trace_id bước 1 và 9, kết quả contract test quét DTO (SC-001).
