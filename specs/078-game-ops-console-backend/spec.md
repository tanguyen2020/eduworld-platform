# Feature Specification: F078 — Game Operations Console backend: workflow nội dung game, thumbnail 3D, catalog cho nhà trường

**Feature Branch**: `078-game-ops-console-backend` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `game` · **Mốc**: M3 · **Phụ thuộc**: F071, F072, F073, F074, F075
**Input**: Backend `/api/v1/game-admin/*` của workspace A (platform-only): soạn bản nháp cấu hình game có version, workflow Draft → Review → Approved → Published → Archived với reviewer khác author, publish ngay/hẹn giờ, rollback đổi con trỏ, gỡ xuất bản, lịch sử phát hành, diff, preview Godot Web bằng token ngắn hạn, media nội dung game; render thumbnail/turntable 3D cho mọi version publish; gán quest/mini-game cho tenant và cung cấp dữ liệu để nhà trường chọn giao bài. FR nguồn: FR-CMS-004, FR-CMS-006, FR-CMS-017, FR-LMS-007.

## Clarifications

### Session 2026-11-16

- Q: Ranh giới với kernel cấu hình của F071 và module của F074–F077? → A: F071 cung cấp kho version + con trỏ + publish dùng cho seed; mỗi feature domain đăng ký module loại cấu hình (validate, `OnPublished`). F078 sở hữu toàn bộ HTTP `/game-admin/*`, workflow duyệt, hẹn giờ, rollback, unpublish, diff, preview, media, thumbnail và gán tenant.
- Q: Catalog quest/mini-game cho nhà trường (`listGameActivities`) do service nào phục vụ? → A: OpenAPI ghi `x-service: learning`; dữ liệu nằm ở module game. F078 phát `content.published` (kèm metadata catalog; schema owner F071) và mở endpoint internal để module learning dựng projection; `tenant_data.assigned/unassigned` do module identity phát sau khi gọi `applyTenantAssignment` (F045); endpoint public và projection thuộc F041. Lý do ở [research.md](research.md) R1.
- Q: Quyền rollback nội dung game? → A: `game.content.publish` (role `liveops`, ADMIN_PORTAL §2.3). ADMIN_PORTAL §3.11 ghi `content.release.rollback` cho Live-ops nhưng §2.3 không cấp permission đó cho `liveops` — lệch tài liệu, ghi báo cáo.
- Q: Loại config thuộc module khác trong enum `GameConfigType` (thi đấu, sticker/GIF)? → A: module game không phục vụ; path enum của module game đã loại chúng (competition: F108; chat: F122).

### Session 2026-11-17

- Q: Permission ghi cho `progression-curves`/`progression-levels`, `shop-items`, `collections`? → A: `game.progression.write` (thay `game.level.write`), `game.shop.write`, `game.collection.write`; `currencies` giữ `game.economy.write` (ADMIN_PORTAL §2.2, API_DESIGN, RB-309). Config type `levels` đổi thành `progression-levels` (RB-308).

### Session 2026-11-20 (owner chốt — việc nhận từ OQ F054-3)

- "Đang dùng ở đâu" của nội dung học tập (FR-CNT-008) gồm quest/mini-game/challenge → **Chốt:** payload `content.published` của cấu hình game mang `content_refs[]` (loại, id, version nội dung học tập được tham chiếu; schema owner F071 thêm field tùy chọn); module learning tiêu thụ và dựng projection `content.external_usages`, `getContentUsages` trả thêm `ref_type` `quest`, `minigame`, `challenge`. Không gọi đồng bộ (R29).

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F078-AC-01: điều kiện "MFA (`acr`/`amr`)" và mã `403 MFA_REQUIRED` bỏ; giữ `azp = eduworld-portal`, role platform, permission theo loại cấu hình (thiếu → `403 PERMISSION_DENIED` + audit). T001, T010, plan, quickstart nhắc MFA là lịch sử. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-22 (ADR-0094 — [Confirmed])

