# Tasks: F116 — Pipeline kiểm tra tin nhắn, danh mục từ nhạy cảm, công tắc kiểm duyệt

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

## Phase 1: Setup

- [ ] T001 Đăng ký permission platform-only `chat.sensitive_terms.read`, `chat.sensitive_terms.write`, `chat.policy.toggle`, `chat.tenant_terms.write`, `chat.tenant_category.toggle` và kiểm khởi động (không nằm trong role tenant trừ `chat.sensitive_terms.read` cho xem category) — file: `src/backend/pkg/authz/catalog.go`
- [ ] T002 [P] Bộ fixture placeholder (term `__test_*__`, câu mẫu có nhãn) và bộ hồi quy CI; test tĩnh chặn từ thật trong seed/test — file: `src/backend/modules/chat/tests/fixtures/moderation/`, `src/backend/modules/chat/tests/static/no_real_terms_test.go`
- [ ] T003 [P] Cấu hình normalize có version (teencode/leetspeak/homoglyph map, không chứa từ nhạy cảm) — file: `src/backend/modules/chat/seed/normalization/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T004 Delta contract: `Problem.category_code`, `MessageCreate.params`, AsyncAPI `chat.message.send.params`, `system.command.failed.details.category_code`, `sendChannelMessage` `403 CHAT_MUTED` — file: `specs/openapi/eduworld-api.v1.yaml`, `specs/asyncapi/eduworld-realtime.v1.yaml` — kiểm: `make specs-lint`, `make specs-breaking`, `make gen MOD=chat`
- [ ] T005 [P] Event payload `sensitive_term.created/updated/deleted`, `sensitive_catalog.published`, `chat_policy.moderation_switch_changed`, `chat_policy.tenant_category_toggled`, `chat.message_held`, `sanction.applied` — file: `specs/events/payloads/chat/` — kiểm: `make specs-lint`
- [ ] T006 ent schema 10 bảng ở data-model (MaxLen theo DATABASE_DESIGN §1.4) — file: `src/backend/modules/chat/ent/schema/` — kiểm: `make migrate-diff MOD=chat name=f116_moderation`, `make migrate-lint MOD=chat`
- [ ] T007 Migration: partition tháng `moderation_decisions`, trigger chặn UPDATE/DELETE `sensitive_catalog_versions`, RLS bảng T/H, CHECK lý do khi tắt bước — file: `src/backend/modules/chat/migrations/`
- [ ] T008 [P] Seed platform: category mặc định (severity/action theo MODERATION §11.2), công tắc platform bật tất cả, quick messages song ngữ — file: `src/backend/modules/chat/seed/moderation/`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Mọi tin nhắn được kiểm tra trước khi phát (Priority: P1) 🎯 MVP

**Goal**: Hiện thực `ContentValidator` thật cho F115.

**Independent Test**: Như spec.md US1.

### Tests cho User Story 1

- [ ] T009 [P] [US1] Unit bảng normalize + engine F116-AC-01, F116-AC-02, F116-AC-03: biến thể có/không dấu, teencode, leetspeak, chen ký tự, lặp, homoglyph; allowlist; severity→action; strictness nâng một mức; mask chỉ đoạn khớp — file: `src/backend/modules/chat/tests/unit/moderation/engine_test.go` — phải FAIL trước khi code
- [ ] T010 [P] [US1] Unit PII F116-AC-04 (bảng MODERATION §3.3, AC-MOD-001, AC-MOD-003) — file: `src/backend/modules/chat/tests/unit/moderation/pii_test.go`
- [ ] T011 [P] [US1] Integration F116-AC-05, F116-AC-06: block không `seq`/không dòng, phản hồi chỉ `category_code`; held lưu `held`, không fan-out, `chat.message_held`, `block_and_alert` → mute hệ thống — file: `src/backend/modules/chat/tests/integration/moderation_decision_test.go`
- [ ] T012 [P] [US1] Integration F116-AC-07, F116-AC-08: spam lặp/đa channel → block; 3 block/10 phút → `mute` + audit; mute → `CHAT_MUTED` kể cả `enabled_all` tắt — file: `src/backend/modules/chat/tests/integration/spam_sanction_test.go`
- [ ] T013 [P] [US1] Integration F116-AC-09: decision log đủ trường cho mọi decision (kể cả `block`), không cột nội dung — file: `src/backend/modules/chat/tests/integration/decision_log_test.go`
- [ ] T014 [P] [US1] Benchmark F116-AC-10: p95 tin 200 ký tự ≤ `moderation_pipeline_latency` — file: `src/backend/modules/chat/tests/bench/pipeline_bench_test.go`
- [ ] T015 [P] [US1] Test F116-AC-11: cùng tin, lớp 2 và lớp 8 → cùng quyết định; chữ ký `Validate` không có tham số khối — file: `src/backend/modules/chat/tests/unit/moderation/grade_parity_test.go`

### Implementation cho User Story 1

- [ ] T016 [P] [US1] Domain normalize, catalog matcher (Aho–Corasick + RE2 có giới hạn), PII, spam rules, bullying rules, context rules, `Decide` — file: `src/backend/modules/chat/internal/domain/moderation/`
- [ ] T017 [US1] `pipeline.Validator` (TechnicalGate gồm sanction, switches, các bước, decision metadata) nối vào port F115 — file: `src/backend/modules/chat/internal/application/pipeline/validator.go`
- [ ] T018 [US1] Spam window Valkey + command `ApplySystemMute` (outbox `sanction.applied`, cache sanction) + `DecisionLogWriter` — file: `src/backend/modules/chat/internal/infrastructure/valkey/spam_window.go`, `src/backend/modules/chat/internal/application/command/apply_system_mute.go`

**Checkpoint**: US1 chạy và kiểm độc lập được; F115 có thể bật cho học sinh.

## Phase 4: User Story 2 — Danh mục từ nhạy cảm quản lý được, có hiệu lực không cần deploy (Priority: P1)

### Tests cho User Story 2

- [ ] T019 [P] [US2] Integration F116-AC-12, F116-AC-13: CRUD nháp, RE2 sai → `SENSITIVE_TERM_INVALID`, xóa mềm, audit/event không nội dung; test chuỗi không ghi tin — file: `src/backend/modules/chat/tests/integration/catalog_admin_test.go` — phải FAIL trước khi code
- [ ] T020 [P] [US2] Integration F116-AC-14, F116-AC-15: hồi quy fail → 422 không version; đạt → version + checksum; ≤ 60 s mọi instance (2 instance api testcontainers), tin đang xử lý dùng bản cũ (TC-CHAT-012) — file: `src/backend/modules/chat/tests/integration/catalog_publish_reload_test.go`
- [ ] T021 [P] [US2] Test tenant isolation + authorization F116-AC-16, F116-AC-17, F116-AC-18, F116-AC-19: term tenant chỉ hiệu lực tenant đích; sửa term platform qua tenant-terms → `SENSITIVE_TERM_INVALID`; toggle không cho phép → `TENANT_TOGGLE_NOT_ALLOWED`; mọi endpoint ghi × {tenant_admin, school_admin, academic_manager, token game} → 403; list tenant không có term; danh mục không có trong response/log nào phía game — file: `src/backend/modules/chat/tests/integration/catalog_authz_test.go`

### Implementation cho User Story 2

- [ ] T022 [US2] Command/query danh mục platform + allowlist + publish (regression runner) — file: `src/backend/modules/chat/internal/application/command/catalog_*.go`, `src/backend/modules/chat/internal/application/command/publish_catalog.go`, `src/backend/modules/chat/internal/infrastructure/regression/`
- [ ] T023 [US2] Command tenant terms + toggle category; query `ListTenantSensitiveCategories` — file: `src/backend/modules/chat/internal/application/command/tenant_terms.go`, `src/backend/modules/chat/internal/application/command/toggle_tenant_category.go`
- [ ] T024 [US2] Compiled cache + pub/sub `ew:chat:sensitive:version` + reconcile 60 s + `atomic.Pointer` swap — file: `src/backend/modules/chat/internal/infrastructure/catalogstore/`
- [ ] T025 [US2] Handler `/admin/chat/sensitive-*`, `/admin/chat/tenant-*`, `/tenant/chat/categories` (middleware portal + MFA + permission) — file: `src/backend/modules/chat/internal/presentation/http/admin/`

## Phase 5: User Story 3 — Công tắc kiểm duyệt có kiểm soát (Priority: P1)

### Tests cho User Story 3

- [ ] T026 [P] [US3] Integration F116-AC-20, F116-AC-21: set switches lưu + history + audit + event, ≤ 60 s; thiếu reason → 422; không quyền → 403 — file: `src/backend/modules/chat/tests/integration/switches_test.go` — phải FAIL trước khi code
- [ ] T027 [P] [US3] Unit/integration F116-AC-22, F116-AC-23, F116-AC-24: công thức AND, mất cache đọc PG không mặc định tắt; tắt một bước chỉ bỏ bước đó, tenant khác không đổi; `enabled_all` tắt vẫn giữ giới hạn kỹ thuật — file: `src/backend/modules/chat/tests/integration/switch_effect_test.go`
- [ ] T028 [P] [US3] Integration F116-AC-25: expirer bật lại đúng hạn, `auto_expire`, metric `policy_disabled` — file: `src/backend/modules/chat/tests/integration/switch_expirer_test.go`
- [ ] T029 [P] [US3] Test tĩnh F116-AC-26: không biến môi trường/cờ code nào tắt pipeline — file: `src/backend/modules/chat/tests/static/no_switch_flag_test.go`
- [ ] T038 [P] [US3] Test F116-AC-30: yêu cầu tắt `pii`/grooming ở platform và tenant → `422 MODERATION_SWITCH_STEP_NOT_ALLOWED`; `enabled_all` tắt vẫn chặn SĐT/mạng xã hội và chạy luật grooming; bản ghi cũ `steps.pii = false` bị bỏ qua (AC-MOD-030) — file: `src/backend/modules/chat/tests/integration/switch_always_on_steps_test.go` — phải FAIL trước khi code
- [ ] T039 [P] [US3] Test F116-AC-31, F116-AC-32: thiếu/quá `expires_at` → `MODERATION_SWITCH_EXPIRY_REQUIRED`; yêu cầu tắt → `pending_approval`, chưa hiệu lực; tự duyệt → `403 MODERATION_SWITCH_SELF_APPROVAL`; `sa.admin2` duyệt → hiệu lực ≤ 60 s, audit hai người; quá 24 giờ chưa duyệt → hết hạn; bật lại không cần duyệt — file: `src/backend/modules/chat/tests/integration/switch_approval_test.go`

### Implementation cho User Story 3

- [ ] T030 [US3] Command `SetModerationSwitches`, query list/history, cache `ew:chat:switches:<tenant_id>` + pub/sub `ew:chat:switches:changed` — file: `src/backend/modules/chat/internal/application/command/set_switches.go`
- [ ] T031 [US3] Job `moderation-switch-expirer` (lock `ew:lock:job:moderation-switch-expirer`) — file: `src/backend/modules/chat/internal/presentation/job/moderation_switch_expirer.go`
- [ ] T040 [US3] Bỏ `pii` khỏi `steps` (schema + API, migration expand/contract đưa `pii` về bật), luật grooming không phụ thuộc công tắc; bảng `chat.moderation_switch_requests` (`pending_approval/approved/rejected/expired`, `proposed_by`, `approved_by`, `expires_at`) + command `RequestModerationSwitchChange`, `ApproveModerationSwitchChange`; job hết hạn yêu cầu chờ duyệt — file: `src/backend/modules/chat/internal/application/command/moderation_switch_request.go`, `src/backend/modules/chat/ent/schema/moderation_switch_request.go`
- [ ] T041 [US3] Contract: `ModerationSwitches.steps` bỏ `pii`, `expires_at` bắt buộc khi tắt, response `202 pending_approval`, operation `approveModerationSwitchChange` (`POST /admin/chat/moderation-switches/requests/{request_id}:approve`, gateway nội bộ, `chat.policy.toggle` + MFA), mã `MODERATION_SWITCH_STEP_NOT_ALLOWED`, `MODERATION_SWITCH_EXPIRY_REQUIRED`, `MODERATION_SWITCH_SELF_APPROVAL` — file: `specs/openapi/eduworld-api.v1.yaml` — kiểm: `make specs-lint`, `node scripts/openapi/split-public.mjs`

## Phase 6: User Story 4 — Quick message song ngữ, không dịch máy chat (Priority: P2)

### Tests cho User Story 4

- [ ] T032 [P] [US4] Integration F116-AC-27, F116-AC-28: list đủ `vi`/`en`; `quick_message_id` sai → `VALIDATION_FAILED`; phát mã + `params`; `params` có PII bị chặn (TC-I18N-008) — file: `src/backend/modules/chat/tests/integration/quick_message_test.go` — phải FAIL trước khi code
- [ ] T033 [P] [US4] Integration F116-AC-29: tin tiếng Anh với giao diện `vi` bị xử lý theo danh mục `en`; không có bước dịch (TC-I18N-009) — file: `src/backend/modules/chat/tests/integration/bilingual_catalog_test.go`

### Implementation cho User Story 4

- [ ] T034 [US4] Query `ListQuickMessages` + handler; kiểm `kind = quick` trong `SendMessage` — file: `src/backend/modules/chat/internal/application/query/list_quick_messages.go`, `src/backend/modules/chat/internal/presentation/http/public/quick_messages.go`

## Phase 7: Polish & Cross-Cutting

- [ ] T035 [P] Metric/alert `ChatModerationDisabled`, `eduworld_chat_filter_latency_seconds`, `eduworld_chat_sensitive_*`, `eduworld_chat_re2_timeouts_total` — file: `src/backend/modules/chat/internal/infrastructure/observability/moderation_metrics.go`
- [ ] T036 [P] Cập nhật tài liệu: DATABASE_DESIGN (DDL `chat.sanctions`, khóa `quick_messages`), VALKEY_DESIGN (`ew:chat:sanction:<user_id>`), CONFIG_DEFAULTS (ngưỡng spam, reload 60 s, timeout RE2), PRD FR-MOD-013/AC-MOD-022 + MODERATION §11.3 (toggle category do Safety Team), API_DESIGN §9.9 (`chat.tenant_terms.write`), CHANGELOG — file: `docs/07-backend/DATABASE_DESIGN.md`, `docs/07-backend/VALKEY_DESIGN.md`, `docs/00-overview/CONFIG_DEFAULTS.md`, `docs/03-social-multiplayer/MODERATION.md`, `docs/07-backend/API_DESIGN.md`, `CHANGELOG.md`
- [ ] T037 Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → US1 (mở khóa F115 cho học sinh) → US2 và US3 song song → US4 → Polish.
- F121 phụ thuộc `chat.message_held`, `chat.sanctions`, decision log của feature này.

## Definition of Done

- [ ] Mọi tiêu chí F116-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration`, `make migrate-lint` xanh; benchmark pipeline đạt
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
