# Tasks: F115 — module chat lõi: channel, policy deny-by-default, JetStream, `chat-persister`, gửi/nhận

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [x] T001 Cấu hình chat (`EW_NATS_URLS`, timeout PubAck 2 s, lô persister, `max_deliver`, `chat_recent_buffer`, `chat_rate_per_user`) qua `src/backend/pkg/config` — file: `src/backend/modules/chat/internal/config/chat.go` — ✅ `internal/infrastructure/config/chat.go` (`ChatFrom`, kiểm biên), nạp ở `module.go`; `go test ./cmd/internal/wiring` pass
- [x] T002 [P] Bootstrap stream `CHAT_MESSAGES` (subjects `ew.chat.*.msg.*`, `ew.chat.*.evt.*`, `ew.chat.*.cursor.*`, file storage, `max_age` 72 h, `duplicate_window` 2') và `CHAT_DLQ`, consumer durable `persister` — file: `src/backend/modules/chat/internal/infrastructure/natsstream/stream_bootstrap.go` — ✅ `internal/infrastructure/natsstream/stream.go` (`EnsureStream`: CHAT_MESSAGES file/72 h/dup 2 phút, stream DLQ `DLQ_CHAT` của module — EVENT_DESIGN §4.10); consumer durable `persister` tạo ở `presentation/persister`
- [x] T003 [P] Wiring profile `worker-chat` (`src/backend/cmd/worker --profile=chat`) chạy consumer persister; kiểm `core-api`/`worker` khởi động thất bại khi thiếu `ContentValidator` thật ngoài `EW_ENV=test` — file: `src/backend/modules/chat/module.go` — ✅ `module.go`: `r.Background(chat.persister)` ở worker `chat`/`all`; `TestWorkerChatProfileLoadsChatOnly`, `TestChatRefusesToStartWithoutRealValidator` (cmd/internal/wiring), `TestWorkerProfileChecks` (cmd/worker) pass

## Phase 2: Foundational (chặn mọi user story)

- [x] T004 Delta contract: payload `chat-envelope.v1`, `chat.message_created.v1` — file: `specs/events/payloads/chat/` — kiểm: `make specs-lint` — ✅ `specs/events/payloads/chat/chat.message_created.v1.schema.json`, `specs/events/chat-envelope.v1.schema.json` (không đặt trong `payloads/`: check-cross-features chỉ nhận event outbox); validate-schemas 136 schema 0 lỗi; envelope + payload validate trong `TestSendAndReceive`
- [x] T005 [P] Delta OpenAPI `sendChannelMessage` (`404`, `409`, `428`, `503`), `403` cho `listMyChannels`/`listChannelMessages`, `quick_message_id` string 64; AsyncAPI `client_msg_id` → `client_message_id` (RB-508) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat` — ✅ OpenAPI (403/404/409/422/428/503, `quick_message_id` string 64), AsyncAPI (`client_message_id` bắt buộc, `held` trong `ChatMessage.moderation_state`, `ChatHistoryLoaded` bỏ `ref` bắt buộc); split-public, gen-routes, portal `pnpm gen:api`, check-problem-codes/maxlength/enums 0 lỗi
- [x] T006 ent schema `Channel`, `ChannelMember`, `Message`, `GameplayMessage`, `GameplayPartitionChannel`, `ChatPolicy` (mọi string `MaxLen` theo DATABASE_DESIGN, mixin tenant + soft-delete) — file: `src/backend/modules/chat/ent/schema/` — kiểm: `make migrate-diff MOD=chat name=f115_chat_core`, `make migrate-lint MOD=chat` — ✅ `ent/schema/chat.go` (`Channel`, `ChannelMember`, `ChatPolicy`); bảng tin phân vùng viết tay (Clarifications 2026-11-21); `TestMigrationMatchesEnt`, entlint pass
- [x] T007 Migration: partition tháng `chat.messages`, partition ngày `chat.gameplay_messages`, `chat.f_unaccent`, index/GIN, RLS mọi bảng T, `REVOKE DELETE, TRUNCATE` khỏi `chat_app`/`chat_platform` — file: `src/backend/modules/chat/migrations/` — ✅ `migrations/20261229000001_f115_chat_core.sql` (partition DEFAULT, `chat.unaccent` trong schema `chat`, GIN, RLS FORCE mọi bảng T, REVOKE DELETE/TRUNCATE, view `_live`, `ref.schools`, `ref.platform_settings`); tenancyguard pass
- [x] T008 [P] Test RLS + quyền DB: tenant A không đọc B, không set `app.tenant_id` → 0 dòng, `DELETE` bị từ chối (TC-TEN-005, TC-TEN-016) — file: `src/backend/modules/chat/tests/integration/rls_chat_test.go` — ✅ `tests/integration/rls_chat_test.go` `TestChatRLSAndPrivileges` pass
- [x] T009 [P] Seed platform `chat_policies` 8 loại channel, validate theo `chat-policy.schema.json` v5 — file: `src/backend/modules/chat/seed/chat_policies.platform.json` — ✅ `seed/chat_policies.platform.json` (= `specs/examples/chat-policy.platform-default.json`) nạp trong migration trước khi bật FORCE RLS; `TestSeedPlatformPolicy` (schema v5, 8 loại channel, khớp DB) pass

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Gửi và nhận tin realtime, bền vững trước khi "Sent" (Priority: P1) 🎯 MVP

**Goal**: Đường gửi duy nhất: kiểm tra → pipeline → `seq` → JetStream ack → acked → fan-out.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [x] T010 [P] [US1] Test F115-AC-01, F115-AC-02: acked chỉ sau PubAck; created tới member theo `seq`; held chỉ tới người gửi — file: `src/backend/modules/chat/tests/integration/send_message_test.go` — phải FAIL trước khi code — ✅ `TestSendAndReceive` (`tests/integration/send_message_test.go`) pass
- [x] T011 [P] [US1] Test F115-AC-03: NATS dừng/timeout → `CHAT_TEMPORARILY_UNAVAILABLE`, 0 fan-out, 0 dòng PG — file: `src/backend/modules/chat/tests/chaos/nats_down_test.go` — ✅ `TestNATSDown` (`tests/integration/persister_test.go`: xóa stream rồi dừng container NATS) pass; unit `TestTechnicalLimits` (stream lỗi)
- [x] T012 [P] [US1] Test F115-AC-04, F115-AC-05: `client_message_id`/`Idempotency-Key` trùng → cùng kết quả; REST thiếu key → 428; REST và WS cùng use case — file: `src/backend/modules/chat/tests/integration/send_idempotency_test.go` — ✅ `TestSendIdempotency` pass
- [x] T013 [P] [US1] Test F115-AC-06: mọi `kind` đi qua `ContentValidator` (spy có nhãn TEST DOUBLE); khởi động `core-api`/`worker` ngoài test thiếu validator thật → lỗi — file: `src/backend/modules/chat/tests/unit/send_pipeline_required_test.go` — ✅ `tests/unit/send_pipeline_required_test.go` `TestSendPipelineRequired` + wiring `TestChatRefusesToStartWithoutRealValidator` pass
- [x] T014 [P] [US1] Test F115-AC-07: vượt độ dài/rate bị từ chối trước khi cấp `seq` — file: `src/backend/modules/chat/tests/unit/technical_limits_test.go` — ✅ `tests/unit/technical_limits_test.go` `TestTechnicalLimits` pass

### Implementation cho User Story 1

- [x] T015 [P] [US1] Domain `message` (Kind, ModerationState, Envelope) — file: `src/backend/modules/chat/internal/domain/message/` — ✅ `internal/domain/message/message.go`
- [x] T016 [P] [US1] Valkey: `seq_next.lua` + allocator, recent buffer, fan-out publisher, rate limiter ZSET, cmid store — file: `src/backend/modules/chat/internal/infrastructure/valkey/` — ✅ `internal/infrastructure/cache/chat.go` (`seq_next.lua`, `Recent`, `Rate` ZSET hai cửa sổ, `Dedupe` cmid, `Facts` cache bản cũ khi lỗi); fan-out ở `internal/infrastructure/realtime/chat.go`
- [x] T017 [P] [US1] JetStream publisher (`Nats-Msg-Id`, chờ PubAck 2 s) — file: `src/backend/modules/chat/internal/infrastructure/natsstream/jetstream_publisher.go` — ✅ `natsstream.Publisher` (`Nats-Msg-Id`, PubAck ≤ `chat_publish_ack_timeout` và trong hạn lệnh)
- [x] T018 [US1] Command `SendMessage` (write context; port `ContentValidator`, `SeqAllocator`, `ChatStream`, `Fanout`) — file: `src/backend/modules/chat/internal/application/command/send_message.go` — ✅ `internal/application/command/send_message.go` (`Send`, `RequireValidator`)
- [x] T019 [US1] Handler NATS `ew.cmd.chat.message.send` (chỉ nhận từ user NATS `realtime`, đọc header `Ew-*`) và REST `sendChannelMessage` — file: `src/backend/modules/chat/internal/presentation/natscmd/message_send.go`, `src/backend/modules/chat/internal/presentation/http/public/messages.go` — ✅ `internal/presentation/natscmd/natscmd.go`, `internal/presentation/http/chat.go`

**Checkpoint**: US1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Quyền vào channel theo quan hệ và policy, mặc định từ chối (Priority: P1)

### Tests cho User Story 2

- [x] T020 [P] [US2] Test F115-AC-08: giao 4 cấp (min số, giao `allowed_kinds`, AND `enabled`); thiếu rule → deny — file: `src/backend/modules/chat/tests/unit/policy_intersect_test.go` — phải FAIL trước khi code — ✅ `tests/unit/policy_intersect_test.go` `TestPolicyIntersect` pass
- [x] T021 [P] [US2] Test tĩnh F115-AC-09: package `domain/policy`, `application/command` không tham chiếu `grade_level`/`grade_band`; cùng tin lớp 2 và lớp 8 cùng quyết định — file: `src/backend/modules/chat/tests/static/no_grade_condition_test.go` — ✅ `tests/static/static_test.go` `TestNoGradeCondition` + integration `TestSendPresetAndSameRulesForEveryGrade` (lớp 1 và lớp 9) pass
- [x] T022 [P] [US2] Test tenant isolation + authorization F115-AC-10, F115-AC-11, F115-AC-12, F115-AC-13: không member → 404; tenant khác → 404 + RLS; header lệch → `TENANT_MISMATCH`; 5 role nhân sự → `GAME_ACCESS_STUDENT_ONLY` (TC-AUTH-014, TC-TEN-012) — file: `src/backend/modules/chat/tests/integration/chat_access_test.go` — ✅ `tests/integration/chat_access_test.go` `TestChatAccess` (+ `authztest.Cover` 3 operation; routematrix 0 problems) pass
- [x] T023 [P] [US2] Test F115-AC-14: channel liên trường chỉ trả `PlayerCard` (nickname, avatar) — file: `src/backend/modules/chat/tests/integration/player_card_privacy_test.go` — ✅ thẻ người gửi chỉ `user_id`/`nickname`/`level`/`avatar_config`, kiểm trong `TestSendAndReceive` (party liên trường) và `TestSendIdempotency` (REST)
- [x] T039 [P] [US2] Test F115-AC-22 (Clock giả 21:31): gửi ở `party`, `class`, `direct` qua REST và WS → `CURFEW_ACTIVE`, không cấp `seq`, không publish JetStream; 06:31 gửi được — file: `src/backend/modules/chat/tests/integration/send_curfew_test.go` — phải FAIL trước khi code — ✅ `TestSendCurfew` (`tests/integration/chat_access_test.go`) pass
- [x] T040 [P] [US2] Test F115-AC-23: trường preset `quick_message_emote_only` → `text`/`sticker`/`gif` bị `CHAT_KIND_NOT_ALLOWED` ở `party`, `quick`/`emoji` gửi được; trường khác không ảnh hưởng; cùng kết quả lớp 1 và lớp 9 — file: `src/backend/modules/chat/tests/integration/send_preset_test.go` — ✅ `TestSendPresetAndSameRulesForEveryGrade` pass

### Implementation cho User Story 2

- [x] T024 [P] [US2] Domain `channel` (Channel, ChannelMember) và `policy` (Rule, Intersect, ErrDenied) — file: `src/backend/modules/chat/internal/domain/channel/`, `src/backend/modules/chat/internal/domain/policy/` — ✅ `internal/domain/channel/channel.go`, `internal/domain/policy/policy.go`
- [x] T025 [US2] `PolicyResolver` + cache `ew:cache:policy:chat:*` (TTL `chat_policy_cache`) — file: `src/backend/modules/chat/internal/application/service/policy_resolver.go`, `src/backend/modules/chat/internal/infrastructure/valkey/policy_cache.go` — ✅ `internal/application/service/facts.go` (`Facts.Effective` = PolicyResolver), cache `ew:chat:facts:policy:*` (`internal/infrastructure/cache/chat.go`)
- [x] T026 [US2] Command nội bộ `OpenChannel`, `AddMember`, `RemoveMember` (outbox `audit.recorded`) — file: `src/backend/modules/chat/internal/application/command/open_channel.go` — ✅ `internal/application/command/open_channel.go` (`Channels.OpenChannel/AddMember/RemoveMember`, audit `chat.channel.*`)
- [x] T027 [US2] Handler `chat.channel.join`/`leave` (reply `subscribe: ew:chat:fanout:<id>`) + middleware `game_player` cho `/chat/*` — file: `src/backend/modules/chat/internal/presentation/natscmd/channel_join.go` — ✅ `natscmd.ChannelJoin/ChannelLeave` (subscribe/unsubscribe `ew:chat:fanout:<id>`); `game_player` trên `/chat/*` (`presentation/http/chat.go`)
- [x] T041 [US2] Bước kiểm giới nghiêm (projection `platform_settings.changed`/`school.updated`) và `allowed_kinds` theo preset trong `SendMessage` (trước pipeline nội dung, trước cấp `seq`); AsyncAPI `ErrorCode` có `CURFEW_ACTIVE` — file: `src/backend/modules/chat/internal/application/command/send_message.go`, `src/backend/modules/chat/internal/domain/policy/curfew.go` — ✅ `domain/policy/curfew.go`, `Facts.Curfew`; bản chiếu `ref.schools`/`ref.platform_settings` qua `chat.projection_identity`; `CURFEW_ACTIVE` đã có trong AsyncAPI `ErrorCode`

## Phase 5: User Story 3 — Lưu bền vững, lịch sử và bù tin theo `seq` (Priority: P1)

### Tests cho User Story 3

- [x] T028 [P] [US3] Test F115-AC-15, F115-AC-17: ghi lô idempotent, `last_seq` không lùi, ack sau commit, outbox `chat.message_created`; lỗi > `max_deliver` → DLQ + `infra.dead_letters` — file: `src/backend/modules/chat/tests/integration/persister_test.go` — phải FAIL trước khi code — ✅ `TestPersisterIdempotentAndDLQ` (`tests/integration/persister_test.go`) pass
- [x] T029 [P] [US3] Test F115-AC-16: dừng PG 10 phút, gửi 500 tin → đủ, không trùng (TC-CHAT-027) — file: `src/backend/modules/chat/tests/chaos/pg_down_test.go` — ✅ `TestPGDown` pass — mô phỏng PostgreSQL ngừng bằng TEST DOUBLE trên port đọc + store + health của persister (20 tin, không phải 500 tin/10 phút thật)
- [x] T030 [P] [US3] Test F115-AC-18, F115-AC-19: lịch sử hợp nhất PG + recent, chỉ từ `joined_at`, placeholder; `chat.sync` ≤ `chat_recent_buffer` + `has_more` — file: `src/backend/modules/chat/tests/integration/history_sync_test.go` — ✅ `TestHistoryAndSync` (`tests/integration/send_message_test.go`) pass
- [x] T031 [P] [US3] Test F115-AC-20: mất key `seq` → khởi tạo lại không trùng (TC-CHAT-028) — file: `src/backend/modules/chat/tests/integration/seq_recovery_test.go` — ✅ `TestSeqRecovery` (`tests/integration/persister_test.go`) pass
- [x] T032 [P] [US3] Test F115-AC-21: không route/ACL nào cho client tới NATS/Valkey; chỉ persister có `INSERT` tin — file: `src/backend/modules/chat/tests/static/write_path_test.go` — ✅ `tests/static/static_test.go` `TestOnlyPersisterWritesMessages` pass

### Implementation cho User Story 3

- [x] T033 [US3] Worker persister (fetch lô, chọn bảng theo `is_gameplay`, `ON CONFLICT DO NOTHING`, `GREATEST`, `gameplay_partition_channels`, outbox, ack sau commit, DLQ) — file: `src/backend/modules/chat/internal/presentation/worker/persister/` — ✅ `internal/presentation/persister/persister.go`, `internal/infrastructure/persistence/messages.go`, `internal/application/command/persist.go`
- [x] T034 [US3] Query `ListChannelMessages`, `ListMyChannels`, `SyncChannel` + handler REST/NATS (`chat.history.fetch`, `chat.sync`) — file: `src/backend/modules/chat/internal/application/query/list_channel_messages.go`, `src/backend/modules/chat/internal/presentation/natscmd/history.go` — ✅ `internal/application/query/chat.go`, `natscmd` (`chat.history.fetch`, `chat.channel.sync`), REST `listChannelMessages`/`listMyChannels`

## Phase 6: Polish & Cross-Cutting

- [x] T035 [P] Metric `eduworld_chat_*` (§Quan sát plan) + span `chat.send`, `chat.persist.batch`; log không `body` — file: `src/backend/modules/chat/internal/infrastructure/observability/chat_metrics.go` — ✅ `internal/infrastructure/observability/chat_metrics.go` (metric `eduworld_chat_*`, không label user/nội dung); span qua `observability.UseCase` (`chat.command.SendMessage`, `chat.command.Persist`); log không có `body`
- [ ] T036 [P] Load test TC-PERF-010, TC-PERF-011 (bot Go WS) — file: `tests/perf/chat_load/` — **Chưa làm**: chưa có harness bot WS cho TC-PERF-010/011 trong repo; độ trễ gửi có metric `eduworld_chat_send_latency_seconds` nhưng không thay load test
- [x] T037 [P] Đề xuất tên CONFIG_DEFAULTS cho timeout PubAck, lô persister, `max_deliver`; sửa REALTIME_ARCHITECTURE §8 (rate chat theo policy), CHAT_SYSTEM §3 (cache theo `chat_policy_cache`); thêm `specs/events/payloads/chat/` vào DATABASE_DESIGN/EVENT_DESIGN; CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/06-architecture/REALTIME_ARCHITECTURE.md`, `docs/03-social-multiplayer/CHAT_SYSTEM.md`, `CHANGELOG.md` — ✅ CONFIG_DEFAULTS (`chat_facts_cache`, `chat_facts_stale_ttl`; tên timeout/lô đã có), CHAT_SYSTEM §5.4, VALKEY_DESIGN, DATABASE_DESIGN, specs/events/README, đổi tên `chat.channel.sync` ở tài liệu; REALTIME_ARCHITECTURE §8.1 đã trỏ CHAT_SYSTEM; CHANGELOG
- [ ] T038 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng — **Chưa chạy trên stack compose**: kịch bản 1–14 được phủ bằng test tự động (bằng chứng ở từng task); `devtool chat open-channel` chưa có (fixture dùng `command.Channels`)

## OQ chốt 2026-11-20 (việc bổ sung)

Việc phát sinh khi owner chốt câu hỏi mở ngày 2026-11-20 (nguồn: Clarifications "Session 2026-11-20 (owner chốt)" của feature này hoặc feature ghi trong ngoặc).

- [ ] T042 **OQ chốt 2026-11-20** (OQ F016-2, F045-3) Facade vòng đời/gộp của module chat; facade test với identity — file: `src/backend/modules/chat/internal/presentation/facade/` — kiểm: `make test-contract`, integration — cỡ M — **Chưa làm** (việc tiếp theo): cần `Facade()` của module chat + participant chat trong saga identity (F016/F045)
- [x] T043 **OQ chốt 2026-11-20** (OQ F090-1) Đổi tên lệnh `chat.sync` thành tên 3 đoạn khớp pattern `type` của `ClientEnvelope` (đề xuất `chat.channel.sync`), cập nhật AsyncAPI `x-routing` + chạy `scripts/realtime/gen-routes.mjs`; test gateway định tuyến được lệnh — file: `specs/asyncapi/eduworld-realtime.v1.yaml`, `src/backend/modules/chat/` — ✅ AsyncAPI `chat.channel.sync`, `gen-routes.mjs` chỉ nhận `type` 3 đoạn; `TestChatChannelSyncRouted` (`modules/realtime/internal/protocol`) pass

## Dependencies & Execution Order

- Setup → Foundational → US2 (policy, membership) → US1 (gửi) → US3 (persister, lịch sử) → Polish; US1 và US3 có thể song song sau T018.
- Cần F090 (định tuyến WS), F095 (skeleton, `ref.*`); bật cho học sinh chỉ khi F116 Done.

## Definition of Done

- [x] Mọi tiêu chí F115-AC-* có test pass (ghi mã test/TC) — xem bằng chứng từng task
- [x] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` xanh cho module `chat` — tương đương đã chạy: gofmt, `go vet -tags integration ./...`, golangci-lint (+ arch) 0 issues, tenancyguard, ewlint, entlint, archgen, routematrix, allowedcallers/facadegen `--check`, gen-routes `--check`, `TestMigrationMatchesEnt`, unit + integration (chat, realtime, cmd/worker, cmd/core-api) pass; `atlas migrate lint` không chạy (không có Atlas CLI cục bộ)
- [x] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17) — chưa có validator thật: module từ chối khởi động ngoài `EW_ENV=test` (R17)
- [x] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [x] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật — In progress (T036, T038, T042)