- Q: Văn bản còn mô tả MFA/`MFA_REQUIRED`/step-up như hành vi hiện hành? → A: [Confirmed] Viết lại theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md), giữ nguyên mã AC: US1, F078-AC-01, Constitution, SC-001. Route platform chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform; thiếu → `403 PERMISSION_DENIED` + `authz.denied`; không còn `403 MFA_REQUIRED` hay hộp đăng nhập lại (step-up). Câu trả lời Clarifications cũ nhắc MFA là lịch sử.

### Session 2026-11-24 (triển khai — owner "chốt hết", [Confirmed])

- Q: Lưu bản nháp/version workflow ở đâu khi bảng version của từng loại (F071–F077) chỉ chứa version đã publish và có trigger cấm sửa? → A: [Confirmed] Hai bảng chung `game.config_items` (UUID `config_id` của console + khóa bảng loại `item_key`, bản sao con trỏ `current_version_id`) và `game.config_versions` (mọi version của workflow; payload khóa từ `in_review`, trigger `game.config_version_guard`). Chỉ version publish được chép sang bảng loại qua lõi phát hành F071 (cùng `version_id`, cùng số version) — người chơi không bao giờ đọc bản nháp. Seed vẫn publish qua lõi; lõi ghi luôn dòng console (`config_versions` `published`). Migration backfill item/version đã publish.
- Q: Lõi F071 tự `archived` version publish cũ mỗi lần publish — rollback cần version `published` khác con trỏ? → A: [Confirmed] Bỏ tự archive: version publish cũ **giữ `published`** (đích rollback, ADR-0043); `archived` chỉ khi Game Designer `archive` (không phải con trỏ). Migration đổi các version bị lõi tự archive trước F078 về `published` (chỉ cột trạng thái, không xóa/sửa nội dung).
- Q: `config_id` của API là UUID nhưng `world-areas`, `story-chapters`, `items` dùng mã (`area_id`, `chapter_code`, `item_id`)? → A: [Confirmed] Loại khóa mã có `config_id` UUID riêng (map ở `config_items`); loại khóa UUID dùng chính khóa làm `config_id`. Khi tạo, UUID tác giả ghi trong payload (vd. file seed) được giữ nếu chưa có item; mã đã tồn tại → `409 CONFLICT` (dùng `createGameConfigVersion`).
- Q: Loại trong enum chưa có module loại (`progression-levels` sinh từ đường cong, `bosses`, `currencies`)? → A: [Confirmed] `404 NOT_FOUND` như loại của module khác (AC-03) cho tới khi có module loại; `challenges` (F076 đã đăng ký) được phục vụ.
- Q: Rollback/gỡ cho loại mà runtime không đi theo con trỏ? → A: [Confirmed] `events` (lịch run do F077 vận hành) không rollback/unpublish; `reward-rules` (dòng thời gian `effective_from`) không rollback, vẫn unpublish được — sửa bằng version mới; trả `422 ROLLBACK_TARGET_INVALID`.
- Q: Mã HTTP của `MEDIA_NOT_READY` (spec ghi 409) và `ROLLBACK_TARGET_INVALID` (spec ghi 409)? → A: [Confirmed] Theo catalog `ProblemCode` (một nguồn, R33): cả hai `422`.
- Q: AC-17 cần đường đọc bằng `preview_token` nhưng OpenAPI chỉ có tạo phiên? → A: [Confirmed] Thêm `getGamePreviewSnapshot` (`GET /game-admin/preview/snapshot`, header `X-Preview-Token`, bearer portal + quyền đọc loại); token chỉ đọc đúng version cho người tạo trong `preview_token_ttl`; khác → `403` + `authz.denied`.
- Q: Facade gán tenant là `applyTenantAssignment` mới hay các operation platform-ops sẵn có của F045? → A: [Confirmed] Dùng `applyTenantAssignments`/`revokeTenantAssignments`/`listResourceAssignments` của thư viện `internal-common` (đã có trong `game.internal.v1.yaml`, `x-allowed-callers: [identity]`) — không thêm operation trùng; `ResourceType` thêm `shop_item`, `event_template`; module identity định tuyến các loại game tới facade game (`GameAssign`). Module game ghi audit `tenant_data.assigned/unassigned`, **không** phát event `tenant_data.*` (owner F045). Gán chỉ khi item có con trỏ (`409 CONTENT_NOT_PUBLISHED`); dựng lại manifest các tenant đổi trong cùng transaction.
- Q: Kiểm "nội dung học tập tham chiếu đã publish" khi `submit_review` (bàn giao của F076 cho activity của challenge; quest, mini-game)? → A: [Confirmed] Facade mới của module learning `checkPublishedLearningContent` (`x-allowed-callers: [game]`, chỉ đọc, item platform có con trỏ và — khi ghim — version đã publish); chưa publish → `422 ITEM_REF_INVALID`; learning không trả lời → `503`.
- Q: Client manifest (`game.client_manifests`) chưa có nguồn ghi? → A: [Confirmed] Tạo bảng; `submit_review` kiểm `area_id`/`*_area_id`, `npc_id`/`*_npc_id`, `scene_ref`, `interactable_id(s)` với bản mới nhất **mỗi nền tảng** đã đăng ký; chưa đăng ký bản nào thì chỉ kiểm tham chiếu nội dung đã publish (lõi F071). Đăng ký manifest khi build Godot thuộc nhóm F (pipeline build), ngoài phạm vi.
- Q: Media game không có bước "báo upload xong"? → A: [Confirmed] Job `game-media-pipeline` (worker jobs) kiểm file đã tới quarantine (kích thước, `sha256`, magic bytes, quét `clamd` khi `EW_MEDIA_SCAN=clamd://…`) rồi ghi bản gốc `media/<sha256>/original.<ext>` (biến thể `original`); hết `media_presign_ttl` không có file → `rejected upload_missing`. Payload tham chiếu media bằng `*_asset_id` hoặc `asset:<uuid>`.
- Q: Lưu thumbnail/turntable vào cột của version publish (trigger bất biến)? → A: [Confirmed] Không sửa version publish: render được ghi `game.media_assets` (`render_*`) + `game.media_usages` (`usage = render_thumbnail/render_turntable`) gắn version; `thumbnail_url`/`turntable_url` đọc từ đó (rỗng khi chưa có). Turntable là sprite sheet 24 khung 6×4 (3072×2048). Retry backoff lũy thừa (`game_render_retry_backoff`), sau `game_render_max_attempts` → `dead` + `infra.dead_letters` + metric.
- Q: Hai module (game, chat) cùng phục vụ path `/game-admin/{config_type}` trong `core-api`? → A: [Confirmed] `httpx.Server.HandleRouted` định tuyến theo giá trị `config_type` (mỗi module giữ chain riêng: idempotency store, nhãn metric); giá trị không ai phục vụ → `404` sau xác thực.
- Q: F073 bàn giao (publish seed mini-game, `content.rolled_back` cho projection learning)? → A: [Confirmed] `make seed-game-content` (`modules/game/cmd/seed-content`) đi qua console (create → submit → approve → publish, kiểm nguồn câu bằng `validateMinigameQuestionSource`) rồi gán tenant qua `/admin/tenant-assignments`; consumer `learning.minigame_config_projection` nhận thêm `content.rolled_back`/`content.unpublished` và giữ con trỏ `ref.minigame_config_pointers` (activity không ghim version đi theo rollback; gỡ → không có version hiện hành).
- Q: T040 (`content_refs[]`)? → A: [Confirmed] Phía game xong: `content.published` mang `content_refs[]` (activity/question set/lesson, version khi ghim). Projection `content.external_usages` và `ref_type` của `getContentUsages` là việc của consumer module learning — chuyển thành việc tiếp theo của F041/F054 (ghi CODE_PLAN).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Ranh giới quyền của Game Operations Console (Priority: P1)

