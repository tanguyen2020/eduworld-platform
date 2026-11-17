# Quickstart: F026 — Phát hành nội dung: publish/rollback/unpublish, snapshot, manifest, ghim version, nơi sử dụng

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F020, F022, F025, F014 (manifest game).
- Lệnh: `make up`, `make migrate`, `make seed`, `make run-core-api`, `make run-worker PROFILE=events`, `make up-app EXCEPT=core-api` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `cm.noidung` (content_admin), `cm.duyet` (content_reviewer), `sa.admin`, `ad.truong.a` (`th-demo-a`), `ad.truong.b` (`pgd-demo`), học sinh `hs.an.3a` (`th-demo-a`), `hs.dung.7a` (`pgd-demo`).
- Dữ liệu: activity `quiz` platform đã `approved` (F025), gán cho `th-demo-a`, không gán `pgd-demo` (F020).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `cm.duyet` publish v1 (`If-Match`, `Idempotency-Key`) | `200`, `content_releases` 1 dòng, outbox `content.published` | F026-AC-01 |
| 2 | Gọi lại cùng key; gọi với `If-Match` cũ | Response cũ; `409 CONTENT_VERSION_CONFLICT` | F026-AC-03 |
| 3 | PATCH nội dung v1 đã publish | `409 CONTENT_NOT_EDITABLE`; UPDATE SQL trực tiếp bị trigger chặn | F026-AC-04 |
| 4 | Tạo draft v2 từ v1; tạo thêm draft thứ hai | v2 `based_on_version_id = v1`; `409 CONFLICT` | F026-AC-05 |
| 5 | Duyệt + publish v2; xem lịch sử | v1 vẫn `published`, `is_current` ở v2 | F026-AC-06, F026-AC-20 |
| 6 | `ad.truong.a` publish item platform; `ad.truong.b` publish item của `th-demo-a` | `403 PERMISSION_DENIED`; `404` | F026-AC-07 |
| 7 | `cm.noidung` rollback về v1 có lý do; rollback về v2 khi v2 archived | Con trỏ = v1, không có v3; `422 ROLLBACK_TARGET_INVALID` | F026-AC-08, F026-AC-09 |
| 8 | Rollback thiếu `reason` / reason 501 ký tự | `422 VALIDATION_FAILED` | F026-AC-11 |
| 9 | `hs.an.3a` bắt đầu attempt (F030 hoặc test `VersionReader`) trên v1; publish v2; đọc lại | Vẫn v1; lượt mới nhận v2 | F026-AC-15 |
| 10 | Quét key Valkey `ew:learning:cv:activity:*` và object CDN | Không có trường đáp án; câu `exam_only` không có object CDN | F026-AC-12, F026-AC-13 |
| 11 | `docker compose stop valkey` rồi mở activity | Vẫn trả đúng nội dung từ DB | F026-AC-14 |
| 12 | Kiểm `ew:learning:current:<th-demo-a>:activity:<id>` sau rollback | = v1 | F026-AC-16 |
| 13 | `hs.an.3a` gọi `getContentManifest` hai lần (lần 2 `If-None-Match`); sau publish gọi lại | `304`; sau publish `200`, `manifest_version` tăng, mục có `activity_type`, `min_client_version` | F026-AC-23, F026-AC-25 |
| 14 | Đo `released_at` → `generated_at` 20 lần publish | p95 ≤ `publish_to_visible_max` | F026-AC-24 |
| 15 | `hs.dung.7a` (`pgd-demo`) tải manifest | Không có activity platform chưa gán | F026-AC-26 |
| 16 | Hẹn `publish_at` < `scheduled_publish_min_lead`; hẹn hợp lệ; chờ giờ; hẹn mới rồi hủy | `422`; `202`; áp dụng ±1 phút; `cancelled` | F026-AC-17, F026-AC-18, F026-AC-19 |
| 17 | `diffContentVersions` bằng `ad.truong.a` (không review) và `cm.duyet` | Không có pointer `answer_key` / có | F026-AC-21 |
| 18 | `listContentReleases` | publish, rollback theo thứ tự mới nhất | F026-AC-22 |
| 19 | `getContentUsages` câu hỏi trong question set; archive con trỏ; archive v cũ | Danh sách tham chiếu; `422`; `archived` | F026-AC-27, F026-AC-28 |
| 20 | Unpublish có lý do; học sinh bắt đầu lượt mới | Con trỏ NULL; `409 CONTENT_NOT_PUBLISHED` | F026-AC-10 |
| 21 | Phát lại cùng event `content.published` cho consumer cache | Không đổi gì lần 2 (`processed_events`) | F026-AC-16 |
| 22 | Publish version thiếu audio `g1_3` / tham chiếu câu chưa publish | `422 AUDIO_REQUIRED` / `409 CONTENT_NOT_PUBLISHED` | F026-AC-02 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=learning && make gen-check MOD=game
make test MOD=learning && make test-integration MOD=learning
make test-integration MOD=game
make migrate-lint MOD=learning
make test-contract
```

## 4. Bằng chứng cần nộp

- Số test pass/fail theo F026-AC-*, trace_id bước 1 và 13, số đo p95 bước 14, kết quả quét snapshot (SC-002).
