# Tasks: F120 — Channel bền vững (world, area, team, DM), club, chính sách theo cấp, tìm kiếm, retention

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Đường dẫn gốc `src/backend/modules/chat/` trừ khi ghi rõ. Fixture chỉ dùng chuỗi placeholder, không từ nhạy cảm thật (R15).

## Phase 1: Setup

- [x] T001 Đăng ký permission dùng trong module (`tenant.settings.read/write`, `school.settings.read/write`, `platform.settings.write`, `social.club.approve`) vào kiểm tra catalog lúc khởi động — file: `src/backend/modules/chat/internal/bootstrap/authz.go`
- [x] T002 [P] Khai báo durable consumer `chat-membership` (`user.blocked`/`user.unblocked`, `school.updated`), `chat-club-goals` (stream `EVT_IDENTITY`, `EVT_LEARNING`, `EVT_GAME`, `EVT_CHAT`; không đăng ký `class.*`/`enrollment.changed`/`study_group.*` — ADR-0095) và lịch job `persistent-chat-retention` trong cấu hình worker — file: `src/backend/modules/chat/config/worker.yaml`
- [x] T003 [P] Fixture event mẫu (`user.blocked`, `user.unblocked`, `school.updated`, `activity.graded`) — file: `src/backend/modules/chat/tests/fixtures/events/`

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: `getChatPolicy`, `putChatPolicy`, `getEffectiveChatPolicy`, `getPlatformChatPolicy`, `putPlatformChatPolicy`, schema `ChatPolicyDocument`/`ChatPolicyPut`/`EffectiveChatPolicy` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`
- [x] T005 [P] Delta contract: `listClubs` thêm `status`, `school_id`; xác nhận delta ADR-0095 của WP-C1 đã áp (`ChannelType` 6 loại, `ClubType` bỏ `study_group`, `ClubCreate` bỏ `class_id`/`add_whole_class`) — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [x] T006 [P] AsyncAPI `chat.channel.membership_changed` — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [x] T007 [P] Payload schema `chat_policy.updated`, `club.created/approved/archived/member_joined/member_left/goal_completed`; thêm `chat_policy.updated` vào EVENT_DESIGN §4.9 — file: `specs/events/payloads/chat/`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint`
- [x] T008 Xác nhận payload schema owner khác đã có (`school.updated` F015, `user.blocked`/`user.unblocked` F095); thiếu → chặn US tương ứng, không tự tạo — file: `specs/events/payloads/` (chỉ đọc)
- [x] T009 ent schema: thêm cột `channels.status/capabilities`, `channel_members.joined_at/left_at/source_event_id`, `messages.anonymized_at`, `chat_policies.reason/updated_by`, `club_goal_progress.last_event_id`, `ref.schools.student_club_creation`; bảng `persistent_retention_runs`; CHECK `club_type` bỏ `study_group`, `chat_policies.scope_type` = `platform/tenant/school`, `channels.status` = `open/read_only/frozen` (ADR-0095); unique channel theo phạm vi — file: `src/backend/modules/chat/ent/schema/*.go` — kiểm: `make migrate-diff MOD=chat`, `make migrate-lint MOD=chat`
- [x] T010 [P] Domain policy: `Rule`, `Intersect`, `ValidateLowerLevel` (siết chặt), `ValidateAlwaysOn`, chặn `grade_*` (+ unit test bảng) — file: `internal/domain/policy/`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Thành viên channel bền vững đúng và cô lập (Priority: P1) 🎯 MVP