Chỉ đội platform (role platform, gateway nội bộ) mới soạn, duyệt, phát hành nội dung game; nhân sự trường không có đường nào vào.

**Why this priority**: Cấu hình game platform-only là quyết định kiến trúc (ADR-0027, R20); mọi story sau dựa vào đó.

**Independent Test**: Gọi mọi operation `/game-admin/*` với token `ad.truong.a`, token nhân sự không có role platform, token `game_designer` cho loại ngoài quyền → `403` + `authz.denied`; qua gateway public → không có route.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F078-AC-01 | HỆ THỐNG PHẢI yêu cầu cho mọi operation `/game-admin/*`, `/game-media*`: `azp = eduworld-portal`, role platform và permission theo loại cấu hình (không MFA — ADR-0094); thiếu điều kiện PHẢI trả `403 PERMISSION_DENIED` và ghi audit `authz.denied` | FR-CMS-004 · AC-CMS-001 · ADR-0027 | TC-CMS-001 |
| F078-AC-02 | Gateway public (APISIX) PHẢI không có route `/api/v1/game-admin/*`, `/api/v1/game-media*`; gọi thẳng module game bằng token tenant PHẢI nhận `403` | AC-CMS-001 · AGENTS R20 | TC-CMS-001 |
| F078-AC-03 | HỆ THỐNG PHẢI lưu mọi cấu hình game với `tenant_id = NULL` (`owner_scope = platform`), bỏ qua `X-Tenant-Id`/`tenant_ids` trên route này; `config_type` không thuộc module game PHẢI trả `404 NOT_FOUND` | FR-CMS-004 · ADR-0027 | — |

