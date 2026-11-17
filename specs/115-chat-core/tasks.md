# Tasks: F115 — module chat lõi: channel, policy deny-by-default, JetStream, `chat-persister`, gửi/nhận

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Cấu hình chat (`EW_NATS_URLS`, timeout PubAck 2 s, lô persister, `max_deliver`, `chat_recent_buffer`, `chat_rate_per_user`) qua `pkg/config` — file: `modules/chat/internal/config/chat.go`
- [ ] T002 [P] Bootstrap stream `CHAT_MESSAGES` (subjects `ew.chat.*.msg.*`, `ew.chat.*.evt.*`, `ew.chat.*.cursor.*`, file storage, `max_age` 72 h, `duplicate_window` 2') và `CHAT_DLQ`, consumer durable `persister` — file: `modules/chat/internal/infrastructure/natsstream/stream_bootstrap.go`
- [ ] T003 [P] Wiring profile `worker-chat` (`cmd/worker --profile=chat`) chạy consumer persister; kiểm `core-api`/`worker` khởi động thất bại khi thiếu `ContentValidator` thật ngoài `EW_ENV=test` — file: `modules/chat/module.go`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: payload `chat-envelope.v1`, `chat.message_created.v1` — file: `specs/events/payloads/chat/` — kiểm: `make specs-lint`
- [ ] T005 [P] Delta OpenAPI `sendChannelMessage` (`404`, `409`, `428`, `503`), `403` cho `listMyChannels`/`listChannelMessages`, `quick_message_id` string 64; AsyncAPI `client_msg_id` → `client_message_id` (RB-508) — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`
- [ ] T006 ent schema `Channel`, `ChannelMember`, `Message`, `GameplayMessage`, `GameplayPartitionChannel`, `ChatPolicy` (mọi string `MaxLen` theo DATABASE_DESIGN, mixin tenant + soft-delete) — file: `modules/chat/ent/schema/` — kiểm: `make migrate-diff MOD=chat name=f115_chat_core`, `make migrate-lint MOD=chat`
- [ ] T007 Migration: partition tháng `chat.messages`, partition ngày `chat.gameplay_messages`, `chat.f_unaccent`, index/GIN, RLS mọi bảng T, `REVOKE DELETE, TRUNCATE` khỏi `chat_app`/`chat_platform` — file: `modules/chat/migrations/`
- [ ] T008 [P] Test RLS + quyền DB: tenant A không đọc B, không set `app.tenant_id` → 0 dòng, `DELETE` bị từ chối (TC-TEN-005, TC-TEN-016) — file: `modules/chat/tests/integration/rls_chat_test.go`
- [ ] T009 [P] Seed platform `chat_policies` 8 loại channel, validate theo `chat-policy.schema.json` v5 — file: `modules/chat/seed/chat_policies.platform.json`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Gửi và nhận tin realtime, bền vững trước khi "Sent" (Priority: P1) 🎯 MVP

**Goal**: Đường gửi duy nhất: kiểm tra → pipeline → `seq` → JetStream ack → acked → fan-out.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T010 [P] [US1] Test F115-AC-01, F115-AC-02: acked chỉ sau PubAck; created tới member theo `seq`; held chỉ tới người gửi — file: `modules/chat/tests/integration/send_message_test.go` — phải FAIL trước khi code
- [ ] T011 [P] [US1] Test F115-AC-03: NATS dừng/timeout → `CHAT_TEMPORARILY_UNAVAILABLE`, 0 fan-out, 0 dòng PG — file: `modules/chat/tests/chaos/nats_down_test.go`
- [ ] T012 [P] [US1] Test F115-AC-04, F115-AC-05: `client_message_id`/`Idempotency-Key` trùng → cùng kết quả; REST thiếu key → 428; REST và WS cùng use case — file: `modules/chat/tests/integration/send_idempotency_test.go`
- [ ] T013 [P] [US1] Test F115-AC-06: mọi `kind` đi qua `ContentValidator` (spy có nhãn TEST DOUBLE); khởi động `core-api`/`worker` ngoài test thiếu validator thật → lỗi — file: `modules/chat/tests/unit/send_pipeline_required_test.go`
- [ ] T014 [P] [US1] Test F115-AC-07: vượt độ dài/rate bị từ chối trước khi cấp `seq` — file: `modules/chat/tests/unit/technical_limits_test.go`

### Implementation cho User Story 1

- [ ] T015 [P] [US1] Domain `message` (Kind, ModerationState, Envelope) — file: `modules/chat/internal/domain/message/`
- [ ] T016 [P] [US1] Valkey: `seq_next.lua` + allocator, recent buffer, fan-out publisher, rate limiter ZSET, cmid store — file: `modules/chat/internal/infrastructure/valkey/`
- [ ] T017 [P] [US1] JetStream publisher (`Nats-Msg-Id`, chờ PubAck 2 s) — file: `modules/chat/internal/infrastructure/natsstream/jetstream_publisher.go`
- [ ] T018 [US1] Command `SendMessage` (write context; port `ContentValidator`, `SeqAllocator`, `ChatStream`, `Fanout`) — file: `modules/chat/internal/application/command/send_message.go`
- [ ] T019 [US1] Handler NATS `ew.cmd.chat.message.send` (chỉ nhận từ user NATS `realtime`, đọc header `Ew-*`) và REST `sendChannelMessage` — file: `modules/chat/internal/presentation/natscmd/message_send.go`, `modules/chat/internal/presentation/http/public/messages.go`

**Checkpoint**: US1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Quyền vào channel theo quan hệ và policy, mặc định từ chối (Priority: P1)

### Tests cho User Story 2

- [ ] T020 [P] [US2] Test F115-AC-08: giao 4 cấp (min số, giao `allowed_kinds`, AND `enabled`); thiếu rule → deny — file: `modules/chat/tests/unit/policy_intersect_test.go` — phải FAIL trước khi code
- [ ] T021 [P] [US2] Test tĩnh F115-AC-09: package `domain/policy`, `application/command` không tham chiếu `grade_level`/`grade_band`; cùng tin lớp 2 và lớp 8 cùng quyết định — file: `modules/chat/tests/static/no_grade_condition_test.go`
- [ ] T022 [P] [US2] Test tenant isolation + authorization F115-AC-10, F115-AC-11, F115-AC-12, F115-AC-13: không member → 404; tenant khác → 404 + RLS; header lệch → `TENANT_MISMATCH`; 5 role nhân sự → `GAME_ACCESS_STUDENT_ONLY` (TC-AUTH-014, TC-TEN-012) — file: `modules/chat/tests/integration/chat_access_test.go`
- [ ] T023 [P] [US2] Test F115-AC-14: channel liên trường chỉ trả `PlayerCard` (nickname, avatar) — file: `modules/chat/tests/integration/player_card_privacy_test.go`
- [ ] T039 [P] [US2] Test F115-AC-22 (Clock giả 21:31): gửi ở `party`, `class`, `direct` qua REST và WS → `CURFEW_ACTIVE`, không cấp `seq`, không publish JetStream; 06:31 gửi được — file: `modules/chat/tests/integration/send_curfew_test.go` — phải FAIL trước khi code
- [ ] T040 [P] [US2] Test F115-AC-23: trường preset `quick_message_emote_only` → `text`/`sticker`/`gif` bị `CHAT_KIND_NOT_ALLOWED` ở `party`, `quick`/`emoji` gửi được; trường khác không ảnh hưởng; cùng kết quả lớp 1 và lớp 9 — file: `modules/chat/tests/integration/send_preset_test.go`

### Implementation cho User Story 2

- [ ] T024 [P] [US2] Domain `channel` (Channel, ChannelMember) và `policy` (Rule, Intersect, ErrDenied) — file: `modules/chat/internal/domain/channel/`, `modules/chat/internal/domain/policy/`
- [ ] T025 [US2] `PolicyResolver` + cache `ew:cache:policy:chat:*` (TTL `chat_policy_cache`) — file: `modules/chat/internal/application/service/policy_resolver.go`, `modules/chat/internal/infrastructure/valkey/policy_cache.go`
- [ ] T026 [US2] Command nội bộ `OpenChannel`, `AddMember`, `RemoveMember` (outbox `audit.recorded`) — file: `modules/chat/internal/application/command/open_channel.go`
- [ ] T027 [US2] Handler `chat.channel.join`/`leave` (reply `subscribe: ew:chat:fanout:<id>`) + middleware `game_player` cho `/chat/*` — file: `modules/chat/internal/presentation/natscmd/channel_join.go`
- [ ] T041 [US2] Bước kiểm giới nghiêm (projection `platform_settings.changed`/`school.updated`) và `allowed_kinds` theo preset trong `SendMessage` (trước pipeline nội dung, trước cấp `seq`); AsyncAPI `ErrorCode` có `CURFEW_ACTIVE` — file: `modules/chat/internal/application/command/send_message.go`, `modules/chat/internal/domain/policy/curfew.go`

## Phase 5: User Story 3 — Lưu bền vững, lịch sử và bù tin theo `seq` (Priority: P1)

### Tests cho User Story 3

- [ ] T028 [P] [US3] Test F115-AC-15, F115-AC-17: ghi lô idempotent, `last_seq` không lùi, ack sau commit, outbox `chat.message_created`; lỗi > `max_deliver` → DLQ + `infra.dead_letters` — file: `modules/chat/tests/integration/persister_test.go` — phải FAIL trước khi code
- [ ] T029 [P] [US3] Test F115-AC-16: dừng PG 10 phút, gửi 500 tin → đủ, không trùng (TC-CHAT-027) — file: `modules/chat/tests/chaos/pg_down_test.go`
- [ ] T030 [P] [US3] Test F115-AC-18, F115-AC-19: lịch sử hợp nhất PG + recent, chỉ từ `joined_at`, placeholder; `chat.sync` ≤ `chat_recent_buffer` + `has_more` — file: `modules/chat/tests/integration/history_sync_test.go`
- [ ] T031 [P] [US3] Test F115-AC-20: mất key `seq` → khởi tạo lại không trùng (TC-CHAT-028) — file: `modules/chat/tests/integration/seq_recovery_test.go`
- [ ] T032 [P] [US3] Test F115-AC-21: không route/ACL nào cho client tới NATS/Valkey; chỉ persister có `INSERT` tin — file: `modules/chat/tests/static/write_path_test.go`

### Implementation cho User Story 3

- [ ] T033 [US3] Worker persister (fetch lô, chọn bảng theo `is_gameplay`, `ON CONFLICT DO NOTHING`, `GREATEST`, `gameplay_partition_channels`, outbox, ack sau commit, DLQ) — file: `modules/chat/internal/presentation/worker/persister/`
- [ ] T034 [US3] Query `ListChannelMessages`, `ListMyChannels`, `SyncChannel` + handler REST/NATS (`chat.history.fetch`, `chat.sync`) — file: `modules/chat/internal/application/query/list_channel_messages.go`, `modules/chat/internal/presentation/natscmd/history.go`

## Phase 6: Polish & Cross-Cutting

- [ ] T035 [P] Metric `eduworld_chat_*` (§Quan sát plan) + span `chat.send`, `chat.persist.batch`; log không `body` — file: `modules/chat/internal/infrastructure/observability/chat_metrics.go`
- [ ] T036 [P] Load test TC-PERF-010, TC-PERF-011 (bot Go WS) — file: `tests/perf/chat_load/`
- [ ] T037 [P] Đề xuất tên CONFIG_DEFAULTS cho timeout PubAck, lô persister, `max_deliver`; sửa REALTIME_ARCHITECTURE §8 (rate chat theo policy), CHAT_SYSTEM §3 (cache theo `chat_policy_cache`); thêm `specs/events/payloads/chat/` vào DATABASE_DESIGN/EVENT_DESIGN; CHANGELOG — file: `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/06-architecture/REALTIME_ARCHITECTURE.md`, `docs/03-social-multiplayer/CHAT_SYSTEM.md`, `CHANGELOG.md`
- [ ] T038 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US2 (policy, membership) → US1 (gửi) → US3 (persister, lịch sử) → Polish; US1 và US3 có thể song song sau T018.
- Cần F090 (định tuyến WS), F095 (skeleton, `ref.*`); bật cho học sinh chỉ khi F116 Done.

## Definition of Done

- [ ] Mọi tiêu chí F115-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` xanh cho module `chat`
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
