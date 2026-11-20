# EduWorld — Bộ tài liệu & đặc tả

> Trạng thái: Draft · Phiên bản: 1.0 · Owner: Technical Writer · Cập nhật: 2026-11-15
> Liên quan: [README gốc](../README.md) · [specs/README](../specs/README.md) · [ADR](adr/README.md) · [CONFIG_DEFAULTS](00-overview/CONFIG_DEFAULTS.md) · [DOCUMENTATION_AUDIT](12-project/DOCUMENTATION_AUDIT.md)

Bộ tài liệu được tổ chức **game-first**: EduWorld là *game trước* trong trải nghiệm, *nền tảng giáo dục* trong mục đích, *thế giới multiplayer xã hội* trong tương tác ([PRODUCT_VISION](00-overview/PRODUCT_VISION.md)). Đặc tả máy đọc được (OpenAPI, AsyncAPI, JSON Schema, SIS contract, event) nằm trong [`../specs/`](../specs/README.md).

## 1. Bản đồ tài liệu

| Thư mục | Nội dung | Tài liệu |
|---|---|---|
| `00-overview/` | Tầm nhìn, yêu cầu, người dùng, thuật ngữ, tham số mặc định | [PRODUCT_VISION](00-overview/PRODUCT_VISION.md) · [PRODUCT_REQUIREMENTS](00-overview/PRODUCT_REQUIREMENTS.md) · [USER_PERSONAS](00-overview/USER_PERSONAS.md) · [GLOSSARY](00-overview/GLOSSARY.md) · [CONFIG_DEFAULTS](00-overview/CONFIG_DEFAULTS.md) |
| `01-game-design/` | Thiết kế game: GDD, thế giới & cốt truyện, nhân vật, tiến trình, nhiệm vụ, thử thách/boss, kinh tế, thi đấu, sự kiện & mùa | [GAME_DESIGN](01-game-design/GAME_DESIGN.md) · [WORLD_DESIGN](01-game-design/WORLD_DESIGN.md) · [CHARACTER_SYSTEM](01-game-design/CHARACTER_SYSTEM.md) · [PROGRESSION_SYSTEM](01-game-design/PROGRESSION_SYSTEM.md) · [QUEST_SYSTEM](01-game-design/QUEST_SYSTEM.md) · [CHALLENGE_BOSS_SYSTEM](01-game-design/CHALLENGE_BOSS_SYSTEM.md) · [INVENTORY_ECONOMY](01-game-design/INVENTORY_ECONOMY.md) · [COMPETITION_SYSTEM](01-game-design/COMPETITION_SYSTEM.md) · [EVENT_SEASON_SYSTEM](01-game-design/EVENT_SEASON_SYSTEM.md) · [LEARNING_MECHANICS](01-game-design/LEARNING_MECHANICS.md) |
| `02-learning/` | Học tập trong game: trải nghiệm, chương trình, nội dung & phát hành, ngân hàng câu hỏi, activity engine, các định dạng, đề thi, phân tích | [LEARNING_EXPERIENCE](02-learning/LEARNING_EXPERIENCE.md) · [CURRICULUM_MODEL](02-learning/CURRICULUM_MODEL.md) · [CONTENT_MODEL](02-learning/CONTENT_MODEL.md) · [QUESTION_BANK](02-learning/QUESTION_BANK.md) · [ACTIVITY_ENGINE](02-learning/ACTIVITY_ENGINE.md) · [VIDEO_LEARNING](02-learning/VIDEO_LEARNING.md) · [AUDIO_LANGUAGE_LEARNING](02-learning/AUDIO_LANGUAGE_LEARNING.md) · [WRITTEN_RESPONSE_RUBRIC](99-deferred/WRITTEN_RESPONSE_RUBRIC.md) · [SIMULATION_ENGINE](02-learning/SIMULATION_ENGINE.md) · [EXAM_ENGINE](02-learning/EXAM_ENGINE.md) · [LEARNING_ANALYTICS](02-learning/LEARNING_ANALYTICS.md) · [LEARNING_EFFICACY_PLAN](02-learning/LEARNING_EFFICACY_PLAN.md) · [CONTENT_PRODUCTION_PLAN](02-learning/CONTENT_PRODUCTION_PLAN.md) · [ASSESSMENT_REGULATION_PROFILE](02-learning/ASSESSMENT_REGULATION_PROFILE.md) |
| `03-social-multiplayer/` | Chơi cùng nhau: multiplayer gameplay, party, ghép trận, bạn bè & club, chat, kiểm duyệt | [MULTIPLAYER_GAMEPLAY](03-social-multiplayer/MULTIPLAYER_GAMEPLAY.md) · [PARTY_SYSTEM](03-social-multiplayer/PARTY_SYSTEM.md) · [MATCHMAKING](03-social-multiplayer/MATCHMAKING.md) · [SOCIAL_SYSTEM](03-social-multiplayer/SOCIAL_SYSTEM.md) · [CHAT_SYSTEM](03-social-multiplayer/CHAT_SYSTEM.md) · [MODERATION](03-social-multiplayer/MODERATION.md) |
| `04-admin-portal/` | Admin Portal 5 workspace (CMS/LMS cho vận hành game), LMS & dashboard giáo viên | [ADMIN_PORTAL](04-admin-portal/ADMIN_PORTAL.md) · [SCHOOL_LMS](04-admin-portal/SCHOOL_LMS.md) |
| `05-ux/` | Hai hệ thống giao diện (game Godot 3D, Admin/Ops), 18 màn học sinh bố cục Web/PC và Mobile | [DESIGN_SYSTEM](05-ux/DESIGN_SYSTEM.md) · [STUDENT_GAME_UX](05-ux/STUDENT_GAME_UX.md) (tổng quan) · [STUDENT_GAME_UX_CORE](05-ux/STUDENT_GAME_UX_CORE.md) · [STUDENT_GAME_UX_LEARNING](05-ux/STUDENT_GAME_UX_LEARNING.md) · [STUDENT_GAME_UX_SOCIAL](05-ux/STUDENT_GAME_UX_SOCIAL.md) |
| `06-architecture/` | Kiến trúc tổng: C4, modular monolith (5 module domain + realtime, 3 binary — ADR-0074), luồng chơi end-to-end, domain model, multi-tenancy, realtime | [SYSTEM_ARCHITECTURE](06-architecture/SYSTEM_ARCHITECTURE.md) · [DOMAIN_MODEL](06-architecture/DOMAIN_MODEL.md) · [MULTITENANCY](06-architecture/MULTITENANCY.md) · [REALTIME_ARCHITECTURE](06-architecture/REALTIME_ARCHITECTURE.md) |
| `07-backend/` | Go microservices: Clean Architecture/CQRS, API, database, event, Valkey, dữ liệu dashboard (rollup) | [BACKEND_DESIGN](07-backend/BACKEND_DESIGN.md) · [API_DESIGN](07-backend/API_DESIGN.md) · [DATABASE_DESIGN](07-backend/DATABASE_DESIGN.md) · [EVENT_DESIGN](07-backend/EVENT_DESIGN.md) · [VALKEY_DESIGN](07-backend/VALKEY_DESIGN.md) · [DASHBOARD_DATA](07-backend/DASHBOARD_DATA.md) |
| `08-game-client/` | Godot client: kiến trúc, dựng thế giới 3D, mini-game, networking, ngân sách hiệu năng máy tầm trung | [GODOT_ARCHITECTURE](08-game-client/GODOT_ARCHITECTURE.md) · [WORLD_RENDERING](08-game-client/WORLD_RENDERING.md) · [MINIGAME_ENGINE](08-game-client/MINIGAME_ENGINE.md) · [GAME_NETWORKING](08-game-client/GAME_NETWORKING.md) · [PERFORMANCE_BUDGET](08-game-client/PERFORMANCE_BUDGET.md) |
| `09-identity/` | Keycloak SSO, ánh xạ định danh & tenant, provisioning, SIS | [KEYCLOAK_SSO](09-identity/KEYCLOAK_SSO.md) · [IDENTITY_MAPPING](09-identity/IDENTITY_MAPPING.md) · [ACCOUNT_PROVISIONING](09-identity/ACCOUNT_PROVISIONING.md) · [SIS_INTEGRATION](09-identity/SIS_INTEGRATION.md) |
| `10-security/` | Kiến trúc & chương trình bảo mật, an toàn trẻ em, quyền riêng tư, chống gian lận | [SECURITY_ARCHITECTURE](10-security/SECURITY_ARCHITECTURE.md) · [CHILD_SAFETY](10-security/CHILD_SAFETY.md) · [DATA_PRIVACY](10-security/DATA_PRIVACY.md) · [ANTI_CHEAT](10-security/ANTI_CHEAT.md) |
| `11-operations/` | Triển khai Docker (Compose/Swarm), local dev, giám sát, sao lưu, Game Analytics | [DEPLOYMENT](11-operations/DEPLOYMENT.md) · [LOCAL_DEVELOPMENT](11-operations/LOCAL_DEVELOPMENT.md) · [MONITORING](11-operations/MONITORING.md) · [BACKUP_RECOVERY](11-operations/BACKUP_RECOVERY.md) · [GAME_ANALYTICS](11-operations/GAME_ANALYTICS.md) |
| `12-project/` | Lộ trình & milestone, kiểm thử, truy vết yêu cầu, rủi ro, câu hỏi mở, audit tài liệu | [ROADMAP](12-project/ROADMAP.md) · [TEST_STRATEGY](12-project/TEST_STRATEGY.md) · [TRACEABILITY_MATRIX](12-project/TRACEABILITY_MATRIX.md) (sinh tự động) · [RISKS](12-project/RISKS.md) · [OPEN_QUESTIONS](12-project/OPEN_QUESTIONS.md) · [DOCUMENTATION_AUDIT](12-project/DOCUMENTATION_AUDIT.md) · [CODE_PLAN_CMS_LMS_FIRST](12-project/CODE_PLAN_CMS_LMS_FIRST.md) · [SPEC_RECONCILIATION_BACKLOG](12-project/SPEC_RECONCILIATION_BACKLOG.md) · [EDU_FIT_REVIEW](12-project/EDU_FIT_REVIEW.md) |
| `adr/` | 69 Architecture Decision Record, mỗi ADR một file; chỉ mục + quyết định hiện hành theo chủ đề | [adr/README](adr/README.md) · [TEMPLATE](adr/TEMPLATE.md) |

