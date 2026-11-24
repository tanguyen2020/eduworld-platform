# Feature Specification: F079 — Workspace A — Game Operations Console UI (world, NPC, quest builder, reward, item, event, season)

**Feature Branch**: `079-game-ops-console-ui` · **Created**: 2026-11-16 · **Status**: Done · **Module**: `src/cms/frontend/apps/portal` (workspace A `game-ops`) · **Mốc**: M3 · **Phụ thuộc**: F051, F077, F078
**Input**: Giao diện workspace A (chỉ platform, gateway nội bộ + role platform; không MFA — ADR-0094) cho Game Designer/Live-ops/System Admin: danh sách và editor có version chung (`ContentEditorShell`) cho mọi `GameConfigType` của module game; builder đặc thù cho quest, hội thoại NPC, thế giới/area, phần thưởng, item/shop/tiền tệ, level & XP, Event Builder 8 bước, season track, template sự kiện; thumbnail/turntable 3D; preview Godot Web. FR nguồn: FR-CMS-004, FR-CMS-017, FR-EVT-002, FR-GAM-007.

## Clarifications

### Session 2026-11-16

- Q: API dùng cho mọi builder? → A: Bộ chung `/api/v1/game-admin/{config_type}` (`listGameConfigs`, `createGameConfig`, `getGameConfigVersion`, `updateGameConfigDraft`, `transitionGameConfigVersion`, `listGameConfigVersions`, `diffGameConfigVersions`, `publishGameConfigVersion`, `rollbackGameConfig`, `unpublishGameConfig`, `listGameConfigReleases`) — payload validate theo JSON Schema của loại ([API_DESIGN §9.13b](../../docs/07-backend/API_DESIGN.md)).
- Q: Thumbnail 3D lấy ở đâu? → A: module game render khi publish (F078, [DESIGN_SYSTEM §7](../../docs/05-ux/DESIGN_SYSTEM.md)); `GameConfigVersion` chưa có trường thumbnail → delta (owner F078).
- Q: Mô phỏng "chơi N phút/ngày đạt level nào", "chơi N ngày/tuần đạt cấp nào" tính ở đâu? → A: Ở server (R1: không business logic trong UI) qua `simulateGameConfig` (delta, owner F074/F077/F078).
- Q: Cấu hình thi đấu, sticker/GIF, giám sát sự kiện đang chạy? → A: Thi đấu: F108; giám sát/dừng khẩn cấp sự kiện (workspace E): F080; sticker/GIF chưa có feature (báo cáo). F079 chỉ dựng khung chung để các loại này dùng lại.

### Session 2026-11-17

- Q: Permission ẩn/hiện builder Level & XP, Shop, Bộ sưu tập? → A: `game.progression.write`, `game.shop.write`, `game.collection.write` (ADMIN_PORTAL §2.2, RB-309; thay `game.level.write`); config type bảng level là `progression-levels`.

### Session 2026-11-20 (owner chốt — việc nhận từ F082)

- Game Dashboard ([TBD] ở ADMIN_PORTAL, F082 Ngoài phạm vi) → **Chốt:** F079 làm Game Dashboard tối thiểu của workspace A: số cấu hình theo trạng thái workflow và số item cần dịch (`translation_status` ≠ `complete`, từ `listGameConfigs` có field của F082). Không có số liệu chơi (thuộc F080).

### Session 2026-11-21 (owner bỏ MFA — [Confirmed])

- Q: Owner bỏ MFA/TOTP cho đăng nhập Admin Portal với mọi tài khoản, kể cả role platform (2026-11-21, [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md)) — phần nào của feature này đổi? → A: F079-AC-19: nhánh "`403 MFA_REQUIRED` → step-up MFA" bỏ; nhánh gateway nội bộ không kết nối được giữ nguyên. T002, T036, plan, quickstart, contracts nhắc MFA là lịch sử. Thay thế chung: route platform (`/admin/*`, `/game-admin/*`) chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform (guard `platform_role`, thiếu → `403 PERMISSION_DENIED` + `authz.denied`); `CrossTenantScope` giữ `platform.cross_tenant.read` + client portal + `reason` + audit; phiên `platform_staff` bị từ chối `refreshPortalToken` sau 8 giờ từ lúc nhập mật khẩu (`EW_OIDC_PLATFORM_MAX_AUTH_AGE`, `401 REFRESH_TOKEN_INVALID`).

