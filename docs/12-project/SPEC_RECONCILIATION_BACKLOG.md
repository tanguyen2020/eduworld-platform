# Backlog đối soát docs ↔ specs ↔ feature spec

> Trạng thái: Draft · Phiên bản: 0.5 · Owner: Technical Architect + Tech Lead · Cập nhật: 2026-11-17 (§3.9 delta theo ADR-0074 modular monolith và ADR-0088; §3.8 delta feature theo ADR-0078/0086 bổ sung và các quyết định Technical Architect 2026-11-17)
> Liên quan: [CODE_PLAN](CODE_PLAN_CMS_LMS_FIRST.md) · [ADR-0082](../adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md) · [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md) · [DOCUMENTATION_AUDIT](DOCUMENTATION_AUDIT.md)

## 1. Mục đích và cách xử lý

Khi viết 91 feature spec (`specs/<NNN>-<slug>/`), các agent phát hiện chỗ tài liệu sở hữu, contract trong `specs/` và feature spec chưa khớp nhau. Tài liệu này là **danh sách việc** để đối soát; nó không định nghĩa quy tắc (R33).

- Mỗi mục có mã `RB-NNN`. Xử lý: sửa **tài liệu sở hữu** (và contract/CONFIG_DEFAULTS nếu có) trong cùng merge request, sau đó đổi trạng thái thành `Đã xử lý` kèm commit/ADR.
- Phần lớn mục đã có task **Polish** trong `tasks.md` của feature liên quan (cột Feature). Feature chọn phương án ghi ở cột "Feature đã chọn" là mặc định tạm [Proposed] cho tới khi tài liệu sở hữu được sửa.
- **Delta contract** (operationId, schema, event cần thêm/sửa) được liệt kê đầy đủ trong `contracts/README.md` §6 của từng feature và có task Phase 2; không lặp lại ở đây.
- Ưu tiên: `P1` chặn code hoặc sai bảo mật/dữ liệu · `P2` gây hiểu sai khi code · `P3` câu chữ, tên.

> **Trạng thái 2026-11-17:** đợt đối soát đã sửa tài liệu sở hữu và CONFIG_DEFAULTS cho mọi mục §2–§4 (kể cả §3.7 và CODE_PLAN D1–D18); feature spec được đồng bộ theo. Phần còn mở duy nhất là **delta contract** trong `specs/` (OpenAPI, AsyncAPI, schema, event), làm theo task Phase 2 của từng feature khi triển khai — ví dụ gỡ `FORBIDDEN_SCOPE`, `joinFriendInWorld`, thêm `system.server.draining`, `competition-rewards`, `progression-levels`, đổi `learning-event` sang `activity.*`.

## 2. Quyết định đã chốt

Technical Architect chốt ngày 2026-11-16: mọi câu hỏi dưới đây **làm theo mặc định đề xuất** (cột 3 là quyết định).

| Mã | Câu hỏi | Quyết định [Confirmed — 2026-11-16] | Feature |
|---|---|---|---|
| RB-001 | Job purge chat gameplay (M4b) cần report/case của moderation (M5). Đưa report/case tối thiểu lên M4b? | Purge fail-closed: chưa có module moderation thì không xóa, chỉ cảnh báo | F117, F121, F119 |
| RB-002 | Chặn một người có kết thúc tình bạn không? | Có (an toàn hơn cho trẻ) | F095 |
| RB-003 | Tên và vị trí cài đặt cho phép xã hội liên trường | `cross_school_social_enabled` | F095 |
| RB-004 | "Thông báo trường" gồm những loại nào? | Chỉ bài giao và kết quả bài giao | F123 |
| RB-005 | Reviewer trùng tác giả trả mã lỗi gì? | `403 PERMISSION_DENIED` | F025 |
| RB-006 | Kích hoạt `model_version` mastery bằng API hay CLI? | CLI có audit | F031 |
| RB-007 | `proposal_id` dùng làm `match_id`? Cài đặt trường cho competition qua `settings_subset` trong `school.updated`? | Có / Có (cần identity đồng ý) | F104, F105 |
| RB-008 | Party chat văn bản tự do ở P4 hay P5? (FR-PTY-001 P5 vs FR-CHAT-009 P4) | P4 theo FR-CHAT-009 | F117 |
| RB-009 | Định dạng giải double elim/vòng bảng/giải đội: P3 (F107) hay P6 (PRD)? | User story P3 trong F107 | F107 |
| RB-010 | `PlayerBootstrap.game_profile` chuyển thành tùy chọn? | Có | F013 |

## 3. Mâu thuẫn tài liệu theo domain

Cột "Feature đã chọn" là **quyết định đã chốt** (Technical Architect 2026-11-16, theo đề xuất). Mục ghi "Chưa chốt" dùng quyết định ở §3.7. Việc còn lại là sửa tài liệu sở hữu và CONFIG_DEFAULTS cho khớp; delta contract làm theo task Phase 2 của feature.

### 3.1. Identity, tenant, phân quyền

