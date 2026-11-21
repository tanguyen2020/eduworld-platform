# Quickstart: F121 — Moderation: báo cáo, sanction, hàng đợi Safety Team liên tenant, thông báo vụ việc cho trường

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F115, F116, F117, F118, F095, F012.
- Lệnh: `make up`, `make migrate`, `make seed`, `make up-app` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)); gọi API admin qua gateway nội bộ local.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (TH Demo A), `hs.dung.7a` (`pgd-demo`), `hs.tudo` (tenant chung), `sm.antoan` (safety_moderator), `sa.admin`, `ad.truong.a`, `ad.truong.b`, `ad.donvi`, `qlcm.truong.a`.
- Tài khoản bổ sung seed local [Confirmed]: `sm.truongca` (safety_moderator + `moderation.case.lead`).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` gửi tin trong party; `hs.binh.3a` `createReport` (`asked_personal_info`, `context.party_id`) | `201`, thông điệp cảm ơn; report có snapshot ±10 tin | F121-AC-01 |
| 2 | `hs.binh.3a` báo cáo lại cùng tin; `hs.chi.3b` báo cáo cùng target | Một case, `report_count` tăng đúng; `severity = p1`, `sla_due_at` đặt | F121-AC-02 |
| 3 | Phát `chat.message_held` mẫu (`block_and_alert`) | Case `source = filter_alert` | F121-AC-03 |
| 4 | Sau bước 2, `hs.an.3a` gửi tin/lời mời | Bị hạn chế tạm thời (`CHAT_MUTED`) | F121-AC-04 |
| 5 | Báo cáo tin đã thu hồi; báo cáo tin của channel không phải thành viên | Case tạo bình thường; `404` | F121-AC-05 |
| 6 | `sm.antoan` `listModerationCases` không chọn tenant | Thấy case `th-demo-a` và `pgd-demo`; audit + metric bypass tăng | F121-AC-06 |
| 7 | `sm.antoan` nhận case (`assignModerationCase`), mở tin, tìm `q`; tài khoản moderator khác mở cùng case | Trả snapshot + ngữ cảnh, audit `moderation.case.*`; người khác `403` | F121-AC-07, F121-AC-08 |
| 8 | `sm.antoan` gọi `getSchoolDashboard`, `searchMessages`; `ad.truong.a` gọi `/admin/moderation/cases` qua gateway public | `403`; gateway public `404` | F121-AC-09 |
| 9 | Token `sm.antoan` chỉ mật khẩu; token client game của `sm.antoan` | `200` (không MFA — ADR-0094); `403 PORTAL_ACCESS_STAFF_ONLY` + audit `authz.denied` | F121-AC-10 |
| 10 | `actOnModerationCase` `remove_message` rồi `restrict` (`restrict_text`) và `close`; gửi lại với `If-Match` cũ | Tin placeholder realtime; `412` | F121-AC-11, F121-AC-12 |
| 11 | `sm.antoan` áp `restrict_multiplayer` 14 ngày; `sm.truongca` áp lại | `403`; thành công, `sanction.applied` | F121-AC-13 |
| 12 | `hs.an.3a` gửi text khi `restrict_text`, cả khi `enabled_all` tắt và trong `match` | Bị từ chối | F121-AC-14 |
| 13 | `revokeSanction` có lý do | Gửi lại được ≤ 60 s, `sanction.lifted` | F121-AC-15 |
| 14 | `hs.binh.3a` mở hộp thông báo trong game | "Báo cáo của em đã được xử lý", không chi tiết hình phạt | F121-AC-16 |
| 15 | `notifySchoolForCase` cho TH Demo A với `hs.an.3a`; thử thêm `hs.dung.7a` | `201`, `ack_due_at` theo p1; thêm học sinh trường khác → `422` | F121-AC-17 |
| 16 | `ad.truong.a`, `ad.truong.b`, `ad.donvi` gọi `listSchoolSafetyNotifications` | Chỉ `ad.truong.a` thấy; payload không có nội dung tin | F121-AC-18 |
| 17 | `ad.truong.a` xác nhận 2 lần; `ad.truong.b` xác nhận thông báo của A | Lần 2 `409`; `404` | F121-AC-19 |
| 18 | Tua đồng hồ quá `ack_due_at` | Nhắc lại trường; quá lần 2 báo trưởng ca; lọc `overdue=true` thấy | F121-AC-20 |
| 19 | Case của `hs.tudo` → `notifySchoolForCase` | `422`; `contact_path = platform_direct` | F121-AC-21 |
| 20 | Case p1 không ai nhận quá `p1_unclaimed_alert_minutes` | Alert `ModerationP1Unclaimed` bắn | F121-AC-22 |
| 21 | Case p2 quá hạn phản hồi | `escalated`, báo trưởng ca | F121-AC-23 |
| 22 | Case category tự hại | `support_guide` có trong chi tiết; không sanction người gửi | F121-AC-24 |
| 23 | Quét OpenAPI; đọc audit log của các bước trên | Không endpoint duyệt chat tự do; audit không có nội dung | F121-AC-25 |
| 24 | Báo cáo tin `match` ngày D+2, chạy purge sau hạn, mở case | Chỉ thấy `moderation_evidence` | F121-AC-26 |

## 3. Lệnh test tự động

```bash
make gen-check MOD=chat
make lint MOD=chat
make test MOD=chat
make test-integration MOD=chat
make test-contract
make migrate-lint MOD=chat
```

## 4. Bằng chứng cần nộp

- Kết quả test, `trace_id` kịch bản 1, 7, 10, 15, 17; trích audit log (không nội dung); ảnh alert kịch bản 20.
