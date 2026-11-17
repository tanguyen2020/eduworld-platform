# Quickstart: F011 — Đồng bộ SIS bằng CSV và provisioning tài khoản

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F010.
- Lệnh: `make up`, `make seed`, `make run-core-api`, `make run-worker PROFILE=jobs` (job SIS), `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `ad.truong.a` (School Admin TH Demo A), `ad.truong.b`, `sa.admin`; học sinh `hs.an.3a`, `hs.chi.3b`.
- Bộ CSV mẫu theo [SIS_INTEGRATION §4.3](../../docs/09-identity/SIS_INTEGRATION.md) đặt ở `src/backend/modules/identity/tests/fixtures/sis/th-demo-a/`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` gọi `createSisImport` với `dry_run=true` | Diff create/update/noop; không có thay đổi DB nghiệp vụ | F011-AC-01 |
| 2 | Thêm dòng sai enum + cột lạ vào `students.csv`, import | Dòng sai `SIS_CONTRACT_VIOLATION` (dòng, cột), file không áp; cột lạ bị bỏ qua, metric tăng | F011-AC-02, F011-AC-03 |
| 3 | Import CSV đúng (không dry-run) với `Idempotency-Key` | `sync_run` `succeeded`; event `school.synced`, `class.synced`, `student.synced`, `enrollment.changed`; audit | F011-AC-04, F011-AC-06 |
| 4 | Import lại cùng file | 0 update | F011-AC-05 |
| 5 | `ad.truong.b` gọi `getSisImport` của TH Demo A | `404` | F011-AC-07 |
| 6 | Kiểm Keycloak user học sinh mới | User local, role `student`, member Organization `th-demo-a`, `external_identities` + `sis_identity_links` active, `user.provisioned` | F011-AC-08, F011-AC-12 |
| 7 | Import học sinh có username trùng (khác hoa thường) với học sinh khác ở `pgd-demo` | Không tạo; issue `duplicate_identifier` | F011-AC-10 |
| 8 | Resolve issue với `resolution = same_person` | Thêm membership vào tài khoản sẵn có | F011-AC-11 |
| 9 | Import học sinh không có enrollment | Issue `missing_class` | F011-AC-15 |
| 10 | Đăng nhập `hs.an.3a` (qua F013 hoặc gọi `LoginSync` trong test) với dữ liệu mới / thiếu lớp / cũ > ngưỡng | 0 lời gọi lookup / connection `csv_import` không gọi lookup, dùng dữ liệu gần nhất hoặc `NO_ACTIVE_ENROLLMENT` / một job `login_refresh` | F011-AC-18, F011-AC-20, F011-AC-21 |
| 11 | Import `status=suspended` cho `hs.chi.3b` | `locked`, `user.status_changed` + `user.access_revoked` | F011-AC-23 |
| 12 | `sa.admin` tạo connection `csv_import` cho `th-demo-a`, gán TH Demo A; tạo connection thứ hai gán lại TH Demo A | Lần đầu OK + audit; lần hai `409 SIS_SCHOOL_ALREADY_CONNECTED`; response không có `secret_ref` | F011-AC-26, F011-AC-27 |

## 3. Lệnh test tự động

```bash
make test MOD=identity
make test-integration MOD=identity
make api-test   # Postman CLI
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id của import ở bước 3, số lời gọi lookup (fake) ở bước 10, ảnh danh sách issue.