## 2. Thứ tự đọc theo vai trò

| Bạn là | Đọc theo thứ tự |
|---|---|
| Người mới | [PRODUCT_VISION](00-overview/PRODUCT_VISION.md) → [GAME_DESIGN](01-game-design/GAME_DESIGN.md) → [WORLD_DESIGN](01-game-design/WORLD_DESIGN.md) → [LEARNING_EXPERIENCE](02-learning/LEARNING_EXPERIENCE.md) → [SYSTEM_ARCHITECTURE](06-architecture/SYSTEM_ARCHITECTURE.md) (§5 luồng chơi end-to-end) → [adr/README](adr/README.md) → [ROADMAP](12-project/ROADMAP.md) → [GLOSSARY](00-overview/GLOSSARY.md) |
| Game designer / Live-ops | [GAME_DESIGN](01-game-design/GAME_DESIGN.md) → [WORLD_DESIGN](01-game-design/WORLD_DESIGN.md) → [PROGRESSION_SYSTEM](01-game-design/PROGRESSION_SYSTEM.md) → [QUEST_SYSTEM](01-game-design/QUEST_SYSTEM.md) → [CHALLENGE_BOSS_SYSTEM](01-game-design/CHALLENGE_BOSS_SYSTEM.md) → [INVENTORY_ECONOMY](01-game-design/INVENTORY_ECONOMY.md) → [COMPETITION_SYSTEM](01-game-design/COMPETITION_SYSTEM.md) → [EVENT_SEASON_SYSTEM](01-game-design/EVENT_SEASON_SYSTEM.md) → [ADMIN_PORTAL](04-admin-portal/ADMIN_PORTAL.md) (Game Operations Console, Live Ops) → [CONFIG_DEFAULTS](00-overview/CONFIG_DEFAULTS.md) |
| EdTech / biên tập nội dung | [LEARNING_EXPERIENCE](02-learning/LEARNING_EXPERIENCE.md) → [CURRICULUM_MODEL](02-learning/CURRICULUM_MODEL.md) → [CONTENT_MODEL](02-learning/CONTENT_MODEL.md) → [QUESTION_BANK](02-learning/QUESTION_BANK.md) → [ACTIVITY_ENGINE](02-learning/ACTIVITY_ENGINE.md) → các định dạng (video, audio, tự luận, mô phỏng) → [SCHOOL_LMS](04-admin-portal/SCHOOL_LMS.md) |
| Backend (Go) | [SYSTEM_ARCHITECTURE](06-architecture/SYSTEM_ARCHITECTURE.md) → [DOMAIN_MODEL](06-architecture/DOMAIN_MODEL.md) → [BACKEND_DESIGN](07-backend/BACKEND_DESIGN.md) → [API_DESIGN](07-backend/API_DESIGN.md) → [DATABASE_DESIGN](07-backend/DATABASE_DESIGN.md) → [EVENT_DESIGN](07-backend/EVENT_DESIGN.md) → [VALKEY_DESIGN](07-backend/VALKEY_DESIGN.md) → [MULTITENANCY](06-architecture/MULTITENANCY.md) |
| Godot | [GAME_DESIGN](01-game-design/GAME_DESIGN.md) → [GODOT_ARCHITECTURE](08-game-client/GODOT_ARCHITECTURE.md) → [WORLD_RENDERING](08-game-client/WORLD_RENDERING.md) → [GAME_NETWORKING](08-game-client/GAME_NETWORKING.md) → [REALTIME_ARCHITECTURE](06-architecture/REALTIME_ARCHITECTURE.md) → [MINIGAME_ENGINE](08-game-client/MINIGAME_ENGINE.md) → [DESIGN_SYSTEM](05-ux/DESIGN_SYSTEM.md) |
| Identity / Security | [KEYCLOAK_SSO](09-identity/KEYCLOAK_SSO.md) → [IDENTITY_MAPPING](09-identity/IDENTITY_MAPPING.md) → [ACCOUNT_PROVISIONING](09-identity/ACCOUNT_PROVISIONING.md) → [SIS_INTEGRATION](09-identity/SIS_INTEGRATION.md) → [SECURITY_ARCHITECTURE](10-security/SECURITY_ARCHITECTURE.md) → [CHILD_SAFETY](10-security/CHILD_SAFETY.md) → [DATA_PRIVACY](10-security/DATA_PRIVACY.md) |
| DevOps | [DEPLOYMENT](11-operations/DEPLOYMENT.md) → [LOCAL_DEVELOPMENT](11-operations/LOCAL_DEVELOPMENT.md) → [MONITORING](11-operations/MONITORING.md) → [BACKUP_RECOVERY](11-operations/BACKUP_RECOVERY.md) → [SECURITY_ARCHITECTURE](10-security/SECURITY_ARCHITECTURE.md) |

