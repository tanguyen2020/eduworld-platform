# Quickstart: F040 — Course, learning path, lịch, nhóm học tập

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020, F021, F026, F031, F032 (rule engine); F011 đã đồng bộ trường/lớp demo.
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events`, `make seed-check` (projection `ref` đã đồng bộ) theo [LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md).
- Tài khoản demo: `ad.truong.a`, `ad.truong.b`, `qlcm.truong.a` (Toán, 2 tenant), `ad.donvi` (tenant admin), `sm.antoan`, `sa.admin`, `hs.an.3a`, `hs.chi.3b`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | Chạy lại đồng bộ lớp 3A (phát lại `class.synced` 2 lần) | `listCourses?class_id=3A` có một course cho mỗi môn khối 3, không trùng; course mang `curriculum_version_id` | F040-AC-01, F040-AC-02 |
| 2 | `ad.truong.a` gọi `putSchoolCurriculumSettings` với version chưa publish, rồi version hợp lệ | `409 CONTENT_NOT_PUBLISHED`; sau đó lưu được, có audit | F040-AC-03 |
| 3 | `ad.truong.a`, `qlcm.truong.a`, `ad.donvi` gọi `listCourses` | Đúng phạm vi trường / môn Toán / toàn tenant `pgd-demo` | F040-AC-04 |
| 4 | `ad.truong.a` `updateCourse` (ẩn Âm nhạc) không `If-Match`, rồi có `If-Match` | `428`; sau đó `status = hidden`, audit | F040-AC-05 |
| 5 | `qlcm.truong.a` tạo path `mastery_gate` 3 node; node trỏ nội dung platform chưa gán | Path lưu, node ghim version; node chưa gán → `403 RESOURCE_NOT_ASSIGNED` | F040-AC-06, F040-AC-07 |
| 6 | Course Tiếng Việt 3A chưa có path; `hs.an.3a` gọi `listMyLearningPaths` | Thấy lộ trình mẫu theo bộ sách; course `hidden` không xuất hiện | F040-AC-08, F040-AC-11 |
| 7 | `hs.an.3a` chưa đạt `developing` LO node 1 → node 2 khóa; `unlockPathNodeForStudent` thiếu lý do / đủ lý do | `422`; node 2 mở, audit | F040-AC-09, F040-AC-10 |
| 8 | `hs.an.3a` chưa có evidence topic kế tiếp trong lịch → `getMyRecommendations` | Có gợi ý `rule_code = R6` | F040-AC-12 |
| 9 | `ad.truong.a` tạo/sửa/xóa mục lịch; `hs.an.3a` gọi `listMySchedule` | Mục hiện trong lịch học sinh; sau xóa biến mất, có trong Thùng rác | F040-AC-13, F040-AC-14, F040-AC-16 |
| 10 | Phát `assignment.published` gắn course Toán 3A hai lần | Một mục lịch `assignment_due` | F040-AC-15 |
| 11 | `ad.truong.a` `createStudyGroup` "Ôn chia có dư" (5 học sinh 3A, 3B, theo khối 3); gửi lại cùng `Idempotency-Key`; `qlcm.truong.a` gọi `listStudyGroups?grade_level=3` và `getStudyGroup` | `201`, một nhóm, audit, outbox có `study_group.created`; list/get thấy nhóm, có `member_ids` + `ETag` | F040-AC-17, F040-AC-18, F040-AC-23 |
| 12 | `ad.truong.a` `updateStudyGroup` thiếu `If-Match`, rồi có `If-Match` bỏ 1 và thêm 1 học sinh; sau đó `deleteStudyGroup` | `428`; `member_count = 5`, outbox `study_group.member_changed` (1 thêm, 1 bớt); sau xóa không còn trong list, có `study_group.deleted` | F040-AC-19, F040-AC-20, F040-AC-23 |
| 13 | `ad.truong.a` tạo nhóm trùng tên khác hoa thường; tạo nhóm có học sinh trường B (`ad.truong.b` quản lý) hoặc tên 65 ký tự | `409 CONFLICT`; `422 VALIDATION_FAILED` | F040-AC-21, F040-AC-22 |
| 14 | `ad.truong.b` đọc course 3A; `qlcm.truong.a` sửa course Tiếng Việt; `sm.antoan`, `hs.an.3a` gọi `listCourses` | `404`; `403 PERMISSION_DENIED`; `403 PERMISSION_DENIED` / `403 PORTAL_ACCESS_STAFF_ONLY` | F040-AC-24, F040-AC-25, F040-AC-26 |
| 15 | `sa.admin` tạo mục lịch không có `X-Tenant-Id`, rồi có | `400 TENANT_HEADER_REQUIRED`; tạo được, audit tenant đích | F040-AC-27 |
| 16 | Gửi lại `createLearningPath` cùng `Idempotency-Key`; gửi `title` 300 ký tự | Cùng path; `422 VALIDATION_FAILED` | F040-AC-28, F040-AC-29 |

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

- Kết quả test (số pass/fail) package `lms`, trace_id kịch bản 1 và 7, bản ghi audit kịch bản 4, 7, 9, 11, 12, 15; dòng outbox kịch bản 11–12.

## 5. Bằng chứng (2026-11-20, local, testcontainers PostgreSQL thật)

| Phạm vi | Test tự động | Kết quả |
|---|---|---|
| US1 + US5 (course, cài đặt curriculum, phạm vi vai trò, If-Match, audit, SA/học sinh/tenant B; T044 lọc năm học) | `tests/integration/lms_test.go` TestLMSCoursesAndSettings | pass |
| US2 (lộ trình ghim version, 409/403, mẫu, trạng thái node, mở khóa thủ công, sửa giữ `node_id`, course ẩn, R6) | TestLMSLearningPaths; domain `lms` TestNodeStates, TestCourseAndPathRules; `recommendation` TestR6 | pass |
| US3 + US4 (lịch, xóa mềm + event, lịch từ `assignment.published` một lần, lịch học sinh; nhóm học tập, 409/422/428/412, event `study_group.*` hợp lệ schema) | `tests/integration/lms_group_test.go` TestLMSScheduleAndStudyGroups; domain TestStudyGroupRules | pass |

Chạy tay trên stack compose: chưa làm (đăng nhập demo bị chặn ở phiên agent; màn portal thuộc F055, game F062).