| Mã | Mâu thuẫn | Feature đã chọn | Ưu tiên | Feature |
|---|---|---|---|---|
| RB-101 | Còn sót scope `class`, `UserRole.source = sis`, `UserRoleScopeType.class` (trái ADR-0072) | Bỏ | P1 | F012, F052, F059 |
| RB-102 | Vị trí catalog permission: `internal/authorization` (ADMIN_PORTAL §2.1, SECURITY_ARCHITECTURE §5.2, D4) vs `pkg/authz` (R20) | `pkg/authz` | P2 | F002 |
| RB-103 | `platform.audit.read` vs `platform.audit.read_all` | `platform.audit.read` | P2 | F015, F059 |
| RB-104 | Key cache Principal `ew:id:` / `ew:identity:` | `ew:<module>:principal:` | P3 | F002 |
| RB-105 | `last_login_at` cùng transaction (IDENTITY_MAPPING §3.3) vs bất đồng bộ (DATABASE_DESIGN §3.12) | Bất đồng bộ | P2 | F013, F014 |
| RB-106 | `INVALID_TOKEN` (IDENTITY_MAPPING) không có trong spec | `UNAUTHENTICATED` | P3 | F002 |
| RB-107 | `AuditLogEntry` (OpenAPI) lệch DATABASE_DESIGN §3.11 (cột, kiểu, `actor_type`) | Theo DATABASE_DESIGN | P2 | F015 |
| RB-108 | Ẩn danh hóa "xóa Keycloak user" (IDENTITY_MAPPING §5, DATA_PRIVACY §4) trái ADR-0070 | Không xóa | P1 | F016 |
| RB-109 | Thứ tự saga dọn học sinh F016 khác ACCOUNT_PROVISIONING §6b; kết quả saga `done/skipped/failed` vs `done/blocked/nothing_to_do` | Theo F016 | P2 | F016 |
| RB-110 | `sis_connections.school_ids` trùng bảng `sis_connection_schools`; enum `SyncRun` lệch DATABASE_DESIGN; `/runs`, `/sis/imports` (SIS_INTEGRATION §10) vs `/sync-runs` | Bảng nối; `/sync-runs` | P2 | F011, F017 |
| RB-111 | Retention: staging SIS 30 vs 90 ngày; login events 90 vs 180 ngày; `sis_lookup_timeout` trùng `sis_login_sync_timeout_ms` | Chưa chốt | P2 | F011, F014, F017 |
| RB-112 | School Admin dùng `/tenant/trash` (ACCOUNT_PROVISIONING) vs chỉ Tenant Admin (ADMIN_PORTAL) | Chỉ Tenant Admin | P2 | F045 |
| RB-113 | `FORBIDDEN_SCOPE` (API_DESIGN) vs `PERMISSION_DENIED` (TC-TEN-002); `CALLER_NOT_ALLOWED` vs `PERMISSION_DENIED` (TC-SVC-006) | `PERMISSION_DENIED`; `CALLER_NOT_ALLOWED` | P2 | F002, F042 |
| RB-114 | AC-CMS-003 `422` vs AC-ORG-024/TC-TEN-011c `400 TENANT_HEADER_REQUIRED` | `400` | P2 | F020 |
| RB-115 | ADMIN_PORTAL §2.1/§8 `me/permissions?tenant_id=` vs header `X-Tenant-Id`; `createAuthSession` trả `PlayerBootstrap` cho portal | Header; portal không dùng `createAuthSession` | P2 | F051 |
| RB-116 | AGENTS R23 chưa liệt kê `/social/*` trong route `game_player` | Thêm | P1 | F095 |

### 3.2. Học tập, nội dung, LMS, đề thi

| Mã | Mâu thuẫn | Feature đã chọn | Ưu tiên | Feature |
|---|---|---|---|---|
| RB-201 | Mô hình Curriculum: OpenAPI "môn × khối có version" vs CURRICULUM_MODEL/DATABASE_DESIGN Curriculum → Version → SubjectGrade; `LearningContentType` thiếu `curriculum` | Theo CURRICULUM_MODEL | P1 | F021 |
| RB-202 | `grade_level` (đơn) vs `grade_levels` (mảng); `topic_id` NULL ở DB nhưng bắt buộc theo CONTENT_MODEL | Mảng; bắt buộc | P1 | F022 |
| RB-203 | `worked_example_ref` uuid vs `{content_item_id, block_id}` | Theo CONTENT_MODEL | P2 | F022, F024 |
| RB-204 | `lo_version_mappings` PK `(from, to)` không biểu diễn `removed` | Thêm PK riêng | P2 | F021 |
| RB-205 | Seed GDPT 2018 để trống `yccd_ref` nhưng publish chặn; đường dẫn seed cũ ở CURRICULUM_MODEL §7 | Seed `GDPT2018_SAMPLE` | P2 | F021 |
| RB-206 | Tên event workflow `content.review_*` (CONTENT_MODEL §6) vs `content.version_*` (EVENT_DESIGN); `exercise.submitted` (ACTIVITY_ENGINE §10, TC-LRN-004) vs `activity.*` | `content.version_*`; `activity.*` | P1 | F025, F030 |
| RB-207 | `evaluations.source = system_reevaluation` vượt `varchar(16)` vs `re_evaluation`; `regrade_requests` cần `tenant_id` NULL + `scope` | `re_evaluation`; thêm cột | P1 | F031, F042 |
| RB-208 | `attempts.context_type` DATABASE_DESIGN §3.4 khác OpenAPI | Theo OpenAPI | P2 | F030 |
| RB-209 | `school_admin` thiếu `learning_content.review` nhưng §3.5 cho duyệt lane tenant; seed thiếu tài khoản reviewer tenant | Thêm quyền + tài khoản | P2 | F025 |
| RB-210 | `license` 64 vs 255 (D17) | 64 tới khi chốt | P3 | F027 |
| RB-211 | Thiếu xác nhận upload xong và nơi lưu lịch hẹn publish | `completeMediaUpload`, `content.scheduled_publications` | P2 | F026, F027 |
| RB-212 | Bảng DATABASE_DESIGN lệch tài liệu domain: `progress_daily(date)`/`class_objective_rollups` vs `bucket_date`/`rm_class_objective_stats`; `assignments.result_visibility`; `exam_sessions.type/duration_seconds` | Theo tài liệu domain | P1 | F041, F042, F043 |
| RB-213 | EVENT_DESIGN thiếu `assignment.closed`, `assignment.submission_updated`, `exam.opened/closed`; API_DESIGN §9.5 thiếu `/analysis`, `/report`, `/library`, `/papers` | Thêm | P2 | F041, F042 |
| RB-214 | `school.settings.curriculum_version_id` không có nơi chứa | `lms.school_curriculum_settings` | P2 | F040 |
| RB-215 | Ân hạn offline thi 30 s vs `deadline_grace` 2 s; EXAM_ENGINE §6 cho lưu đệm local vs cấm ghi đĩa ở client | Chỉ RAM | P1 | F042, F064 |
| RB-216 | `lms.learning_paths` có version (DATABASE_DESIGN §3.13) nhưng SCHOOL_LMS không có workflow phát hành | Chưa chốt | P2 | F040 |
| RB-217 | VIDEO_LEARNING §8 vs DATABASE_DESIGN §2.6a: câu chèn video lưu ở đâu; `learning.*` vs `assessment.*`; `min_score_ratio` vs `min_cue_score_ratio` | `attempt_items/answers` | P2 | F065 |
| RB-218 | Transcript bài nghe bắt buộc (ADMIN_PORTAL) vs tùy chọn (AUDIO); bảng `learning.*` vs `content.*/assessment.*`; tên event simulation lệch `learning-event.schema` | Theo DATABASE_DESIGN | P2 | F066 |
| RB-219 | `learning_content.write` (ADMIN_PORTAL) vs `learning_content.activity.write` | `.activity.write` | P2 | F058 |
| RB-220 | LEARNING_ANALYTICS §8 `/learning/mastery/me` chưa có trong spec | `getMyProgress` | P3 | F031 |
| RB-221 | `assessment.integrity_signals` chưa có feature sở hữu rõ | F030 tạo; F065 dự phòng | P2 | F030, F065 |
| RB-222 | Chưa feature nào gắn dialog chấm lại cho đề thi trên portal | Chưa chốt | P2 | F055, F058 |