## 3. Quy ước tài liệu

### 3.1. Header và cấu trúc

```markdown
# <Tiêu đề>

> Trạng thái: Draft | Review | Approved · Phiên bản: x.y · Owner: <vai trò> · Cập nhật: YYYY-MM-DD
> Liên quan: <link tài liệu / ADR>

## 1. Mục đích và phạm vi
...
## n. Acceptance criteria (hoặc link TEST_STRATEGY)
```

### 3.2. Nhãn mức độ xác nhận

| Nhãn | Ý nghĩa |
|---|---|
| **[Confirmed]** | Có trong Master Prompt / Addendum hoặc đã được stakeholder chốt |
| **[Proposed]** | Đội dự án đề xuất, dùng làm mặc định cho tới khi review |
| **[TBD]** | Chưa quyết, phụ thuộc Open Question (`OQ-xx`) |

Không biến giả định thành yêu cầu đã xác nhận.

Ngày 2026-11-20 owner dự án chốt toàn bộ phương án `[Proposed]` đang có trong `docs/` và `specs/` (trừ `docs/99-deferred/`, `specs/_deferred/`): nhãn được đổi thành `[Confirmed]`, nguồn là quyết định này. Đề xuất mới phát sinh sau ngày đó vẫn ghi `[Proposed]` cho tới khi owner chốt. Cùng ngày, owner chốt mọi `[TBD]`, mọi Open Question còn mở (trừ mục hoãn theo ADR-0082) và mọi ADR còn `Proposed` ([OPEN_QUESTIONS](12-project/OPEN_QUESTIONS.md), [chỉ mục ADR](adr/README.md)); nhãn `[TBD]` chỉ dùng lại cho câu hỏi mới.

