# Quickstart: F032 — Chế độ luyện tập, hỗ trợ tự học (gợi ý, chẩn đoán, `rapid_guess`), gợi ý ôn tập tự động

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F030, F031 (và nền F020–F026 có seed GDPT Toán 3 với câu có `hints[]`, `options[].feedback`, câu anh em).
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` (theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (TH Demo A), `hs.dung.7a` (khối 7, tenant `pgd-demo`), `ad.truong.a` (nhân sự, dùng thử bị chặn), `hs.tudo` (tenant chung).
- Gọi API qua `make api-test` (Postman CLI) hoặc client HTTP với token game của học sinh và `X-Tenant-Id`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` gọi `startAdHocAttempt` `{mode: practice, source: {kind: selection, subject_id, topic_id}}` | Attempt `in_progress`, không có `deadline_at`; payload không có `answer_key`, `hints`, `feedback`, `explanation` | F032-AC-01, F032-AC-04, F032-AC-05 |
| 2 | Trả lời sai câu 1 (phương án có `feedback`) | `ActivityResponseResult` có `is_correct=false`, `explanation`, `feedback` đúng phương án | F032-AC-02 |
| 3 | Thử lại câu 1 tới quá số lượt | Lần thử lại `is_scored=false`; quá lượt → `409 ANSWER_LIMIT_REACHED` | F032-AC-03 |
| 4 | Gọi lại start cùng `Idempotency-Key`; gửi body có `question_count: 99` | Cùng `attempt_id`; `422 VALIDATION_FAILED` | F032-AC-07, F032-AC-08 |
| 5 | Câu 2: `revealNextHint` ×2 rồi trả lời đúng; gọi tiếp tới quá `total_levels` | Cấp 1, cấp 2; `question.answered.hints_used = 2`; `409 HINTS_EXHAUSTED` | F032-AC-10, F032-AC-12, F032-AC-14 |
| 6 | Gửi lại `revealNextHint` cùng key; `hs.binh.3a` gọi hint trên attempt của `hs.an.3a` | Cùng cấp, không tăng; `404 NOT_FOUND` | F032-AC-13, F032-AC-15 |
| 7 | Câu 3 `single_choice` trả lời sau ~800 ms | Điểm ghi; `rapid_guess=true` trong DB; mastery LO không đổi; response không có cờ | F032-AC-16, F032-AC-17, F032-AC-18, F032-AC-20 |
| 8 | Ngày D sai một câu → `listMyWrongQuestions`; `startAdHocAttempt {kind: retry_wrong}` | Câu có `next_due_at`; attempt `retry_wrong` nhận câu anh em khác `question_id` | F032-AC-21, F032-AC-22, F032-AC-23 |
| 9 | `startAdHocAttempt {kind: selection, interleave: true, topic_ids: [t1, t2]}` | Không quá 2 câu liên tiếp cùng topic | F032-AC-24 |
| 10 | `hs.binh.3a` (chưa có evidence) làm `diagnostic` Toán 3; gọi `revealNextHint` trong bài | `403 HINTS_NOT_ALLOWED`; sau nộp LO có ≥ 2 câu nhận `prior_value`, level `not_started` | F032-AC-11, F032-AC-25, F032-AC-26, F032-AC-27 |
| 11 | `hs.an.3a` gọi `getMyRecommendations` sau bước 1–8 | ≤ 3 gợi ý, có `recommendation_id`, `rule_code`, lý do vi/en | F032-AC-29, F032-AC-30, F032-AC-31 |
| 12 | Start attempt với `recommendation_id` vừa nhận; lặp lại với id của học sinh khác | Gợi ý `started`; id lạ bị bỏ qua, attempt vẫn tạo | F032-AC-32 |
| 13 | `hs.an.3a` và `hs.dung.7a` gọi `getMyMastery` | Lớp 3 không có `value`; lớp 7 có `value`; không có nhãn TT27/TT22 | F032-AC-34 |
| 14 | `ad.truong.a` gọi `getMyRecommendations`; `hs.an.3a` gửi `X-Tenant-Id` của `pgd-demo` | `403 PERMISSION_DENIED`; `403 TENANT_MISMATCH` | F032-AC-09 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make lint MOD=learning
make test MOD=learning
make test-integration MOD=learning
make migrate-lint MOD=learning
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo package (`practice`, `recommendation`, `mastery`), kết quả golden test mastery, trace_id của kịch bản 5 và 10, log không chứa `response`/nội dung gợi ý.
