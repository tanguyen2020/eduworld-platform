# Quickstart: F016 — Vòng đời tài khoản: khóa, thu hồi truy cập, dọn học sinh không đăng nhập, ẩn danh hóa

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F011, F014, F015.
- Lệnh: `make up`, `make seed`, `make up-app`, chạy native khi cần `make run-worker PROFILE=jobs` (job `student-inactive-cleanup`, `retention-anonymizer`, saga) hoặc `make run-worker PROFILE=events` (consumer) ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `ad.truong.a`, `ad.truong.b`, `ad.donvi`, `sa.admin`; học sinh `hs.binh.3a`, `hs.chi.3b`, `hs.lau.khongvao`, `hs.sapbidon`, `hs.locked`.
- Cài đặt tenant `th-demo-a`: `inactive_student_cleanup_days` = 90 (qua F015); Clock giả cho worker trong test.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.binh.3a` đang chơi; `ad.truong.a` `lockTenantUser` (`reason`, `Idempotency-Key`) | `locked`, audit, `user.status_changed` + `user.access_revoked` | F016-AC-01 |
| 2 | `hs.binh.3a` gọi API game sau ≤ 60 s | `403 ACCOUNT_LOCKED`; game session `account_locked` (consumer F014, kiểm ở bước 13); Keycloak `enabled=false` ≤ 1 phút | F016-AC-02, F016-AC-03 |
| 3 | `ad.truong.b` khóa `hs.binh.3a` | `404` | F016-AC-05 |
| 4 | `unlockTenantUser` rồi `gameLogin` | Đăng nhập được, `user.access_restored` | F016-AC-04 |
| 5 | Import SIS `transferred_out` cho `hs.chi.3b` đang chơi | Phiên bị thu hồi; tạo session sau bị từ chối | F016-AC-07, F016-AC-08 |
| 6 | Chạy `student-inactive-cleanup --once` (Clock giả) | `hs.sapbidon` trong preview + `student.cleanup_scheduled`; `hs.lau.khongvao` bị soft-delete `inactive_cleanup`; trên Keycloak user vẫn `enabled`, không còn Organization membership/phiên; token cũ bị từ chối | F016-AC-09, F016-AC-10, F016-AC-13 |
| 7 | `ad.truong.a` loại trừ một học sinh, chạy lại | Học sinh không bị dọn, audit | F016-AC-11 |
| 8 | Học sinh ứng viên đang có game session | module game trả `blocked` (`active_game_session`); saga bù; tính `skipped_active` | F016-AC-14 |
| 9 | Tạo dữ liệu để vượt ngưỡng duyệt, chạy worker; `sa.admin` duyệt | `awaiting_approval` + alert; sau duyệt mới dọn | F016-AC-12 |
| 10 | Tắt module game giữa saga rồi bật lại | Saga thử lại, hội tụ, không bước nào chạy hai lần | F016-AC-15 |
| 11 | Học sinh đã dọn đăng nhập lại (còn active SIS) | Keycloak xác thực được (user vẫn `enabled`); `student.restored`, membership Organization thêm lại, dữ liệu cũ dùng lại | F016-AC-17 |
| 13 | `hs.an.3a` đăng nhập game (F013) + tạo session (F014); `ad.truong.a` khóa; lặp lại với SIS `suspended` và với dọn thật (Clock giả) | Trong ≤ 1 phút: session `account_locked`/`cleanup`; refresh → `401 REFRESH_TOKEN_INVALID`; event đúng schema | F016-AC-24 |
| 14 | `ad.truong.a` gọi `resetStudentPassword` cho `hs.an.3a` (đang chơi) với `reason`, rồi gọi lại cùng `Idempotency-Key`; `hs.an.3a` tự gọi | `200` + `initial_password` (một lần, `no-store`); máy đang chơi bị đóng WS `4403` sau `session.session.ended` (`password_changed`), refresh → `401 REFRESH_TOKEN_INVALID`, đăng nhập lại bằng mật khẩu mới được; lần gọi lại không có mật khẩu (`password_redacted`); học sinh tự gọi → `403` | F016-AC-25, F016-AC-26, F016-AC-27 |
| 12 | `sa.admin` `createDataRequest` ẩn danh hóa học sinh `deactivated` | `status = anonymized`, username `anon.<user_id>`, erasure log, không `DELETE` | F016-AC-20, F016-AC-21 |

## 3. Lệnh test tự động

```bash
make test MOD=identity
make test-integration MOD=identity
make test-integration MOD=game
make test-integration MOD=learning
make test-contract
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), đo độ trễ thu hồi (REST, Keycloak), log saga chaos, log truy vấn chứng minh không có `DELETE`.
