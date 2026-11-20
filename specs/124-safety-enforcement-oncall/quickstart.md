# Quickstart: F124 — Thực thi xử phạt và vận hành Safety Team: consumer `sanction.applied` (khóa tài khoản), email thông báo vụ việc cho trường, vòng trực, break-glass

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F015, F016, F121 (F131 cho bước Alertmanager trên staging).
- Lệnh: `make up` (gồm `mailpit` :8025), `make seed`, `make run-core-api`, `make run-worker PROFILE=events`, `make run-worker PROFILE=chat` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `sm.antoan` (safety_moderator), `sm.truongca` (safety_moderator + `moderation.case.lead`, seed [Confirmed] của F121), `sa.admin`, `sa.admin2` (system_admin thứ hai, seed [Confirmed]), `ad.truong.a`, `ad.truong.b`, `ad.donvi`, `hs.binh.3a`, `hs.an.3a`. Mật khẩu chung `Demo@12345`, MFA tắt ở local.
- Đồng hồ giả cho job: `EW_CLOCK_OFFSET` chỉ ở local/test.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.binh.3a` đang chơi; `sm.truongca` `createSanction` `account_lock` (F121) | ≤ 60 s REST `403 ACCOUNT_LOCKED`; WS đóng; Keycloak user `enabled=false` ≤ 1 phút; `identity.account_locks` có dòng `moderation_sanction` | F124-AC-01 |
| 2 | Phát lại cùng `sanction.applied` (NATS CLI) | Không có khóa thứ hai, `processed_events` chặn | F124-AC-03 |
| 3 | `ad.truong.a` khóa `hs.binh.3a` (lý do nghỉ học), rồi `sm.truongca` `revokeSanction` | Vẫn `locked` (còn khóa `school_admin`); không phát `user.access_restored` | F124-AC-02 |
| 4 | `ad.truong.a` gọi `unlockTenantUser` khi còn khóa kiểm duyệt (lặp bước 1) | `409 ACCOUNT_LOCK_HELD_BY_SAFETY`; khóa của trường được gỡ | F124-AC-05 |
| 5 | Thu hồi sanction khi không còn khóa nào | `active`, `user.access_restored`, đăng nhập lại được | F124-AC-02 |
| 6 | `hs.binh.3a` đăng nhập khi bị khóa; `ad.truong.a` xem danh sách người dùng | Học sinh chỉ thấy `ACCOUNT_LOCKED`; nhân sự thấy `lock_sources = [moderation_sanction]`, không `case_id` | F124-AC-06 |
| 7 | Phát `sanction.applied` với `tenant_id` = `pgd-demo` cho `hs.binh.3a` | Không đổi trạng thái; DLQ + audit `security.tenant_mismatch` | F124-AC-04 |
| 8 | `sm.antoan` `notifySchoolForCase` cho TH Demo A | mailpit: 1 email tới `ad.truong.a`, không tới `ad.truong.b`; email không có tên học sinh/mã case/loại vụ việc | F124-AC-07, F124-AC-09, F124-AC-11 |
| 9 | Tiến đồng hồ quá `ack_due_at` | Email `school_safety_reminder` lần 1 | F124-AC-08 |
| 10 | Tắt mailpit, gửi thông báo mới, bật lại | Thử lại có backoff rồi `sent`; khi để tắt quá số lần → `failed`, alert `EmailDeliveryFailing` | F124-AC-10 |
| 11 | `sm.truongca` tạo ca primary `sm.antoan`, secondary `sm.truongca`; tạo ca trùng giờ primary | Thành công; lần trùng → `409 ONCALL_SHIFT_OVERLAP`; `sm.antoan` tạo ca → `403` | F124-AC-12, F124-AC-16 |
| 12 | `hs.an.3a` báo cáo `asked_personal_info` (case p1) | Case `open`, `assignee` = `sm.antoan`, `assigned_via = rotation`; email `moderation_oncall_page` tới `sm.antoan` | F124-AC-13 |
| 13 | Tiến đồng hồ 1×, 2× `p1_unclaimed_alert_minutes`, rồi quá hạn phản hồi `report_sla` | Gọi `secondary`, `lead`, `system_admin` mỗi mức một lần; nhận case ở giữa → dừng gọi | F124-AC-14 |
| 14 | Xóa mọi ca, tạo case p1 | Gọi thẳng trưởng ca; `eduworld_moderation_oncall_coverage{oncall_role="primary"} = 0`, alert `ModerationOncallGap` | F124-AC-15 |
| 15 | `sa.admin` `createBreakGlassRequest` (user `hs.binh.3a`, 7 ngày, 60 phút, lý do 30 ký tự); tự duyệt | `pending`; email tới `sa.admin2`; tự duyệt → `403` | F124-AC-17, F124-AC-18 |
| 16 | `sa.admin2` duyệt; `sa.admin` `listBreakGlassMessages`; `sa.admin2` gọi cùng API | `sa.admin` thấy tin của học sinh trong khoảng; audit `break_glass.messages_viewed`, metric `reason="break_glass"`; `sa.admin2` → `403 BREAK_GLASS_NOT_ACTIVE` | F124-AC-19 |
| 17 | Tiến đồng hồ quá `expires_at`; gọi lại | `403 BREAK_GLASS_NOT_ACTIVE`; `listBreakGlassGrants` hiện `expired`, số tin đã xem | F124-AC-20 |
| 18 | `sm.antoan` và `ad.donvi` gọi `createBreakGlassRequest`; `reason` 1001 ký tự | `403`; `422 VALIDATION_FAILED` | F124-AC-21 |
| 19 | Quét audit/log/event của bước 15–17 | Không có nội dung tin; tin gameplay đã purge không trả | F124-AC-22 |

## 3. Lệnh test tự động

```bash
make test MOD=identity && make test-integration MOD=identity
make test MOD=chat && make test-integration MOD=chat
make test-contract          # payload chat → identity
make migrate-lint MOD=identity && make migrate-lint MOD=chat
```

## 4. Bằng chứng cần nộp

- Kết quả test (số pass/fail), trace_id kịch bản 1, 8, 12, 16; ảnh mailpit bước 8 (che địa chỉ); trích audit bước 16.
