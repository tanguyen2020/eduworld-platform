# Quickstart: F041 — Giao bài của nhà trường: assignment, `ResultRelease`, gia hạn/miễn, `my-assignments`

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F030, F032, F040 (course của 3A/3B), F026 (question set Toán 3 published). Không cần F042 (kịch bản 6 kiểm hành vi khi chưa có adapter ExamSession).
- Lệnh: `make up`, `make seed`, `make seed-check`, `make run-core-api`, `make run-worker PROFILE=events` theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md).
- Tài khoản demo: `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a` (Toán), `ad.donvi`, `sm.antoan`, `sa.admin`, `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` `createAssignment` question set Toán 3, đối tượng lớp 3A, `mode = practice`, `Idempotency-Key`; gửi lại cùng key | `draft`, `assignment.created`; replay trả cùng bài | F041-AC-01, F041-AC-29 |
| 2 | Tạo bài với nội dung chưa publish; nội dung platform chưa gán; activity `written_response` | `409 CONTENT_NOT_PUBLISHED`; `403 RESOURCE_NOT_ASSIGNED`; `422 ACTIVITY_TYPE_NOT_SUPPORTED` | F041-AC-02, F041-AC-03 |
| 3 | `qlcm.truong.a` tạo bài cho course Tiếng Việt 3A; `ad.truong.a` giao cho lớp của THCS Demo B | `403 PERMISSION_DENIED`; `404 NOT_FOUND` | F041-AC-04, F041-AC-05 |
| 4 | Tạo bài `graded` không chỉ định release; `graded` + `follow_latest`; `title` 200 ký tự | Mặc định `after_close`/`after_close`; `422`; `422` | F041-AC-06, F041-AC-07 |
| 5 | `publishAssignment` bài bước 1 (`open_at` ≤ now) | `open`, `assignment.published` có đối tượng 3A, audit | F041-AC-08, F041-AC-17 |
| 6 | `qlcm.truong.a` tạo bài `graded` blueprint "Phép nhân" 15 phút cho 3A khi chưa có adapter ExamSession của F042 | `422 VALIDATION_FAILED` (`errors[].field = content_ref`), không có bài mới (tạo ExamSession: quickstart F042 bước 19) | F041-AC-09 |
| 7 | `updateAssignment` bài `open` đổi `content_ref`; đổi `due_at` với `If-Match` cũ / mới | `409 ASSIGNMENT_NOT_EDITABLE`; `412`; thành công | F041-AC-10 |
| 8 | `hs.an.3a`, `hs.chi.3b` gọi `listMyAssignments` | 3A thấy bài, 3B không | F041-AC-12 |
| 9 | Publish version mới của question set; `hs.an.3a` bắt đầu attempt `context.assignment_id`, làm, nộp | Attempt dùng version ghim; submission `submitted`, có điểm, `assignment.completed` | F041-AC-13, F041-AC-14, F041-AC-15 |
| 10 | Thêm học sinh mới vào 3A (`enrollment.changed`) khi bài `open` | Học sinh mới có trong đối tượng | F041-AC-16 |
| 11 | `hs.binh.3a` bị `extend` +2 ngày; `hs.an.3a` được `excuse`; `remindAssignment` 2 lần cùng ngày | Gia hạn có audit; `excused`; lần 2 `429 RATE_LIMITED` | F041-AC-22, F041-AC-23, F041-AC-24 |
| 12 | Đồng hồ giả qua `due_at`; bài `manual`: học sinh gọi review; gọi `releaseAssignmentResults` trước/sau khi đóng | `closed`, `missed` cho người chưa nộp (trừ excused); review không có đáp án; trước đóng `409`; sau `results_released` | F041-AC-18, F041-AC-19, F041-AC-20, F041-AC-21 |
| 13 | `qlcm.truong.a` gọi `listAssignmentSubmissions` | Danh sách học sinh với trạng thái, điểm, `rapid_guess_count` | F041-AC-25 |
| 14 | `ad.truong.b` đọc bài 3A; `ad.donvi` tạo bài; `sm.antoan`, `hs.an.3a` gọi `listAssignments`; `sa.admin` tạo bài không `X-Tenant-Id` | `404`; `403 PERMISSION_DENIED`; `403`/`403 PORTAL_ACCESS_STAFF_ONLY`; `400 TENANT_HEADER_REQUIRED` | F041-AC-26, F041-AC-27, F041-AC-28 |
| 15 | `listGameActivities` (projection rỗng, rồi nạp event mẫu theo schema F078); `hs.binh.3a` còn bài hạn < 48 giờ → `getMyRecommendations` | Danh sách rỗng rồi có quest được gán; gợi ý `R1` ưu tiên 1 | F041-AC-30, F041-AC-31 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make lint MOD=learning
make test MOD=learning
make test-integration MOD=learning
make migrate-lint MOD=learning
make test-contract
```

## 4. Bằng chứng cần nộp

- Số test pass/fail package `assignment`, trace_id kịch bản 5, 9, 12; bản ghi audit kịch bản 5, 11, 12, 14.

## 5. Bằng chứng (2026-11-20, local, Kiri)

Kịch bản chạy bằng integration test (đồng hồ giả, PostgreSQL + NATS testcontainers) thay cho stack chạy tay:

| Kịch bản | Test |
|---|---|
| 1, 2, 4, 6 | `TestAssignmentCreate` (+ unit `TestAssignmentContentActivityType` cho `written_response`) |
| 3, 14 | `TestAssignmentScope` |
| 5, 7 | `TestAssignmentPublishUpdateCancel` |
| 8, 9 | `TestAssignmentStudentFlow`, `TestAssignmentVersionPinning`, `TestAssignmentQuestionSet`, `TestAssignmentReEvaluated` |
| 10 | `TestAssignmentEnrollmentChanges` |
| 11, 13 | `TestAssignmentStudentExceptions` |
| 12 | `TestAssignmentCloseAndRelease` |
| 15 | `TestAssignmentGameCatalog` (projection nạp trực tiếp — event F078 chưa có, F041-2), unit `TestR1AssignmentDue` |

Lệnh: `go test -tags integration -run TestAssignment ./modules/learning/tests/integration/` → 11 PASS; toàn bộ integration module learning PASS; gate tĩnh (gofmt, vet, golangci + arch, tenancyguard, ewlint, entlint, archgen, routematrix, allowedcallers, facadegen, specs, fixture Godot, RTM) 0 lỗi.