### 3.3. Game, thế giới, kinh tế

| Mã | Mâu thuẫn | Feature đã chọn | Ưu tiên | Feature |
|---|---|---|---|---|
| RB-301 | `AvatarConfig` (OpenAPI, UUID, ≤ 8 emote) vs CHARACTER_SYSTEM §5.1 (mã, ≤ 6 emote, slot) | CHARACTER_SYSTEM | P1 | F070 |
| RB-302 | Nickname duy nhất theo trường (DATABASE_DESIGN) vs theo tenant; `maxLength` nickname 32/64/128 | Theo tenant; một độ dài chuẩn | P1 | F070, F095, F109 |
| RB-303 | OpenAPI `QuestType`/`QuestProgressStatus`/`step_type` theo quest schema v1 (7 loại) vs v2 (12 loại); `quest_progress` PK chặn quest lặp lại; `content_ref` thiếu `activity`; thiếu `allow_continue_on_disconnect` | Theo v2; `quest_progress_id` | P1 | F072, F103 |
| RB-304 | DATABASE_DESIGN `display_name`/`title` một ngôn ngữ, trái ADR-0056 | i18n | P2 | F071 |
| RB-305 | Đường bắt đầu mini-game: `startMiniGame` (game) vs `startActivityAttempt` (learning) | Tự chơi/quest qua game; bài giao qua learning | P2 | F073 |
| RB-306 | EVENT_DESIGN §3.2 learning tiêu thụ `minigame.completed` trùng evidence | Bỏ | P2 | F073 |
| RB-307 | `ref.mastery_summary` chưa có trong DATABASE_DESIGN §2.17 | Thêm | P3 | F071 |
| RB-308 | `item_id` uuid (OpenAPI) vs `varchar(64)` (DB, schema); `xp_required` vs `xp_to_next`; config type `levels` trùng `game.level_definitions` | Chưa chốt | P1 | F074, F075 |
| RB-309 | Live-ops thiếu `content.release.rollback` (§2.3 vs §3.11); `game.progression.write` vs `game.level.write`; `game.collection.write`, `game.shop.write` chưa có trong catalog | Bổ sung catalog | P2 | F078, F079 |
| RB-310 | `listGameActivities` ghi `x-service: learning` nhưng dữ liệu ở game | module game + projection learning | P2 | F078 |
| RB-311 | Nhịp đẩy community goal 10 s vs 30 s; `event_runs.suspended_reason varchar(500)` < 1000; DATABASE_DESIGN thiếu `economy_adjustments`, `preview_sessions`, `render_jobs`, `media_assets.kind = render_*` | Chưa chốt | P2 | F077, F078 |
| RB-312 | Rollup GAME_ANALYTICS §6 cột chiều vs DATABASE_DESIGN §2.8b `dims_hash` | `dims_hash` | P2 | F080 |

### 3.4. Realtime, party, thi đấu

| Mã | Mâu thuẫn | Feature đã chọn | Ưu tiên | Feature |
|---|---|---|---|---|
| RB-401 | Close code thu hồi quyền 4003 vs 4403; tắt node `1012` + `system.server.draining` vs `4500` | 4403; `system.server.draining` | P1 | F090, F130 |
| RB-402 | `ws_connections_per_user = 2` trái ADR-0031 (1) | 1 | P1 | F090 |
| RB-403 | `EW_RT_HUB_*` ở gateway; REALTIME §6 module game đọc key Valkey của competition | Hub ở module competition | P2 | F091 |
| RB-404 | Enum presence lệch (MULTIPLAYER §5, AsyncAPI, `PlayerCard`) | Một enum chung | P2 | F091, F092 |
| RB-405 | `PARTY_INVITE_NOT_ALLOWED` vs `PARTY_POLICY_VIOLATION`; `PARTY_ACTIVITY_NOT_ELIGIBLE` 409 vs 422; rate lời mời 50/giờ vs 50/ngày | Mã đầu; 409; chưa chốt rate | P2 | F100 |
| RB-406 | "Tạo channel cùng transaction" (PARTY_SYSTEM §2, CHAT_SYSTEM §4.1) không khả thi giữa hai database | Outbox + consumer | P1 | F100, F115 |
| RB-407 | Party `study_group` gắn `club_id` lỗi thời (ADR-0084); enum loại hoạt động party lệch, thiếu `boss` | Bỏ `club_id`; thêm `boss` | P2 | F100, F101 |
| RB-408 | `party.leader_changed` vs `party.leader.transferred`; mã AC-PTY PRD vs PARTY_SYSTEM §14 | `party.leader.transferred` | P2 | F100 |
| RB-409 | Party push qua `ew:rt:user`; key `ew:pty:*` vs `ew:party:*`; tên scene lệch | `ew:party:*`; theo GODOT_ARCHITECTURE | P2 | F100, F102 |
| RB-410 | CHALLENGE_BOSS §7 leader tạo encounter party trực tiếp; REALTIME §4.1 chưa định tuyến `boss.*` | Qua Party Lobby | P2 | F103 |
| RB-411 | `/lms/challenges` ở learning (API_DESIGN §9.14) vs competition (SCHOOL_LMS §7, COMPETITION §10); COMPETITION §10.2 cho Tenant Admin tạo giải liên trường | module competition; theo ADMIN_PORTAL | P1 | F107, F108 |
| RB-412 | `rating_history` vs `rating_changes`; `definition_id` vs `leaderboard_id`; key Valkey lệch; `tournament.match.scheduled` vs `tournament.match_scheduled` | Chưa chốt | P2 | F104–F107 |
| RB-413 | Rating/`rating_delta` hiển thị cho học sinh (GAME_NETWORKING §7) trái COMPETITION §8.2; FR-RNK-003 chỉ ẩn lớp 1–5 | Ẩn mọi khối | P1 | F106, F109 |
| RB-414 | Hết grace 1v1: "câu còn lại 0" vs "thua do abandon"; thời điểm tạo question plan | Theo MULTIPLAYER §9.2; chưa chốt thời điểm | P2 | F105 |
| RB-415 | `competition.config.reward.write` không có `config_type`; vị trí leaderboard ±5 vs ±3 | Chưa chốt | P3 | F106, F108 |
| RB-416 | Season track (F077, M3) cần lịch mùa F106 (M4b); `pkg/roomhost` (F076) chồng lấn F091 | Fixture tới M4b | P2 | F076, F077 |