### 3.3. Một nguồn sở hữu (không lặp, không đứt quãng)

- **Mỗi quy tắc, tham số, sơ đồ có đúng một tài liệu sở hữu.** Tài liệu khác chỉ link, không chép lại nội dung hay con số.
- **Tham số mặc định:** chỉ mục ở [CONFIG_DEFAULTS](00-overview/CONFIG_DEFAULTS.md); đổi giá trị phải sửa tài liệu sở hữu và CONFIG_DEFAULTS trong cùng merge request.
- **Quyết định kiến trúc:** ở [adr/](adr/README.md); tài liệu khác link tới file ADR, không chép quyết định.
- **Yêu cầu gốc:** ở [PRODUCT_REQUIREMENTS](00-overview/PRODUCT_REQUIREMENTS.md); tài liệu domain mở rộng chi tiết và AC.
- **Contract API/event:** ở `specs/` (openapi, asyncapi, schemas, events, sis); tài liệu chỉ mô tả ý nghĩa và link.
- **Feature spec (SDD, ADR-0082):** tiêu chí chấp nhận EARS và kế hoạch triển khai của từng tính năng ở `specs/<NNN>-<slug>/` (chuẩn GitHub Spec Kit); danh mục, thứ tự, trạng thái ở [CODE_PLAN](12-project/CODE_PLAN_CMS_LMS_FIRST.md); constitution ở [`.specify/memory/constitution.md`](../.specify/memory/constitution.md). Mã FR chỉ do PRD cấp.
- **Nội dung không làm trong version này:** `docs/99-deferred/`, `specs/_deferred/` — không triển khai.