### Session 2026-11-22 (ADR-0094 — [Confirmed])

- Q: Văn bản còn mô tả MFA/`MFA_REQUIRED`/step-up như hành vi hiện hành? → A: [Confirmed] Viết lại theo [ADR-0094](../../docs/adr/ADR-0094-bo-mfa-totp-cho-dang-nhap-admin-portal.md), giữ nguyên mã AC: Input, F079-AC-19, bảng FR/tham số/Constitution. Route platform chỉ còn gateway nội bộ + `azp = eduworld-portal` + `user_type = staff` + role platform; thiếu → `403 PERMISSION_DENIED` + `authz.denied`; không còn `403 MFA_REQUIRED` hay hộp đăng nhập lại (step-up). Câu trả lời Clarifications cũ nhắc MFA là lịch sử.

### Session 2026-11-24 (triển khai — owner "chốt hết", [Confirmed])

- Q: Delta contract nào thực sự cần (T004, T005) và hình dạng? → A: [Confirmed] Thêm vào `eduworld-api.v1.yaml` (module game, hiện thực + test trong cùng feature): `listGameConfigs` thêm `from`/`to` (`format: date`, chỉ `events`, giao khoảng ngày ICT, ≤ 62 ngày, loại khác → `422`); `GameConfigVersion.render_status` (`pending`/`ready`/`failed`/`not_applicable` — job `render_thumbnail` chờ/thử lại → `pending`, `dead` → `failed`) và `calendar_warnings[]` (chỉ khi có `from`/`to`); `getGameClientManifest` (`GET /game-admin/client-manifests/current`, giao của bản mới nhất mỗi nền tảng — cùng tập giá trị `submit_review` kiểm); `simulateGameConfig` là **GET** `…/versions/{version}:simulate` với query `minutes_per_day`/`days_per_week`/`weeks` (truy vấn chỉ đọc phía Query — CQRS R26, không ghi nên không cần `Idempotency-Key`) chỉ cho `progression-curves`/`season-tracks`; `getGameConfigCounts` (`GET /game-admin/stats/config-counts`, đếm GROUP BY trong SQL cho Game Dashboard). **Không** thêm `getGameConfigSchema` (mọi loại module game đang phục vụ đều có schema trong `specs/schemas`; portal dùng bản sao sinh bằng `pnpm gen:schemas`) và **không** thêm `q`/`sort` (không AC nào cần).
- Q: Mô phỏng tính thế nào (AC-10, AC-15)? → A: [Confirmed] Server: đường cong — XP/ngày chơi = min(`minutes_per_day` × `xp_simulation_minute_rate`, `xp_daily_cap`), level theo công thức + `level_overrides`, trả kèm bảng ngưỡng (biểu đồ AC-09 vẽ từ đó); season track — điểm/ngày chơi = `points.daily_cap`, cấp = ⌊tổng / `per_tier`⌋ (tối thiểu 1, tối đa số cấp). Tham số mới `xp_simulation_minute_rate` = 14 XP/phút (suy từ mục tiêu PROGRESSION_SYSTEM §4.1 "20–30 phút/ngày ≈ 300–400 XP/ngày"), env `EW_GAME_XP_SIMULATION_PER_MINUTE`, đăng ký CONFIG_DEFAULTS. Portal không tính gì; mô phỏng chạy trên bản đã lưu (autosave) của version.
- Q: `effective_from` khi publish đường cong (AC-11)? → A: [Confirmed] `PublishRequest` không có trường này và F078 không dùng `effective_from` của `transition`: đường cong có hiệu lực từ thời điểm publish (ngay hoặc `publish_at` hẹn giờ ICT); hộp xác nhận nêu "không giảm level, không thu hồi XP đã cấp"; lỗi field của server hiện trong hộp thoại.
- Q: Thư viện form schema / graph (plan Technical Context)? → A: [Confirmed] Không thêm phụ thuộc (`@rjsf`, React Flow): `SchemaForm` viết trong ops-ui (entry riêng `@eduworld/ops-ui/game-console`, không kéo MathJax vào chunk game-ops) hỗ trợ tập con JSON Schema của các schema game (`$ref`, `allOf`/`if-then` để server kiểm, enum, const ẩn, `{vi,en}` song ngữ có trạng thái dịch, mảng thêm/xóa/đổi chỗ); graph hội thoại NPC = các node + liên kết chọn từ danh sách node id + sơ đồ cạnh và cảnh báo node không tới được; bước quest kéo-thả bằng HTML5 drag & drop + nút Lên/Xuống (bàn phím).
- Q: Preview Godot Web nhận dữ liệu thế nào (AC-17)? → A: [Confirmed] Portal tạo phiên (`createGamePreviewSession`), tự đọc `getGamePreviewSnapshot` với `X-Preview-Token` (token chỉ trong bộ nhớ, không log) rồi `postMessage({type: 'eduworld:preview-snapshot', snapshot})` tới đúng origin của bản Godot Web; iframe `sandbox="allow-scripts allow-same-origin"`, URL không chứa token. URL bản Godot Web là khóa runtime tùy chọn `gamePreviewUrl` (`VITE_GAME_PREVIEW_URL` khi dev); vắng → nút Preview thay bằng thông báo. Phía Godot nghe message thuộc nhóm F.
- Q: Bộ chọn tham chiếu khi chưa có manifest (AC-07)? → A: [Confirmed] Area/NPC/scene lấy từ manifest khi `registered = true`; chưa đăng ký bản nào → lấy từ vùng thế giới / hội thoại NPC đã publish; item, quest, challenge, shop item, collection, mini-game, reward rule, chương truyện luôn từ cấu hình đã publish (mọi trang `listGameConfigs status=published`). Mã định danh của chính loại đang soạn (`area_id` của vùng, `item_id` của vật phẩm) do người soạn nhập.
- Q: Cảnh báo lịch "quá nhiều sự kiện cùng khối" (ADMIN_PORTAL §3.15)? → A: [Confirmed] Chưa có ngưỡng trong CONFIG_DEFAULTS → server chỉ trả `decoration_area_overlap`; ngưỡng khối ghi câu hỏi mở cho owner EVENT_SEASON_SYSTEM. Cảnh báo "mốc thưởng cần > 3 phiên/tuần" không có phép tính phía server → hiển thị lời nhắc cố định ở bước Phần thưởng, không chặn.
- Q: Màu theo loại sự kiện trên lịch (AC-14)? → A: [Confirmed] Dùng token theme Admin (`colorPrimary`, `colorInfo`, `colorSuccess`, `colorWarning`, `colorError`…), không thêm nhóm màu nghiệp vụ mới vào `design/shared-tokens` (không đổi design token).
- Q: Loại `bosses`, `currencies`, `progression-levels` (F078 trả `404`)? → A: [Confirmed] Danh sách hiện "chưa có module loại" và không gọi API; khi module loại có, registry `served` đổi `true`.
- Q: Tạo mới khi server kiểm toàn bộ payload ngay từ `createGameConfig`? → A: [Confirmed] Màn "Tạo mới" giữ bản nháp trong bộ nhớ và gửi một lần (`Idempotency-Key`); Event Builder tạo bản nháp khi rời bước 5 (trước Preview), lỗi `422` đưa về bước chứa field. Sau khi có version, autosave `updateGameConfigDraft` (lỗi `422` hiện trên field và chặn "Gửi duyệt").
- Q: Thành phần phát hành dùng chung với Content Studio? → A: [Confirmed] Tách sang `apps/portal/src/shared/release/` (giờ ICT, hằng số editor, `DiffTable`, `ReleaseTimelineView`) và `shared/files/sha256.ts`; Content Studio dùng lại chúng (re-export, không đổi hành vi); `ReasonDialog` của shell cho lý do rollback/gỡ/trả lại.
- Q: Game Dashboard (T045) và màn Media? → A: [Confirmed] Dashboard đếm bằng `getGameConfigCounts`; "số item cần dịch" chờ `translation_status` của F082 (hiện thông báo, không đoán). Media: tải lên (`presignGameMediaUpload` → PUT → theo dõi `getGameMediaAsset`) và tra cứu theo mã — contract không có danh sách media game.
- Q: E2E Playwright và quickstart với backend thật (T035 E2E, T039, T042, T043 E2E)? → A: [Confirmed] Trong feature này tiêu chí được kiểm bằng Vitest dựng cả portal (router, phiên, chunk lazy) với MOCK backend test-only; chạy Playwright/axe/visual và quickstart trên stack compose thật là việc tiếp theo (ghi CODE_PLAN).

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Danh sách và editor có version chung (Priority: P1) 🎯 MVP

