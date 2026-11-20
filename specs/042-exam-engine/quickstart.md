# Quickstart: F042 — Exam Engine: 7 hình thức đề, random theo blueprint, thi có giờ, chấm lại đề

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F024 (blueprint "Phép nhân" Toán 3 đã publish, đủ pool), F030, F031, F041.
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `qlcm.truong.a` (Academic Manager môn Toán, `th-demo-a`), `ad.truong.a` (School Admin TH Demo A), `ad.truong.b` (trường khác), `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b`, `hs.dung.7a`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `qlcm.truong.a` gọi `createExamSession` (blueprint "Phép nhân", `chapter`, 15 phút, 3A, 4 biến thể, gia hạn 25% cho `hs.binh.3a`); gọi lại cùng key | Session `scheduled`/`open`, 4 paper, seed lưu; cùng session; audit `exam_session.created` | F042-AC-01, F042-AC-02, F042-AC-05, F042-AC-07 |
| 2 | Tạo session với blueprint `feedback_timing = immediate`; `title` 300 ký tự | `422 VALIDATION_FAILED` | F042-AC-03, F042-AC-08 |
| 3 | `qlcm.truong.a` tạo đề môn Tiếng Việt; `ad.truong.b` tạo đề cho lớp 3A | `403 PERMISSION_DENIED`; `404 NOT_FOUND` | F042-AC-06 |
| 4 | `qlcm.truong.a` gọi `listExamSessionPapers`; `hs.an.3a` gọi cùng API | Thấy 4 biến thể; học sinh `403` | F042-AC-04 |
| 5 | `hs.chi.3b` start đề của 3A; `hs.an.3a` start trước `window_start` | `403 EXAM_NOT_ASSIGNED`; `403 EXAM_NOT_OPEN` | F042-AC-11 |
| 6 | `hs.an.3a` start, lưu 5 câu, kiểm payload | `deadline_at` đúng, `server_time`; payload không đáp án, media signed URL private; save không trả `is_correct` | F042-AC-09, F042-AC-10, F042-AC-12 |
| 7 | `hs.an.3a` thoát, start lại sau 2 phút; start từ thiết bị thứ hai | Cùng attempt, `deadline_at` không đổi; `409 EXAM_ACTIVE_ON_OTHER_DEVICE`, `take_over` → audit | F042-AC-13, F042-AC-14 |
| 8 | `hs.binh.3a` start (gia hạn) và để hết giờ; gửi câu trả lời sau hạn | `deadline_at` dài hơn 25%; `422 EXAM_TIME_OVER`; job tự nộp `auto_timeout` | F042-AC-05, F042-AC-15, F042-AC-16 |
| 9 | `hs.an.3a` nộp hai lần cùng key, rồi khác key | Cùng kết quả; `409 EXAM_ALREADY_SUBMITTED` | F042-AC-18 |
| 10 | `hs.an.3a` gọi `getExamResult` khi session `after_close` còn mở | `403 EXAM_RESULTS_NOT_RELEASED` | F042-AC-20 |
| 11 | Đóng cửa sổ; học sinh chưa bắt đầu | Attempt còn mở được nộp; participation `absent`; `results_released` | F042-AC-16, F042-AC-20 |
| 12 | `hs.an.3a` gọi `getExamResult` sau công bố; `getMyRecommendations` | Phân tích theo LO, câu sai có giải thích, 1–3 LO yếu; gợi ý R5 | F042-AC-19, F042-AC-22, F042-AC-23 |
| 13 | Session `manual`: `ad.truong.a` gọi `releaseExamResults` hai lần cùng key | Một event `exam.results_released`, audit | F042-AC-21 |
| 14 | Xóa Valkey (`valkey-cli FLUSHALL`) rồi đọc kết quả | Kết quả không đổi | F042-AC-24 |
| 15 | Giả lập attempt đúng 100% rất nhanh | Integrity signal `suspicious_speed`, điểm giữ nguyên | F042-AC-25 |
| 16 | `hs.dung.7a` gọi `listExamLibrary`, làm đề `practice_test` hai lần | Chỉ đề khối 7; hai bộ câu khác nhau; kết quả ngay | F042-AC-26, F042-AC-27 |
| 17 | `qlcm.truong.a` tạo `createRegradeRequest` `exam_session` `drop_item`; `ad.truong.a` gọi cùng API | Điểm mới, revision lưu điểm cũ, `exam.regraded`, audit; `403` | F042-AC-28, F042-AC-29, F042-AC-30 |
| 18 | `hs.an.3a` gọi `getExamResult` của `hs.binh.3a`; gửi `X-Tenant-Id` của `pgd-demo` | `404 NOT_FOUND`; `403 TENANT_MISMATCH` | F042-AC-17 |
| 19 | `qlcm.truong.a` tạo + publish bài giao `graded` blueprint "Phép nhân" 15 phút cho 3A (`createAssignment` → `publishAssignment`); rồi tạo bài thứ hai và `cancelAssignment` | Bài có `exam_session_id`, session ghim blueprint version, cùng đối tượng và cửa sổ; bài hủy → session `cancelled` | F042-AC-31 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make lint MOD=learning
make test MOD=learning
make test-integration MOD=learning
make migrate-lint MOD=learning
```

## 4. Bằng chứng cần nộp

- Số test pass/fail package `exam`, trace_id của kịch bản 6, 8, 17; log job `exam.autosubmit`; xác nhận log không chứa câu trả lời.

## 5. Bằng chứng (2026-11-20, local)

- Integration (testcontainers, `go test -tags integration -run TestExam ./modules/learning/tests/integration/`): `TestExamSessionCreate` (kịch bản 1–4), `TestExamAttemptFlow` (5–8, 11, 18), `TestExamResultRelease` (9–15), `TestExamLibraryPerStudent` (16), `TestExamRegrade` (17), `TestExamSessionOfAssignment` (19) — PASS; bộ integration learning đầy đủ PASS.
- Unit: `internal/domain/exam/...`, `TestCheckMatrix`, `TestMigrationMatchesEnt` PASS.
- Kịch bản 14 (Valkey): đường đọc kết quả chỉ dùng PostgreSQL (`assessment.attempts/attempt_items/evaluations`), không có cache.
- Gate: gofmt, vet, golangci (default + arch), tenancyguard, ewlint, entlint, archgen, routematrix, allowedcallers, facadegen, problemgen, check-features/cross-features/portal-catalog/problem-codes/maxlength/enums, validate-schemas, split-public, game fixtures, RTM — 0 lỗi.