**Goal**: Thành viên channel club/`direct` đúng (`joined_at`/`left_at`), cô lập tenant/trường; không có channel `class`/`study_group`/khối (ADR-0095).

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [x] T011 [P] [US1] Test kiểm âm F120-AC-01, F120-AC-02, F120-AC-03 (Removed — ADR-0095): `channel_type` `class`/`study_group` → `422`; không durable consumer nào đăng ký `class.*`, `enrollment.changed`, `study_group.*` (test tĩnh cấu hình worker) — file: `tests/unit/no_class_channel_test.go` — phải FAIL trước khi code
- [x] T012 [P] [US1] Test F120-AC-04: rời/bị gỡ khỏi club → `left_at`, lịch sử `404` — file: `tests/integration/club_channel_membership_test.go`
- [x] T013 [P] [US1] Test F120-AC-04: vào lại club tạo hiệu lực mới, chỉ thấy tin từ `joined_at` mới — file: `tests/integration/club_channel_rejoin_test.go`
- [x] T014 [P] [US1] Test F120-AC-04: `user.blocked` giao 2 lần xử lý 1 lần (TC-SVC-003); lệnh club trùng `Idempotency-Key` chỉ một dòng thành viên; staff không thành member; xem từ `joined_at` — file: `tests/integration/membership_idempotency_test.go`
- [x] T015 [P] [US1] Test F120-AC-05 tenant isolation: người ngoài/nhân sự/tenant khác → `404`; repository bỏ tenant → 0 dòng (RLS, TC-CHAT-009, TC-TEN-005) — file: `tests/integration/channel_isolation_test.go`
- [x] T016 [P] [US1] Contract test consumer với schema `user.blocked`, `user.unblocked`, `school.updated` — file: `tests/contract/relation_events_test.go`

### Implementation cho User Story 1