Game Designer mở Quest Builder, tạo quest nháp, autosave, gửi duyệt; Content Reviewer duyệt; Live-ops publish (ngay/hẹn giờ), xem diff, rollback có lý do.

**Why this priority**: Mọi builder đặc thù dựa trên khung này (FR-CMS-004, FR-CMS-006).

**Independent Test**: Vòng đời một `reward-rules` trên backend thật: tạo → sửa → gửi duyệt → duyệt (người khác) → publish → rollback; network có `If-Match`, `Idempotency-Key`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F079-AC-01 | KHI người có permission ghi của loại (`game.quest.write`, `game.npc.write`, `game.world.write`, `game.reward_rule.write`, `game.item.write`, `game.shop.write`, `game.collection.write`, `game.economy.write`, `game.progression.write`, `game.event.write`, `game.season_track.write`…) mở danh sách một `config_type`, Portal PHẢI hiển thị `listGameConfigs` (lọc `status`, cursor) với cột mã, tiêu đề, trạng thái (Tag màu từ `shared-tokens`), version, người sửa, thumbnail; menu chỉ hiện loại người dùng có quyền theo `getMyPermissions` | FR-CMS-004 · [ADMIN_PORTAL §3.3, §3.12](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-001 |
| F079-AC-02 | KHI soạn, `ContentEditorShell` PHẢI sinh form từ JSON Schema của loại (`required`, `maxLength`, enum, bộ đếm ký tự), autosave nháp qua `updateGameConfigDraft` với `If-Match`, và NẾU server trả `409 CONTENT_VERSION_CONFLICT`/`412`, THÌ PHẢI dừng autosave, báo người khác đang sửa và cho tải lại | FR-CMS-006 · [ADMIN_PORTAL §1.4](../../docs/04-admin-portal/ADMIN_PORTAL.md) · ADR-0041 | TC-CMS-002 |
| F079-AC-03 | KHI chuyển trạng thái, Portal PHẢI gọi `transitionGameConfigVersion` (`submit_review`/`approve`/`reject`/`archive`, ghi chú khi từ chối) và `publishGameConfigVersion` (ngay hoặc `publish_at`) với `Idempotency-Key`; nút duyệt PHẢI ẩn với tác giả nhưng server là nơi chặn (reviewer ≠ author) | FR-CMS-006 · [ADMIN_PORTAL §3.5, §4](../../docs/04-admin-portal/ADMIN_PORTAL.md) · ADR-0043 | TC-CMS-001 |
| F079-AC-04 | KHI mở tab Phiên bản, Portal PHẢI hiển thị `listGameConfigVersions`, version đã publish chỉ đọc (không nút Sửa, có "Tạo bản nháp từ phiên bản này"), diff theo field qua `diffGameConfigVersions`, lịch sử `listGameConfigReleases`; rollback/unpublish PHẢI bắt buộc lý do và gọi `rollbackGameConfig`/`unpublishGameConfig` | FR-CMS-006 · ADR-0043 · AGENTS R11 | TC-CMS-003 |
| F079-AC-05 | NẾU server trả `422 VALIDATION_FAILED` (gồm tham chiếu `area_id`/`npc_id`/`scene_ref` không có trong client manifest) hoặc `422 TRANSLATION_INCOMPLETE`, THÌ Portal PHẢI map lỗi về đúng field theo `errors[].field` và chặn gửi duyệt | FR-CMS-004 · FR-EVT-008 · [ADMIN_PORTAL §3.3](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-EVT-010 |

---

### User Story 2 — Builder thế giới, NPC, quest, phần thưởng, item (Priority: P1)

Game Designer dựng area mới, hội thoại NPC dạng graph, quest nhiều bước kéo-thả, rule phần thưởng có cap, item cosmetic và mặt hàng Shop giá Xu.

**Why this priority**: Nội dung gameplay cốt lõi M3 (FR-CMS-004).

**Independent Test**: Tạo quest 3 bước (talk_npc → activity → collect) tham chiếu NPC và area có trong manifest; tham chiếu scene không tồn tại → lỗi field.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F079-AC-06 | KHI soạn `quests`, Portal PHẢI cung cấp danh sách bước kéo-thả (talk_npc, go_to, activity, collect, minigame, party_activity) với điều kiện, phần thưởng, lịch; KHI soạn `npc-dialogues`, PHẢI cung cấp graph editor (node thoại song ngữ, lựa chọn, điều kiện, action) — mọi payload theo `quest.schema.json`/`npc-dialogue.schema.json` | FR-CMS-004 · [ADMIN_PORTAL §3.3](../../docs/04-admin-portal/ADMIN_PORTAL.md) · [QUEST_SYSTEM](../../docs/01-game-design/QUEST_SYSTEM.md) | TC-GAM-012 |
| F079-AC-07 | KHI chọn area/NPC/scene/activity/item tham chiếu, Portal PHẢI dùng bộ chọn lấy từ client manifest hiện hành và danh sách nội dung đã publish (không cho nhập ID tự do) | FR-CMS-004 · [ADMIN_PORTAL §3.3](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F079-AC-08 | KHI soạn `reward-rules`, `items`, `shop-items`, `currencies`, `world-areas`, `story-chapters`, `minigame-configs`, `achievements`, `challenges`, `bosses`, Portal PHẢI dùng form theo schema của loại; trường giá PHẢI chỉ nhận tiền trong game (không có trường tiền thật) | FR-CMS-004 · AGENTS R19 · [ADMIN_PORTAL §3.12](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-ECO-003 |

---

### User Story 3 — Level & XP Configuration (Priority: P1)

Game Designer chỉnh đường cong XP, xem biểu đồ, danh hiệu theo mốc level, bảng nguồn XP và cap, chạy mô phỏng tiến độ, publish với `effective_from`.

**Why this priority**: FR-GAM-007 — đường cong không hardcode trong Godot, phải cấu hình được.

**Independent Test**: Sửa `base`/`exponent` → biểu đồ cập nhật từ server; ngưỡng không tăng đơn điệu → lỗi; publish → TC-GAM-006 giữ level học sinh.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F079-AC-09 | KHI người có `game.progression.write` soạn `progression-curves`, Portal PHẢI cho khai báo công thức (`base`, `exponent`, `linear`, `max_level`) hoặc bảng `levels[]` ghi đè, danh hiệu theo khoảng level, phần thưởng lên cấp, catalog nguồn XP có mã và cap; hiển thị biểu đồ và bảng ngưỡng do server trả | FR-GAM-007 · [PROGRESSION_SYSTEM §3.1, §4](../../docs/01-game-design/PROGRESSION_SYSTEM.md) | TC-GAM-006 |
| F079-AC-10 | KHI bấm "Mô phỏng", Portal PHẢI gọi `simulateGameConfig` (phút/ngày, số tuần) và hiển thị level đạt được theo tuần; Portal PHẢI không tự tính tiến độ phía client | FR-GAM-007 · AGENTS R1 · [ADMIN_PORTAL §3.12](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F079-AC-11 | NẾU ngưỡng không tăng đơn điệu, cap ≤ 0 hoặc thiếu `effective_from` khi publish, THÌ Portal PHẢI hiển thị lỗi field từ server và chặn publish; hộp xác nhận publish PHẢI nêu "không giảm level, không thu hồi XP đã cấp" | FR-GAM-007 · [PROGRESSION_SYSTEM §4.2](../../docs/01-game-design/PROGRESSION_SYSTEM.md) | TC-GAM-006 |

---

### User Story 4 — Event Builder, lịch sự kiện, season track, template cho tenant (Priority: P1)

Live-ops tạo "Tuần lễ Toán học" qua wizard 8 bước (thông tin song ngữ → đối tượng → nội dung → phần thưởng → lịch → preview → gửi duyệt → lên lịch), xem lịch tháng có cảnh báo chồng chéo, cấu hình season track 30 cấp miễn phí.

**Why this priority**: FR-EVT-002; sự kiện là công cụ live-ops chính của M3.

**Independent Test**: Tạo sự kiện thiếu bản `en` → bị chặn; tham chiếu quest chưa publish → lỗi; publish → sự kiện xuất hiện trên lịch ở trạng thái Scheduled.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F079-AC-12 | KHI người có `game.event.write` tạo `events`, Portal PHẢI chạy wizard 8 bước theo [ADMIN_PORTAL §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md): form song ngữ vi/en cạnh nhau có chỉ báo đầy đủ bản dịch, đối tượng (phạm vi, tenant/trường/khối/môn), nội dung tham chiếu chỉ từ nội dung đã publish, phần thưởng từ catalog, lịch ICT, preview, gửi duyệt, lên lịch | FR-EVT-002 · FR-EVT-008 · [EVENT_SEASON_SYSTEM §4, §11](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) | TC-EVT-010 |
| F079-AC-13 | KHI server trả lỗi validate sự kiện (`TRANSLATION_INCOMPLETE`, lễ hội ngắn hơn `event_festival_min_duration_days`, phần thưởng không hợp lệ, gói trang trí vượt `event_decoration_bundle_max_mb`), Portal PHẢI chặn bước tương ứng; cảnh báo mốc thưởng cần > 3 phiên/tuần PHẢI hiển thị không chặn | FR-EVT-002 · FR-EVT-007 · [ADMIN_PORTAL §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-EVT-010 |
| F079-AC-14 | KHI mở Lịch sự kiện, Portal PHẢI hiển thị lịch tháng/tuần theo giờ ICT từ `listGameConfigs` (`events`, khoảng ngày), màu theo loại (token), cảnh báo chồng chéo do server trả | FR-EVT-002 · [ADMIN_PORTAL §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F079-AC-15 | KHI soạn `season-tracks`, Portal PHẢI hiển thị bảng cấp × phần thưởng, nguồn điểm mùa + trần/ngày, mô phỏng qua `simulateGameConfig`; `paid_track` PHẢI không có control bật; KHI soạn `event-templates`, Portal PHẢI cho khai báo `allowed_params` | FR-EVT-002 · FR-EVT-005 · FR-EVT-006 · AGENTS R19 | TC-EVT-007 |

---

### User Story 5 — Thumbnail/turntable và preview Godot Web (Priority: P2)

Danh sách item hiển thị thumbnail theo version; mở chi tiết xem turntable; bấm Preview mở khung Godot Web với bản nháp.

**Why this priority**: FR-CMS-017 nhận diện nội dung; preview giảm lỗi publish.

**Independent Test**: Publish item mới → thumbnail đổi; version cũ giữ ảnh cũ; render lỗi → placeholder + trạng thái.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F079-AC-16 | Portal PHẢI hiển thị thumbnail 512×512 và turntable của đúng version trong danh sách, bộ chọn item/phần thưởng và chi tiết; KHI `render_status` là `pending`/`failed`, PHẢI hiển thị placeholder + nhãn trạng thái thay cho ảnh | FR-CMS-017 · [DESIGN_SYSTEM §7](../../docs/05-ux/DESIGN_SYSTEM.md) · ADR-0047 | TC-UI-009 |
| F079-AC-17 | KHI bấm Preview, Portal PHẢI gọi `createGamePreviewSession` và nhúng `GamePreviewFrame` (iframe riêng, Godot Web) với `preview_token`; giao diện game PHẢI chỉ nằm trong khung, phần còn lại theo Admin/Ops | FR-CMS-006 · ADR-0047 · AGENTS R32 | TC-UI-010 |

---

### User Story 6 — Bảo vệ workspace A (Priority: P1)

**Why this priority**: FR-CMS-011 — 4 lớp chặn, tenant không chạm cấu hình game.

**Independent Test**: `ad.truong.a`, `qlcm.truong.a`, `sm.antoan` mở `/game-ops/*` → 403, không tải chunk; token không MFA → step-up.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F079-AC-18 | KHI người dùng không có permission `game.*` nào mở URL `/game-ops/*`, Portal PHẢI hiển thị trang 403 và không tải chunk workspace A | FR-CMS-011 · ADR-0027 · AGENTS R20 | TC-CMS-AUTHZ-05 |
| F079-AC-19 | NẾU API trả `403 PERMISSION_DENIED`, THÌ Portal PHẢI hiển thị trang 403 kèm `trace_id` (không có step-up MFA — ADR-0094); NẾU gateway nội bộ không kết nối được, THÌ hiển thị "Cần kết nối mạng quản trị (VPN)" | FR-CMS-011 · [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-WS-02 |
| F079-AC-20 | HỆ THỐNG PHẢI gửi `Idempotency-Key` cho mọi POST ghi (giữ nguyên khi thử lại), giới hạn mọi ô theo `maxLength` có bộ đếm, hiển thị lỗi theo `code` + `trace_id`, và không import component/Theme game ngoài `GamePreviewFrame` | AGENTS R10, R32 · ADR-0041 | TC-TEN-024 |
| F079-AC-21 | KHI menu/action của feature (`menu.game_ops.world`, `menu.game_ops.npcs`, `menu.game_ops.quests`, `menu.game_ops.minigames`, `menu.game_ops.progression`, `menu.game_ops.items`, `menu.game_ops.shop`, `menu.game_ops.rewards`, `menu.game_ops.events`, `menu.game_ops.review`, `menu.game_ops.media`, `menu.game_ops.dashboard` và action con) đang tắt cho phạm vi phiên (`disabled_menus`/`disabled_actions` của `getMyPermissions`), Portal PHẢI ẩn menu và ẩn/khóa nút tương ứng (`<ActionGate actionKey>`); NẾU API trả `403 FEATURE_DISABLED`, Portal PHẢI hiện thông báo tính năng tạm tắt và tải lại quyền | FR-CMS-018 · ADR-0090 | TC-CMS-MENU-02 |

### Edge Cases

- NẾU loại cấu hình chưa có JSON Schema (`world-areas`, `items`, `currencies`…), THÌ form dùng schema `GameConfigInput.payload` do server mô tả (delta F078 `getGameConfigSchema`) — chưa có thì hiển thị editor JSON có validate server, ghi rõ trong UI.
- NẾU người dùng mở workspace A trên điện thoại, THÌ chỉ xem danh sách/chi tiết, builder hiển thị "Tác vụ này cần màn hình lớn hơn".
- NẾU publish hẹn giờ bị sửa trước giờ, THÌ hiển thị trạng thái "Đã hẹn" và cho hủy hẹn (unpublish nháp hẹn).
- NẾU System Admin gán nội dung game cho tenant, THÌ dùng hộp gán của F057, không tự dựng.
- NẾU hai sự kiện trang trí cùng vùng, THÌ lịch hiển thị cảnh báo do server trả, không tự suy luận.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CMS-004 | UI Game Content: quest, mini-game, level, NPC dialogue, reward, world, item, achievement, challenge, boss | [ADMIN_PORTAL §3.3, §3.12](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-017 | Hiển thị thumbnail/turntable theo version | [DESIGN_SYSTEM §7](../../docs/05-ux/DESIGN_SYSTEM.md) |
| FR-EVT-002 | Event Builder, lịch, season track, template | [ADMIN_PORTAL §3.15](../../docs/04-admin-portal/ADMIN_PORTAL.md) · [EVENT_SEASON_SYSTEM](../../docs/01-game-design/EVENT_SEASON_SYSTEM.md) |
| FR-GAM-007 | UI đường cong XP, catalog nguồn XP, danh hiệu | [PROGRESSION_SYSTEM §3–§4](../../docs/01-game-design/PROGRESSION_SYSTEM.md) |
| FR-CMS-006 | Workflow, version, preview Godot Web, publish, rollback (phía UI game) | [ADMIN_PORTAL §1.4, §4–§6](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-011 | Lớp chặn bundle + hiển thị 403 | [ADMIN_PORTAL §1.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-EVT-008 | Biên tập song ngữ bắt buộc cho sự kiện platform | [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-018 | Menu/action của feature theo danh mục `specs/portal/menu-catalog.yaml`, ẩn khi tắt, xử lý `403 FEATURE_DISABLED` | [ADMIN_PORTAL §2.7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |

### Key Entities

- **GameConfig / GameConfigVersion**: cấu hình game theo loại, version, trạng thái workflow, payload theo schema, thumbnail.
- **ContentRelease**: lịch sử publish/rollback/unpublish.
- **Event (cấu hình)**, **EventTemplate**, **SeasonTrack**, **ProgressionCurve**: loại cấu hình có builder riêng.
- **PreviewSession**: token xem bản nháp trong Godot Web.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `xp_curve` | Giá trị mặc định hiển thị khi tạo curve mới (server trả) |
| `xp_daily_cap` | Gợi ý trần khi cấu hình nguồn XP |
| `event_festival_min_duration_days` | Thông điệp lỗi độ dài lễ hội |
| `event_decoration_bundle_max_mb` | Thông điệp lỗi gói trang trí |
| `season_track_tiers` / `season_track_points_per_tier` | Mặc định bảng season track |
| `school_event_max_duration_days` | Giới hạn `allowed_params` của template |
| `portal_session_sso` | Phiên platform (tối đa 8 giờ từ lúc nhập mật khẩu; không step-up — ADR-0094) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Workspace A chỉ gateway nội bộ + role platform; chunk lazy-load không phải lớp bảo mật | AGENTS R20 · ADR-0027 |
| Không business logic trong UI (mô phỏng, validate nghiệp vụ ở server) | AGENTS R1 |
| Admin/Ops design system; game UI chỉ trong `GamePreviewFrame` | AGENTS R32 · ADR-0047 |
| Không hardcode nội dung game trong portal | AGENTS R2 |
| WCAG 2.1 AA | NFR-A11Y-003 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Game Designer tạo và gửi duyệt một quest 3 bước trong ≤ 10 phút trên dữ liệu seed.
- **SC-002**: 100% thao tác ghi workspace A có `Idempotency-Key` hoặc `If-Match` trong network log E2E.
- **SC-003**: 0 người dùng role tenant tải được chunk workspace A (network log E2E).
- **SC-004**: Mọi version publish có thumbnail hiển thị đúng version trong ≤ 1 lần tải lại sau khi render xong.

## Ngoài phạm vi

- Backend workflow, thumbnail render, client manifest, catalog cho nhà trường: F078; vòng đời sự kiện, community goal, season track backend: F077; đường cong XP/ledger backend: F074.
- Giám sát sự kiện đang chạy, dừng khẩn cấp/tiếp tục/hủy, Live dashboard, Game Analytics (workspace E): F080.
- Cấu hình thi đấu (matchmaking, ranking, season, tournament template): F108. Sticker pack/GIF UI: chưa có feature (báo cáo). Dịch nội dung game (xuất/nhập file dịch): chưa có feature riêng — đề xuất gộp F078/F079 sau.
- Hộp gán dữ liệu platform cho tenant: F057. Sự kiện trường của tenant (workspace C): chưa có feature UI (báo cáo).

## Assumptions

- [Confirmed] Thư mục `src/cms/frontend/apps/portal/src/workspaces/game-ops/`, route `/game-ops/*` (F051).
- [Confirmed] Thumbnail đọc từ trường mới `GameConfigVersion.media` (delta F078); trước khi có, cột thumbnail ẩn.
- Permission ghi `progression-curves`/`progression-levels` là `game.progression.write`; `shop-items` là `game.shop.write`, `collections` là `game.collection.write` (RB-309); UI chỉ dựa `getMyPermissions`.
