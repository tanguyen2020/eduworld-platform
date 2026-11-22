# Tasks: F117 — Chat gameplay luôn có (`match`/`team`/`party`) và purge chat gameplay

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 [P] Hàm thuần `ChannelID(kind, scopeID, teamID)` UUIDv5 + test vector cố định (dùng chung với module competition) — file: `src/backend/pkg/chatid/chatid.go`, `src/backend/pkg/chatid/chatid_test.go` — ✅ `pkg/chatid` (`Party`, `Match`, `Team(match, team_no)`), `TestChannelIDVectors` PASS
- [x] T002 [P] DSN role `chat_purger` (secret file, chỉ nạp trong lệnh job purge) + cấu hình lịch job — file: `src/backend/modules/chat/internal/config/purge.go` — ✅ `internal/infrastructure/config/purge.go` (`PurgeFrom`); DSN `EW_DB_CHAT_PURGER_DSN`/`_URL_FILE` chỉ mở trong `registerPurge` (worker chat/all)
- [x] T003 [P] Durable consumer `chat-gameplay` trên `EVT_COMPETITION` (`ew.evt.competition.party.*`, `ew.evt.competition.match.*`) — file: `src/backend/modules/chat/internal/infrastructure/natsstream/competition_consumer.go` — ✅ consumer `chat.gameplay_channels` (`messaging.GameplayChannelsEvents`, `MaxSchema: 2`), khai báo trong worker events (`TestWorkerProfilesRegisterRelaysConsumersAndJobs` PASS)

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: payload `chat.channel_opened`, `chat.channel_closed`, `chat.gameplay_purged`, `chat.daily_stats`, `chat-cursor` — file: `specs/events/payloads/chat/` — kiểm: `make specs-lint` — ✅ 4 schema (`chat.channel_opened`, `chat.channel_closed`, `chat.gameplay_purged`, `chat.daily_stats`); `chat-cursor` bỏ (Clarifications 2026-11-22); validate-schemas 0 lỗi, `checkEvents` trong integration
- [x] T005 [P] Phối hợp F105/F101: schema payload `party.*`, `match.started` (`participants[]`, `mode`), `match.completed`, `match.aborted`; `chat_channel_ids` trong message trận AsyncAPI — file: `specs/events/payloads/competition/`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make test-contract` — ✅ schema `party.*` (v2), `match.*` đã có (F100/F104/F105); `chat_channel_ids` trong payload trận: ngoài phạm vi (F105, Clarifications 2026-11-22)
- [x] T006 [P] Phối hợp F090: định dạng fan-out nội bộ có `exclude_user_ids` — file: `docs/06-architecture/REALTIME_ARCHITECTURE.md` — ✅ không cần đổi: module chat gửi `target.user_ids` đã lọc block/mute (Clarifications 2026-11-22)
- [x] T007 ent schema `RefParty`, `GameplayChatDailyStat`, `GameplayPurgeRun`, `ModerationEvidence`, `EphemeralMessage` — file: `src/backend/modules/chat/ent/schema/` — kiểm: `make migrate-diff MOD=chat name=f117_gameplay`, `make migrate-lint MOD=chat` — ✅ bảng viết tay (projection, partitioned, aggregate — như `gameplay_partition_channels`): `ref.parties`, `gameplay_chat_daily_stats`, `gameplay_purge_runs`, `ephemeral_messages`; `moderation_evidence` giữ nguyên (F121); `TestMigrationMatchesEnt` PASS, atlas.sum
- [x] T008 Migration: role `chat_purger` + GRANT/REVOKE đúng DATABASE_DESIGN §3.0, policy `purger_gameplay_only`, partition ngày `ephemeral_messages`, trigger bất biến `moderation_evidence` — file: `src/backend/modules/chat/migrations/` — ✅ `20261231000004_f117_gameplay_purge.sql` (role `chat_purger` + RLS `purger_gameplay_only`, hàm SECURITY DEFINER partition, CHECK always-on)
- [x] T009 [P] Test quyền DB F117-AC-20: `chat_purger` chỉ DELETE bảng cho phép; `chat_app`/`chat_platform` không DELETE/TRUNCATE (TC-TEN-016, TC-CHAT-024) — file: `src/backend/modules/chat/tests/integration/db_privileges_purger_test.go` — ✅ `TestPurgerPrivileges` (gameplay_purge_test.go) PASS, gồm `scripts/ci/db-privileges.sql` 0 vi phạm

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Chat có ngay khi vào party hoặc trận (Priority: P1) 🎯 MVP

**Goal**: Channel gameplay tự tạo, tự join, đóng read-only.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T010 [P] [US1] Integration F117-AC-01, F117-AC-04: `party.*` → channel `party`, thêm/gỡ thành viên, `chat.channel_opened` (TC-CHAT-017) — file: `src/backend/modules/chat/tests/integration/gameplay_party_channel_test.go` — phải FAIL trước khi code — ✅ `TestGameplayPartyChannel` PASS
- [x] T011 [P] [US1] Integration F117-AC-02: `match.started` 1v1 và 2v2 → `match` + `team` mỗi đội; đội khác `404` — file: `src/backend/modules/chat/tests/integration/gameplay_match_channel_test.go` — ✅ `TestGameplayMatchChannels` PASS
- [x] T012 [P] [US1] Integration F117-AC-03: event trùng, lệch thứ tự → không trùng (TC-SVC-003) — file: `src/backend/modules/chat/tests/integration/gameplay_idempotency_test.go` — ✅ trong `TestGameplayPartyChannel`, `TestGameplayMatchChannels` (giao lại, lệch thứ tự, end trước start)
- [x] T013 [P] [US1] Integration F117-AC-05, F117-AC-06: channel đóng → `CHAT_CHANNEL_CLOSED`, lịch sử vẫn đọc; đổi thiết bị → `chat.channel.sync` đủ (TC-CHAT-020) — file: `src/backend/modules/chat/tests/integration/gameplay_close_resume_test.go` — ✅ trong `TestGameplayPartyChannel` (409 WS+REST, lịch sử 200, `chat.channel.sync` thiết bị 2)
- [x] T014 [P] [US1] Test tenant isolation + authorization F117-AC-07: tenant lệch/nhân sự trong `participants` bị bỏ qua; học sinh tenant khác không truy cập được channel (TC-TEN-012) — file: `src/backend/modules/chat/tests/integration/gameplay_tenant_test.go` — ✅ trong `TestGameplayMatchChannels` (tenant lệch, nhân sự/bot, tenant B 404, event tenant B → DLQ)

### Implementation cho User Story 1

- [x] T015 [P] [US1] Domain `GameplayChannel` (Open/Close, `PurgeAfter` với sàn 3 ngày) — file: `src/backend/modules/chat/internal/domain/channel/gameplay.go` — ✅ `domain/channel/gameplay.go`; `TestGameplayRetention` PASS
- [x] T016 [US1] Command `OpenGameplayChannel`, `SyncGameplayMembers`, `CloseGameplayChannel` (outbox `chat.channel_opened`, `chat.channel_closed`) — file: `src/backend/modules/chat/internal/application/command/gameplay_channel.go` — ✅ `command/gameplay_channel.go` (`ApplyParty`, `ApplyMatch`)
- [x] T017 [US1] Consumer party/trận + projection `ref.parties` — file: `src/backend/modules/chat/internal/presentation/consumer/competition_party.go`, `src/backend/modules/chat/internal/presentation/consumer/competition_match.go` — ✅ `presentation/consumer/competition.go` + `persistence/gameplay.go` (`ref.parties`)

**Checkpoint**: US1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Không cấp nào tắt được chat gameplay (Priority: P1)

### Tests cho User Story 2

- [x] T018 [P] [US2] Unit F117-AC-08, F117-AC-09: `ApplyAlwaysOn` sau `Intersect`; `ValidateAlwaysOn` từ chối `enabled:false`, thiếu kind, `allowed_hours`, slow mode > 10 → `CHAT_POLICY_ALWAYS_ON`; schema v5 từ chối (TC-CHAT-018) — file: `src/backend/modules/chat/tests/unit/policy_always_on_test.go` — phải FAIL trước khi code — ✅ `TestApplyAlwaysOn`, `TestValidateAlwaysOn`, `TestCheckPolicyDocument` PASS (FAIL trước khi code: build lỗi `Rule.AllowedHours` với policy.go của main)
- [x] T019 [P] [US2] Test tĩnh F117-AC-10: không flag/env/setting nào tắt `match`/`team`/`party` — file: `src/backend/modules/chat/tests/static/always_on_no_flag_test.go` — ✅ `TestAlwaysOnHasNoSwitch` (tests/static/gameplay_static_test.go) PASS
- [x] T020 [P] [US2] Integration F117-AC-11: mute vẫn member, vẫn nhận, gửi `CHAT_MUTED` (TC-CHAT-019) — file: `src/backend/modules/chat/tests/integration/gameplay_mute_test.go` — ✅ `TestGameplayMutedStudent` (pipeline thật) PASS
- [x] T021 [P] [US2] Integration F117-AC-12: block ẩn hai chiều, mute ẩn một chiều trong fan-out (`exclude_user_ids`) và lịch sử; không rời trận (TC-SOC-003, TC-CHAT-005) — file: `src/backend/modules/chat/tests/integration/gameplay_block_mute_test.go` — ✅ `TestGameplayBlockMuteVisibility` PASS (file gameplay_visibility_test.go)
- [x] T038 [P] [US2] Test F117-AC-24: trường preset → channel `party`/`match` vẫn tạo + join, `allowed_kinds = [quick, emoji]`; rule school bỏ `text` không qua preset → `422 CHAT_POLICY_ALWAYS_ON`; trường không preset giữ đủ loại — file: `src/backend/modules/chat/tests/integration/gameplay_preset_test.go` — phải FAIL trước khi code — ✅ `TestAlwaysOnPreset` + `TestGameplayPresetAndAlwaysOnWrites` PASS

### Implementation cho User Story 2

- [x] T022 [P] [US2] `ApplyAlwaysOn`, `ValidateAlwaysOn` (export cho đường ghi policy F120) — file: `src/backend/modules/chat/internal/domain/policy/always_on.go` — ✅ + `command/policy_write.go` (`CheckPolicyDocument`) + CHECK `chat_policies_always_on_chk`
- [x] T023 [US2] Tính `exclude_user_ids` (cache block/mute) khi fan-out + lọc trong query lịch sử — file: `src/backend/modules/chat/internal/application/service/visibility.go` — ✅ `ChannelFacts.Hidden` (block hai chiều, mute một chiều) dùng ở fan-out và `query.Chat.page`
- [x] T039 [US2] Nhánh preset trong `ResolveGameplayPolicy`; sửa `specs/schemas/chat-policy.schema.json` (cho phép `allowed_kinds` của `match`/`team`/`party` chỉ khi `preset = quick_message_emote_only`) — file: `src/backend/modules/chat/internal/domain/policy/gameplay_policy.go`, `specs/schemas/chat-policy.schema.json` — ✅ preset trong `policy.ApplyAlwaysOn`; schema v5 không cần sửa (Clarifications 2026-11-22)

## Phase 5: User Story 3 — Lịch sử, chưa đọc và thời hạn lưu trong trận/party (Priority: P2)

### Tests cho User Story 3

- [x] T024 [P] [US3] Integration F117-AC-13, F117-AC-14: gộp con trỏ ≤ 1 lần/2 s, `last_read_seq` không lùi, `chat.unread.updated` đa thiết bị; `listMyChannels` có `status`, `purge_after`, `unread_count` (TC-CHAT-039) — file: `src/backend/modules/chat/tests/integration/gameplay_unread_test.go` — phải FAIL trước khi code — ✅ `TestGameplayUnread` PASS

### Implementation cho User Story 3

- [x] T025 [US3] NATS `ew.cmd.chat.receipt.read` → `ew:chat:cursor_buf` → JetStream `cursor`; persister cập nhật `last_read_seq`; cache `ew:chat:unread:<user_id>` — file: `src/backend/modules/chat/internal/presentation/natscmd/receipt_read.go`, `src/backend/modules/chat/internal/infrastructure/valkey/cursor_buffer.go` — ✅ `natscmd/receipt_read.go`, `cache/cursor.go`, `command/mark_read.go`; ghi bền vững trực tiếp thay JetStream `cursor` (Clarifications 2026-11-22)

## Phase 6: User Story 4 — Purge chat gameplay đúng hạn, giữ bằng chứng (Priority: P1)

### Tests cho User Story 4

- [x] T026 [P] [US4] Integration F117-AC-15: `purge_after` theo retention tenant; giá trị < 3 → sàn 3 + cảnh báo — file: `src/backend/modules/chat/tests/integration/channel_closer_test.go` — phải FAIL trước khi code — ✅ `TestGameplayPartyChannel` (sàn 3), `TestGameplayRetentionProjection`
- [x] T027 [P] [US4] Integration F117-AC-16, F117-AC-23: D+N−1 còn, D+N xóa đủ bảng theo lô, partition drop khi mọi channel purge, API `404` sau purge (TC-CHAT-021) — file: `src/backend/modules/chat/tests/integration/gameplay_purge_test.go` — ✅ `TestGameplayPurge` PASS
- [x] T028 [P] [US4] Integration F117-AC-17, F117-AC-18: sao bằng chứng (report/case/sanction) trước khi xóa; case mở → hoãn, đóng → purge lần sau (TC-CHAT-022, TC-CHAT-023) — file: `src/backend/modules/chat/tests/integration/purge_evidence_test.go` — ✅ trong `TestGameplayPurge` (bằng chứng đọc lại qua `Safety.CaseEvidence`, case mở → hoãn → đóng → purge)
- [x] T029 [P] [US4] Integration F117-AC-19: chưa có bảng moderation hoặc sao bằng chứng lỗi → không xóa, run `blocked`, metric — file: `src/backend/modules/chat/tests/integration/purge_fail_closed_test.go` — ✅ `TestGameplayPurge` (run blocked, rollback cả channel) + unit `TestGameplayPurgeFailClosed`
- [x] T030 [P] [US4] Integration F117-AC-20, F117-AC-21, F117-AC-22: 0 dòng team cố định (club)/`direct` bị xóa; ephemeral drop partition; run record + audit + `chat.gameplay_purged` + metric (TC-SOC-009) — file: `src/backend/modules/chat/tests/integration/purge_scope_test.go` — ✅ trong `TestGameplayPurge` (club/direct không đổi, ephemeral, run + audit + event)
- [x] T031 [P] [US4] Test tĩnh F117-AC-23: `DELETE`/`DROP` chỉ xuất hiện trong `internal/infrastructure/purge` — file: `src/backend/modules/chat/tests/static/hard_delete_scope_test.go` — ✅ `TestHardDeleteOnlyInPurge` PASS

### Implementation cho User Story 4

- [x] T032 [US4] Job `chat-channel-closer` + consumer cài đặt retention (`tenant.updated`, `platform_settings.changed`, dự phòng facade `identity` `getTenant`) — file: `src/backend/modules/chat/internal/presentation/job/channel_closer.go`, `src/backend/modules/chat/internal/presentation/consumer/identity_settings.go` — ✅ đóng channel + `purge_after` trong consumer `chat.gameplay_channels` (không worker `chat-channel-closer`); retention chiếu từ `platform_settings.changed` (Clarifications 2026-11-22)
- [x] T033 [US4] Purger: rollup, evidence copier, xóa lô, partition dropper, `ModerationReadiness` — file: `src/backend/modules/chat/internal/infrastructure/purge/` — ✅ `infrastructure/purge/store.go`
- [x] T034 [US4] Job `gameplay-chat-purge` (lock, run record, audit, outbox `chat.gameplay_purged`, `chat.daily_stats`) + `partition-maintainer` — file: `src/backend/modules/chat/internal/presentation/job/gameplay_chat_purge.go`, `src/backend/modules/chat/internal/presentation/job/partition_maintainer.go` — ✅ `command/gameplay_purge.go`, `command/partition_maintainer.go`, `presentation/job/gameplay.go`, `registerPurge` (module.go); `TestWorkerChatProfileLoadsChatOnly` PASS

## Phase 7: Polish & Cross-Cutting

- [x] T035 [P] Metric/alert purge (`backlog > 2 ngày`, `blocked`, lỗi) và `eduworld_chat_gameplay_open_latency_seconds` — file: `src/backend/modules/chat/internal/infrastructure/observability/gameplay_metrics.go` — ✅ `observability/gameplay_metrics.go` (`eduworld_chat_purge_*`, `eduworld_chat_active_channels`, …)
- [x] T036 [P] Cập nhật tài liệu: CHAT_SYSTEM §4 (`match.completed`, id tất định, "cùng transaction" → outbox + consumer), EVENT_DESIGN §4.8 (`match.started` payload), DATABASE_DESIGN (`gameplay_purge_runs.status = blocked`, `moderation_evidence.source_ref`), TEST_STRATEGY TC-CHAT-018 (v5), PRD FR-PTY-001 (P4), CHANGELOG — file: `docs/03-social-multiplayer/CHAT_SYSTEM.md`, `docs/07-backend/EVENT_DESIGN.md`, `docs/07-backend/DATABASE_DESIGN.md`, `docs/12-project/TEST_STRATEGY.md`, `CHANGELOG.md` — ✅ CHAT_SYSTEM §4.1/§6.1/§8.1, EVENT_DESIGN §4.9, DATABASE_DESIGN §2.12/§2.17/§3.0, VALKEY_DESIGN, DEPLOYMENT, BACKEND_DESIGN, SYSTEM_ARCHITECTURE, CONFIG_DEFAULTS, CHANGELOG
- [ ] T037 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — ⏳ chưa chạy trên compose (stack local của owner đang chạy, không được tạo lại/seed lại); kịch bản 1–16 đã được integration test phủ

## Dependencies & Execution Order

- Setup → Foundational → US1 → US2 → US3 và US4 song song → Polish.
- Cần F101/F105 phát event đủ payload; F121 để tắt chế độ fail-closed của purge; F090 áp `exclude_user_ids`.

## Definition of Done

- [x] Mọi tiêu chí F117-AC-* có test pass (ghi mã test/TC) — 24/24, xem từng task
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint`, `make test-contract` xanh
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — test double chỉ trong tests, gắn nhãn TEST DOUBLE
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