### 3.5. Chat, kiểm duyệt, an toàn

| Mã | Mâu thuẫn | Feature đã chọn | Ưu tiên | Feature |
|---|---|---|---|---|
| RB-501 | CHAT_SYSTEM §4 (channel `class`) còn `org.teaching_assignments` (trái ADR-0072) | Bỏ | P1 | F120 |
| RB-502 | Bật/tắt nhóm từ cấp tenant: Tenant Admin (FR-MOD-013, AC-MOD-022, MODERATION §11.3) vs Safety Team (ADR-0072) | Safety Team | P1 | F116, F121, F122 |
| RB-503 | `chat.sensitive_terms.write` (API_DESIGN) vs `chat.tenant_terms.write` | `chat.tenant_terms.write` | P2 | F116 |
| RB-504 | `/chat/moderation/*` (MODERATION §5) vs `/admin/moderation/*`; enum trạng thái case, sanction, reason lệch; snapshot ±10 vs ±20 tin; `sanction_type` AsyncAPI khác | `/admin/moderation/*`; theo MODERATION | P1 | F121 |
| RB-505 | Mã AC-CHAT PRD (001–014) vs CHAT_SYSTEM §13 (001–043) trùng tiền tố khác nghĩa | Theo PRD | P2 | F115–F118 |
| RB-506 | Rate limit chat lệch REALTIME §8, CHAT_SYSTEM §3.1, API_DESIGN; cache policy 60 s vs 5 phút | Chưa chốt | P2 | F115 |
| RB-507 | `match.ended` vs `match.completed`; `match.started` thiếu `participants[]`, `mode` | `match.completed` | P1 | F105, F117 |
| RB-508 | Mention không hợp lệ: lỗi (API_DESIGN) vs loại khỏi tin (CHAT_SYSTEM, TC-CHAT-035); `client_message_id` vs `client_msg_id` | Loại khỏi tin; một tên | P2 | F118 |
| RB-509 | TC-CHAT-018 ghi chat-policy v4 (hiện v5); CHILD_SAFETY §3.1 thiếu sticker/GIF; STUDENT_GAME_UX_SOCIAL §7 ghim "Nhà trường" trái ADR-0072 | Sửa | P3 | F118, F119 |
| RB-510 | Audit `moderation.case.viewed` vs `moderation_case.viewed`; metric bypass `eduworld_tenancy_bypass_total` vs `eduworld_tenant_filter_bypass_total` | Một tên | P2 | F121 |
| RB-511 | `student_club_creation`: `disabled/enabled` (F015) vs `disabled/requires_approval/auto` (SOCIAL_SYSTEM) | Theo SOCIAL_SYSTEM | P2 | F015, F120 |
| RB-512 | Giới hạn lời mời kết bạn 10/giờ, 30/ngày (API_DESIGN §8) vs `friend_limits` 20/ngày; `friendships/blocks/mutes` PK cặp chặn tạo lại; thiếu `social.notifications`, `ref.blocks` | UUID + unique partial | P2 | F095, F123 |
| RB-513 | Enum loại thông báo: `kind` (REST) vs `category` (WS) | Một enum | P2 | F123 |

### 3.6. Client, portal, vận hành

