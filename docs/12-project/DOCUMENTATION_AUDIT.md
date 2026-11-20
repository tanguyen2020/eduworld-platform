# Documentation Audit — đối chiếu với Master Prompt "Specs & Documentation Re-architecture"

> Trạng thái: Review · Phiên bản: 1.3 · Owner: Principal Architect + Technical Writer · Cập nhật: 2026-11-15
> Liên quan: [ADR](../adr/README.md) · [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md) · [OPEN_QUESTIONS](OPEN_QUESTIONS.md) · [ROADMAP](ROADMAP.md) · [docs/README](../README.md)

## 1. Phạm vi và phương pháp

- **Đối tượng audit:** toàn bộ `docs/` (62 tài liệu, ~21.000 dòng), `specs/` (12 OpenAPI, 1 AsyncAPI, 11 JSON Schema, 4 ví dụ, event envelope), `scripts/`, các file root (README, AGENTS, CONTRIBUTING, CHANGELOG, VIBE_CODING_PROMPT).
- **Mã nguồn:** repository **chưa có code triển khai** (không có `services/`, `src/game/`, `src/cms/frontend/`). Mọi nội dung hiện có là đặc tả, không phải chức năng đã tồn tại.
- **Cách đánh giá:** đối chiếu từng mục §5–§34 của Master Prompt mới với tài liệu hiện có (đọc + grep từ khóa), chạy lại toàn bộ công cụ kiểm tra.
- **Phân loại** theo Master Prompt §4: **CONFIRMED**, **PROPOSED**, **TBD**, **CONFLICT**, **DEPRECATED**.

## 2. Kết quả kiểm tra tự động (2026-11-15)

| Kiểm tra | Kết quả |
|---|---|
| OpenAPI lint (Redocly + Spectral), 12 file | 12/12 hợp lệ, 0 lỗi |
| AsyncAPI parser | Hợp lệ |
| JSON Schema (Ajv strict) + ví dụ | 11/11 schema, 4/4 ví dụ hợp lệ |
| `maxLength` cho mọi chuỗi trong spec | 1.371 trường, 0 vi phạm |
| Link tương đối + anchor | 0 hỏng (ngoài `scripts/node_modules`) |
| **Cú pháp Mermaid** | **21/102 sơ đồ lỗi** (chưa từng được kiểm tra trước đây) → đang sửa, thêm `scripts/docs/check-mermaid.mjs` vào CI |

## 3. Mức đáp ứng theo từng mục của Master Prompt

Ký hiệu: ✅ đáp ứng · 🟡 một phần · ❌ chưa có.