---

### User Story 2 — Soạn bản nháp và workflow duyệt (Priority: P1)

Game Designer soạn quest, mini-game, NPC, reward, item, challenge, sự kiện… dưới dạng bản nháp có version; gửi duyệt; Content Reviewer khác người soạn duyệt hoặc trả lại có lý do.

**Why this priority**: FR-CMS-006 — không có workflow thì nội dung game không vào được sản phẩm.

**Independent Test**: `sa.admin` tạo quest draft → `submit_review` → `sa.admin` tự `approve` → `403`; `cm.duyet` `approve` → `approved`; sửa version đã publish → `409 CONTENT_NOT_EDITABLE`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F078-AC-04 | KHI người có quyền ghi của loại gọi `createGameConfig_game` với `Idempotency-Key`, module game PHẢI tạo item và version 1 `draft` (`author_id`, `ETag`) sau khi payload qua JSON Schema và kiểm tra domain của loại; vi phạm PHẢI trả `422 VALIDATION_FAILED` chỉ rõ field | FR-CMS-006 · FR-CMS-004 | TC-CMS-001 |
| F078-AC-05 | KHI gọi `updateGameConfigDraft_game` với `If-Match` khớp, module game PHẢI cập nhật version `draft`; NẾU version không ở `draft` THÌ PHẢI trả `409 CONTENT_NOT_EDITABLE`; `ETag` lệch PHẢI trả `412 VERSION_MISMATCH`; thiếu `If-Match` PHẢI trả `428 PRECONDITION_REQUIRED` | FR-CMS-006 · AGENTS R11 | TC-CMS-002 |
| F078-AC-06 | KHI người có quyền ghi gọi `createGameConfigVersion_game` trên item đã publish, module game PHẢI tạo version mới `draft` sao chép từ version chỉ định (`based_on_version_id`); version đã publish PHẢI giữ bất biến | FR-CMS-006 · ADR-0043 | TC-CMS-002 |
| F078-AC-07 | KHI `transitionGameConfigVersion_game` `submit_review`, module game PHẢI kiểm: payload đầy đủ, media `ready` (`409 MEDIA_NOT_READY`), `area_id`/`npc_id`/`scene_ref` có trong client manifest hiện hành và nội dung tham chiếu đã publish (`422 ITEM_REF_INVALID`), đủ `vi`/`en` (`422 TRANSLATION_INCOMPLETE`); rồi chuyển `in_review`, phát `content.version_submitted` | FR-CMS-006 · NFR-I18N-002 · [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001 |
| F078-AC-08 | KHI người có `game.content.review` `approve` hoặc `reject` (lý do bắt buộc), module game PHẢI chuyển `approved`/`rejected`, phát `content.version_approved`/`content.version_rejected`; NẾU người duyệt là author THÌ PHẢI trả `403 PERMISSION_DENIED`; chuyển không hợp lệ PHẢI trả `422 INVALID_STATE_TRANSITION`; `archive` version đang là con trỏ PHẢI bị từ chối | FR-CMS-006 · [ADMIN_PORTAL §4](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-001 |
| F078-AC-09 | KHI gọi `listGameConfigs_game`, `getGameConfigVersion_game`, `listGameConfigVersions_game`, `diffGameConfigVersions_game`, module game PHẢI trả dữ liệu cho người có quyền ghi của loại, `game.content.review` hoặc `game.content.publish`; diff PHẢI liệt kê thay đổi theo field path | FR-CMS-006 | — |
| F078-AC-10 | NẾU `title` vượt 200 ký tự, `note`/`change_note`/`reason` vượt giới hạn của spec, hoặc payload vượt kích thước cho phép, THÌ module game PHẢI trả `422 VALIDATION_FAILED` | ADR-0041 | TC-TEN-024 |

---

### User Story 3 — Phát hành, rollback, gỡ xuất bản (Priority: P1)

Live-ops phát hành ngay hoặc hẹn giờ; khi phát hiện lỗi, dùng lại phiên bản cũ hoặc gỡ xuất bản kèm lý do; học sinh đang chơi không bị cắt.

**Why this priority**: Đưa nội dung tới học sinh an toàn (ADR-0043).

**Independent Test**: Publish quest v2 khi `hs.an.3a` đang chơi v1 → lượt cũ giữ v1, lượt mới nhận v2; rollback về v1 → con trỏ v1, không tạo version mới.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F078-AC-11 | KHI người có `game.content.publish` gọi `publishGameConfigVersion_game` (version `approved`, `If-Match`, `Idempotency-Key`) không có `publish_at`, module game PHẢI dựng snapshot, đổi con trỏ, ghi phát hành, chạy `OnPublished` của loại, phát `content.published`, dựng lại manifest các tenant liên quan (`content.manifest_updated`) trong `publish_to_visible_max` | FR-CMS-006 · ADR-0043 | TC-CMS-003 |
| F078-AC-12 | KHI có `publish_at` ≥ hiện tại + `scheduled_publish_min_lead`, module game PHẢI trả `202`, phát `content.publish_scheduled` và job PHẢI phát hành đúng giờ (±1 phút) một lần; `publish_at` sớm hơn PHẢI trả `422 VALIDATION_FAILED`; hủy hẹn giờ trước giờ chạy PHẢI được ghi audit | FR-CMS-006 · [ADMIN_PORTAL §3.11](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-003 |
| F078-AC-13 | KHI gọi `rollbackGameConfig_game` với lý do, module game PHẢI chỉ đổi con trỏ sang version `published` khác con trỏ hiện tại (không tạo version), phát `content.rolled_back`, dựng lại manifest; đích không hợp lệ PHẢI trả `409 ROLLBACK_TARGET_INVALID` | FR-CMS-006 · AGENTS R31 | TC-CMS-003 |
| F078-AC-14 | KHI gọi `unpublishGameConfig_game` với lý do, module game PHẢI đặt con trỏ rỗng, gỡ khỏi manifest, phát `content.unpublished` và trả danh sách nơi đang tham chiếu (quest/sự kiện/bài giao) | FR-CMS-006 · ADR-0043 | TC-CMS-004 |
| F078-AC-15 | TRONG KHI một lượt chơi đã ghim version (quest progress, level session, challenge attempt), module game PHẢI tiếp tục phục vụ đúng version đó sau publish/rollback/unpublish; lượt mới PHẢI nhận version của con trỏ | FR-CMS-006 · AC-CNT-001 | TC-CNT-001 |
| F078-AC-16 | HỆ THỐNG PHẢI ghi audit cho mọi thao tác ghi (`content.version_created`, `content.submitted`, `content.approved`, `content.rejected`, `content.published`, `content.publish_scheduled`, `content.rolled_back`, `content.unpublished`, `content.archived`, `game_config.changed` kèm `diff_summary`, `reason`) và `listGameConfigReleases_game` PHẢI trả dòng thời gian phát hành | FR-CMS-006 · [ADMIN_PORTAL §7](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-004 |

---

### User Story 4 — Preview và media nội dung game (Priority: P2)

Reviewer xem bản nháp chạy trong Godot Web trước khi duyệt; Game Designer tải ảnh, âm thanh, model 3D cho nội dung game.

**Why this priority**: Giảm lỗi hiển thị trước publish; media là đầu vào thumbnail.

**Independent Test**: `createGamePreviewSession` cho quest v3 draft → token 30 phút đọc được v3, không đọc v2; không phát sinh game session/XP; upload `.glb` 60 MB → `413`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F078-AC-17 | KHI gọi `createGamePreviewSession`, module game PHẢI cấp `preview_token` hạn 30 phút chỉ đọc đúng version chỉ định; dùng token cho version/item khác PHẢI trả `403`; preview PHẢI không tạo game session, attempt, XP hay learning event | FR-CMS-006 · AGENTS R23 · [ADMIN_PORTAL §5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F078-AC-18 | KHI gọi `presignGameMediaUpload`, module game PHẢI tạo asset `quarantined` và URL tải lên giới hạn loại/dung lượng theo CONTENT_MODEL §7; loại sai PHẢI trả `415`, quá dung lượng `413`; version tham chiếu asset chưa `ready` PHẢI không gửi duyệt được; `getGameMediaAsset` PHẢI trả trạng thái và biến thể | FR-CMS-006 · ADR-0044 | — |

---

### User Story 5 — Thumbnail và turntable 3D tự động (Priority: P2)

Mỗi version publish của item, NPC, khu vực, boss có ảnh tĩnh và ảnh xoay 360° render từ chính asset của game để nhận diện trong CMS.

**Why this priority**: FR-CMS-017; CMS không phải tải Godot chỉ để nhận diện nội dung.

**Independent Test**: Publish item có model 3D → asset `render_thumbnail` 512×512 và `render_turntable` 24 khung gắn version; tắt renderer → publish vẫn thành công, job retry rồi DLQ.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F078-AC-19 | KHI version có asset 3D (item, NPC, khu vực, boss, quest marker) được publish, worker module game PHẢI render bằng Godot headless ảnh 512×512 và turntable 24 khung từ đúng asset của version, lưu object storage theo khóa `media/<sha256>/…`, đăng ký media `render_thumbnail`/`render_turntable` gắn version; chạy lại PHẢI không tạo bản trùng | FR-CMS-017 · ADR-0047 · [DESIGN_SYSTEM §7](../../docs/05-ux/DESIGN_SYSTEM.md) | — |
| F078-AC-20 | NẾU render lỗi, THÌ module game PHẢI không chặn hay hoàn tác publish, retry có backoff, sau số lần tối đa chuyển DLQ và tăng metric; danh sách/version PHẢI trả `thumbnail_url` khi asset `ready` và để trống khi chưa có | FR-CMS-017 · NFR-AVAIL-005 | — |

---

### User Story 6 — Gán nội dung game cho tenant và catalog nhà trường (Priority: P2)

System Admin gán quest/mini-game đã publish cho tenant; nhà trường chỉ thấy và chọn giao những nội dung được gán, không sửa được.

**Why this priority**: FR-LMS-007; nối Game Operations Console với LMS.

**Independent Test**: Gán quest Q cho `th-demo-a` → manifest và catalog của `th-demo-a` có Q, `pgd-demo` không có; gỡ gán → Q biến mất khỏi catalog, lượt chơi cũ vẫn xem được.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F078-AC-21 | KHI module identity gọi `applyTenantAssignment` (assign/unassign; `resource_type` quest, minigame_config, item, shop_item, event_template) với `Idempotency-Key`, module game PHẢI ghi/soft-delete dòng gán, dựng lại manifest tenant và trả kết quả để module identity phát `tenant_data.assigned`/`tenant_data.unassigned` (owner F045; module game PHẢI KHÔNG tự phát event này); NẾU module ngoài `x-allowed-callers` (`identity`) import facade này, `make arch-check` PHẢI fail; tài nguyên chưa publish PHẢI trả `409 CONTENT_NOT_PUBLISHED` | FR-LMS-007 · ADR-0029 | TC-TEN-019 |
| F078-AC-22 | KHI quest hoặc mini-game config được publish/rollback, `content.published`/`content.rolled_back` PHẢI mang metadata catalog (tiêu đề song ngữ, `grade_levels`, `subject_id`, `objective_ids`, `estimated_minutes`, `thumbnail_url`); nội dung chưa gán cho tenant PHẢI không xuất hiện trong manifest hay dữ liệu catalog của tenant đó | FR-LMS-007 · AGENTS R8 | TC-TEN-019 |
| F078-AC-23 | KHI module learning gọi `listAssignedGameActivities` theo tenant (phân trang cursor), module game PHẢI trả quest/mini-game đã publish và đang gán cho tenant đó cùng metadata catalog; NẾU module ngoài `x-allowed-callers` (`learning`) import facade này, `make arch-check` PHẢI fail | FR-LMS-007 · [DATABASE_DESIGN §2.17](../../docs/07-backend/DATABASE_DESIGN.md) | — |

### Edge Cases

- NẾU hai người publish cùng item đồng thời, THÌ `If-Match` làm một bên nhận `412`, con trỏ đổi đúng một lần.
- NẾU publish hẹn giờ nhưng version bị archive trước giờ chạy, THÌ job bỏ qua, ghi audit `content.publish_scheduled` bị hủy.
- NẾU client manifest mới bỏ một `scene_ref` đang được nội dung published tham chiếu, THÌ báo cáo cảnh báo cho live-ops; nội dung đã publish không tự gỡ.
- NẾU unpublish quest đang được bài giao (`assignment`) tham chiếu, THÌ bài giao giữ version ghim; quest không xuất hiện cho giao mới.
- NẾU System Admin (kiêm `game.*`) tự soạn và tự duyệt, THÌ vẫn bị chặn reviewer ≠ author.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CMS-004 | API Game Content platform-only `/api/v1/game-admin/*` cho mọi loại cấu hình game | [ADMIN_PORTAL §3.3, §3.12](../../docs/04-admin-portal/ADMIN_PORTAL.md) · [ADR-0027](../../docs/adr/ADR-0027-cms-tach-vung-tenant-chi-thao-tac-learning-management-cau-hi.md) |
| FR-CMS-006 | Workflow, version, preview, publish ngay/hẹn giờ, rollback, unpublish, audit | [CONTENT_MODEL §5–6](../../docs/02-learning/CONTENT_MODEL.md) · [ADR-0043](../../docs/adr/ADR-0043-phat-hanh-noi-dung-draft-publish-con-tro-phien-ban-giu-phien.md) |
| FR-CMS-017 | Thumbnail/turntable render từ asset 3D cho mọi version publish | [DESIGN_SYSTEM §7](../../docs/05-ux/DESIGN_SYSTEM.md) · [ADR-0047](../../docs/adr/ADR-0047-hai-he-thong-giao-dien-game-godot-3d-va-admin-ops-cms-lms.md) |
| FR-LMS-007 | Gán quest/mini-game cho tenant, dữ liệu catalog chỉ đọc cho nhà trường (endpoint `listGameActivities`: F041) | [SCHOOL_LMS](../../docs/04-admin-portal/SCHOOL_LMS.md) · [ADR-0029](../../docs/adr/ADR-0029-system-admin-gan-du-lieu-cho-tenant-va-clone-du-lieu-giua-te.md) |

### Key Entities

- **GameConfigItem / GameConfigVersion**: một nội dung game platform (quest, mini-game, NPC, item, reward rule, challenge, sự kiện…) và các version có trạng thái workflow.
- **ContentRelease**: lịch sử con trỏ (publish, rollback, unpublish, hẹn giờ).
- **PreviewSession**: quyền đọc ngắn hạn một version nháp cho Godot Web.
- **MediaAsset (game)**: file nội dung game ở object storage, gồm ảnh render thumbnail/turntable.
- **TenantResourceAssignment**: gán tài nguyên platform cho tenant.
- **ClientManifest**: danh sách area/NPC/scene có trong bản build client, dùng kiểm tham chiếu.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `scheduled_publish_min_lead` | Thời gian tối thiểu của hẹn giờ |
| `publish_to_visible_max` | SLO từ publish tới học sinh mới nhận |
| `assets_per_version_max` | Số asset tối đa mỗi version |
| `game_text_required_languages` | Ngôn ngữ bắt buộc trước publish |
| `idempotency_record_ttl` | Lưu kết quả thao tác có `Idempotency-Key` |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Route chỉ gateway nội bộ, `eduworld-portal` + role platform; chặn ở backend | AGENTS R7, R20 · ADR-0027 |
| Version publish bất biến, rollback chỉ đổi con trỏ, lượt chơi ghim version | AGENTS R11, R31 · ADR-0043 |
| Media chỉ object storage, khóa `media/<sha256>` bất biến | ADR-0044 · AGENTS R31 |
| Nội dung game không bao giờ thuộc tenant; gán theo ADR-0029 | AGENTS R22 · ADR-0029 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% operation `/game-admin/*` trả `403` với mọi role tenant và với token nhân sự không có role platform (test sinh từ OpenAPI).
- **SC-002**: 95% lần publish tới học sinh mới nhận trong thời gian `publish_to_visible_max`.
- **SC-003**: 0 lượt chơi đang ghim bị đổi version sau publish/rollback trong test 1.000 lượt.
- **SC-004**: ≥ 99% version publish có asset 3D có thumbnail trong ≤ 10 phút trên staging.
- **SC-005**: 0 nội dung chưa gán xuất hiện trong manifest/catalog của tenant khác.

## Ngoài phạm vi

- Giao diện workspace A (builder, ContentEditorShell, GamePreviewFrame) → F079; Godot Web export cho preview (client) → nhóm F.
- Endpoint public `listGameActivities` và projection catalog ở module learning, giao quest cho lớp → F041/F055.
- API `/admin/tenant-assignments` và saga gán/clone ở module identity → F045; UI gán → F057.
- Kiểm tra domain từng loại cấu hình → F071–F077, F103 (đăng ký module); vận hành sự kiện đang chạy → F077.
- Cấu hình thi đấu → F108; sticker/GIF → module chat (F122).
- Dịch nội dung game (`exportGameTranslations`, `importGameTranslations`, `getGameTranslationJob`) — chưa có feature trong danh mục; đề xuất bổ sung (ghi báo cáo).
- Game Analytics, Live dashboard → F080.

## Assumptions

- Kernel kho version + publish (dùng cho seed) có từ F071; F078 dùng lại `gameconfig.Publisher` cho đường HTTP.
- `listGameActivities` giữ `x-service: learning` và module learning dựng projection từ event của F078 [Confirmed]; nếu đổi sang module game phục vụ trực tiếp cần sửa OpenAPI và F041.
- Renderer chạy trong container riêng `game-renderer` (Godot headless) do worker module game điều phối qua hàng đợi [Confirmed] ([research.md](research.md) R2).
- Local chưa có tài khoản `game_designer`, `liveops`; quickstart dùng `sa.admin` (kiêm `game.*`) và `cm.duyet` (`game.content.review`); task Polish bổ sung tài khoản demo.