| Mã | Mâu thuẫn | Feature đã chọn | Ưu tiên | Feature |
|---|---|---|---|---|
| RB-601 | Ba đường dẫn cấu hình Godot (GODOT_ARCHITECTURE §4, KEYCLOAK_SSO §11, LOCAL_DEVELOPMENT §4) | `res://config/env/<env>.json` | P2 | F060 |
| RB-602 | `MathAssetRef` không có `latex` (GODOT §6.3 dùng làm fallback); nhắc nghỉ qua notification (GODOT §10) chưa có kênh ở M-C | `speech_text`; `PlayerBootstrap.policies` | P3 | F063, F064 |
| RB-603 | Tên thư mục workspace D: `platform-admin` (F051) vs `platform` (F050, F057) | `platform-admin` | P3 | F050, F057 |
| RB-604 | Cổng admin `:9090` (DEPLOYMENT §14) vs 91xx/92xx; `EW_HEALTH_ADDR`/`EW_METRICS_ADDR` vs `EW_ADMIN_ADDR` | `EW_ADMIN_ADDR=:9090` trong container | P2 | F001, F130 |
| RB-605 | Metric: `eduworld_svcclient_*` / `eduworld_internal_client_*`; `eduworld_ratelimit_rejections_total` vs `rejected_total`; DLQ `CHAT_DLQ` vs `DLQ_<SERVICE>` | Theo API_DESIGN, DEPLOYMENT | P3 | F003, F131 |
| RB-606 | `CONCURRENT_MODIFICATION` không có trong `ProblemCode` | `409 CONFLICT` | P3 | F003 |
| RB-607 | DATABASE_DESIGN §1.2 thiếu schema `catalog`, `analytics`; `infra.dead_letters` thiếu kiểu cột | Bổ sung | P2 | F003 |
| RB-608 | `login_storm` (3.000/10 phút, 50/s) vs `login_capacity_target` (5.000, 100/s); ROADMAP §3.7 còn 3.000 | `login_capacity_target` | P2 | F132 |
| RB-609 | ROADMAP M6 lỗi thời: "PKCE 4 nền tảng" (trái ADR-0068), pentest QR/PIN (ngoài Release), "preset thấp Web" (ADR-0057) | Sửa ROADMAP | P2 | F132, F133 |
| RB-610 | `content_manifest_url` (DEPLOYMENT §9) vs `world_manifest_url` (`ClientConfig`); NFR-AVAIL-005 TC-SVC-010…013 vs ROADMAP …014 | Theo spec | P3 | F014, F132 |
| RB-611 | Thư mục tài liệu vận hành mới đề xuất: `docs/11-operations/{runbooks,alert-drills,restore-tests}/`, `docs/12-project/chaos-reports/`, `docs/10-security/pentest/` | Tạo khi triển khai | P3 | F130–F132 |
| RB-613 | Phím tắt: `J` vừa là Thành tựu (STUDENT_GAME_UX_SOCIAL §4) vừa là Nhật ký nhiệm vụ (STUDENT_GAME_UX §4.1); `B` (Shop) chưa có trong §4.1 | Theo STUDENT_GAME_UX §4.1 | P3 | F069 |
| RB-614 | Wireframe S06 hiện số câu đúng từng bạn, trái CHALLENGE_BOSS §3.5 và AsyncAPI (`contribution_met`) | Theo contract | P2 | F110 |
| RB-615 | Lớp 1–3 không hiện độ hiếm bằng chữ (SOCIAL §3) vs quy tắc trợ năng "màu + chữ" | Màu + biểu tượng | P3 | F069 |
| RB-616 | `scripts/social/social_service.gd`: F119 (M4b) tạo, nhưng F110 (M4a) chạy trước | F110 tạo, F119 dùng lại (sửa plan F119) | P2 | F110, F119 |
| RB-617 | GAME_NETWORKING §4.2 chuyển `quest.*` → `NotificationService` vs GODOT_ARCHITECTURE §6 `QuestService`; GODOT §3 thiếu thư mục `scenes/{progression,inventory,achievement,challenge}`, `scripts/{social,challenge,progression}` | `QuestService`; bổ sung thư mục | P3 | F069, F110 |
| RB-618 | Academic Manager xem báo cáo game: GAME_ANALYTICS §8 (không) vs ADMIN_PORTAL §2.2/§2.3, SECURITY_ARCHITECTURE (`analytics.school.read` gồm báo cáo game) | Theo permission; scope `subject` → 403 | P2 | F080 (chuyển từ F081 khi tách F081, 2026-11-17) |
| RB-619 | Tên thư mục workspace web: `school`/`content`/`platform` (F050) vs `school-lms`/`content-studio`/`platform-admin` (F051, F052) vs `workspaces/school/` (F056); component ẩn ô < 5 trùng: `SuppressedValue` (F056) và `SuppressedCell` (F080) | Theo F051/F052; gộp một component | P2 | F050, F056, F080 |
| RB-620 | ADMIN_PORTAL §3.15 yêu cầu nhân sự xem tiến độ community goal theo lớp nhưng `getCommunityGoal` chỉ cho học sinh | `getSchoolEventProgress` (F081) | P3 | F081 |
| RB-621 | F016 chỉ có một `status_reason`; F124 khóa theo nhiều nguồn (`identity.account_locks`) để trường mở khóa không gỡ khóa của Safety Team | Bảng khóa nhiều nguồn; đồng bộ F016 | P1 | F016, F124 |
| RB-622 | EVENT_DESIGN chưa ghi module identity tiêu thụ `sanction.applied/lifted`, `moderation.school_notified`; chưa có hạ tầng gửi email (SMTP) trong tài liệu | Thêm consumer; biến `EW_SMTP_*` | P2 | F124 |
| RB-623 | UI lịch trực Safety Team và break-glass chưa thuộc feature nào | Bổ sung vào F122 | P2 | F122, F124 |
| RB-624 | Cơ chế loại tenant QA khỏi analytics chưa có trong GAME_ANALYTICS, DASHBOARD_DATA, TEST_STRATEGY | Cột `is_qa` trên tenant | P2 | F131, F134 |
| RB-625 | "Tham gia bạn": `joinFriendInWorld` (STUDENT_GAME_UX_SOCIAL §11, F071) vs `hub.instance.join {follow_user_id}` (F091) | Theo F091; bỏ hoặc định nghĩa lại `joinFriendInWorld` | P2 | F071, F091, F111 |
| RB-626 | Tắt tạo club: `403 PERMISSION_DENIED` (F120-AC-17) vs `SOCIAL_SCOPE_VIOLATION` (AC-SOC-006); UX ẩn tab Club cho `g1_3` vs ADR-0020 quyền xã hội như nhau | Một mã lỗi; chỉ khác UX | P2 | F111, F120 |
| RB-612 | Tài khoản demo còn thiếu: reviewer tenant, `game_designer`, `liveops`, platform thứ hai có `game.economy.adjust`, `competition_admin`; seed thiếu học sinh lớp 6–9 | Bổ sung LOCAL_DEVELOPMENT | P2 | F025, F078, F108, F109 |

### 3.7. Quyết định cho các mục "Chưa chốt" [Confirmed — 2026-11-16]

