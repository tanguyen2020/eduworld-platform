# Quickstart: F116 — Pipeline kiểm tra tin nhắn, danh mục từ nhạy cảm, công tắc kiểm duyệt

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F115 (đường gửi); Portal (F122) không bắt buộc — gọi API `/api/v1/admin/chat/*` qua APISIX local (`:9080`, route nội bộ bật ở local) bằng token `eduworld-portal`.
- Lệnh: `make up`, `make seed` (seed danh mục **placeholder** + quick messages), `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=chat` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `sa.admin` (System Admin), `sm.antoan` (Safety Moderator), `ad.donvi` (Tenant Admin `pgd-demo`), `ad.truong.a`, học sinh `hs.an.3a`, `hs.binh.3a` (`th-demo-a`), `hs.dung.7a` (`pgd-demo`). Channel thử như quickstart F115.
- Term thử dùng placeholder `__demo_insult__` (không dùng từ thật trong repo, R15).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | An gửi tin chứa SĐT dạng "không chín một hai…", email, "kết bạn zalo" | `CHAT_CONTENT_BLOCKED` + `category_code = pii`; Bình không nhận; PG không có dòng | F116-AC-04, F116-AC-05 |
| 2 | `sa.admin` tạo term `__demo_insult__` (category `insult`), `testSensitiveTerms` với bản viết chen "_ _demo…" | Kết quả `block`; không có tin nào được ghi | F116-AC-12, F116-AC-13 |
| 3 | `publishSensitiveCatalog`; đợi ≤ 60 s; An gửi biến thể không dấu/leet của term | Bị chặn ở cả `th-demo-a` và `pgd-demo`; `eduworld_chat_sensitive_catalog_version` tăng | F116-AC-02, F116-AC-14, F116-AC-15 |
| 4 | Publish với bộ hồi quy fixture "fail" | `422 SENSITIVE_CATALOG_REGRESSION_FAILED`, không có version mới | F116-AC-14 |
| 5 | `sm.antoan` thêm term tenant cho `pgd-demo` + publish | Chỉ `hs.dung.7a` bị chặn; `hs.an.3a` không | F116-AC-16 |
| 6 | `sm.antoan` toggle category `allow_tenant_toggle = false` | `403 TENANT_TOGGLE_NOT_ALLOWED` | F116-AC-17 |
| 7 | `ad.donvi` gọi `setModerationSwitches`, `createTenantSensitiveTerm`; gọi `listTenantSensitiveCategories` | Không route/`403 PERMISSION_DENIED`; list chỉ có category, không term | F116-AC-18, F116-AC-19 |
| 8 | `sa.admin` tắt `steps.spam` cho `pgd-demo` không `reason` | `422 MODERATION_SWITCH_REASON_REQUIRED` | F116-AC-21 |
| 9 | `sa.admin` tắt `steps.spam` cho `pgd-demo` có `reason`, `expires_at` +5 phút; `sa.admin2` duyệt; Dũng gửi dồn tin lặp; An gửi dồn tin lặp; Dũng gửi SĐT | Dũng: tin lặp gửi được, decision log `steps_skipped=[spam]`; An: bị chặn; SĐT của Dũng vẫn bị chặn; metric `policy_disabled` = 1 | F116-AC-20, F116-AC-22, F116-AC-23 |
| 10 | Tắt `enabled_all` cho `pgd-demo`; Dũng gửi tin quá dài, gửi dồn | `VALIDATION_FAILED`, `CHAT_RATE_LIMITED`; tin hợp lệ có `unchecked` | F116-AC-24 |
| 11 | Chờ `expires_at`, chạy `go -C src/backend run ./cmd/worker --profile=chat --job=moderation-switch-expirer --once` | Bước bật lại, history có `source = auto_expire`, audit | F116-AC-25 |
| 12 | An gửi 3 tin bị chặn trong 10 phút | Sanction `mute` hệ thống; tin kế tiếp `CHAT_MUTED`; `chat.sanction.applied` | F116-AC-07, F116-AC-08 |
| 13 | An (`vi-VN`) gửi quick message; Bình đặt `en-US` | Bình nhận `quick_message_id`, hiển thị `text.en`; `params` chứa SĐT → chặn | F116-AC-27, F116-AC-28 |
| 14 | Bình (giao diện `vi`) gửi câu tiếng Anh chứa term `en` placeholder | Bị xử lý theo danh mục `en` | F116-AC-29 |
| 15 | `sa.admin` yêu cầu tắt `steps.pii` (platform và `pgd-demo`); tắt `enabled_all` của `pgd-demo` (đã duyệt) rồi Dũng gửi SĐT | `422 MODERATION_SWITCH_STEP_NOT_ALLOWED`; SĐT vẫn bị chặn | F116-AC-30 |
| 16 | `sa.admin` tắt `steps.spam` không `expires_at`, rồi `expires_at` +30 giờ | `422 MODERATION_SWITCH_EXPIRY_REQUIRED` cả hai | F116-AC-31 |
| 17 | `sa.admin` tắt `steps.bullying` +2 giờ; `sa.admin` tự duyệt; `sa.admin2` duyệt; tạo yêu cầu khác và tua 25 giờ | `pending_approval`; `403 MODERATION_SWITCH_SELF_APPROVAL`; hiệu lực sau duyệt, audit hai người; yêu cầu thứ hai `expired` | F116-AC-32 |

## 3. Lệnh test tự động

```bash
make test MOD=chat
make test-integration MOD=chat
make test-latency MOD=chat                                                # p95/p99 theo moderation_pipeline_latency (không -race)
go -C src/backend test -run XXX -bench=Pipeline ./modules/chat/tests/bench/...  # ns/op, allocs/op
go -C src/backend test ./modules/chat/tests/static/...                  # không cờ tắt trong code/env, không từ thật trong seed
```

## 4. Bằng chứng cần nộp

- Kết quả test và benchmark p95, báo cáo hồi quy (recall/false positive trên bộ placeholder CI), trace_id kịch bản 3, ảnh audit kịch bản 9 và 11.
