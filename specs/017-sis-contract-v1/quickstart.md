# Quickstart: F017 — SIS `contract_v1`: provider API theo EduWorld SIS Contract, webhook, conformance kit, `sis-mock` (tách từ F011)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F011, F004, F003, F001.
- Lệnh: `make up` (gồm `sis-mock` cổng 4010), `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Secret local: `deploy/compose/secrets/sis-demo-a` (API key bất kỳ, Prism không kiểm) và `sis-demo-a-webhook` (chuỗi HMAC thử) — tạo từ `.example`, không commit.
- Tài khoản: `sa.admin`, `ad.truong.a` (TH Demo A, `th-demo-a`), `ad.truong.b` (`pgd-demo`), `qlcm.truong.a`.
- Gọi API qua gateway nội bộ local; token lấy theo LOCAL_DEVELOPMENT.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `sa.admin` `POST /api/v1/sis/connections` (`X-Tenant-Id: th-demo-a`, `Idempotency-Key`) `provider_kind=contract_v1`, `base_url=http://sis-mock:4010/eduworld/v1` (local cho phép http qua cờ dev), `auth_type=api_key`, `secret_ref=sis-demo-a`, `contract_version=1.2.0`; `setSisConnectionSchools` TH Demo A | 201; response không có `secret_ref`, có `has_secret=true`; audit `sis_connection.created` | F017-AC-01, F017-AC-04 |
| 2 | `POST …/{id}:health-check` | `compatible=true`, `capabilities` từ example `Health` lưu trên connection | F017-AC-02 |
| 3 | Sửa example `Health.contract_version` thành `2.0.0` trong fake SIS (test tích hợp) | `422 SIS_CONTRACT_VERSION_UNSUPPORTED`, connection `contract_error` | F017-AC-02 |
| 4 | `triggerSisSync` `kind=full`, `dry_run=true` | Run `succeeded`, `dry_run=true`, `stats` có create/update; không có `student.synced` trên NATS | F017-AC-03 |
| 5 | `triggerSisSync` `kind=full` (thật); gọi lại ngay khi run đang chạy | Run đầu `succeeded`, staging/apply qua F011; lần hai `409 SIS_SYNC_ALREADY_RUNNING` | F017-AC-05, F017-AC-07 |
| 6 | Fake SIS: 1 học sinh sai enum `status`, 1 trường lạ `phone` | `partially_failed`, 1 lỗi `SIS_CONTRACT_VIOLATION`; `phone` không có trong staging; metric trường lạ tăng | F017-AC-06 |
| 7 | Xem `sis.sync_completed` và `missing_in_source` sau khi bỏ 1 học sinh khỏi fake SIS | Event có `mode=full`, `connection_status=active`; học sinh bị đánh dấu `missing_in_source` | F017-AC-08 |
| 8 | Fake SIS trả trường `TH-LA-999` chưa ánh xạ | Issue `unmapped_school`; không tạo trường | F017-AC-09 |
| 9 | `curl -H "Prefer: code=503" http://localhost:4010/eduworld/v1/schools`; fake SIS 503×2 rồi 200; 429 `Retry-After: 2` | Prism trả 503 problem+json; worker retry rồi thành công; chờ ≥ 2 s sau 429 | F017-AC-10, F017-AC-11 |
| 10 | Fake SIS trả 401 | Run `failed`, connection `auth_error`, event `connection_status=auth_error`, không retry | F017-AC-12 |
| 11 | Fake SIS lỗi liên tiếp tới ngưỡng; `triggerSisSync` | Connection `paused` (`circuit_open`); trigger → `409 SIS_CONNECTION_NOT_ACTIVE`; `sa.admin` `PATCH status=active` có lý do → `active` | F017-AC-13, F017-AC-14 |
| 12 | Connection có `delta`; fake `listChanges` 2 trang rồi 410 ở lần sau | Watermark = `next_cursor` trang cuối sau apply; 410 → full sync `trigger=fallback_full`, watermark không đổi | F017-AC-15, F017-AC-16 |
| 13 | Delta có `hs.chi.3b` `status=suspended` | User `locked` trong cùng run; game session mới bị chặn | F017-AC-17 |
| 14 | Gửi webhook ký đúng tới `/internal/sis/webhooks/{id}`; gửi lại cùng `delivery_id`; ký sai; timestamp lệch | 202 + job; 409; 401; 401 | F017-AC-18, F017-AC-19 |
| 15 | Webhook tới connection thiếu capability `webhook`; gọi qua gateway public | 404; route không tồn tại | F017-AC-20 |
| 16 | Làm job `webhook_batch` lỗi (DB tạm dừng) | Redeliver; hết lượt vào `DLQ_IDENTITY`; SyncRun `mode=webhook` `failed` | F017-AC-21 |
| 17 | `hs.an.3a` đăng nhập lần đầu sau khi xóa dữ liệu đồng bộ (seed rỗng) | Adapter gọi `lookupStudent` 1 lần, bootstrap có lớp 3A | F017-AC-22 |
| 18 | Connection không có `student_lookup`; fake SIS chậm hơn timeout | Không gọi mạng / trả lỗi tạm thời, F011 dùng dữ liệu cũ | F017-AC-23 |
| 19 | k6 200 đăng nhập nhánh B trên một connection treo + một connection bình thường | ≤ `sis_login_sync_concurrency` lookup đồng thời; connection thứ hai không bị ảnh hưởng | F017-AC-24 |
| 20 | `postman collection run specs/postman/sis-conformance/sis-conformance.postman_collection.json -e specs/postman/sis-conformance/env.sis-mock.postman_environment.json` | Mọi test bắt buộc pass; capability không khai báo báo "skipped" | F017-AC-25, F017-AC-26, F017-AC-27 |
| 21 | `ad.truong.a` trigger cho TH Demo A; `ad.truong.b` trigger connection TH Demo A; `qlcm.truong.a` tạo connection | 202; 404; 403 | F017-AC-28 |
| 22 | Trigger không `Idempotency-Key`; xem audit | 428 `IDEMPOTENCY_KEY_REQUIRED`; audit có actor `system:sis-sync:<id>` | F017-AC-29 |
| 23 | Tạo connection `base_url` 3.000 ký tự / `ftp://` | `422 VALIDATION_FAILED` | F017-AC-30 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=identity
make test MOD=identity
make test-integration MOD=identity        # fake SIS httptest, testcontainers
postman collection run specs/postman/sis-conformance/sis-conformance.postman_collection.json \
  -e specs/postman/sis-conformance/env.sis-mock.postman_environment.json
make specs-lint
```

## 4. Bằng chứng cần nộp

- Kết quả go test/Postman (số pass/fail), `trace_id` kịch bản 5, 14, 17; ảnh log không chứa `secret_ref`/PII (kịch bản 1, 10); metric kịch bản 6, 19.
