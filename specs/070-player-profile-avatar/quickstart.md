# Quickstart: F070 — Hồ sơ người chơi, nickname, tạo avatar lần đầu, trang phục, emote

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F014, F062 (và F061 cho luồng Godot).
- Lệnh: `make up`, `make seed` (nạp `modules/game/seed/character_catalog.v1.json`), `make run-core-api`, `make run-worker PROFILE=events` ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Tài khoản demo: `hs.an.3a`, `hs.binh.3a` (cùng tenant `th-demo-a`), `hs.dung.7a` (tenant `pgd-demo`), `ad.truong.a` (nhân sự).
- Token game bằng `gameLogin` (F013) + `createGameSession` (F014).

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `hs.an.3a` lần đầu: `createGameSession` rồi `getGameProfile` | `setup_required = true`, `avatar_config = null` | F070-AC-01 |
| 2 | `getCharacterCatalog` | 6 preset, dáng, bảng màu, 6 emote mặc định, tên vi/en | F070-AC-02 |
| 3 | `updateAvatar` với preset M2 + màu tóc `hair_col_03` | `200`; `avatar.created` một lần; `emote_slots` = 6 emote mặc định | F070-AC-03, F070-AC-19 |
| 4 | `updateAvatar` với `skin_tone = tone_99` | `422 VALIDATION_FAILED` | F070-AC-04 |
| 5 | `suggestNicknames?count=6` (vi) rồi `updateGameProfile {nickname_option_id}` | 6 gợi ý ≤ 24 ký tự; nickname đặt; `setup_required = false`; `game_profile.updated` | F070-AC-07, F070-AC-08 |
| 6 | `hs.binh.3a` dùng option của `hs.an.3a`; gửi `{ "nickname": "Tên tự gõ" }` | `409 NICKNAME_UNAVAILABLE`; `422 VALIDATION_FAILED` | F070-AC-09, F070-AC-10 |
| 7 | `hs.an.3a` đổi nickname vượt giới hạn tháng | `409 NICKNAME_CHANGE_LIMIT` | F070-AC-11 |
| 8 | `updateAvatar` với `outfit.item_id` chưa sở hữu; phụ kiện `acc_cap_star` vào slot `back` | `422 ITEM_NOT_OWNED`; `422 EQUIP_SLOT_INVALID` | F070-AC-13, F070-AC-14 |
| 9 | `saveAvatarPreset` ô 1, 2, 3 rồi `listAvatarPresets` | 3 bộ của chính học sinh | F070-AC-16 |
| 10 | Token `ad.truong.a` gọi `getGameProfile`; `hs.an.3a` gửi `X-Tenant-Id` của `pgd-demo` | `403 GAME_ACCESS_STUDENT_ONLY`; `403 TENANT_MISMATCH` | F070-AC-21, F070-AC-22 |
| 11 | Godot: đăng nhập `hs.an.3a` (tài khoản reset) → S02 luồng `g1_3` → Quảng trường | ≤ 3 bước, không gõ phím, xem trước ≤ 100 ms | F070-AC-05, F070-AC-06 |
| 12 | Godot: mở emote wheel, chọn "Vẫy tay" | Clip phát một lần rồi về Locomotion | F070-AC-20 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-contract
make godot-test
```

## 4. Bằng chứng cần nộp

- Kết quả test (pass/fail), trace_id kịch bản 3 và 5, truy vấn `game.nickname_changes`, ảnh S02 trên DEV-PC-MID và DEV-AND-MID1.
