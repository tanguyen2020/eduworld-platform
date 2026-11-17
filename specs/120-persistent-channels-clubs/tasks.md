# Tasks: F120 — Channel bền vững (world, area, class, study group, team, DM), club, chính sách theo cấp, tìm kiếm, retention

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Đường dẫn gốc `modules/chat/` trừ khi ghi rõ. Fixture chỉ dùng chuỗi placeholder, không từ nhạy cảm thật (R15).

## Phase 1: Setup

- [ ] T001 Đăng ký permission dùng trong module (`tenant.settings.read/write`, `school.settings.read/write`, `platform.settings.write`, `social.club.approve`) vào kiểm tra catalog lúc khởi động — file: `modules/chat/internal/bootstrap/authz.go`
- [ ] T002 [P] Khai báo durable consumer `chat-membership`, `chat-club-goals` (stream `EVT_IDENTITY`, `EVT_LEARNING`, `EVT_GAME`, `EVT_CHAT`) và lịch job `persistent-chat-retention` trong cấu hình worker — file: `modules/chat/config/worker.yaml`
- [ ] T003 [P] Fixture event mẫu (`class.synced`, `enrollment.changed`, `study_group.*`, `user.blocked`, `activity.graded`) — file: `modules/chat/tests/fixtures/events/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: `getChatPolicy`, `putChatPolicy`, `getEffectiveChatPolicy`, `getPlatformChatPolicy`, `putPlatformChatPolicy`, schema `ChatPolicyDocument`/`ChatPolicyPut`/`EffectiveChatPolicy` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`
- [ ] T005 [P] Delta contract: `ClubType` bỏ `study_group`, `listClubs` thêm `status`, `school_id` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`
- [ ] T006 [P] AsyncAPI `chat.channel.membership_changed` — file: `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`
- [ ] T007 [P] Payload schema `chat_policy.updated`, `club.created/approved/archived/member_joined/member_left/goal_completed`; thêm `chat_policy.updated` vào EVENT_DESIGN §4.9 — file: `specs/events/payloads/chat/`, `docs/07-backend/EVENT_DESIGN.md` — kiểm: `make specs-lint`
- [ ] T008 Xác nhận payload schema owner khác đã có (`study_group.*` F040, `enrollment.changed`/`class.created`/`school.updated` F010/F015, `user.blocked` F095); thiếu → chặn US tương ứng, không tự tạo — file: `specs/events/payloads/` (chỉ đọc)
- [ ] T009 ent schema: thêm cột `channels.status/capabilities`, `channel_members.joined_at/left_at/source_event_id`, `messages.anonymized_at`, `chat_policies.reason/updated_by`, `club_goal_progress.last_event_id`, `ref.schools.student_club_creation`; bảng `persistent_retention_runs`; CHECK `club_type` bỏ `study_group`; unique channel theo phạm vi — file: `modules/chat/ent/schema/*.go` — kiểm: `make migrate-diff MOD=chat`, `make migrate-lint MOD=chat`
- [ ] T010 [P] Domain policy: `Rule`, `Intersect`, `ValidateLowerLevel` (siết chặt), `ValidateAlwaysOn`, chặn `grade_*` (+ unit test bảng) — file: `internal/domain/policy/`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — Channel lớp và nhóm học tập (Priority: P1) 🎯 MVP

**Goal**: Channel `class`/`study_group` tự đồng bộ theo event.

**Independent Test**: như spec.md US1.

### Tests cho User Story 1

- [ ] T011 [P] [US1] Test F120-AC-01: `class.synced` created/closed → một channel, `read_only` khi đóng; event đến trước lớp — file: `tests/integration/consumer_class_test.go` — phải FAIL trước khi code
- [ ] T012 [P] [US1] Test F120-AC-02: `enrollment.changed` transferred → `left_at`, lịch sử cũ `404` — file: `tests/integration/consumer_enrollment_test.go`
- [ ] T013 [P] [US1] Test F120-AC-03: `study_group.created/member_changed/deleted` → channel, thêm/bớt đúng ID, `archived` — file: `tests/integration/consumer_study_group_test.go`
- [ ] T014 [P] [US1] Test F120-AC-04: giao event 2 lần xử lý 1 lần (TC-SVC-003); staff không thành member; xem từ `joined_at` — file: `tests/integration/membership_idempotency_test.go`
- [ ] T015 [P] [US1] Test F120-AC-05 tenant isolation: người ngoài/nhân sự/tenant khác → `404`; repository bỏ tenant → 0 dòng (RLS, TC-CHAT-009, TC-TEN-005) — file: `tests/integration/channel_isolation_test.go`
- [ ] T016 [P] [US1] Contract test consumer với schema `class.synced`, `study_group.*`, `enrollment.changed` — file: `tests/contract/membership_events_test.go`

### Implementation cho User Story 1

- [ ] T017 [P] [US1] Domain `channel.Channel`, `Membership` (trạng thái, chỉ học sinh) — file: `internal/domain/channel/channel.go`, `internal/domain/channel/membership.go`
- [ ] T018 [US1] Command `SyncClassChannel`, `SyncStudyGroupChannel` (outbox `chat.channel.membership_changed` qua realtime) — file: `internal/application/command/channel/sync_class_channel.go`, `sync_study_group_channel.go`
- [ ] T019 [US1] Consumer `class_synced.go`, `enrollment_changed.go`, `study_group.go` (processed_events) — file: `internal/presentation/consumer/`
- [ ] T020 [US1] Lọc `joined_at`/`left_at` trong `listChannelMessages`, `listMyChannels` — file: `internal/application/query/channel/list_messages.go`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Chính sách chat theo cấp (Priority: P1)

### Tests cho User Story 2

- [ ] T021 [P] [US2] Test F120-AC-06: `putChatPolicy` mỗi cấp với quyền đúng, `If-Match`, audit before/after, outbox `chat_policy.updated` — file: `tests/integration/chat_policy_put_test.go` — phải FAIL trước khi code
- [ ] T022 [P] [US2] Test F120-AC-07, F120-AC-08, F120-AC-09: nới lỏng → `CHAT_POLICY_RELAX_NOT_ALLOWED`; `grade_band` → `VALIDATION_FAILED` (TC-CHAT-010); always-on → `CHAT_POLICY_ALWAYS_ON` (TC-CHAT-018) — file: `internal/domain/policy/validate_test.go`
- [ ] T023 [P] [US2] Test F120-AC-10: effective policy giao 4 cấp + sanction + quan hệ; thiếu rule = deny; invalidate cache khi event — file: `internal/domain/policy/effective_test.go`, `tests/integration/policy_cache_test.go`
- [ ] T024 [P] [US2] Test F120-AC-11 authorization + tenant isolation: tenant/trường khác → `404`, cấp platform → `403`, gateway public không có route platform (TC-TEN-003) — file: `tests/integration/chat_policy_authz_test.go`
- [ ] T025 [P] [US2] Test giới hạn độ dài `reason` (`maxLength` 500) và kích thước `rules` (TC-TEN-024) — file: `tests/integration/chat_policy_limits_test.go`
- [ ] T055 [P] [US2] Test F120-AC-26: School Admin đặt preset có audit + `chat_policy.updated`, hiệu lực ≤ 60 s; preset theo khối/lớp → 422; trường khác không đổi — file: `modules/chat/tests/integration/school_preset_test.go` — phải FAIL trước khi code
- [ ] T056 [P] [US2] Test F120-AC-27: `allowed_hours` 06:00–22:00 ở cấp trường khi `platform_curfew` 21:30–06:30 → `422 CHAT_POLICY_RELAX_NOT_ALLOWED`; 07:00–21:00 hợp lệ — file: `modules/chat/tests/integration/allowed_hours_curfew_test.go`

### Implementation cho User Story 2

- [ ] T026 [US2] Command `PutChatPolicy`, query `GetChatPolicy`, `GetEffectivePolicy` — file: `internal/application/command/policy/put_chat_policy.go`, `internal/application/query/policy/`
- [ ] T027 [US2] `PolicyCache` Valkey (TTL `chat_policy_cache`, invalidate theo `chat_policy.updated`) — file: `internal/infrastructure/cache/policy_cache.go`
- [ ] T028 [US2] Handler `getChatPolicy`, `putChatPolicy`, `getEffectiveChatPolicy`, `getPlatformChatPolicy`, `putPlatformChatPolicy` — file: `internal/presentation/http/public/chat_policy.go`
- [ ] T057 [US2] Command `SetSchoolChatPreset` + validate `allowed_hours` theo giới nghiêm hiệu lực trong đường ghi chat policy; contract `chat_policy_preset` ở schema/API chat policy — file: `modules/chat/internal/application/command/set_school_chat_preset.go`, `modules/chat/internal/domain/policy/validate_rule.go`, `specs/schemas/chat-policy.schema.json`

## Phase 5: User Story 3 — World, Area, DM (Priority: P2)

### Tests cho User Story 3

- [ ] T029 [P] [US3] Test F120-AC-12: World theo trường / theo tenant khi `world_scope=tenant`, lưu ephemeral — file: `tests/integration/world_channel_test.go` — phải FAIL trước khi code
- [ ] T030 [P] [US3] Test F120-AC-13: Area chỉ nhận khi `room_id` gateway gắn khớp — file: `tests/integration/area_channel_test.go`
- [ ] T031 [P] [US3] Test F120-AC-14, F120-AC-15: DM chỉ bạn bè không block (`CHAT_NOT_ALLOWED`, TC-CHAT-005); `user.blocked` → `frozen`; nickname với người ngoài trường (TC-SOC-006) — file: `tests/integration/direct_channel_test.go`
- [ ] T032 [P] [US3] Test F120-AC-16: quét OpenAPI/AsyncAPI không có voice; `capabilities.voice=false` (TC-AUD-004) — file: `tests/unit/no_voice_test.go`
- [ ] T058 [P] [US3] Test F120-AC-28: học sinh tenant `common` mở DM, gửi `world` → `CHAT_NOT_ALLOWED`; `area` `text` → `CHAT_KIND_NOT_ALLOWED`, `quick` được; System Admin bật lại có `reason` + audit; `tenant_admin` bật lại → 403 — file: `modules/chat/tests/integration/common_tenant_defaults_test.go` — phải FAIL trước khi code

### Implementation cho User Story 3

- [ ] T033 [US3] Resolver channel `world`/`area` (lười, quyền động theo `ref.students`, `room_id` từ gateway) — file: `internal/application/command/channel/resolve_ephemeral_channel.go`
- [ ] T034 [US3] Tạo `direct` lần đầu, consumer `user.blocked`/`user.unblocked` → `FreezeDirectChannel` — file: `internal/application/command/channel/freeze_direct.go`, `internal/presentation/consumer/user_blocked.go`
- [ ] T059 [US3] Rule mặc định tenant `common` (seed chat policy cấp tenant: `world`/`direct` tắt, `area` chỉ `quick` + `emoji`) + command `SetCommonTenantSocialOverride` (System Admin, `reason`, audit) — file: `modules/chat/internal/application/command/set_common_tenant_social_override.go`, `deploy/seed/chat/common_tenant_policy.json`

## Phase 6: User Story 4 — Club (Priority: P2)

### Tests cho User Story 4

- [ ] T035 [P] [US4] Test F120-AC-17: tạo club theo `student_club_creation`, tên qua pipeline (`451 CHAT_CONTENT_BLOCKED`), từ chối `study_group`, `Idempotency-Key` (TC-SOC-004) — file: `tests/integration/club_create_test.go` — phải FAIL trước khi code
- [ ] T036 [P] [US4] Test F120-AC-18: `approveClub` chỉ `social.club.approve` trường của club; tạo channel `team` cố định; `409 CLUB_NOT_APPROVED` — file: `tests/integration/club_approve_test.go`
- [ ] T037 [P] [US4] Test F120-AC-19: `CLUB_FULL`, `CLUB_LIMIT_REACHED` đồng thời 2 request; `SOCIAL_SCOPE_VIOLATION` khác tenant/block (TC-SOC-005, TC-SOC-006) — file: `tests/integration/club_limits_test.go`
- [ ] T038 [P] [US4] Test F120-AC-20: tham gia/rời/gỡ đồng bộ channel cùng transaction; archive → `read_only` (TC-SOC-008) — file: `tests/integration/club_membership_test.go`
- [ ] T039 [P] [US4] Test F120-AC-21: goal progress từ event, trùng `event_id` không cộng, không điểm/mastery (TC-SOC-007) — file: `tests/integration/club_goal_test.go`
- [ ] T040 [P] [US4] Test authorization + tenant isolation mọi endpoint `/social/clubs*` (học sinh trường khác, nhân sự tenant khác → `404`/`403`) — file: `tests/integration/club_authz_test.go`

### Implementation cho User Story 4

- [ ] T041 [P] [US4] Domain `club.Club`, `Member`, `Goal` — file: `internal/domain/club/`
- [ ] T042 [US4] Command club (create, approve, join request, invitation, leave, remove, archive, goal, event) + outbox `club.*` — file: `internal/application/command/club/`
- [ ] T043 [US4] Handler `/social/clubs*` (operationId theo contracts §1); consumer `school.updated` → `ref.schools`; consumer nguồn goal — file: `internal/presentation/http/public/clubs.go`, `internal/presentation/consumer/club_goal_sources.go`

## Phase 7: User Story 5 — Tìm kiếm (Priority: P2)

### Tests cho User Story 5

- [ ] T044 [P] [US5] Test F120-AC-22, F120-AC-23: "nhan vat" khớp "nhân vật"; chỉ channel thành viên/tenant; không thu hồi/chặn/World/ẩn danh hóa/nội dung gốc (TC-CHAT-040); `q` vượt `maxLength` → 422 — file: `tests/integration/search_test.go` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T045 [US5] Query `SearchMessages` (tsquery unaccent, trigram ≤ 3 ký tự, cursor) — file: `internal/application/query/search/search_messages.go`, `internal/infrastructure/search/tsquery.go`

## Phase 8: User Story 6 — Retention (Priority: P2)

### Tests cho User Story 6

- [ ] T046 [P] [US6] Test F120-AC-24: job ẩn danh hóa theo lô, 0 `DELETE`, audit tổng hợp, chỉ channel bền vững (TC-CHAT-024) — file: `tests/integration/persistent_retention_test.go` — phải FAIL trước khi code
- [ ] T047 [P] [US6] Test F120-AC-25: tin thuộc case mở sao sang `moderation_evidence` trước (TC-CHAT-022); API không trả tin ẩn danh — file: `tests/integration/persistent_retention_evidence_test.go`

### Implementation cho User Story 6

- [ ] T048 [US6] Command `RunPersistentRetention` + port `EvidenceCopier`, `ModerationCaseChecker` (bounded context moderation cùng module chat) — file: `internal/application/command/retention/run_persistent_retention.go`
- [ ] T049 [US6] Job `persistent-chat-retention` (profile `worker-chat`, khóa phân tán, ghi `persistent_retention_runs`) — file: `internal/presentation/job/persistent_retention.go`

## Phase N: Polish & Cross-Cutting

- [ ] T050 [P] Cập nhật DATABASE_DESIGN §2.11–§2.12 (bỏ study group khỏi club, cột mới, `persistent_retention_runs`), CHAT_SYSTEM §2, §4 (club = team cố định, membership study group từ module learning, bỏ `teaching_assignments`), SOCIAL_SYSTEM §6.1 — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/03-social-multiplayer/CHAT_SYSTEM.md`, `docs/03-social-multiplayer/SOCIAL_SYSTEM.md`
- [ ] T051 [P] Đăng ký `persistent_chat_retention_days` ở CONFIG_DEFAULTS §12 (sở hữu CHAT_SYSTEM) — file: `docs/00-overview/CONFIG_DEFAULTS.md`
- [ ] T052 [P] Metric mới + alert lag membership — file: `modules/chat/internal/infrastructure/metrics/metrics.go`, `deploy/observability/prometheus/rules/chat.yaml`
- [ ] T053 [P] CHANGELOG — file: `CHANGELOG.md`
- [ ] T054 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational (T004–T010) → US1 → US2 → US3, US4, US5 song song → US6 (cần module moderation F121 cho kiểm case mở; trước đó port trả "không có case") → Polish.
- US1 consumer `study_group.*` bị chặn nếu F040 chưa có payload schema (T008).

## Definition of Done

- [ ] Mọi tiêu chí F120-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make test-contract`, `make migrate-lint` (MOD=chat) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