- [x] T017 [P] [US1] Domain `channel.Channel`, `Membership` (trạng thái, chỉ học sinh) — file: `internal/domain/channel/channel.go`, `internal/domain/channel/membership.go`
- [x] T018 [US1] Command `JoinChannel`/`LeaveChannel` dùng chung cho club và `direct` (outbox `chat.channel.membership_changed` qua realtime) — file: `internal/application/command/channel/membership.go`
- [x] T019 [US1] **Removed (ADR-0095)** — không còn consumer `class_synced.go`, `enrollment_changed.go`, `study_group.go`; consumer quan hệ chặn làm ở T034
- [x] T020 [US1] Lọc `joined_at`/`left_at` trong `listChannelMessages`, `listMyChannels` — file: `internal/application/query/channel/list_messages.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Chính sách chat theo cấp (Priority: P1)

### Tests cho User Story 2

- [x] T021 [P] [US2] Test F120-AC-06: `putChatPolicy` mỗi cấp với quyền đúng, `If-Match`, audit before/after, outbox `chat_policy.updated` — file: `tests/integration/chat_policy_put_test.go` — phải FAIL trước khi code
- [x] T022 [P] [US2] Test F120-AC-07, F120-AC-08, F120-AC-09: nới lỏng → `CHAT_POLICY_RELAX_NOT_ALLOWED`; `grade_band` → `VALIDATION_FAILED` (TC-CHAT-010); always-on → `CHAT_POLICY_ALWAYS_ON` (TC-CHAT-018) — file: `internal/domain/policy/validate_test.go`
- [x] T023 [P] [US2] Test F120-AC-10: effective policy giao 3 cấp (platform/tenant/school) + sanction + quan hệ; thiếu rule = deny; invalidate cache khi event — file: `internal/domain/policy/effective_test.go`, `tests/integration/policy_cache_test.go`
- [x] T024 [P] [US2] Test F120-AC-11 authorization + tenant isolation: tenant/trường khác → `404`, cấp platform → `403`, gateway public không có route platform (TC-TEN-003) — file: `tests/integration/chat_policy_authz_test.go`
- [x] T025 [P] [US2] Test giới hạn độ dài `reason` (`maxLength` 500) và kích thước `rules` (TC-TEN-024) — file: `tests/integration/chat_policy_limits_test.go`
- [x] T055 [P] [US2] Test F120-AC-26: School Admin đặt preset có audit + `chat_policy.updated`, hiệu lực ≤ 60 s; preset theo khối → 422; trường khác không đổi — file: `src/backend/modules/chat/tests/integration/school_preset_test.go` — phải FAIL trước khi code
- [x] T056 [P] [US2] Test F120-AC-27: `allowed_hours` 06:00–22:00 ở cấp trường khi `platform_curfew` 21:30–06:30 → `422 CHAT_POLICY_RELAX_NOT_ALLOWED`; 07:00–21:00 hợp lệ — file: `src/backend/modules/chat/tests/integration/allowed_hours_curfew_test.go`

### Implementation cho User Story 2

- [x] T026 [US2] Command `PutChatPolicy`, query `GetChatPolicy`, `GetEffectivePolicy` — file: `internal/application/command/policy/put_chat_policy.go`, `internal/application/query/policy/`
- [x] T027 [US2] `PolicyCache` Valkey (TTL `chat_policy_cache`, invalidate theo `chat_policy.updated`) — file: `internal/infrastructure/cache/policy_cache.go`
- [x] T028 [US2] Handler `getChatPolicy`, `putChatPolicy`, `getEffectiveChatPolicy`, `getPlatformChatPolicy`, `putPlatformChatPolicy` — file: `internal/presentation/http/public/chat_policy.go`
- [x] T057 [US2] Command `SetSchoolChatPreset` + validate `allowed_hours` theo giới nghiêm hiệu lực trong đường ghi chat policy; contract `chat_policy_preset` ở schema/API chat policy — file: `src/backend/modules/chat/internal/application/command/set_school_chat_preset.go`, `src/backend/modules/chat/internal/domain/policy/validate_rule.go`, `specs/schemas/chat-policy.schema.json`

## Phase 5: User Story 3 — World, Area, DM (Priority: P2)

### Tests cho User Story 3

- [x] T029 [P] [US3] Test F120-AC-12: World theo trường / theo tenant khi `world_scope=tenant`, lưu ephemeral — file: `tests/integration/world_channel_test.go` — phải FAIL trước khi code
- [x] T030 [P] [US3] Test F120-AC-13: Area chỉ nhận khi `room_id` gateway gắn khớp — file: `tests/integration/area_channel_test.go`
- [x] T031 [P] [US3] Test F120-AC-14, F120-AC-15: DM chỉ bạn bè không block (`CHAT_NOT_ALLOWED`, TC-CHAT-005); `user.blocked` → `frozen`; nickname với người ngoài trường (TC-SOC-006) — file: `tests/integration/direct_channel_test.go`
- [x] T032 [P] [US3] Test F120-AC-16: quét OpenAPI/AsyncAPI không có voice; `capabilities.voice=false` (TC-AUD-004) — file: `tests/unit/no_voice_test.go`
- [x] T058 [P] [US3] Test F120-AC-28: học sinh tenant `common` mở DM, gửi `world` → `CHAT_NOT_ALLOWED`; `area` `text` → `CHAT_KIND_NOT_ALLOWED`, `quick` được; System Admin bật lại có `reason` + audit; `tenant_admin` bật lại → 403 — file: `src/backend/modules/chat/tests/integration/common_tenant_defaults_test.go` — phải FAIL trước khi code

### Implementation cho User Story 3

- [x] T033 [US3] Resolver channel `world`/`area` (lười, quyền động theo `ref.students`, `room_id` từ gateway) — file: `internal/application/command/channel/resolve_ephemeral_channel.go`
- [x] T034 [US3] Tạo `direct` lần đầu, consumer `user.blocked`/`user.unblocked` → `FreezeDirectChannel` — file: `internal/application/command/channel/freeze_direct.go`, `internal/presentation/consumer/user_blocked.go`
- [x] T059 [US3] Rule mặc định tenant `common` (seed chat policy cấp tenant: `world`/`direct` tắt, `area` chỉ `quick` + `emoji`) + command `SetCommonTenantSocialOverride` (System Admin, `reason`, audit) — file: `src/backend/modules/chat/internal/application/command/set_common_tenant_social_override.go`, `deploy/seed/chat/common_tenant_policy.json`

## Phase 6: User Story 4 — Club (Priority: P2)

### Tests cho User Story 4

- [x] T035 [P] [US4] Test F120-AC-17: tạo club theo `student_club_creation`, tên qua pipeline (`451 CHAT_CONTENT_BLOCKED`), từ chối `club_type` ngoài enum (kể cả `study_group`) và `add_whole_class`, `Idempotency-Key` (TC-SOC-004) — file: `tests/integration/club_create_test.go` — phải FAIL trước khi code
- [x] T036 [P] [US4] Test F120-AC-18: `approveClub` chỉ `social.club.approve` trường của club; tạo channel `team` cố định; `409 CLUB_NOT_APPROVED` — file: `tests/integration/club_approve_test.go`
- [x] T037 [P] [US4] Test F120-AC-19: `CLUB_FULL`, `CLUB_LIMIT_REACHED` đồng thời 2 request; `SOCIAL_SCOPE_VIOLATION` khác tenant/block (TC-SOC-005, TC-SOC-006) — file: `tests/integration/club_limits_test.go`
- [x] T038 [P] [US4] Test F120-AC-20: tham gia/rời/gỡ đồng bộ channel cùng transaction; archive → `read_only` (TC-SOC-008) — file: `tests/integration/club_membership_test.go`
- [x] T039 [P] [US4] Test F120-AC-21: goal progress từ event, trùng `event_id` không cộng, không điểm/mastery (TC-SOC-007) — file: `tests/integration/club_goal_test.go`
- [x] T040 [P] [US4] Test authorization + tenant isolation mọi endpoint `/social/clubs*` (học sinh trường khác, nhân sự tenant khác → `404`/`403`) — file: `tests/integration/club_authz_test.go`

### Implementation cho User Story 4

- [x] T041 [P] [US4] Domain `club.Club`, `Member`, `Goal` — file: `internal/domain/club/`
- [x] T042 [US4] Command club (create, approve, join request, invitation, leave, remove, archive, goal, event) + outbox `club.*` — file: `internal/application/command/club/`
- [x] T043 [US4] Handler `/social/clubs*` (operationId theo contracts §1); consumer `school.updated` → `ref.schools`; consumer nguồn goal — file: `internal/presentation/http/public/clubs.go`, `internal/presentation/consumer/club_goal_sources.go`

## Phase 7: User Story 5 — Tìm kiếm (Priority: P2)

### Tests cho User Story 5

- [x] T044 [P] [US5] Test F120-AC-22, F120-AC-23: "nhan vat" khớp "nhân vật"; chỉ channel thành viên/tenant; không thu hồi/chặn/World/ẩn danh hóa/nội dung gốc (TC-CHAT-040); `q` vượt `maxLength` → 422 — file: `tests/integration/search_test.go` — phải FAIL trước khi code

### Implementation cho User Story 5

- [x] T045 [US5] Query `SearchMessages` (tsquery unaccent, trigram ≤ 3 ký tự, cursor) — file: `internal/application/query/search/search_messages.go`, `internal/infrastructure/search/tsquery.go`

## Phase 8: User Story 6 — Retention (Priority: P2)

### Tests cho User Story 6

- [x] T046 [P] [US6] Test F120-AC-24: job ẩn danh hóa theo lô, 0 `DELETE`, audit tổng hợp, chỉ channel bền vững (club `team` cố định, `direct`) (TC-CHAT-024) — file: `tests/integration/persistent_retention_test.go` — phải FAIL trước khi code
- [x] T047 [P] [US6] Test F120-AC-25: tin thuộc case mở sao sang `moderation_evidence` trước (TC-CHAT-022); API không trả tin ẩn danh — file: `tests/integration/persistent_retention_evidence_test.go`

### Implementation cho User Story 6

- [x] T048 [US6] Command `RunPersistentRetention` + port `EvidenceCopier`, `ModerationCaseChecker` (bounded context moderation cùng module chat) — file: `internal/application/command/retention/run_persistent_retention.go`
- [x] T049 [US6] Job `persistent-chat-retention` (profile `worker-chat`, khóa phân tán, ghi `persistent_retention_runs`) — file: `internal/presentation/job/persistent_retention.go`

## Phase N: Polish & Cross-Cutting

- [x] T050 [P] Cập nhật DATABASE_DESIGN §2.11–§2.12 (bỏ study group khỏi club, CHECK không còn `class`/`study_group`, cột mới, `persistent_retention_runs`), CHAT_SYSTEM §2, §4 (club = team cố định; channel lớp/nhóm học tập bỏ theo ADR-0095 — WP-C1, bỏ `teaching_assignments`), SOCIAL_SYSTEM §6.1 (đã sửa theo ADR-0095 ở WP-S 2026-11-21) — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/03-social-multiplayer/CHAT_SYSTEM.md`, `docs/03-social-multiplayer/SOCIAL_SYSTEM.md`
- [x] T051 [P] Đăng ký `persistent_chat_retention_days` ở CONFIG_DEFAULTS §12 (sở hữu CHAT_SYSTEM) — file: `docs/00-overview/CONFIG_DEFAULTS.md`
- [ ] T052 [P] Metric mới + alert lag membership — file: `src/backend/modules/chat/internal/infrastructure/metrics/metrics.go`, `deploy/observability/prometheus/rules/chat.yaml` — Một phần: metric `eduworld_chat_retention_anonymized_total{tenant}`, `eduworld_chat_policy_rejected_total{code}` đã có (`infrastructure/observability/persistent_metrics.go`); repo chưa có `deploy/observability/prometheus/rules/` nên chưa thêm alert, và chưa có metric `membership_sync_lag` (thành viên club đồng bộ cùng transaction, chỉ freeze DM đi qua event).
- [x] T053 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T061 Lọc block/mute phía server khi fan-out và trả lịch sử World/Area (Clarifications 2026-11-23, orchestrator chốt): không giao tin tới kết nối có quan hệ block hai chiều với người gửi, không giao tin của người gửi đang bị mute; test integration chứng minh người bị block không nhận tin qua realtime và qua lịch sử bộ đệm — file: `src/backend/modules/chat/internal/infrastructure/realtime/persistent.go`, `src/backend/modules/chat/internal/application/command/persistent.go`, `src/backend/modules/chat/tests/integration/persistent_channels_test.go`
- [ ] T054 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — Chưa chạy tay trên compose (stack local của owner không được động tới); kịch bản quickstart được phủ bằng integration test (bảng AC→test).

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc). Chưa làm.