| § | Yêu cầu | Mức | Hiện có | Thiếu |
|---|---|---|---|---|
| 3 | Giữ stack Godot/Go/APISIX/Keycloak/Valkey/SIS | ✅ | ADR-0001…0047 | PostgreSQL đang ghi Accepted, Master Prompt yêu cầu Proposed/TBD → CONFLICT C3 |
| 4 | Audit + phân loại CONFIRMED/PROPOSED/TBD/CONFLICT/DEPRECATED | 🟡 | Nhãn [Confirmed]/[Confirmed]/[TBD]; ADR có Superseded | Chưa có nhãn CONFLICT/DEPRECATED trong tài liệu; báo cáo này là audit đầu tiên |
| 5.1 | Game World: hub, map, zone, instance, NPC, portal, spawn, discovery, storyline | 🟡 | GAME_WORLD_DESIGN, 3D_WORLD_DESIGN, ADR-0012 | **World map, portal/teleport, discovery point, hidden location, exploration progress, world storyline/chapter, unlock conditions từng khu** |
| 5.2 | Character & avatar | 🟡 | CHARACTER_SYSTEM | **Luồng tạo avatar lần đầu**, body type, equipment slot, avatar preset, appearance theo progression |
| 5.3 | Character controller | ✅ | CHARACTER_SYSTEM, GAME_NETWORKING | Accessibility settings điều khiển, mobile controls chi tiết |
| 6 | Progression: XP, level, XP curve, level rewards, caps, tách 3 domain progression | 🟡 | DOMAIN_MODEL §8/§10 (5 loại số liệu), ADR-0018, XP cap/diminishing (ANTI_CHEAT) | **Tài liệu progression riêng**: công thức XP curve cấu hình được, level title, level-up event/reward, level history, XP source catalog, CMS cấu hình |
| 7 | Quest: 12 loại, objective nhiều bước, failure, cooldown, repeatability | 🟡 | QUEST_SYSTEM, quest.schema.json | Loại Main/Side/Social/Seasonal/Boss quest; failure conditions, cooldown, repeatability, level requirement |
| 8 | Challenge engine + Boss battle nhiều phase | ❌ | — | **Toàn bộ** (challenge types, 5 mức độ khó, boss phase/health/contribution/retry) |
| 9 | World progression & exploration | ❌ | Chỉ "đảo đổi màu theo mastery" (GAME_DESIGN) | Unlock vùng, discovery, collectible, map completion, story chapter |
| 10 | Achievement & collection (pet, knowledge card, trade policy) | 🟡 | Achievement/Badge entity, event | **Achievement builder chi tiết, collection system, pet, knowledge card, trade policy (tắt mặc định)** |
| 11 | Inventory, item, economy, currency ledger, shop | 🟡 | Inventory/Item, reward grant idempotent, không tiền thật | **Currency (coins/gems) + ledger kiểm toán, shop/reward catalog, item history, consumable policy** |
| 12 | Live ops: daily/weekly/seasonal/community goal, Event Builder | 🟡 | Season (thi đấu), daily challenge, world event (nhắc tới) | **Event & Season system, Event Builder trong CMS, community goals, learning campaign, lifecycle start/end** |
| 13.1 | Friends | ✅ | SOCIAL_CHAT_DESIGN, DOMAIN_MODEL | Tách tài liệu friends riêng (tùy chọn) |
| 13.2 | Party | ✅ | PARTY_SYSTEM (Addendum đầy đủ) | — |
| 13.3 | Guild/Club bền vững (vai trò, join request, club event) | ❌ | Team/Study Group (mô tả ngắn) | **Toàn bộ guild/club** |
| 13.4 | Multiplayer, matchmaking theo khối lớp | ✅ | MULTIPLAYER_DESIGN, MATCHMAKING, REALTIME_ARCHITECTURE | — |
| 13.5 | Chat + rich messaging + 3D emote | ✅ | CHAT_SYSTEM, ADR-0034/0035 | Danh sách emote 3D (wave, clap…) gắn chat |
| 14 | Child safety & moderation (có **chính sách theo khối/tuổi**) | 🟡 | CHILD_SAFETY, MODERATION, CHILD_PROTECTION, ADR-0030 | **CONFLICT C2**: chat thống nhất mọi lớp (ADR-0020) vs yêu cầu policy theo grade; image/attachment moderation (sticker/GIF đã duyệt sẵn) |
| 15.1 | Đa định dạng học tập | 🟡 | 11 dạng câu hỏi, mini-game, exam | **Video learning (chapter, subtitle, embedded question, resume), audio/speaking/pronunciation, written response/essay/rubric/teacher review, simulation/virtual lab, project-based** |
| 15.2 | Chuỗi Curriculum → … → Learning Record | ✅ | CURRICULUM_MODEL, CONTENT_MODEL | Thực thể Evaluation tách riêng |
| 15.3 | Activity Registry mở rộng | ❌ | Minigame plugin (Godot) | **Activity Registry phía backend + ADR "Learning Activity Extensibility"** |
| 15.4 | Learning attempt + version | ✅ | EXERCISE_ENGINE, ADR-0015/0043 | — |
| 15.5 | Evaluation: rubric, teacher review, AI optional | ❌ | Auto-grading 11 dạng | **Rubric, teacher review, ADR AI-assisted grading** |
| 16 | Admin Portal **5 workspace** + UI editor (drag-drop, version compare, rollback) | 🟡 | ADR-0027 (2 bundle), CMS_DESIGN, ADR-0043, ADR-0047 | **CONFLICT C6** (2 bundle vs 1 portal 5 workspace); thiếu tài liệu từng workspace: Game Operations Console, Learning Content Studio, Live Ops, Lesson/Activity Builder, Moderation Console, information architecture |
| 17 | Backend modular monolith, 33 bounded context | 🟡 | ADR-0036 (6 microservice), BACKEND_DESIGN | **CONFLICT C1** (Master Prompt: không microservice sớm); thiếu context Challenge, Achievement, Economy, Event/Season, Guild |
| 18 | Godot modular structure | ✅ | GODOT_ARCHITECTURE | Thư mục challenge/progression/collection/video/audio/simulation |
| 19 | Realtime: tách Application vs Gameplay realtime, ADR transport | ✅ | ADR-0011, REALTIME_ARCHITECTURE | Ghi rõ đánh giá transport theo tick rate cho boss/co-op |
| 20 | Data model ~40 thực thể, ledger | 🟡 | DOMAIN_MODEL, DATABASE_DESIGN | Guild, Challenge, Currency Ledger, Written Response, Rubric, Event, ERD tổng, data dictionary |
| 21 | Identity/SSO/SIS | ✅ | 04-identity/*, ADR-0006/0021/0026/0046 | SIS outage handling đã có; realm vẫn Proposed (đúng yêu cầu) |
| 22 | Security, privacy, compliance VN | ✅ | 09-security/*, SECURITY_ARCHITECTURE (STRIDE) | Threat model tách tài liệu riêng (tùy chọn) |
| 23 | Tách Game Analytics vs Learning Analytics | 🟡 | LEARNING_ANALYTICS, MONITORING | **Game Analytics** (DAU/WAU/MAU, retention, funnel quest, event participation) |
| 24 | API theo nhóm `/player`, `/quests`, `/guilds`… | 🟡 | 189 operation, nhóm `/game/*`, `/multiplayer/*`… | **CONFLICT C7** (đặt tên nhóm route); thiếu API challenge/boss/achievement/economy/event/guild/activity/written response |
| 25 | UX spec từng màn (purpose, roles, components, states, validation, permissions) | 🟡 | DESIGN_SYSTEM, TEACHER_DASHBOARD, CMS_DESIGN §1.1 | **Student game UX từng màn** (login, avatar, HUD, world map, quest journal, challenge, inventory, event center…), CMS UX từng màn |
| 26 | Test strategy + 20 kịch bản nghiệm thu | 🟡 | TEST_STRATEGY (rất chi tiết) | Kịch bản 3 (tạo avatar), 8 (level up), 9 (mở khóa khu) chưa có test case |
| 27 | Roadmap Phase 0–5 (Playable MVP…) | 🟡 | ROADMAP (Release 1.0 hoàn chỉnh, ~40 sprint) | **CONFLICT C5** về cách gọi MVP; thiếu phase cho Challenge/Boss/Guild/Video/Audio/Simulation |
| 28 | Cấu trúc tài liệu đích | 🟡 | 12 thư mục hiện có | Quyết định giữ cấu trúc hiện tại hay chuyển (CONFLICT C4); thiếu ~30 tài liệu chủ đề |
| 29 | 15 ADR bắt buộc | 🟡 | 47 ADR | Thiếu ADR: Learning Activity Extensibility, Reward/Currency Ledger, Party vs Persistent Group, Voice Chat Scope (đang là OQ-19), Simulation Execution, AI-assisted Grading, Cross-school Social Interaction; ADR tách file riêng trong `docs/adr/` (hiện gộp 1 file) |
| 30 | Chất lượng tài liệu, Mermaid hợp lệ | 🟡 | Header chuẩn, link sạch | 21 sơ đồ lỗi (đang sửa) |
| 33D | Requirements Traceability Matrix | ❌ | PRD có bảng truy vết ngắn | **RTM đầy đủ** PR → FR → component → API/data → test → AC |
| 33E | Open Decisions Register có options/khuyến nghị | 🟡 | OPEN_QUESTIONS (21 OQ, có mặc định) | Cột "Options" và "Why it matters" tách riêng |

**Tóm tắt:** phần **nền tảng kỹ thuật** (identity, SIS, tenancy, bảo mật, chat, moderation, party, matchmaking, ranking, versioning nội dung, hạ tầng, specs) **đáp ứng tốt**. Phần **game design chuyên sâu** (progression, challenge/boss, exploration, collection, economy, live-ops event) và **học tập đa định dạng** (video, audio, tự luận/rubric, mô phỏng, activity registry) **chưa đủ**. CMS cần tổ chức lại theo workspace.

## 4. Mâu thuẫn (CONFLICT) cần stakeholder quyết định

| ID | Master Prompt mới | Quyết định hiện có | Đề xuất xử lý |
|---|---|---|---|
| C1 | Bắt đầu bằng **Golang Modular Monolith**, không tạo microservice sớm (§17) | ADR-0036: **6 microservice**, stakeholder chọn 2026-11-15 | Giữ quyết định mới nhất của stakeholder (microservice) nếu vẫn đúng ý; nếu theo Master Prompt mới thì supersede ADR-0036…0040, quay lại modular monolith có ranh giới service sẵn để tách sau |
| C2 | Chính sách giao tiếp/an toàn **theo khối lớp và độ tuổi** (§2.2, §14) | ADR-0020: chat **không phân biệt theo lớp** (stakeholder chọn) | Cần chốt lại. Phương án dung hòa: giữ tính năng chat như nhau, nhưng cho phép **giới hạn theo độ tuổi** ở phạm vi giao tiếp (ai được nhắn) và mức kiểm duyệt |
| C3 | PostgreSQL là **Proposed/TBD** (§3, §20) | ADR-0003 Proposed, ADR-0037 Accepted (DB riêng mỗi service, PostgreSQL) | Ghi PostgreSQL là "đề xuất đang dùng làm mặc định", mở OQ xác nhận |
| C4 | Cấu trúc tài liệu đích (`01-requirements`, `02-game-design`, …, `docs/adr/`) | Cấu trúc 12 thư mục hiện tại | Master Prompt cho phép giữ cấu trúc cũ nếu phù hợp. Đề xuất: **giữ cấu trúc hiện tại**, bổ sung thư mục/tài liệu còn thiếu, tách ADR thành `docs/adr/ADR-NNNN-*.md` + giữ file chỉ mục |
| C5 | Phase 1 "Playable MVP" | Stakeholder: "game hoàn chỉnh, không chỉ MVP" | Giữ Release 1.0 hoàn chỉnh; dùng "playable slice" làm mốc nội bộ, không phải phạm vi phát hành |
| C6 | **Một Admin Portal, 5 workspace** (Game Ops, Learning Studio, School & LMS, Platform Admin, Live Ops) | ADR-0027: 2 bundle `cms` (tenant) và `admin` (platform, VPN + MFA) | Dung hòa: **một portal shell có workspace switcher**; workspace Game Ops/Live Ops/Platform chỉ có trong bundle `admin` (giữ 4 lớp chặn của ADR-0027) |
| C7 | Nhóm API `/api/v1/player`, `/quests`, `/party`, `/guilds`… | `/api/v1/game/*`, `/multiplayer/*`, `/competition/*` | Giữ nhóm hiện tại (đã có 189 operation, x-service); bổ sung nhóm mới theo cùng quy ước, ghi bảng ánh xạ tên |

### 4.1. Quyết định của stakeholder (2026-11-15)

| ID | Quyết định | Ghi nhận |
|---|---|---|
| C1 | **Giữ 6 microservice** (ADR-0036…0042). Master Prompt §17 về modular monolith không áp dụng — **đã thay 2026-11-17:** [ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) Accepted chọn modular monolith có ranh giới cưỡng chế (ADR-0036 Superseded) | ADR-0074 |
| C2 | **Giữ chat như nhau cho mọi lớp** (ADR-0020). Khác biệt theo khối chỉ ở UX, nội dung học, ghép trận | ADR-0020, ADR-0054 |
| C3 | PostgreSQL tiếp tục là mặc định đang dùng (chưa có yêu cầu đổi) | Giữ ADR-0037 |
| C4 | Giữ cấu trúc tài liệu hiện tại, bổ sung tài liệu còn thiếu | §6 |
| C5 | Giữ Release 1.0 hoàn chỉnh | ROADMAP |
| C6 | **Một Admin Portal duy nhất, 5 workspace** | ADR-0027 mục 7 (sửa), ADR-0022 |
| C7 | Giữ nhóm route hiện tại, thêm nhóm mới theo cùng quy ước | API_DESIGN |
| Phạm vi | **Chỉ bổ sung phần thiếu quan trọng**: progression, challenge/boss, inventory/economy, học tập đa định dạng, activity registry, 7 ADR (ADR-0048…0054). UX từng màn, RTM, tách ADR thành file riêng, guild/club chi tiết, event/season, game analytics để lượt sau | — |

## 5. Nội dung DEPRECATED (đã thay thế, giữ để truy vết)

| Nội dung | Thay bởi |
|---|---|
| ADR-0001 Modular monolith 3 binary | ADR-0036 |
| ADR-0003 một DB schema theo module (phần DB) | ADR-0037 |
| ADR-0019 Valkey Streams (phần transport) | ADR-0042 |
| ADR-0025 sqlc + goose | ADR-0039/0040 |
| ADR-0015 rollback = publish lại | ADR-0043 |
| Kubernetes/KEDA trong DEPLOYMENT | ADR-0024 (Docker Swarm) |
| "World chat tắt trong MVP", chat theo grade band | ADR-0020 (sửa) |
| MinIO cho local dev | floci (ADR-0044, 2026-11-15) |

## 6. Tài liệu đề xuất bổ sung (nếu tiếp tục giai đoạn cập nhật)

> Ghi chú: thư mục trong §3–§6.1 là cấu trúc **trước** đợt hợp nhất; vị trí hiện tại xem §8.2.

Giữ cấu trúc hiện tại (C4), thêm:

| Thư mục | Tài liệu mới |
|---|---|
| `01-product/` | `PROGRESSION_SYSTEM.md`, `CHALLENGE_BOSS_SYSTEM.md`, `WORLD_PROGRESSION.md`, `ACHIEVEMENT_COLLECTION.md`, `INVENTORY_ECONOMY.md`, `EVENT_SEASON_SYSTEM.md` |
| `05-learning/` | `ACTIVITY_ENGINE.md` (Activity Registry), `VIDEO_LEARNING.md`, `AUDIO_LANGUAGE_LEARNING.md`, `WRITTEN_RESPONSE_RUBRIC.md`, `SIMULATION_ENGINE.md`, `LEARNING_PATH.md` |
| `07-multiplayer/` | `GUILD_CLUB.md`, `FRIENDS.md` |
| `08-cms-lms/` | `INFORMATION_ARCHITECTURE.md` (5 workspace), `GAME_OPERATIONS_CONSOLE.md`, `LEARNING_CONTENT_STUDIO.md`, `LIVE_OPERATIONS.md`, `MODERATION_CONSOLE.md` |
| `10-devops/` hoặc mới `12-analytics/` | `GAME_ANALYTICS.md` |
| `13-ux/` (mới) | `STUDENT_GAME_UX.md`, `GAME_HUD.md`, `CMS_ADMIN_UX.md`, `LEARNING_UX.md` |
| `11-project/` | `TRACEABILITY_MATRIX.md`, `DEPENDENCIES.md` |
| `adr/` (mới) | Tách 47 ADR thành file riêng + 7 ADR còn thiếu (§3 mục 29) |
| specs | Schema: `challenge`, `boss`, `achievement`, `collection`, `event-season`, `progression-curve`, `activity` (registry), `rubric`, `video-activity`; OpenAPI cho các domain mới |

## 6.1. Đã thực hiện trong lượt cập nhật (2026-11-15)

| Hạng mục | Kết quả |
|---|---|
| Game progression, challenge/boss, inventory/economy | ✅ PROGRESSION_SYSTEM, CHALLENGE_BOSS_SYSTEM, INVENTORY_ECONOMY; QUEST_SYSTEM 12 loại; CHARACTER_SYSTEM (tạo avatar, emote 3D); 5 schema |
| Học tập đa định dạng + Activity Registry | ✅ ACTIVITY_ENGINE, VIDEO_LEARNING, AUDIO_LANGUAGE_LEARNING, WRITTEN_RESPONSE_RUBRIC, SIMULATION_ENGINE; 8 schema activity |
| CMS một portal, 5 workspace | ✅ CMS_DESIGN §1.2–1.4, §10.1 (đặc tả 7 màn chính) |
| 7 ADR bắt buộc còn thiếu | ✅ ADR-0048…0054 (Proposed) |
| Công thức | ✅ ADR-0055 + tài liệu liên quan |
| Specs | ✅ 213 path / 245 operation / 126 mã lỗi khớp catalog; AsyncAPI 108 message; 24 schema |
| Mermaid | ✅ 113/113 hợp lệ, kiểm tra trong CI |
| **Để lượt sau** | UX từng màn của game (§25), RTM đầy đủ (§33D), tách ADR thành file riêng, Guild/Club chi tiết (có ADR-0050), Event & Season / Live-ops event builder, Game Analytics, World map/storyline chi tiết |

## 7. Kết luận

- **Đáp ứng:** khoảng 55–60% yêu cầu của Master Prompt mới được phủ đầy đủ, chủ yếu ở nền tảng kỹ thuật, identity/SIS, an toàn, chat, multiplayer, vận hành [ước lượng định tính].
- **Chưa đáp ứng:** game progression chuyên sâu, challenge/boss, exploration, economy, live-ops event, guild/club, học tập đa định dạng, activity registry, UX từng màn, game analytics, RTM, 7 ADR bắt buộc.
- **Chặn trước khi cập nhật:** cần stakeholder quyết định 7 mâu thuẫn ở §4, đặc biệt C1 (monolith vs microservice) và C2 (chat theo khối lớp).
- **Bước tiếp theo đề xuất:** sau khi chốt §4, thực hiện Stage 3–7 của Master Prompt: viết các tài liệu ở §6, cập nhật specs, RTM, rồi validate lại toàn bộ.

## 8. Hợp nhất tài liệu (2026-11-15)

Stakeholder yêu cầu: *"review lại tất cả specs và docs xem đã phù hợp game chưa? merge các tài liệu lại để thống nhất không thừa và bị đứt quãng"*. Bản sao lưu trước khi hợp nhất: `d:\game\eduWorld_backup\` (ngoài repo).

### 8.1. Cấu trúc mới (game-first)

`00-overview` → `01-game-design` → `02-learning` → `03-social-multiplayer` → `04-admin-portal` → `05-ux` → `06-architecture` → `07-backend` → `08-game-client` → `09-identity` → `10-security` → `11-operations` → `12-project` → `adr/`. Bản đồ chi tiết: [docs/README](../README.md).

| Thay đổi | Số lượng | Công cụ |
|---|---|---|
| ADR tách từ một file ~1.000 dòng thành file riêng `docs/adr/ADR-NNNN-<slug>.md` + `README.md` (chỉ mục, quyết định hiện hành theo chủ đề) + `TEMPLATE.md`; mỗi ADR chuẩn hóa: quyết định hiện hành liền mạch + "Lịch sử thay đổi" | 55 ADR | `scripts/docs/split-adr.mjs` |
| Tài liệu di chuyển/đổi tên (link tự sửa) | 67 file | `scripts/docs/move-docs.mjs` |
| Tài liệu gộp vào tài liệu đích, file nguồn xóa, link chuyển hướng | 9 file | `scripts/docs/redirect-merged.mjs` |
| Tài liệu mới | 1 ([CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md)) | — |

### 8.2. Bảng file cũ → file mới

| File cũ | File hiện tại | Cách xử lý |
|---|---|---|
| `docs/01-product/GAME_DESIGN.md` | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md) | di chuyển |
| `docs/01-product/GAMEPLAY_LOOP.md` | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md) | di chuyển + **gộp** |
| `docs/01-product/GAME_WORLD_DESIGN.md` | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) | di chuyển + đổi tên |
| `docs/01-product/PROGRESSION_SYSTEM.md` | [PROGRESSION_SYSTEM](../01-game-design/PROGRESSION_SYSTEM.md) | di chuyển |
| `docs/01-product/CHALLENGE_BOSS_SYSTEM.md` | [CHALLENGE_BOSS_SYSTEM](../01-game-design/CHALLENGE_BOSS_SYSTEM.md) | di chuyển |
| `docs/01-product/INVENTORY_ECONOMY.md` | [INVENTORY_ECONOMY](../01-game-design/INVENTORY_ECONOMY.md) | di chuyển |
| `docs/01-product/COMPETITION_DESIGN.md` | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | di chuyển + đổi tên |
| `docs/06-game/QUEST_SYSTEM.md` | [QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md) | di chuyển |
| `docs/06-game/CHARACTER_SYSTEM.md` | [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) | di chuyển |
| `docs/07-multiplayer/RANKING.md` | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | di chuyển + **gộp** |
| `docs/07-multiplayer/LEADERBOARD.md` | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | di chuyển + **gộp** |
| `docs/07-multiplayer/TOURNAMENT.md` | [COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md) | di chuyển + **gộp** |
| `docs/01-product/LEARNING_EXPERIENCE.md` | [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) | di chuyển |
| `docs/05-learning/CURRICULUM_MODEL.md` | [CURRICULUM_MODEL](../02-learning/CURRICULUM_MODEL.md) | di chuyển |
| `docs/05-learning/CONTENT_MODEL.md` | [CONTENT_MODEL](../02-learning/CONTENT_MODEL.md) | di chuyển |
| `docs/05-learning/QUESTION_BANK.md` | [QUESTION_BANK](../02-learning/QUESTION_BANK.md) | di chuyển |
| `docs/05-learning/ACTIVITY_ENGINE.md` | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | di chuyển |
| `docs/05-learning/EXERCISE_ENGINE.md` | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) | di chuyển + **gộp** |
| `docs/05-learning/EXAM_ENGINE.md` | [EXAM_ENGINE](../02-learning/EXAM_ENGINE.md) | di chuyển |
| `docs/05-learning/VIDEO_LEARNING.md` | [VIDEO_LEARNING](../02-learning/VIDEO_LEARNING.md) | di chuyển |
| `docs/05-learning/AUDIO_LANGUAGE_LEARNING.md` | [AUDIO_LANGUAGE_LEARNING](../02-learning/AUDIO_LANGUAGE_LEARNING.md) | di chuyển |
| `docs/05-learning/WRITTEN_RESPONSE_RUBRIC.md` | [WRITTEN_RESPONSE_RUBRIC](../99-deferred/WRITTEN_RESPONSE_RUBRIC.md) | di chuyển |
| `docs/05-learning/SIMULATION_ENGINE.md` | [SIMULATION_ENGINE](../02-learning/SIMULATION_ENGINE.md) | di chuyển |
| `docs/05-learning/LEARNING_ANALYTICS.md` | [LEARNING_ANALYTICS](../02-learning/LEARNING_ANALYTICS.md) | di chuyển |
| `docs/07-multiplayer/MULTIPLAYER_DESIGN.md` | [MULTIPLAYER_GAMEPLAY](../03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) | di chuyển + đổi tên |
| `docs/07-multiplayer/PARTY_SYSTEM.md` | [PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md) | di chuyển |
| `docs/07-multiplayer/MATCHMAKING.md` | [MATCHMAKING](../03-social-multiplayer/MATCHMAKING.md) | di chuyển |
| `docs/07-multiplayer/CHAT_SYSTEM.md` | [CHAT_SYSTEM](../03-social-multiplayer/CHAT_SYSTEM.md) | di chuyển |
| `docs/07-multiplayer/MODERATION.md` | [MODERATION](../03-social-multiplayer/MODERATION.md) | di chuyển |
| `docs/01-product/SOCIAL_CHAT_DESIGN.md` | [SOCIAL_SYSTEM](../03-social-multiplayer/SOCIAL_SYSTEM.md) | di chuyển + đổi tên |
| `docs/08-cms-lms/CMS_DESIGN.md` | [ADMIN_PORTAL](../04-admin-portal/ADMIN_PORTAL.md) | di chuyển + đổi tên |
| `docs/08-cms-lms/LMS_DESIGN.md` | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) | di chuyển + đổi tên |
| `docs/08-cms-lms/TEACHER_DASHBOARD.md` | [SCHOOL_LMS](../04-admin-portal/SCHOOL_LMS.md) | di chuyển + **gộp** |
| `docs/01-product/DESIGN_SYSTEM.md` | [DESIGN_SYSTEM](../05-ux/DESIGN_SYSTEM.md) | di chuyển |
| `docs/02-architecture/SYSTEM_ARCHITECTURE.md` | [SYSTEM_ARCHITECTURE](../06-architecture/SYSTEM_ARCHITECTURE.md) | di chuyển |
| `docs/02-architecture/DOMAIN_MODEL.md` | [DOMAIN_MODEL](../06-architecture/DOMAIN_MODEL.md) | di chuyển |
| `docs/02-architecture/MULTITENANCY.md` | [MULTITENANCY](../06-architecture/MULTITENANCY.md) | di chuyển |
| `docs/02-architecture/REALTIME_ARCHITECTURE.md` | [REALTIME_ARCHITECTURE](../06-architecture/REALTIME_ARCHITECTURE.md) | di chuyển |
| `docs/03-backend/BACKEND_DESIGN.md` | [BACKEND_DESIGN](../07-backend/BACKEND_DESIGN.md) | di chuyển |
| `docs/03-backend/API_DESIGN.md` | [API_DESIGN](../07-backend/API_DESIGN.md) | di chuyển |
| `docs/03-backend/DATABASE_DESIGN.md` | [DATABASE_DESIGN](../07-backend/DATABASE_DESIGN.md) | di chuyển |
| `docs/03-backend/EVENT_DESIGN.md` | [EVENT_DESIGN](../07-backend/EVENT_DESIGN.md) | di chuyển |
| `docs/03-backend/VALKEY_DESIGN.md` | [VALKEY_DESIGN](../07-backend/VALKEY_DESIGN.md) | di chuyển |
| `docs/06-game/GODOT_ARCHITECTURE.md` | [GODOT_ARCHITECTURE](../08-game-client/GODOT_ARCHITECTURE.md) | di chuyển |
| `docs/06-game/3D_WORLD_DESIGN.md` | [WORLD_RENDERING](../08-game-client/WORLD_RENDERING.md) | di chuyển + đổi tên |
| `docs/06-game/MINIGAME_ENGINE.md` | [MINIGAME_ENGINE](../08-game-client/MINIGAME_ENGINE.md) | di chuyển |
| `docs/06-game/GAME_NETWORKING.md` | [GAME_NETWORKING](../08-game-client/GAME_NETWORKING.md) | di chuyển |
| `docs/04-identity/KEYCLOAK_SSO.md` | [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) | di chuyển |
| `docs/04-identity/IDENTITY_MAPPING.md` | [IDENTITY_MAPPING](../09-identity/IDENTITY_MAPPING.md) | di chuyển |
| `docs/04-identity/SIS_INTEGRATION.md` | [SIS_INTEGRATION](../09-identity/SIS_INTEGRATION.md) | di chuyển |
| `docs/04-identity/ACCOUNT_PROVISIONING.md` | [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) | di chuyển |
| `docs/02-architecture/SECURITY_ARCHITECTURE.md` | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) | di chuyển |
| `docs/09-security/SECURITY.md` | [SECURITY_ARCHITECTURE](../10-security/SECURITY_ARCHITECTURE.md) | di chuyển + **gộp** |
| `docs/09-security/DATA_PRIVACY.md` | [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) | di chuyển |
| `docs/09-security/ANTI_CHEAT.md` | [ANTI_CHEAT](../10-security/ANTI_CHEAT.md) | di chuyển |
| `docs/09-security/CHILD_PROTECTION.md` | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | di chuyển + **gộp** |
| `docs/01-product/CHILD_SAFETY.md` | [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) | di chuyển |
| `docs/10-devops/DEPLOYMENT.md` | [DEPLOYMENT](../11-operations/DEPLOYMENT.md) | di chuyển |
| `docs/10-devops/LOCAL_DEVELOPMENT.md` | [LOCAL_DEVELOPMENT](../11-operations/LOCAL_DEVELOPMENT.md) | di chuyển |
| `docs/10-devops/MONITORING.md` | [MONITORING](../11-operations/MONITORING.md) | di chuyển |
| `docs/10-devops/BACKUP_RECOVERY.md` | [BACKUP_RECOVERY](../11-operations/BACKUP_RECOVERY.md) | di chuyển |
| `docs/11-project/ROADMAP.md` | [ROADMAP](ROADMAP.md) | di chuyển |
| `docs/11-project/MILESTONES.md` | [ROADMAP](ROADMAP.md) | di chuyển + **gộp** |
| `docs/11-project/TEST_STRATEGY.md` | [TEST_STRATEGY](TEST_STRATEGY.md) | di chuyển |
| `docs/11-project/RISKS.md` | [RISKS](RISKS.md) | di chuyển |
| `docs/11-project/OPEN_QUESTIONS.md` | [OPEN_QUESTIONS](OPEN_QUESTIONS.md) | di chuyển |
| `docs/11-project/DOCUMENTATION_AUDIT.md` | [DOCUMENTATION_AUDIT](DOCUMENTATION_AUDIT.md) | di chuyển |
| `docs/02-architecture/ARCHITECTURE_DECISIONS.md` | [adr/README](../adr/README.md) + 55 file ADR | tách |

### 8.3. Review "phù hợp game" — phát hiện và xử lý

| # | Phát hiện | Xử lý | Tài liệu |
|---|---|---|---|
| G-1 | Thể loại ghi "educational adventure + quiz competition", mang tính quiz | Định vị lại: **phiêu lưu 3D multiplayer RPG-lite**, học tập lồng trong nhiệm vụ; bản sắc "game trước trong trải nghiệm, giáo dục trong mục đích, thế giới multiplayer xã hội trong tương tác" | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md), [PRODUCT_VISION](../00-overview/PRODUCT_VISION.md) |
| G-2 | Khu vực đặt tên theo chức năng nhà trường (Learning/Practice/Exam Zone) như menu phần mềm; không có cốt truyện | 8 vùng đất "Quần đảo Tri Thức" bao trọn 7 khu chức năng bắt buộc; mạch truyện 7 chương "Thắp lại Tia Sáng", boss cuối chương; chương môn học mở song song (không khóa học tập) | [WORLD_DESIGN](../01-game-design/WORLD_DESIGN.md) |
| G-3 | Không có quy tắc chống biến game thành chuỗi quiz | Quy tắc L-1…L-6 (GDD) và §2.1 LEARNING_EXPERIENCE: khối học tập ngắn rồi xen tương tác thế giới; mọi hoạt động học tập có ngữ cảnh game (NPC, boss, khám phá) | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md), [LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md) |
| G-4 | Tài liệu học tập viết theo kiểu LMS ("module exercise", "màn làm bài") | Viết lại theo ngữ cảnh game; Activity & Attempt Engine là lõi chung | [ACTIVITY_ENGINE](../02-learning/ACTIVITY_ENGINE.md) |
| G-5 | Core loop bắt đầu bằng "chọn hoạt động: quest/luyện tập/ôn thi" | Bắt đầu bằng gợi ý phiêu lưu và khám phá (Mây); bảng động lực người chơi theo khối | [GAME_DESIGN](../01-game-design/GAME_DESIGN.md), [USER_PERSONAS](../00-overview/USER_PERSONAS.md) |
| G-6 | Roadmap chỉ liệt kê hạng mục kỹ thuật | Mỗi phase nêu "trải nghiệm người chơi khi xong phase" | [ROADMAP](ROADMAP.md) |
| G-7 | Tài liệu kỹ thuật không có luồng chơi xuyên suốt | Thêm luồng chơi end-to-end theo service (đăng nhập → hub → quest → activity → chấm → XP/level/mở khóa → party/boss/trận → chat → mastery) | [SYSTEM_ARCHITECTURE §5](../06-architecture/SYSTEM_ARCHITECTURE.md) |
| G-8 | Chưa có danh mục màn hình học sinh trong game | Danh mục 17 màn (chi tiết từng màn [TBD]) | [DESIGN_SYSTEM §5.7](../05-ux/DESIGN_SYSTEM.md) |
| G-9 | 4/20 kịch bản nghiệm thu Master Prompt §26 không có test case | Bổ sung; bảng ánh xạ 20 kịch bản → TC | [TEST_STRATEGY §6.0](TEST_STRATEGY.md) |
| G-10 | Tham số mặc định rải rác, con số lặp ở nhiều tài liệu | Quy tắc "một nguồn sở hữu" + sổ [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md) | [docs/README §3.3](../README.md) |

### 8.4. Mâu thuẫn đã xử lý trong đợt hợp nhất

| Chủ đề | Trước | Sau (nguồn sở hữu) |
|---|---|---|
| Season | 8 tuần vs theo học kỳ | 8 tuần ([COMPETITION_SYSTEM](../01-game-design/COMPETITION_SYSTEM.md)) |
| Soft reset rating | 0,5 vs 30% | 0,5 |
| Tier | 6 tier thiên nhiên vs 5 tier kim loại | 6 tier |
| Số rating thô | hiện cho lớp 6–9 vs ẩn | ẩn với mọi học sinh |
| Daily/weekly | 5 thử thách tuần vs 2–3 | 3 daily quest/ngày, 3 weekly quest/tuần ([QUEST_SYSTEM](../01-game-design/QUEST_SYSTEM.md)) |
| Lời mời party hết hạn | 2 phút vs 5 phút | 5 phút ([PARTY_SYSTEM](../03-social-multiplayer/PARTY_SYSTEM.md)) |
| Push notification | "FCM Phase 5" vs không push | Không push ngoài game cho học sinh trong Release 1.0 (FR-NTF-003) |
| Lượt thử mỗi câu | "tối đa 2" vs theo khối | g1_3: 3, khối khác: 2 ([LEARNING_EXPERIENCE](../02-learning/LEARNING_EXPERIENCE.md)) |
| SLA báo cáo | 2 bảng khác nhau (CHILD_SAFETY, SECURITY) | Chỉ [MODERATION §6.3](../03-social-multiplayer/MODERATION.md) |
| Xoay vòng secret | 180 vs 90 ngày | 90 ngày |
| CSV import | 20 MB vs 10 MB | 10 MB |
| DDL outbox | 2 phiên bản cột khác nhau | Một bản ở [DATABASE_DESIGN §3.9](../07-backend/DATABASE_DESIGN.md) |
| Resilience typed client | code mẫu lệch API_DESIGN | Theo [API_DESIGN §12.3](../07-backend/API_DESIGN.md) |
| Tên message WS ở client | lệch AsyncAPI | Theo AsyncAPI |
| Thứ tự kiểm tra tenant/tài khoản khi đăng nhập | MULTITENANCY vs IDENTITY_MAPPING | Chỉ [IDENTITY_MAPPING §4.1](../09-identity/IDENTITY_MAPPING.md) |
| Mã trùng | `NFR-AVAIL-003` 2 định nghĩa; `AC-CMP-001/002` 2 định nghĩa | `NFR-AVAIL-005`; COMPETITION_SYSTEM đổi thành `AC-CMP-101/102` |
| Team/Study Group | khái niệm riêng | Gộp vào Club/Guild (ADR-0050) |

Mâu thuẫn còn mở (cần chốt) liệt kê ở [CONFIG_DEFAULTS §19](../00-overview/CONFIG_DEFAULTS.md).

### 8.5. Việc còn lại

| Việc | Ghi chú |
|---|---|
| UX chi tiết 17 màn học sinh và màn Admin Portal còn lại | **Màn học sinh: đã làm** ([STUDENT_GAME_UX](../05-ux/STUDENT_GAME_UX.md), 18 màn Web/PC + Mobile); màn Admin Portal còn lại: ADMIN_PORTAL §10.1 |
| Requirements Traceability Matrix đầy đủ (PR → FR → component → API/data → TC → AC) | **Đã làm**: [TRACEABILITY_MATRIX](TRACEABILITY_MATRIX.md) (mục 9.3) |
| Event & Season / live-ops event builder, Game Analytics (DAU/WAU/retention/funnel) | **Đã làm**: [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md), [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) (ADR-0058) |
| Hợp nhất 2 họ attempt API (`/exercises/attempts` và `/learning/activity-attempts`) | **Đã làm** (ADR-0059): một họ `/learning/activity-attempts` |
| Spec cho club/guild, friend duel, mute; config type `world-areas`, `world-events`, `story-chapters`; area id mới; event `story.chapter_completed`; bảng `story_chapters` | Đang được đồng bộ vào specs/API/DB |
| Chốt các mâu thuẫn tham số còn mở | [CONFIG_DEFAULTS §19](../00-overview/CONFIG_DEFAULTS.md) |

## 9. Đợt cập nhật song ngữ, máy tầm trung, đăng nhập và truy vết (2026-11-15)

### 9.1. Đã làm

| Hạng mục | Kết quả | ADR / tài liệu |
|---|---|---|
| Song ngữ Việt – Anh | `i18nText`, gettext `.po`, `Accept-Language`, import/export bản dịch | ADR-0056 |
| Máy tầm trung | Hạng thiết bị, preset, ngân sách hiệu năng; FPS đo theo p10 | ADR-0057, [PERFORMANCE_BUDGET](../08-game-client/PERFORMANCE_BUDGET.md) |
| UX 18 màn học sinh (Web/PC + Mobile) | Xong (mục 8.5 dòng 1) | [STUDENT_GAME_UX](../05-ux/STUDENT_GAME_UX.md) |
| Event & Season, Game Analytics | Xong (mục 8.5 dòng 3) | ADR-0058, [EVENT_SEASON_SYSTEM](../01-game-design/EVENT_SEASON_SYSTEM.md), [GAME_ANALYTICS](../11-operations/GAME_ANALYTICS.md) |
| Danh mục quốc gia / tỉnh / trường; một trường một lớp | `specs/reference-data/*` + validator | ADR-0060 |
| Đồng bộ SIS khi đăng nhập | SIS contract v1.1.0 | ADR-0061 |
| Dashboard dữ liệu lớn | Chỉ đọc rollup, read replica, timeout, cache | ADR-0062, [DASHBOARD_DATA](../07-backend/DASHBOARD_DATA.md) |
| Đăng nhập, tenant, header, portal, RBAC, định danh | Username/password không chọn trường; form trong game qua backend; không tự đăng nhập; `X-Tenant-Id`; portal riêng + Role/UserRole; SSO trước, tenant sau | ADR-0063 … ADR-0069 |
| Spec đồng bộ (SPEC-3) | 312 path / 364 operation / 344 schema / 151 `ProblemCode`; `check-problem-codes.mjs` | [API_DESIGN](../07-backend/API_DESIGN.md) |
| Ma trận truy vết (mục 8.5 dòng 2) | Xong, sinh tự động, có báo cáo mồ côi | [TRACEABILITY_MATRIX](TRACEABILITY_MATRIX.md) |

### 9.2. Kết quả kiểm tra tự động

| Kiểm tra | Kết quả |
|---|---|
| Redocly 12 file OpenAPI, Spectral, AsyncAPI parser | 12/12 hợp lệ, 0 lỗi (lượt SPEC-3) |
| JSON Schema + ví dụ, reference data | 33 schema, 14 ví dụ; 4 quốc gia, 97 tỉnh/thành, 0 lỗi (lượt SPEC-3) |
| `maxLength` | 2.463 trường chuỗi, 0 vi phạm (lượt SPEC-3) |
| Mã lỗi API_DESIGN ↔ `ProblemCode` | 151 = 151 (lượt SPEC-3) |
| Mermaid, link + anchor, RTM `--check` | 112 sơ đồ, 0 lỗi; 3.522 link, 0 hỏng; RTM khớp |

### 9.3. Kết quả ma trận truy vết

- 295 FR (287 thuộc Release 1.0), 30 US, 93 AC trong PRD; 371 TC; 364 operation. Không có FR/AC/US **mồ côi thật** (mọi FR có tài liệu sở hữu, operation và TC ít nhất theo domain).
- **Liên kết tường minh còn mỏng:** 234/287 FR chưa được tài liệu thiết kế nhắc mã; 273/287 FR chưa có operation nhắc mã hoặc path; chưa TC nào ghi mã FR/AC của PRD. Việc tiếp theo: ghi mã FR/AC trong bảng TC và `description` của operation.
- **Phát hiện cần sửa nguồn:**
  - `AC-UX` có hai kiểu đánh số (`AC-UX-0001…` và `AC-UX-101…`) trong STUDENT_GAME_UX*; `AC-UX`, `AC-PERF` chưa gắn domain FR/NFR.
  - `AC-PTY-012…015` được nhắc trong PARTY_SYSTEM nhưng chưa được định nghĩa.
  - TC-I18N, TC-UI chưa ánh xạ domain FR.
  - PRD `US-AUTH-001/002` còn mô tả "một nút Đăng nhập", "lần sau tự vào luôn", trái ADR-0064/0068 (fork cập nhật PRD theo ADR-0069 đang xử lý).

### 9.4. Còn mở

| Mục | Ghi chú |
|---|---|
| 78 dòng endpoint trong API_DESIGN chưa có trong OpenAPI | Chủ yếu CRUD CMS/LMS/admin, moderation chat, tournament; trước đây không đánh dấu "spec: chưa" |
| KEYCLOAK_SSO còn nhắc `svc-keycloak-authn`, `tenants:resolve` | Fork cập nhật tài liệu identity theo ADR-0069 xử lý |
| OQ cũ còn mở: OQ-02, 04, 08, 12, 14, 16, 17, 18 (OQ-19, 20, 21 đã đóng, xem §9.5) | [OPEN_QUESTIONS](OPEN_QUESTIONS.md) |

**Quyết định `[Confirmed]` đang chờ người duyệt:**

- **Đăng nhập & RBAC:**
  - plugin APISIX kiểm tra header tenant;
  - dùng `INVALID_CREDENTIALS` cả khi tài khoản bị khóa tạm;
  - `login_ticket` lưu Valkey 2 phút;
  - p95 đăng nhập ≤ 400 ms;
  - certificate pinning trong game;
  - chống leo thang quyền khi gán role;
  - UserRole nguồn SIS không thu hồi tay;
  - tenant được tạo role tùy biến;
  - soft-delete giữ username/email đến khi ẩn danh hóa;
  - `/auth/game/*` không dùng `Idempotency-Key`.
- **Spec:**
  - bỏ `POST /learning/events` (thay bằng `reportActivityProgress` + `/game/telemetry`);
  - `RolePermission` không có schema riêng;
  - `/admin/schools:import` bắt buộc `X-Tenant-Id`;
  - `/cms/*` header tùy chọn cho role platform;
  - học sinh/trường/lớp chỉ trong `PlayerBootstrap`.
- **Hiệu năng:** đổi p5 → p10 nhưng giữ ngưỡng FPS 27/30/50 (tiêu chí lỏng hơn một chút).
- **Dashboard:** các tham số `[Confirmed]` trong DASHBOARD_DATA (ngưỡng khoảng thời gian, grain, TTL cache).
- **Danh mục:** các tham số `[Confirmed]` trong ADR-0060 (mã chính thức tỉnh `official_code` còn TBD).
- **Game:** tên boss chương 3–6.

### 9.5. Đã đóng sau báo cáo (2026-11-15)

| Mục | Quyết định |
|---|---|
| OQ-19 voice chat | Không làm trong Release 1.0 ([ADR-0070](../adr/ADR-0070-release-1-khong-voice-chat-khong-xoa-vat-ly-khong-purge-analytics.md)) |
| OQ-20 xóa vật lý theo luật | Không làm trong Release 1.0; chỉ soft-delete + ẩn danh hóa; ngoại lệ duy nhất ADR-0033 (ADR-0070) |
| Ngoại lệ purge analytics thô | Không làm; partition giữ lại sau ẩn danh hóa; theo dõi dung lượng (ADR-0070, RISK-46) |
| OQ-21 object storage production | floci cho mọi môi trường ([ADR-0071](../adr/ADR-0071-object-storage-production-dung-floci.md)); rủi ro được chấp nhận (RISK-41) |

**Cần người duyệt lưu ý thêm:**

- File ghi âm giọng nói học sinh (`ew-student-submissions/recordings/`) trước đây xóa sau 90 ngày; theo ADR-0070 nay chỉ ẩn khỏi API, file vẫn giữ trong bucket private [TBD: xác nhận hoặc thêm ngoại lệ xóa].
- Lifecycle tự xóa object ở `ew-media-quarantine` (file tạm 7 ngày) và `ew-exports` (file xuất báo cáo) vẫn giữ như ADR-0044; cần người duyệt xác nhận đây không thuộc phạm vi "không xóa vật lý" của ADR-0070.
- Bảng kỹ thuật `infra.outbox_events` (EVENT_DESIGN, DATABASE_DESIGN) vẫn mô tả dọn bằng gỡ partition sau 30 ngày; cần xác nhận có được coi là ngoại lệ kỹ thuật (không chứa dữ liệu cá nhân trực tiếp) hay giữ lại vĩnh viễn. → Đã chốt 2026-11-17: ngoại lệ có giới hạn cho `outbox_events`, `processed_events`, `idempotency_records` (ADR-0089 §5).
- `erase_student` đổi từ xóa user Keycloak sang vô hiệu hóa + ẩn danh hóa user Keycloak (DATA_PRIVACY).

## 10. Đợt bỏ vai trò giáo viên và review độ phù hợp giáo dục (2026-11-15)

| Mục | Kết quả |
|---|---|
| Review độ phù hợp cho game giáo dục | [EDU_FIT_REVIEW](EDU_FIT_REVIEW.md): 6 vấn đề sống còn (K1–K6), phát hiện theo 5 chiều, trạng thái xử lý ở §9 của tài liệu đó |
| Mô hình vai trò | [ADR-0072](../adr/ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md) (Accepted): không có vai trò giáo viên; học sinh tự học, nhà trường quản trị; kiểm duyệt tập trung ở Platform Safety Team (`safety_moderator`); trả lời OQ-07 |
| Mục §3 bảng trên: 15.1 (written response/essay/rubric/teacher review, audio/speaking, project-based), 15.5 (rubric, teacher review) | Không còn là yêu cầu của Release: các hoạt động cần chấm tay nằm ngoài Release theo ADR-0072 mục 3; thiết kế tham khảo giữ ở [WRITTEN_RESPONSE_RUBRIC](../99-deferred/WRITTEN_RESPONSE_RUBRIC.md) |
| Mốc dự án | M3 "Teacher Loop" đổi thành **M3 School Loop** ([ROADMAP §3.4](ROADMAP.md)) |
| Test | TC chấm tay đánh dấu Removed (ADR-0072); thêm TC-LRN-011…018, TC-CNT-010, TC-CHAT-042…043 [Confirmed] ([TEST_STRATEGY](TEST_STRATEGY.md)) |
| Rủi ro, câu hỏi mở | RISK-47…49 ([RISKS](RISKS.md)); OQ-23, OQ-24 ([OPEN_QUESTIONS](OPEN_QUESTIONS.md)) |
| ADR từ EDU_FIT_REVIEW | 2026-11-17: ADR-0073, 0074, 0075, 0078, 0080 Accepted; 0076, 0079 Rejected; 0077 hoãn (ADR-0082); trạng thái trong [Chỉ mục ADR](../adr/README.md) |
