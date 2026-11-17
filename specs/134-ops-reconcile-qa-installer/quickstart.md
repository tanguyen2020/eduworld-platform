# Quickstart: F134 — Công cụ vận hành bổ sung: lệnh `reconcile-<module>`, loại tenant QA khỏi analytics, trang tải bộ cài cho trường

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F080, F131, F133 (và F010, F043/F044, F057 cho phần liên quan).
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app`, `cd web && pnpm dev` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `sa.admin` (system_admin), `ad.truong.a`, `ad.truong.b`, `ad.donvi` (tenant_admin `pgd-demo`), `qlcm.truong.a` (academic_manager), `hs.an.3a`, `hs.dung.7a`.
- Chỉ mục bộ cài local: `deploy/installer/school-kit/publish-index.sh --channel production --local` ghi vào floci `ew-game-bundles` (khóa ký local), `EW_CLIENT_RELEASES_BASE_URL` trỏ floci.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` làm 20 activity (game cộng XP qua `activity.graded`); ghi T; làm thêm 5 activity | Có event trong `infra.outbox_events` learning và `processed_events` game | — |
| 2 | Khôi phục `eduworld_learning` về T (script test `scripts/ops/test-restore.sh learning T`), reset consumer learning về T | learning mất 5 attempt sau T | — |
| 3 | `make reconcile MOD=learning RESTORE_POINT=T FROM=T-1h TO=now` | Báo cáo: `lost_after_restore = 5` (`activity.graded`, consumer game), `unexplained = 0`, exit 0 | F134-AC-01, F134-AC-03 |
| 4 | Dừng module game, chạy lại bước 3 | `status = incomplete`, peer game lỗi, exit 3 | F134-AC-03 |
| 5 | Chạy với `FROM` cách `TO` hơn `outbox_retention` | Exit 1, không gọi module nào | F134-AC-04 |
| 6 | Lặp bước 2 nhưng **không** reset consumer learning; chạy lại bước 3 sau `--settle` | Chiều `inbound`: event của identity/game sau T chưa được learning xử lý → `unexplained` > 0, exit 2; reset consumer rồi chạy lại → 0 | F134-AC-02 |
| 7 | `curl` `/internal/v1/ops/processed-events` qua APISIX và trực tiếp `core-api`; thêm import facade `listProcessedEventIds` vào module `realtime` rồi `make arch-check`; gọi facade với `limit` vượt trần | `404`; `arch-check` đỏ; `VALIDATION_FAILED` | F134-AC-05 |
| 8 | Kiểm DB sau bước 3 | Không có thay đổi bảng nghiệp vụ; audit `ops.reconcile.completed` | F134-AC-06 |
| 9 | `sa.admin` `updateTenant(pgd-demo, is_qa=true)`; thử với `common` | Audit `tenant.qa_flag_changed`, `ref.tenants.is_qa` game/learning = true; `common` → `422` | F134-AC-07 |
| 10 | `hs.dung.7a` chơi 10 phút; chạy rollup | Không dòng `events_raw` của `pgd-demo`; `eduworld_analytics_events_dropped_total{reason="qa_tenant"}` tăng | F134-AC-08 |
| 11 | `sa.admin` xem Game Analytics và tổng hợp học tập platform, lọc `tenant_ids=pgd-demo` | Không tính `pgd-demo` (kể cả dữ liệu cũ); `excluded_tenant_ids` có `pgd-demo` | F134-AC-09 |
| 12 | `ad.donvi` mở báo cáo game tenant; `ad.truong.a` gọi summary của `pgd-demo` | `200` rỗng; `403`/`404` | F134-AC-10 |
| 13 | Portal D → Tenant → bật/tắt "Tenant QA"; mở dropdown tenant | Hộp xác nhận hệ quả; nhãn "QA" cạnh `pgd-demo` | F134-AC-11 |
| 14 | Chạy `publish-index.sh` (khối g1_3, g4_5 Windows + Android) | Chỉ mục + `.sig` hợp lệ theo schema | F134-AC-12 |
| 15 | `ad.truong.a` mở C → "Tải bộ cài" | Thấy 4 bộ cài (2 nền tảng × g1_3/g4_5), SHA-256, nút tải, hướng dẫn `device.cfg` | F134-AC-13, F134-AC-17 |
| 16 | Sửa 1 byte chỉ mục trên floci; tải lại trang | `503 UNAVAILABLE`, không link nào | F134-AC-14 |
| 17 | `ad.truong.b` gọi với `school_id` TH Demo A; `qlcm.truong.a` gọi | `404`; `403 PERMISSION_DENIED` | F134-AC-15 |
| 18 | Quét response | Không dữ liệu tenant/secret; chuỗi trong giới hạn schema | F134-AC-16 |
| 19 | `sa.admin` tắt `menu.school_lms.installers` cho `th-demo-a`; `ad.truong.a` tải lại portal | Menu "Tải bộ cài" biến mất; `listSchoolClientInstallers` → `403 FEATURE_DISABLED` | F134-AC-18 |

## 3. Lệnh test tự động

```bash
make test MOD=learning && make test-integration MOD=learning     # gồm reconcile restore test (TC-SVC-014)
make test MOD=game && make test-integration MOD=game
make test MOD=identity && make test-contract
make web-test && make web-lint
```

## 4. Bằng chứng cần nộp

- Báo cáo JSON bước 3 và 6, exit code; trace_id bước 9–11; ảnh màn bước 13, 15, 16; kết quả test (số pass/fail).