- [x] T060 **OQ chốt 2026-11-20** (OQ-09) Seed chat policy mặc định platform: `allowed_hours` 06:30–21:30 từ cấu hình `chat_allowed_hours` cho channel bền vững, `world_scope = school`; test học sinh nhắn 21:45 → `CHAT_NOT_ALLOWED`, giá trị `world_scope` khác `school`/`tenant` → `422` — file: `src/backend/modules/chat/cmd/seed/`, `src/backend/modules/chat/tests/integration/` — kiểm: `make test-integration MOD=chat` — cỡ S

## AC → test (bằng chứng, 2026-11-23)

| Tiêu chí | Test | Loại |
|---|---|---|
| F120-AC-01, AC-02, AC-03 (Removed — kiểm âm) | `tests/static/f120_static_test.go` `TestNoClassChannelNorConsumer`; `TestChatPolicyValidation` (scope `class` → 422); `TestChannelTypesWithoutClass` (F115) | static, integration |
| F120-AC-04 | `TestClubChannelMembership` (joined_at/left_at, vào lại, `Idempotency-Key`), `TestDirectChannel` (`user.blocked` giao 2 lần → 1 lần, `source_event_id`), `TestClubLimits` (nhân sự không là member) | integration |
| F120-AC-05 | `TestClubChannelMembership` (người ngoài/tenant khác 404, nhân sự 403, RLS 0 dòng) | integration |
| F120-AC-06 | `TestChatPolicyPutLevels` | integration |
| F120-AC-07 | `TestPolicyValidateLevelRelax`, `TestChatPolicyValidation` | unit, integration |
| F120-AC-08 | `TestPolicyGradeKeys`, `TestChatPolicyValidation`, `policyschema.TestValidate`, `TestNoGradeCondition` (F115) | unit, integration, static |
| F120-AC-09 | `TestChatPolicyValidation` (party off → `CHAT_POLICY_ALWAYS_ON`), `TestValidateAlwaysOn`, `TestCheckPolicyDocument` (F117) | integration, unit |
| F120-AC-10 | `TestPolicyEffectiveThreeLevels`, `TestChatPolicyPutLevels` (cache thế hệ: hiệu lực ngay) | unit, integration |
| F120-AC-11 | `TestChatPolicyAuthz`, `TestOperationsMatchSpec` (route platform `x-internal`) | integration, contract |
| F120-AC-12 | `TestWorldChannel` | integration |
| F120-AC-13 | `TestAreaChannel` | integration |
| F120-AC-14 | `TestDirectChannel`, `TestPersistentChannelIDs` | integration, unit |
| F120-AC-15 | `TestDirectChannel` (frozen, `pii` khác trường) | integration |
| F120-AC-16 | `TestNoVoice`, `TestWorldChannel`/`TestDirectChannel` (`capabilities.voice = false`) | static, integration |
| F120-AC-17 | `TestClubCreate`, `TestClubDomain` | integration, unit |
| F120-AC-18 | `TestClubApprove` | integration |
| F120-AC-19 | `TestClubLimits` | integration |
| F120-AC-20 | `TestClubChannelMembership` | integration |
| F120-AC-21 | `TestClubGoals` | integration |
| F120-AC-22, AC-23 | `TestSearchMessages` | integration |
| F120-AC-24, AC-25 | `TestPersistentRetention` | integration |
| F120-AC-26 | `TestChatPolicyPutLevels` (preset, hiệu lực ngay), `TestChatPolicyValidation` (preset theo khối/cấp tenant → 422), `TestPolicyValidateLevelRelax` | integration, unit |
| F120-AC-27 | `TestPolicyAllowedHoursCurfew`, `TestChatPolicyValidation` | unit, integration |
| F120-AC-28 | `TestCommonTenantDefaults`, `TestChatPolicyAuthz` (role tenant → 403) | integration |
| Authz mọi endpoint mới (T024, T040) | `TestChatPolicyAuthz`, `TestClubAuthz`, `TestDirectChannel`, `TestSearchMessages` (`authztest.Cover`, `routematrix check`) | integration |

## Dependencies & Execution Order

- Setup → Foundational (T004–T010) → US1 → US2 → US3, US4, US5 song song → US6 (cần module moderation F121 cho kiểm case mở; trước đó port trả "không có case") → Polish.
- US1 không phụ thuộc module learning (không còn consumer lớp/nhóm học tập — ADR-0095); consumer `user.blocked` cần schema F095 (T008).

## Definition of Done

- [x] Mọi tiêu chí F120-AC-* có test pass (ghi mã test/TC) — bảng AC → test ở trên
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract` (MOD=chat) xanh — chạy từng lệnh tương đương 2026-11-23; `migrate-lint` (Atlas trên Docker) thay bằng `atlas-sum` + áp migration thật trong integration
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — test double chỉ trong test (`nameChecker`, `switchValidator`), có nhãn
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
