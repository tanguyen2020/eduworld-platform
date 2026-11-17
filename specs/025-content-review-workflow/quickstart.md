# Quickstart: F025 — Workflow duyệt nội dung, song ngữ, lồng tiếng `g1_3`

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020, F021, F022 (và bảng media của F027).
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo (mật khẩu seed): `cm.noidung` (content_admin), `cm.duyet` (content_reviewer), `qlcm.truong.a` (academic_manager, tenant `pgd-demo`), `ad.donvi` (tenant_admin `pgd-demo`), `ad.truong.a` (school_admin `th-demo-a`), `sa.admin` (system_admin).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `cm.noidung` tạo câu hỏi lớp 3 có đủ `stem.audio_ref` (F022), `submit_review` với `If-Match` + `Idempotency-Key` | `200`, status `in_review`; có task lane `platform_learning`; outbox `content.version_submitted` | F025-AC-01 |
| 2 | Gửi lại bước 1 cùng key | Response giống hệt, header `Idempotent-Replayed: true`, không thêm transition | F025-AC-09 |
| 3 | `cm.noidung` tự `approve` | `403 PERMISSION_DENIED`, audit `authz.denied` | F025-AC-05 |
| 4 | `cm.duyet` mở hàng đợi rồi `approve` | Task thấy được; version `approved`, `reviewer_id = cm.duyet` | F025-AC-04, F025-AC-13 |
| 5 | Câu hỏi khác thiếu trường bắt buộc / có ảnh `processing` → `submit_review` | `422 VALIDATION_FAILED` / `422 MEDIA_NOT_READY` | F025-AC-02, F025-AC-03 |
| 6 | `cm.duyet` `reject` không `note`, rồi có `note` | `422`; sau đó `rejected`; tác giả `revise` → `draft` | F025-AC-06, F025-AC-07 |
| 7 | `approve` trên version `draft` | `422 INVALID_STATE_TRANSITION` | F025-AC-08 |
| 8 | Câu hỏi `grade_levels=[2]` không audio → `submit_review`; thêm `audio_exempt_reason`, gửi lại; reviewer approve không cờ rồi có cờ | `422 AUDIO_REQUIRED` liệt kê `/stem/audio_ref`; rồi `in_review`; `422` rồi `approved` | F025-AC-15, F025-AC-16 |
| 9 | `qlcm.truong.a` (tenant `pgd-demo`) gửi duyệt câu hỏi tenant; `ad.donvi` mở hàng đợi và duyệt; `ad.truong.a` (`th-demo-a`) gọi transition cùng ID | Lane `tenant`; `ad.truong.a` nhận `404` | F025-AC-10, F025-AC-12 |
| 10 | `ad.donvi` gọi transition trên câu hỏi platform được gán | `403 PERMISSION_DENIED` | F025-AC-10 |
| 11 | `sa.admin` gọi hàng đợi với `tenant_ids=pgd-demo,th-demo-a` | Có cột `tenant_id`, audit `cross_tenant.accessed` | F025-AC-13 |
| 12 | Lưu câu hỏi `content_language=vi` không có `en`; xóa bản `vi` của đề rồi gửi duyệt | `translation_status = partial`; `422 TRANSLATION_INCOMPLETE` | F025-AC-18, F025-AC-19 |
| 13 | `hs.an.3a` mở activity với `Accept-Language: en-US` khi chỉ có `vi` (qua F030) | Header `Content-Language: vi` | F025-AC-20 |
| 14 | Export → sửa `en` → import `dry_run` → import | Draft mới; version published không đổi | F025-AC-22, F025-AC-23 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning
make test MOD=learning
make test-integration MOD=learning
make migrate-lint MOD=learning
make specs-lint
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo mã F025-AC-*, trace_id của bước 1 và 8, truy vấn đối soát `content_transitions` (SC-001, SC-004).