### 3.4. Mã định danh

| Loại | Định dạng | Ví dụ |
|---|---|---|
| Functional requirement | `FR-<DOMAIN>-NNN` | `FR-QB-012` |
| Non-functional requirement | `NFR-<CAT>-NNN` | `NFR-PERF-003` |
| User story | `US-<DOMAIN>-NNN` | `US-PTY-004` |
| Acceptance criterion | `AC-<DOMAIN>-NNN` | `AC-AUTH-007` |
| Test case | `TC-<DOMAIN>-NNN` | `TC-GAM-004` |
| ADR | `ADR-NNNN` (file `adr/ADR-NNNN-<slug>.md`) | `ADR-0007` |
| Open question | `OQ-NN` | `OQ-03` |
| Risk | `RISK-NN` | `RISK-05` |

Mã domain: `GDD` (game design tổng), `WLD` (thế giới/khám phá), `GAME` (world/character), `GAM` (XP/level/progression), `QST` (quest), `CHL` (challenge/boss), `ECO` (tiền tệ/shop/collection/achievement), `EVT` (sự kiện live-ops, community goal, season track), `RWD` (reward), `CMP` (thi đấu), `RNK` (ranking), `LDB` (leaderboard), `TRN` (tournament), `MG` (mini-game), `MP` (multiplayer), `PTY` (party), `MM` (matchmaking), `SOC` (bạn bè/club), `CHAT`, `MOD` (kiểm duyệt), `NTF` (thông báo), `CUR` (curriculum), `CNT` (nội dung), `QB` (question bank), `EXR` (activity/attempt), `EXM` (đề thi), `PRG` (learning progress/mastery), `ANL` (analytics), `CMS`, `LMS`, `AUTH`, `ORG` (tenant/trường/lớp), `SIS`, `AUD` (audit), `OPS`. Category NFR: `PERF`, `SCAL`, `AVAIL`, `SEC`, `PRIV`, `SAFE`, `A11Y`, `I18N`, `OBS`, `MAINT`, `COMP`, `DEPL`.

### 3.5. Quy ước kỹ thuật dùng chung

- Ngôn ngữ: tiếng Việt; thuật ngữ kỹ thuật giữ tiếng Anh ([GLOSSARY](00-overview/GLOSSARY.md)).
- Sơ đồ: Mermaid (kiểm tra cú pháp trong CI).
- API: `/api/v1/<domain>/...`, JSON `snake_case`, ID UUIDv7, thời gian ISO-8601 UTC, mọi chuỗi có `maxLength` (ADR-0041).
- Event: domain event `<entity>.<past_verb>`; WS message 3 phần `<domain>.<entity>.<action>` ([EVENT_DESIGN](07-backend/EVENT_DESIGN.md)).
- Valkey key: `ew:<module>:<entity>:<id>[:<sub>]`; kênh realtime `ew:rt:*` ([VALKEY_DESIGN](07-backend/VALKEY_DESIGN.md)).
- PostgreSQL: mỗi module domain một database trên một cụm (ADR-0074); `tenant_id` + soft-delete trên mọi bảng nghiệp vụ ([DATABASE_DESIGN](07-backend/DATABASE_DESIGN.md)).

### 3.6. Quy trình cập nhật

1. Thay đổi kiến trúc → tạo ADR mới từ [TEMPLATE](adr/TEMPLATE.md); ADR cũ chuyển Superseded, ghi lịch sử thay đổi.
2. Thay đổi API → sửa `specs/` trước, rồi tài liệu, rồi code.
3. Trả lời Open Question → cập nhật [OPEN_QUESTIONS](12-project/OPEN_QUESTIONS.md) (nhật ký trả lời) và đổi nhãn liên quan.
4. Di chuyển/đổi tên tài liệu: `node scripts/docs/move-docs.mjs <map.json>` (tự sửa link). Gộp tài liệu: đưa nội dung vào file đích, thay file nguồn bằng một dòng `<!-- MERGED-INTO: docs/<đích> -->`, rồi chạy `node scripts/docs/redirect-merged.mjs` (xóa file nguồn, chuyển link). Tách ADR: `scripts/docs/split-adr.mjs` (đã dùng 2026-11-15).
5. Kiểm tra trước khi merge: `make docs-check` (Mermaid + link + anchor) và kiểm tra specs ([specs/README](../specs/README.md)).
6. Ghi [CHANGELOG](../CHANGELOG.md).