| Mã | Quyết định |
|---|---|
| RB-111 | Staging SIS giữ 30 ngày (SIS_INTEGRATION); login events giữ 90 ngày (tối thiểu hóa dữ liệu); dùng `sis_login_sync_timeout_ms`, bỏ `sis_lookup_timeout` |
| RB-110 | Enum `SyncRun.status` theo DATABASE_DESIGN (`queued`, `running`, `succeeded`, `partially_failed`, `failed`); `source` có `csv_import` |
| RB-216 | Learning path **không** có version: sửa có audit, áp dụng tiếp cho học sinh, tiến độ giữ theo node id; bỏ khỏi DATABASE_DESIGN §3.13 |
| RB-222 | Dialog chấm lại đề thi thuộc F055 (`createRegradeRequest`, `scope = exam_session`) |
| RB-302 | Nickname `maxLength` 32 ở mọi nơi |
| RB-308 | `item_id` là mã `varchar(64)` (OpenAPI đổi theo); trường đường cong XP là `xp_required` (PROGRESSION_SYSTEM); config type `levels` đổi thành `progression_levels` |
| RB-311 | Community goal đẩy mỗi 30 s (CONFIG_DEFAULTS); `suspended_reason varchar(1000)`; thêm `economy_adjustments`, `preview_sessions`, `render_jobs`, `media_assets.kind = render_thumbnail/render_turntable` vào DATABASE_DESIGN |
| RB-404 | Enum presence theo MULTIPLAYER_GAMEPLAY §5 |
| RB-405 | Rate lời mời party 50/ngày (sửa CONFIG_DEFAULTS) |
| RB-412 | `rating_changes`, `leaderboard_id`; key Valkey theo VALKEY_DESIGN (thêm scope vào key queue); event `tournament.match_scheduled` |
| RB-414 | Question plan tạo khi tạo trận (REALTIME_ARCHITECTURE §7.1) |
| RB-415 | Thêm `config_type = competition-rewards` (kebab theo `GameConfigType`); vị trí leaderboard ±3 |
| RB-506 | Rate limit chat theo CHAT_SYSTEM §3.1 (tài liệu sở hữu); cache policy 5 phút |
| RB-508 | Một tên `client_message_id` |
| RB-513 | Enum `kind` dùng chung REST và WS |
| RB-605 | Metric theo API_DESIGN §12.3 (gọi facade `eduworld_internal_client_*` nhãn `caller_module`/`callee_module`/`operation`, tích hợp ngoài `eduworld_external_client_*` theo [MONITORING](../11-operations/MONITORING.md), ADR-0074); `eduworld_ratelimit_rejected_total`; DLQ `DLQ_<MODULE>` |
| RB-610 | `world_manifest_url` theo `ClientConfig`; chaos test TC-SVC-010…014 |

### 3.8. Delta cho feature theo ADR-0078/0086 bổ sung (2026-11-17)

Quyết định Technical Architect 2026-11-17 đã áp vào tài liệu sở hữu (cột "Tài liệu sở hữu"); **feature spec và contract trong `specs/` chưa sửa**. Mỗi dòng: cập nhật `spec.md` (EARS `F<NNN>-AC-*`), `contracts/README.md` §6, `tasks.md`, rồi contract (`specs/openapi`, `specs/schemas/chat-policy.schema.json`, problem catalog) trước khi feature sang `In progress`. Tham số đã đăng ký ở [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md).

