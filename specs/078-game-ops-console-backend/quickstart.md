# Quickstart: F078 — Game Operations Console backend: workflow nội dung game, thumbnail 3D, catalog cho nhà trường

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md)

## 1. Chuẩn bị

- Phụ thuộc đã Done: F071–F075 (module loại), F045 (identity gọi gán), F041 (projection catalog — cho kịch bản 12).
- Lệnh: `make up`, `make seed`, `make up-app EXCEPT=core-api`, `make run-core-api`, `make run-worker PROFILE=events`, container `game-renderer` (`docker compose -f deploy/compose/game-renderer.yml up`), `make render-thumbnails` để thử thủ công ([LOCAL_DEVELOPMENT](../../docs/11-operations/LOCAL_DEVELOPMENT.md)).
- Gọi qua gateway nội bộ (APISIX admin route local) bằng token `eduworld-portal` (chỉ mật khẩu, không MFA — ADR-0094).
- Tài khoản demo: `sa.admin` (system_admin kiêm `game.*`, author/publisher), `cm.duyet` (content_reviewer, `game.content.review`), `cm.noidung` (content_admin — không có `game.*`), `ad.truong.a`, `ad.truong.b`, `hs.an.3a`.

## 2. Kịch bản kiểm tra

| # | Bước | Kết quả mong đợi | Tiêu chí |
|---|---|---|---|
| 1 | `ad.truong.a` gọi `listGameConfigs_game` qua gateway public và gọi thẳng service | Không có route; `403` + `authz.denied` | F078-AC-01, F078-AC-02 |
| 2 | `cm.noidung` gọi `createGameConfig_game` (`quests`) | `403 PERMISSION_DENIED` | F078-AC-01 |
| 3 | `sa.admin` tạo quest draft payload sai schema rồi đúng schema | `422` chỉ rõ field; `201` + `ETag` | F078-AC-04 |
| 4 | `sa.admin` `updateGameConfigDraft_game` với `If-Match` cũ; không `If-Match` | `412`; `428` | F078-AC-05 |
| 5 | `submit_review` khi `scene_ref` không có trong client manifest; khi thiếu `en` | `422 ITEM_REF_INVALID`; `422 TRANSLATION_INCOMPLETE` | F078-AC-07 |
| 6 | `sa.admin` tự `approve`; `cm.duyet` `approve` | `403`; `approved` | F078-AC-08 |
| 7 | `sa.admin` publish ngay; `hs.an.3a` đang chơi version cũ | Con trỏ đổi, `content.published`, manifest tăng version; lượt cũ giữ version | F078-AC-11, F078-AC-15 |
| 8 | Tạo version mới từ bản publish (`createGameConfigVersion_game`), sửa, duyệt, hẹn giờ `publish_at` < lead rồi hợp lệ | `422`; `202`, publish đúng giờ | F078-AC-06, F078-AC-12 |
| 9 | `rollbackGameConfig_game` về v1 có lý do; về version không publish | Con trỏ v1, không version mới; `409 ROLLBACK_TARGET_INVALID` | F078-AC-13 |
| 10 | `unpublishGameConfig_game` quest đang được bài giao tham chiếu | Con trỏ rỗng, trả `usages[]`, audit có lý do | F078-AC-14, F078-AC-16 |
| 11 | `createGamePreviewSession` cho draft v3; dùng token đọc v2 | Token 30 phút; `403`; không có game session/XP | F078-AC-17 |
| 12 | Publish item có `.glb` | `render_thumbnail` + `render_turntable` gắn version; `thumbnail_url` trong list | F078-AC-19, F078-AC-20 |
| 13 | `sa.admin` gán quest cho `th-demo-a` qua `/admin/tenant-assignments` | `tenant_data.assigned`; manifest `th-demo-a` có quest; `ad.truong.a` thấy quest trong `listGameActivities`, `ad.truong.b` không | F078-AC-21, F078-AC-22, F078-AC-23 |

## 3. Lệnh test tự động

```bash
make test MOD=game
make test-integration MOD=game
make test-contract
make godot-test        # script render_thumbnails
```

## 4. Bằng chứng cần nộp

- Kết quả test theo mã AC (gồm test 403 sinh từ OpenAPI), trace_id kịch bản 7, đo thời gian publish → manifest, ảnh thumbnail/turntable kịch bản 12, audit kịch bản 6–10.
