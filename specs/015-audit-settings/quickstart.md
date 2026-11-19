# Quickstart: F015 — Audit log tập trung và cài đặt platform/tenant/trường

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F012 (và F010, F013 cho hook khung giờ).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `sa.admin`, `ad.donvi` (Tenant Admin `pgd-demo`), `ad.truong.a`, `ad.truong.b`, `cm.noidung`, học sinh `hs.an.3a`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` gán UserRole (F012) → kiểm `audit.audit_logs` | Một dòng `user_role.granted`, partition tháng hiện tại | F015-AC-01 |
| 2 | Phát lại cùng event (NATS redeliver) | Vẫn một dòng | F015-AC-01 |
| 3 | `UPDATE`/`DELETE audit.audit_logs` bằng role `identity_app` | Lỗi quyền | F015-AC-02 |
| 4 | Chạy `partition-maintainer --once` | Partition các tháng tới tồn tại | F015-AC-03 |
| 5 | Phát `audit.recorded` có `metadata.password` | Dòng ghi không có `password`, metric vi phạm tăng | F015-AC-04 |
| 6 | `ad.truong.a` gọi `listTenantAuditLogs`; `ad.truong.b` lọc trường TH Demo A | Chỉ trường mình; trường khác bị loại | F015-AC-07, F015-AC-09 |
| 7 | `sa.admin` gọi `listPlatformAuditLogs?tenant_ids=th-demo-a,pgd-demo`; `cm.noidung` gọi cùng API | Có `tenant_id`, audit `cross_tenant.accessed`; `403` | F015-AC-08 |
| 8 | `sa.admin` `putChatRetentionSettings` 2 ngày; 7 ngày + `reason` | `422 RETENTION_BELOW_MINIMUM`; thành công, `platform_settings.changed` | F015-AC-11, F015-AC-12 |
| 9 | `sa.admin` đặt `allow_tenant_override=false`; `ad.donvi` đổi `inactive_student_cleanup_days` | `403 PERMISSION_DENIED` | F015-AC-13 |
| 10 | `ad.truong.a` `updateSchoolSettings` khung giờ 07:00–17:00; `hs.an.3a` `gameLogin` lúc 20:00 (Clock giả) | `403 ACCESS_OUTSIDE_ALLOWED_HOURS` | F015-AC-16, F015-AC-18 |
| 11 | `ad.truong.a` sửa cài đặt trường THCS Demo B | `404`/`403 PERMISSION_DENIED` | F015-AC-17 |
| 12 | `sa.admin` bật flag cho `th-demo-a`; bootstrap `hs.an.3a` và `hs.dung.7a` | Chỉ `th-demo-a` có flag | F015-AC-20 |
| 13 | `sa.admin` cập nhật `platform_curfew` 21:30–06:30 có `reason`; `ad.truong.a` thử cùng API | Lưu + audit + `platform_settings.changed`; `ad.truong.a` → `403` | F015-AC-21 |
| 14 | `ad.truong.a` đặt khung giới nghiêm trường 22:00–06:00, rồi 21:00–07:00; xem bootstrap `hs.an.3a` | `422 VALIDATION_FAILED`; sau đó lưu được; bootstrap có khung 21:00–07:00 | F015-AC-22 |
| 15 | `make migrate MOD=identity` hai lần; `SELECT count(*) FROM platform.portal_menu_catalog`; `INSERT` bằng role `identity_app` | Số dòng = số mục catalog, lần 2 không đổi; `INSERT` bị từ chối | F015-AC-23 |
| 16 | `INSERT` thẳng flag `flag_kind = menu` với khóa `menu.khong_ton_tai.x` | Vi phạm FK | F015-AC-24 |
| 17 | `sa.admin` `putFeatureFlag menu.school_lms.assignments` `{scope: tenant, state: disabled, reason}` + `If-Match` + `X-Tenant-Id: th-demo-a` | `200`, version tăng, audit `feature_flag.updated`, event `platform_settings.changed` scope `feature_flag` | F015-AC-25 |
| 18 | Tắt `menu.platform_admin.feature_toggles`; thiếu `reason`; thiếu `If-Match`; `ad.donvi` gọi | `422 FEATURE_NOT_DISABLEABLE`; `422`; `428`; `403`/404 (gateway public) | F015-AC-26 |
| 19 | Sau kịch bản 17: `ad.truong.a` gọi `listAssignments` và `getMyPermissions`; `ad.truong.b` gọi cùng API; đợi `feature_toggle_cache_ttl` sau khi `inherit` | `403 FEATURE_DISABLED` + `disabled_menus` có menu; `200`; bật lại trong ≤ TTL | F015-AC-27 |
| 20 | `listPortalMenuCatalog?tenant_id=th-demo-a`; `listFeatureFlags?flag_kind=action` | Cây có `can_disable`, mặc định/ghi đè/hiệu lực, `catalog_version`; audit `cross_tenant.accessed` | F015-AC-28 |
| 21 | Bootstrap game của `hs.an.3a` sau kịch bản 17 | `feature_flags` không chứa khóa `menu.*`/`action.*` | F015-AC-29 |

## 3. Lệnh test tự động

```bash
make test MOD=identity
make test-integration MOD=identity
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), truy vấn `pg_partitions` cho audit, trace_id kịch bản 7 và 10.

## 5. Bằng chứng (2026-11-19, local)

Lệnh tương đương target `make` (chưa có `make` trên Windows): `go test ./modules/identity/... ./pkg/outbox/ ./pkg/audit/`, `go test -tags integration ./modules/identity/tests/integration/` (PostgreSQL + Valkey thật, testcontainers).

| Phần | Test | Kết quả |
|---|---|---|
| US1 (AC-01…AC-06) | `TestAuditSettingsSchema`, `TestAuditSink`, `domain/audit` | PASS |
| US2 (AC-07…AC-10) | `TestAuditLogQueries` | PASS |
| US3 (AC-11…AC-15, AC-21) | `TestPlatformAndTenantSettings` | PASS |
| US4 (AC-16…AC-19, AC-22) | `TestSchoolSettings`, `TestGameLoginSchoolPolicy`, `domain/settings` | PASS |
| US5 (AC-20, AC-23…AC-29) | `TestFeatureFlagsAndMenuCatalog`, `TestAuditSettingsSchema`, `domain/settings` `TestToggles` | PASS |
| Contract | `tests/contract/settings_routes_test.go` | PASS |

Chưa làm (cần binary đã nối — OQ F010-2): gọi qua APISIX, đo SC-003 trên dữ liệu 12 tháng, tải SC-001.