| Mã | Delta | Tài liệu sở hữu | Ưu tiên | Feature |
|---|---|---|---|---|
| RB-801 | **PII và grooming luôn chạy**: bỏ `pii` khỏi `moderation_switches.steps` (schema + API công tắc; cấu hình cũ `steps.pii = false` bị bỏ qua và migrate về bật); luật grooming của `async_analysis` chạy cả khi bước tắt; mã lỗi `MODERATION_SWITCH_STEP_NOT_ALLOWED`; tắt bước khác bắt buộc `expires_at` ≤ 24 giờ (`MODERATION_SWITCH_EXPIRY_REQUIRED`) và phê duyệt của System Admin thứ hai (`pending_approval`, `:approve`, `MODERATION_SWITCH_SELF_APPROVAL`); AC-MOD-030, AC-MOD-031; TC-CHAT-044, TC-CHAT-045 | MODERATION §3, §12; CHILD_SAFETY §3.3 | P1 | F116 (UI công tắc + phê duyệt: F122) |
| RB-802 | **Giờ giới nghiêm platform** `platform_curfew` (21:30–06:30) cho chat: module chat từ chối gửi ở mọi channel trong giờ giới nghiêm; `allowed_hours` cấp trường không nới quá giờ ngoài giới nghiêm (`CHAT_POLICY_RELAX_NOT_ALLOWED`); AC-CHS-044; TC-CHAT-046 | CHILD_SAFETY §4.1; CHAT_SYSTEM §3 | P1 | F115, F120 |
| RB-803 | `Đã xử lý` (2026-11-17, §5) — **Giờ giới nghiêm cho game**: module identity từ chối `/auth/game/login` và `token:refresh` `mode=resume` (giữ token phía client), module game từ chối `POST /game/sessions` và kết thúc phiên đúng giờ (cảnh báo trước `curfew_warning_minutes`), realtime không cấp ticket; một mã lỗi giới nghiêm thống nhất trong problem catalog; cài đặt `platform_curfew` (platform) + khung giới nghiêm trường (chỉ siết thêm); AC-AUTH-032/162; TC-AUTH-092 | CHILD_SAFETY §4.1; KEYCLOAK_SSO §5.1 | P1 | F013, F014, F015, F090; UI cài đặt: F052, F057 |
| RB-804 | **Preset "chỉ quick message + emote"** `chat_policy_preset = quick_message_emote_only` cấp trường: `allowed_kinds` mọi channel còn `quick` + `emoji`, ngoại lệ duy nhất được thu hẹp `allowed_kinds` của `match`/`team`/`party` (channel vẫn always-on) — sửa chat-policy schema v5 (hiện từ chối thiếu loại tin ở channel always-on); pilot áp mặc định; AC-CHS-045; TC-CHAT-048 | CHILD_SAFETY §3.5; CHAT_SYSTEM §3 | P1 | F115, F120; UI chọn preset: F122 (workspace C) |
| RB-805 | **Tenant `common`**: `common_tenant_social_defaults` — tắt kết bạn/mã bạn bè (`SOCIAL_SCOPE_VIOLATION`), DM và World chat; `area` chỉ quick + emoji; System Admin bật lại có audit; AC-SOC-011, AC-CHS-046; TC-CHAT-047 | CHILD_SAFETY §3.6; MULTITENANCY §8.3; SOCIAL_SYSTEM §4.1, §7 | P1 | F095, F120 |
| RB-806 | **Windows mặc định máy dùng chung** (`shared_device_default`): bộ cài Windows có lựa chọn "Máy dùng chung" (mặc định) / "Máy cá nhân" ghi `device.cfg`; thiếu `device.cfg` trên Windows = `true`; `SecureStorageFactory` chọn `MemoryOnly`; AC-AUTH-030/160, AC-UX-0114; TC-AUTH-090 | GODOT_ARCHITECTURE §5.1, §11; KEYCLOAK_SSO §9 | P1 | F060 (`Config`/`device.cfg`), F061, F133 (bộ cài), F134 (trang tải bộ cài) |
| RB-807 | **Màn "Tiếp tục là <nickname>?"** (`resume_confirm_screen`, mọi nền tảng): trạng thái `ResumeConfirm` giữa `token:refresh` `mode=resume` và `POST /game/sessions`; "Không phải tôi" → `gameLogout` + xóa token; khóa dịch `ui.login.resume_confirm.*`; AC-AUTH-031/161, AC-UX-0113; TC-AUTH-091 | STUDENT_GAME_UX_CORE S01; GODOT_ARCHITECTURE §5; GAME_NETWORKING §3 | P1 | F061 (+ F133 kiểm thử 4 nền tảng) |
| RB-808 | **Bỏ QR/PIN** (ADR-0076 Rejected): gỡ mọi tham chiếu `login_credentials`, `svc-keycloak-authn`, `login-credentials:verify`, `quick_login` khỏi spec/feature (nếu có); TC-AUTH-041/042 không chạy | KEYCLOAK_SSO §4, §8.1; SECURITY_ARCHITECTURE §3.2 | P2 | F013, F004 |
| RB-809 | **Object storage staging/production theo ADR-0075** (thay ADR-0071): stack/secret không còn `floci` ngoài local/CI; spike chọn backend; replication bucket riêng tư; alert `ObjectStorageReplicationLag`, `ObjectStorageNodeDown` | DEPLOYMENT §9b; BACKUP_RECOVERY §6, RB-05; MONITORING | P1 | F130, F131 |
| RB-810 | **CI GitHub Actions** thay GitLab CI (ADR-0088): workflow theo path filter (`go.yml` ma trận module, `specs.yml`, `web.yml`, `godot.yml`, `docs.yml`, `release.yml`…), environment `staging`/`production`, self-hosted runner cho deploy | DEPLOYMENT §10; CONTRIBUTING | P1 | F001, F060, F133 |
| RB-811 | **Pilot và cổng hiệu quả** (ADR-0073, ADR-0080): viết spec F035; ánh xạ feature của lát cắt pilot (quest 4 loại, boss solo, 4–5 mini-game, co-op `interdependent`, chat quick + emote) sang mốc M-P1 trong CODE_PLAN §5; F133 phát hành 4 nền tảng từ pilot | ROADMAP §2–§3.5; LEARNING_EFFICACY_PLAN | P1 | F035, CODE_PLAN §4–§5, F133 |

### 3.9. Delta theo ADR-0074 (modular monolith) và ADR-0088 (2026-11-17)

**Trạng thái:** tài liệu vận hành, dữ liệu, định danh, bảo mật và dự án đã viết lại theo [ADR-0074](../adr/ADR-0074-modular-monolith-co-ranh-gioi-cuong-che.md) (DEPLOYMENT, LOCAL_DEVELOPMENT, MONITORING, BACKUP_RECOVERY, DATABASE_DESIGN, MULTITENANCY, REALTIME_ARCHITECTURE, KEYCLOAK_SSO, 09-identity, 10-security, ROADMAP, RISKS, TEST_STRATEGY, GLOSSARY, CONFIG_DEFAULTS). Phép đổi tên cơ học (`services/<x>-service/` → `modules/<x>/`, `pkg/svcclient` → `pkg/modcall`, `SVC=` → `MOD=`) đã chạy trên `specs/`, nhưng **ngữ nghĩa trong feature spec và contract chưa sửa**. Mỗi dòng dưới đây cần xử lý trước khi feature liên quan sang `In progress`.

| Mã | Delta | Tài liệu sở hữu | Ưu tiên | Feature |
|---|---|---|---|---|
| RB-901 | Feature spec/plan/tasks còn mô tả token `svc-*`, gọi HTTP `/internal/v1/*`, retry/circuit breaker giữa module, contract test HTTP (Pact) → đổi sang facade sinh từ spec internal gọi trong tiến trình (`pkg/modcall`), `x-allowed-callers` là tên module, facade test + `make arch-check`; TC-SVC-004…007, 009…011, 013, 015 đã viết lại, thêm TC-SVC-016/017 | SECURITY_ARCHITECTURE §5.5; TEST_STRATEGY §6.7; BACKEND_DESIGN | P1 | F001, F002, F003, F004 và mọi feature có `contracts/` internal |
| RB-902 | Feature vận hành còn giả định image/pipeline/stack theo service, `go.work`, tag `<service>/vX.Y.Z` → 3 image `core-api`/`realtime`/`worker` + `migrate`, một `go.mod`, `app.stack.yml`, tag `backend/vX.Y.Z`, workflow `go.yml`/`release.yml` | DEPLOYMENT §3, §10–§12; LOCAL_DEVELOPMENT §3, §7 | P1 | F001, F130, F131, F132 |
| RB-903 | Biến môi trường `EW_SVC_<NAME>_URL`, `EW_SVC_CLIENT_*`, `EW_INTERNAL_HTTP_ADDR`, `EW_DB_DSN` một database → bỏ; dùng `EW_DB_<MODULE>_*`, `EW_KC_ADMIN_CLIENT_*`, `EW_EXT_CLIENT_*`, `EW_WORKER_PROFILE` | LOCAL_DEVELOPMENT §4; CONFIG_DEFAULTS | P2 | F001, F002, F003 |
| RB-904 | Room host (hub instance, battle/co-op state machine) thuộc module competition, chạy trong `core-api` như handler NATS `ew.cmd.competition.*`/`ew.in.room.*` với state và lease theo room trong Valkey, chấm điểm qua facade module learning trong tiến trình; binary `realtime` không nạp module domain — feature thi đấu/party (vd. `pkg/roomhost`, RB-416) cần cập nhật plan nếu còn giả định room host là process riêng hoặc gọi HTTP sang learning | REALTIME_ARCHITECTURE §1, §5.1, §7; DEPLOYMENT §3.1 | P2 | F076, F091 và feature party/battle/co-op (M4a, M4b) |

