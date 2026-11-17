# Quickstart: F095 — Bạn bè, block, mute (module social)

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F090, F091 (cho US4), F070, F010/F011.
- Lệnh ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)): `make up`, `make seed`, `make seed-check`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=chat`.
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a`, `hs.chi.3b` (`th-demo-a`); `hs.dung.7a` (THCS Demo B) và `hs.em.4a` (TH Demo C) cùng tenant `unit` `pgd-demo`; `ad.truong.a`.
- Token game lấy bằng `gameLogin`; gửi `X-Tenant-Id` của phiên.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` `sendFriendRequest` tới `hs.binh.3a` (key K1); gửi lại K1 | `201 pending`, event `friend_request.sent`, audit; lần 2 cùng response | F095-AC-01, F095-AC-04 |
| 2 | `hs.binh.3a` `listFriendRequests?direction=incoming` rồi `acceptFriendRequest` | Thấy lời mời; `204`; `friend.added`; cả hai có trong `listFriends` | F095-AC-05, F095-AC-11, F095-AC-13 |
| 3 | `hs.an.3a` gửi tới `hs.dung.7a` (tenant khác) | `404` | F095-AC-02 |
| 4 | `hs.dung.7a` gửi tới `hs.em.4a` (khác trường, `pgd-demo` chưa bật liên trường); bật `cross_school_social_enabled` rồi gửi lại | `403 SOCIAL_SCOPE_VIOLATION`; sau khi bật → `201` | F095-AC-02 |
| 5 | `hs.chi.3b` gửi tới `hs.an.3a`, `hs.an.3a` từ chối; `hs.chi.3b` xem outgoing | Vẫn `pending`, không thông báo; sau `friend_request_ttl` (job `--once` với Clock giả) → `expired` | F095-AC-06, F095-AC-07 |
| 6 | `hs.an.3a` `createFriendCode`; `hs.chi.3b` `redeemFriendCode`; nhập mã sai | `201` mã 6–12 ký tự; lời mời `source = friend_code`; `404 FRIEND_CODE_INVALID` | F095-AC-08 |
| 7 | Thử seed 11 lời mời pending từ một học sinh | Lời mời thứ 11 → `409 FRIEND_LIMIT_REACHED` | F095-AC-03 |
| 8 | Duyệt OpenAPI tìm endpoint tìm kiếm người chơi | Không có | F095-AC-09 |
| 9 | Token `ad.truong.a` gọi `listFriends`, `blockUser` | `403 GAME_ACCESS_STUDENT_ONLY` | F095-AC-10 |
| 10 | `hs.an.3a` `removeFriend` `hs.binh.3a` | `204`, `friend.removed`, không thông báo | F095-AC-12 |
| 11 | `hs.an.3a` `blockUser` `hs.chi.3b` (đang có lời mời chờ); `hs.chi.3b` gửi lời mời | Lời mời `cancelled`, `user.blocked`; `403 SOCIAL_SCOPE_VIOLATION` | F095-AC-14, F095-AC-15 |
| 12 | `unblockUser`, `listBlocks`; chặn chính mình | Không khôi phục bạn; `422` | F095-AC-16, F095-AC-18, F095-AC-19 |
| 13 | `muteUser`/`unmuteUser` `hs.binh.3a`, `listMutes` | `user.muted`/`user.unmuted`; danh sách đúng | F095-AC-17 |
| 14 | `hs.an.3a`, `hs.binh.3a` là bạn; `hs.binh.3a` mở WS (F090) rồi gửi `presence.status.set` với `hidden = true` | `hs.an.3a` nhận `presence.friend.updated online`, sau đó `offline`; `hs.chi.3b` không nhận gì | F095-AC-20, F095-AC-21, F095-AC-22 |
| 15 | Học sinh tenant `common` gửi lời mời kết bạn và tạo mã bạn bè | `403 SOCIAL_SCOPE_VIOLATION`; không có lời mời | F095-AC-23 |

## 3. Lệnh test tự động

```bash
make test MOD=chat
make test-integration MOD=chat
make test-contract          # event social → competition consumer
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 1 và 11, danh sách event trong `infra.outbox_events` sau kịch bản 11, kết quả test isolation sinh từ OpenAPI.
