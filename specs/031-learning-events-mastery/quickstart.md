# Quickstart: F031 — Learning events, mastery v1, evaluation và re-evaluate

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020, F021, F030 (và F026 để publish version sửa lỗi).
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: học sinh `hs.an.3a`, `hs.binh.3a` (`th-demo-a`); `ad.truong.a` (school_admin `th-demo-a`), `ad.truong.b` (school_admin `pgd-demo`), `qlcm.truong.a` (academic_manager, scope môn Toán `th-demo-a`), `cm.noidung` (content_admin), `sa.admin`.
- Dữ liệu: activity `quiz` Toán lớp 3 (LO `MATH.G3.NUM.MUL_TABLE.2`) đã publish, gán `th-demo-a`; `mastery_model_configs` có `we-1.0` `active` (seed); lesson Toán lớp 3 `completion_rule = all_required_activities` gồm 2 activity `required` + 1 tùy chọn, đã publish, gán `th-demo-a` (US6).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` làm và nộp quiz (F030) | `learning_events` có `activity.started`, `question.answered` × n, `activity.submitted`, `activity.graded`, đủ envelope, hợp lệ schema | F031-AC-01 |
| 2 | Phát lại `activity.graded` vào consumer; ghi lại event cùng `idempotency_key` | Không thêm dòng/không đổi mastery | F031-AC-02, F031-AC-12 |
| 3 | `UPDATE`/`DELETE` `learning.learning_events` bằng role app | Lỗi quyền | F031-AC-03 |
| 4 | Chạy job partition, liệt kê partition | Có tháng hiện tại + ≥ 2 tháng kế | F031-AC-04 |
| 5 | Ghi event có payload ngoài schema (ví dụ `nickname`) | Bị từ chối | F031-AC-05 |
| 6 | `docker compose stop valkey`, `hs.binh.3a` nộp quiz | Event + evaluation + mastery vẫn ghi | F031-AC-06 |
| 7 | Truy vấn repository không `TenantScope` | 0 dòng | F031-AC-07 |
| 8 | Kiểm `objective_masteries` của `hs.an.3a` sau bước 1 | `value`, `evidence_count`, `model_version = we-1.0`; có `mastery_history` | F031-AC-08 |
| 9 | `make test MOD=learning` (golden test) | Mọi dòng §3.3 khớp | F031-AC-09 |
| 10 | `hs.an.3a` đạt 10/10 một lần trong ngày | Level ≠ `mastered` | F031-AC-10 |
| 11 | Đặt clock giả +104 ngày, `getMyProgress` | Level giảm theo decay, `value` lưu không đổi | F031-AC-11, F031-AC-15 |
| 12 | Phát chuỗi evidence đủ `n_min`/`d_min`; phát thêm sau decay | Một `learning_objective.mastered`; `progressed` mỗi lần đổi | F031-AC-13 |
| 13 | Gửi đồng thời 2 evidence cùng học sinh | Kết quả bằng tuần tự | F031-AC-14 |
| 14 | `ad.truong.a` xem `getStudentProgress` của `hs.an.3a`; `ad.truong.b` xem; `qlcm.truong.a` xem môn Tiếng Việt | `200` + audit `learning.profile_viewed`; `404`; `403 PERMISSION_DENIED` | F031-AC-16 |
| 15 | Kiểm `progress_daily` sau 2 attempt | Bộ đếm đúng, phát lại event không cộng đôi | F031-AC-17 |
| 16 | `cm.noidung` publish version sửa đáp án, `createRegradeRequest` `use_new_key` | `202 queued`; job → evaluation mới `re_evaluation`, cũ `superseded`, `attempt.re_evaluated`; attempt gốc không đổi | F031-AC-18, F031-AC-20 |
| 17 | `ad.truong.a` gọi `createRegradeRequest`; `qlcm.truong.a` cho môn ngoài scope | `403 PERMISSION_DENIED`; `403 PERMISSION_DENIED` | F031-AC-19 |
| 18 | Kiểm mastery LO bị ảnh hưởng sau bước 16 | Tính lại theo kết quả mới, event cũ còn nguyên | F031-AC-21 |
| 19 | Gửi lại cùng key; gửi yêu cầu khác khi đang `running` | Yêu cầu cũ; `409 CONFLICT` | F031-AC-22 |
| 20 | `worker learning mastery-recompute --model-version=we-1.1` (lệnh vận hành của `src/backend/cmd/worker`) | Báo cáo chênh lệch; `objective_masteries` không đổi | F031-AC-23 |
| 21 | `sa.admin` kích hoạt `we-1.1` | Bản ghi mang `we-1.1`; không có `mastered` lặp | F031-AC-24 |
| 22 | `hs.an.3a` bắt đầu activity 1 của lesson (`context_type = lesson`); `cm.noidung` publish version bài mới; `hs.an.3a` gọi `getLesson` | Có `lesson_sessions` `in_progress` ghim version cũ; `getLesson` trả version cũ, activity 1 chưa `completed`; start activity không thuộc version ghim → `422 VALIDATION_FAILED` | F031-AC-25 |
| 23 | `hs.an.3a` gọi `completeLesson` khi mới nộp activity 1 | `409 CONFLICT`, `errors[]` chỉ activity 2 | F031-AC-28 |
| 24 | Nộp activity 2 (bắt buộc cuối) | Session `completed`; một learning event và một outbox `lesson.completed` (`lesson_version_id` cũ, hợp lệ `lesson.completed.v1`, không PII); `progress_daily.lessons_completed` +1 | F031-AC-26, F031-AC-29, F031-AC-30 |
| 25 | Nộp activity tùy chọn; phát lại `activity.graded`; gọi `completeLesson` hai lần (key khác nhau) | Không event mới; `completeLesson` trả cùng `completed_at` | F031-AC-27, F031-AC-28 |
| 26 | Mở lại bài (version mới) và hoàn thành | Session mới ghim version mới; thêm đúng một `lesson.completed` cho version mới | F031-AC-27 |
| 27 | `ad.truong.a` gọi `getLesson`/`completeLesson`; `hs.an.3a` với `X-Tenant-Id` của `pgd-demo` | `403 PERMISSION_DENIED`; `403 TENANT_MISMATCH` | F031-AC-29 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make test MOD=learning               # golden test mastery
make test-integration MOD=learning   # PostgreSQL + NATS + Valkey (có kịch bản dừng Valkey)
make migrate-lint MOD=learning
make specs-lint
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo F031-AC-*, kết quả golden test (SC-001), độ trễ mastery p95 (SC-002), trace_id bước 1 và 16.

## 5. Bằng chứng US1–US3 (2026-11-20, local, testcontainers PostgreSQL + NATS JetStream thật)

| Kịch bản | Test tự động | Kết quả |
|---|---|---|
| 1 (envelope, schema) | `persistence/write` TestLearningEventEnvelopeValidation; integration TestMasteryOnGraded (`validateStoredEvents`: 6 event của attempt dựng lại envelope hợp lệ `learning-event.schema.json`, `event_id` UUIDv7) | pass |
| 2 (giao lại event) | TestMasteryOnGraded: cùng `activity.graded` giao 2 lần → `processed_events` 1 dòng mỗi consumer, history/`progress_daily` không đổi | pass |
| 3 (append-only) | TestLearningRecordAppendOnlyAndIsolated (role app/platform: UPDATE/DELETE `learning_events`, partition `learning_events_2026_11`, `mastery_history`, `attempt_answers` → permission denied) | pass |
| 4 (partition) | TestLearningPartitionsJob (clock 2027-11-15: 8 partition mới, gauge 3 tháng tới, chạy lại 0, partition mới không có quyền trực tiếp) | pass |
| 5 (payload ngoài schema) | TestLearningEventEnvelopeValidation (`nickname`, `email`, `world_position`, `chat_text` → từ chối, lỗi không chứa giá trị) | pass |
| 6 (Valkey dừng) | — | chưa có test chaos (OQ F031-1) |
| 7 (thiếu scope) | TestLearningRecordAppendOnlyAndIsolated (0 dòng ở 4 bảng; repository trong scope tenant B không thấy dữ liệu tenant A) | pass |
| 8, 9, 10, 11, 12 | domain `mastery` TestGoldenTable, TestGoldenAdditional, TestMasteredNeedsEvidenceOnDays, TestEffectiveDecay, TestWeights, TestReplay; TestMasteryOnGraded (M 0,720 → 0,837, `we-1.0`, mastered ngày 2 đúng một lần, tính lại không phát lại, decay 104 ngày → proficient, giá trị lưu giữ nguyên) | pass |
| 13 (đồng thời) | TestMasteryConcurrentEvidence (2 attempt áp song song = tuần tự: M 0,837, n 6, 2 ngày, 1 `mastered`) | pass |
| 14 (`getStudentProgress`) | TestStudentProgressAuthz (200 + 3 audit `learning.profile_viewed`; trường khác/tenant B 404; môn ngoài scope, grant theo môn thiếu `subject_id`, không quyền, token học sinh 403) | pass |
| 15 (`progress_daily`) | TestMasteryOnGraded (1 activity, 3 câu, 3 đúng; không cộng đôi) | pass |
| Không hồi quy F030 | TestAttemptAdHoc, TestAttemptRespondSubmitReview, TestAttemptPauseResumeExpire (payload `score` mới được kiểm schema) | pass |
| 16–27 (US4–US6) | — | chưa làm |

SC-001: 100% dòng golden §3.3 khớp (sai số ≤ 0,0005). SC-002: metric `eduworld_mastery_update_lag_seconds` có, chưa đo p95 trên stack thật.

Toàn bộ test tích hợp module learning (trước khi merge): `TESTCONTAINERS_RYUK_DISABLED=true GOMAXPROCS=2 go test -p 1 -tags integration -timeout 75m ./modules/learning/tests/integration/` → 78/78 pass (1169 s). Gate tĩnh: gofmt, `go vet -tags integration`, golangci-lint (default + arch) 0 issues, tenancyguard, ewlint, entlint, `archgen --check` 0 vi phạm, `routematrix check` 0 problems, facadegen/allowedcallers check, check-maxlength/enums/features/cross-features, validate-schemas, check-portal-catalog, check-problem-codes, `split-public --check`, portal `gen:check`; unit + contract learning và `cmd/internal/wiring` 36 gói ok.

## 6. Bằng chứng US4 (2026-11-20, testcontainers)

| Kịch bản | Test tự động | Kết quả |
|---|---|---|
| 16 (use_new_key, job) | `tests/integration/regrade_test.go` TestRegradeUseNewKey: 202 `queued` → job: 2 attempt, evaluation cũ `superseded`, mới `re_evaluation`, attempt/câu trả lời gốc không đổi, 2 `attempt.re_evaluated` (actor staff + outbox); TestRegradePlatformContent: `content_admin`, yêu cầu không tenant, Bypass `re_evaluation` + audit `cross_tenant.accessed`, accept_all | pass |
| 17 (quyền) | TestRegradeUseNewKey: school_admin/không quyền/môn khác → 403; tenant B, content_admin trên câu tenant → 404 | pass |
| 18 (mastery) | TestRegradeUseNewKey: `attempt.re_evaluated` → M giảm, history `re_evaluation`, event gốc giữ nguyên | pass |
| 19 (idempotency, 409, superseded) | TestRegradeUseNewKey: cùng key → cùng yêu cầu; key khác → 409; `EVALUATION_NOT_EDITABLE` (use case + trigger) | pass |
| Domain | `internal/domain/evaluation` TestRegradeActions, TestSupersede | pass |