## 4. Tham số chưa đăng ký CONFIG_DEFAULTS

Đăng ký cùng tài liệu sở hữu (R33) trước khi feature tương ứng sang `In progress`.

| Mã | Tham số đề xuất | Tài liệu sở hữu | Feature |
|---|---|---|---|
| RB-701 | `content_review_sla_days`, `content_snapshot_cache_ttl`, `media_max_bytes_by_kind`, `media_presign_ttl`, `media_signed_url_ttl`, `preview_token_ttl`, chu kỳ autosave, giới hạn duyệt hàng loạt | CONTENT_MODEL, ADMIN_PORTAL | F025–F027, F053, F054 |
| RB-702 | `param_sampling_max_tries` | QUESTION_BANK | F023 |
| RB-703 | Ngưỡng item analysis (≥ 30 lượt), ân hạn đồng bộ offline của thi | EXAM_ENGINE | F042, F044, F064 |
| RB-704 | Cảnh báo < 60 ngày cho cấu hình dọn học sinh | ACCOUNT_PROVISIONING | F016, F057 |
| RB-705 | `min_client_version`, `game_session_heartbeat_timeout`, `audit_log_retention`, retention học sinh và login events | GAME_NETWORKING, DATA_PRIVACY | F013–F015 |
| RB-706 | `nickname_change_monthly_limit`, `nickname_suggestion_ttl` | CHARACTER_SYSTEM | F070 |
| RB-707 | `party_invite_repeat_rules`, `party_pending_invites_max`, `party_create_rate`, `party_kick_rate`, `party_activity_min_members_timeout`, `party_lobby_cmd_rate` | PARTY_SYSTEM | F100, F101 |
| RB-708 | `ws_frame_rate`, `ws_inflight_commands`, `ws_send_queue`, `ws_accept_rate`, `audit_hub_membership` | REALTIME_ARCHITECTURE | F090, F091 |
| RB-709 | Ngưỡng spam, chu kỳ reload danh mục, timeout PubAck/RE2, trần sticker/GIF, throttle typing, `persistent_chat_retention_days` | CHAT_SYSTEM | F115–F120 |
| RB-710 | Ngưỡng vi phạm, backoff, rate limit, circuit breaker của SIS `contract_v1` | SIS_INTEGRATION | F017 |

## 5. Đã xử lý

| Mục | Xử lý |
|---|---|
| Đối soát tài liệu theo §2–§4 | Sửa tài liệu sở hữu (learning, portal, backend, identity, security, game, social, client, vận hành), đăng ký khoảng 60 tham số CONFIG_DEFAULTS (2026-11-17) |
| Quyết định bổ sung khi đối soát | RB-626: tắt tạo club trả `403 PERMISSION_DENIED`, `SOCIAL_SCOPE_VIOLATION` chỉ cho vi phạm phạm vi; D17: `license varchar(255)`; config type dạng kebab (`progression-levels`, `competition-rewards`); mã AC cục bộ CHAT_SYSTEM đổi sang `AC-CHS-NNN`; tên thư mục workspace web `game-ops`/`content-studio`/`school-lms`/`platform-admin`/`live-ops` |
| D15 — đăng nhập game có tạo game session? | [ADR-0083](../adr/ADR-0083-dang-nhap-game-va-tao-game-session-tach-biet.md) |
| Study group thuộc module nào | [ADR-0084](../adr/ADR-0084-nhom-hoc-tap-thuoc-learning-service.md) |
| Dọn học sinh: disable Keycloak hay khôi phục khi đăng nhập lại | [ADR-0085](../adr/ADR-0085-don-hoc-sinh-giu-user-keycloak.md) |
| D16 — dấu vết OIDC cũ của game; S01 "nhớ lựa chọn trường" | Đã sửa GODOT_ARCHITECTURE, KEYCLOAK_SSO, GAME_NETWORKING, STUDENT_GAME_UX_CORE |
| SIS `contract_v1` nằm trong F011 (M-C1) trái OQ-01 | Tách sang F017 (M2) |
| Khoảng trống danh mục | Thêm F058, F059, F069, F081, F082, F110, F124, F134; hợp nhất tài khoản → F045; thử thách lớp/giải trường → F108 |
| Phụ thuộc thiếu trong danh mục | Thêm F025→F027, F026→F014, F027→F022; F091 thuộc module competition |
| RB-803 — giờ giới nghiêm cho game (ADR-0078) | Backend: F013-AC-32, F014-AC-25/26, F015-AC-21/22, F090-AC-31 (`403 CURFEW_ACTIVE`). Contract: `getSchoolSettings`/`updateSchoolSettings` (`SchoolSettings.curfew_window`, chỉ mở rộng → `422 VALIDATION_FAILED`), `getPlatformCurfewSettings`/`putPlatformCurfewSettings` trong `specs/openapi/eduworld-api.v1.yaml`; action `action.school_lms.settings.edit_school` và `action.platform_admin.system_settings.view`/`.edit` trong `specs/portal/menu-catalog.yaml`. UI: F052-AC-29/30 (khung trường), F057-AC-23 (`platform_curfew`); không có khung cấp tenant (CHILD_SAFETY §4.1) (2026-11-17) |
