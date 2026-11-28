# Tasks: F136 — Nickname tự gõ qua kiểm duyệt (giữ "Gợi ý"), báo cáo và đặt lại nickname

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Quy ước: test viết trước và phải FAIL trước khi code của story đó; fake/fixture gắn nhãn TEST DOUBLE (R17); danh mục từ trong test chỉ placeholder (R15); thứ tự module: nhận (chat, identity) → gọi (game) → client → portal.

## Phase 1: Setup

- [x] T001 [P] Delta contract game public: `GameProfileUpdate` `oneOf`, mã lỗi `NICKNAME_NOT_ALLOWED`/`429`/`503`, `GameProfile.nickname_changes_left`/`nickname_change_available_at`, operation `resetStudentNickname` (contracts D1) — file: `specs/openapi/modules/game.v1.yaml` — **Bằng chứng (2026-11-28):** bundle `eduworld-api.v1.yaml` → split `game.v1.yaml`: `nickname` + `not: required both` (không dùng `oneOf` để giữ additive), 422/429/503, `nickname_changes_left`/`nickname_change_available_at`, `resetStudentNickname`; oasdiff `tufin/oasdiff:v1.11.7` 0 breaking
- [x] T002 [P] Delta contract chat public: `ReportCreate.context.reported_field`, `action = nickname_reset`, `reported_nickname` trong bằng chứng case (D2) — file: `specs/openapi/modules/chat.v1.yaml` — **Bằng chứng (2026-11-28):** `reported_field`, `action = nickname_reset`, `SanctionType` + `nickname_reset`, `evidence[].reported_nickname`
- [x] T003 [P] Delta facade: `moderateDisplayText` context `nickname` + caller `game` (D3); `screenNicknamePersonalData` (D4) kèm ví dụ — file: `specs/openapi/modules/chat.internal.v1.yaml`, `specs/openapi/modules/identity.internal.v1.yaml` — **Bằng chứng (2026-11-28):** chat `moderateDisplayText` context `nickname`, `x-allowed-callers: [competition, game]`; identity `screenNicknamePersonalData` + `NicknameScreenRequest/Result` kèm ví dụ
- [x] T004 [P] Schema event (owner F116): enum `sanction_type` thêm `nickname_reset` (D5) — file: `specs/events/payloads/chat/sanction.applied.v1.schema.json` — **Bằng chứng (2026-11-28):** enum `sanction_type` + `nickname_reset`; `validate-schemas` 276 schema 0 lỗi
- [x] T005 [P] Permission `lms.student.nickname_reset` + action portal; mã lỗi `NICKNAME_NOT_ALLOWED` vào bảng mã (D6, D7) — file: `src/backend/pkg/authz/catalog.go`, `specs/portal/menu-catalog.yaml`, `docs/07-backend/API_DESIGN.md` — **Bằng chứng (2026-11-28):** catalog `PermLmsStudentNicknameReset` (school_admin qua `lms.*`, tenant_admin tường minh), action `action.school_lms.students.reset_nickname`, `NICKNAME_NOT_ALLOWED` (API_DESIGN §4.1, `check-problem-codes` 185/185)
- [x] T006 Sinh lại bundle/facade và kiểm: `make gen MOD=game`, `MOD=chat`, `MOD=identity`, `make specs-lint`, `make specs-breaking`, `node scripts/specs/check-portal-catalog.mjs` (D8) — file: `specs/openapi/eduworld-api.v1.yaml`, `src/backend/modules/*/api/` — **Bằng chứng (2026-11-28):** archgen, `go generate ./pkg/problem`, allowedcallers, facadegen, routematrix gen, infragen, sisgen, routegroups, realtime routes; check-maxlength 0 vi phạm, check-portal-catalog 0 lỗi, check-enums 0 lỗi, split-public/sync-platform-ops `--check` khớp; specs-breaking 0 breaking

## Phase 2: Foundational (chặn mọi user story)

- [x] T007 Ent schema `nickname_key` `MaxLen(32)` + migration expand (cột, backfill từ `nickname`, unique partial `(tenant_id, nickname_key)`), `make migrate-diff`, `make migrate-lint MOD=game` — file: `src/backend/modules/game/ent/schema/gameprofile.go`, `src/backend/modules/game/migrations/` — **Bằng chứng (2026-11-28):** ent `nickname_key` `MaxLen(32)` + unique partial, `nickname_changes.source_ref_id`; migration `20270204000136_f136_nickname.sql` (expand + backfill an toàn khi trùng + hàm SECURITY DEFINER), `atlas-sum.mjs`; `TestMigrationMatchesEnt` pass; `TestNicknameKeyBackfill` FAIL (chưa có file) → PASS. Không chạy `make migrate-diff`/`migrate-lint` (cần Atlas trong mạng compose dùng chung — không đụng stack): DDL kiểm bằng golden test + integration migrate thật
- [x] T008 [P] Domain thuần `profile.Nickname` (chuẩn hóa, kiểm định dạng, `Key()`) — test trước — file: `src/backend/modules/game/internal/domain/profile/nickname.go`, `src/backend/modules/game/internal/domain/profile/nickname_test.go` — **Bằng chứng (2026-11-28):** `TestParseNickname`, `TestNicknameKey`, `TestParseNicknameRules`: FAIL (stub: 30 assertion) → PASS
- [x] T009 Port `NicknameModeration`, `NicknamePersonalDataScreen`, `NicknameAttemptCounter` + fake TEST DOUBLE — file: `src/backend/modules/game/internal/application/port/nickname_screen.go`, `src/backend/modules/game/internal/testdouble/` — **Bằng chứng (2026-11-28):** port `nickname_screen.go` + metric port; TEST DOUBLE trong `change_nickname_test.go` và `tests/integration/nickname_helpers_test.go` (không tạo thư mục `testdouble/`)

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 3 — Server kiểm tra và kiểm duyệt nickname tự gõ (Priority: P1)

US3 làm trước US1/US2 vì là điều kiện an toàn để mở nhập tự do.

### Tests cho User Story 3

- [x] T010 [P] [US3] Unit F136-AC-10: bảng chuỗi mẫu (độ dài theo `nickname_min_length`/32, ký tự, chữ số theo `nickname_max_digits`, NFC, khoảng trắng) (TC-GAM-013) — file: `src/backend/modules/game/internal/domain/profile/nickname_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** xem T008 (FAIL → PASS)
- [x] T011 [P] [US3] Unit command F136-AC-11, F136-AC-12, F136-AC-14, F136-AC-16: thứ tự kiểm tra, blocked → `422` chung, lỗi/timeout facade → `503` không lưu, response không lộ dữ liệu (TC-GAM-013) — file: `src/backend/modules/game/internal/application/command/change_nickname_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** `change_nickname_test.go` 9 test: FAIL (8 test fail với stub) → PASS
- [x] T012 [P] [US3] Facade test bên nhận: chat `moderateDisplayText` `context = nickname` (PII luôn chạy, danh mục placeholder, không trả từ khớp) và identity `screenNicknamePersonalData` (họ tên, ≥ 2 từ, tên trường, học sinh không họ tên) (TC-CHAT-010, TC-SVC-005) — file: `src/backend/modules/chat/tests/contract/moderate_display_text_nickname_test.go`, `src/backend/modules/identity/tests/contract/screen_nickname_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** chat `TestModerateDisplayTextNicknameFacade` (pipeline thật + danh mục placeholder, PII khi tắt công tắc) FAIL `VALIDATION_FAILED` → PASS; identity `TestScreenNicknamePersonalDataFacade` FAIL → PASS; bên gọi game `TestChatNicknameModerationFacade`, `TestIdentityNicknameScreenFacade` PASS (file ở `tests/unit/moderation/` và `internal/presentation/facade/` theo cấu trúc thật)
- [x] T013 [P] [US3] Integration F136-AC-13, F136-AC-15, F136-AC-30: trùng không phân biệt hoa thường + đồng thời, `429` theo `nickname_attempt_rate`, log/audit không có chuỗi bị từ chối (TC-GAM-013) — file: `src/backend/modules/game/tests/integration/nickname_free_text_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** `TestNicknameKeyUniqueConcurrent`, `TestNicknameAttemptRate`, `TestNicknameAttemptCounterValkey`, `TestNicknameRejectNoLeak`: FAIL (red run 12/12 fail) → PASS

### Implementation cho User Story 3

- [x] T014 [US3] Module chat: nhận `context = nickname` trong facade `moderateDisplayText` (dùng pipeline F116, quyết định nhị phân, `hold_for_review` coi như `blocked`) — file: `src/backend/modules/chat/internal/presentation/facade/moderate_display_text.go` — **Bằng chứng (2026-11-28):** facade chat nhận `nickname` (quyết định khác allow/unchecked = blocked)
- [x] T015 [US3] Module identity: query `ScreenNicknamePersonalData` (read context, so sánh không dấu với `full_name` và tên trường) + facade — file: `src/backend/modules/identity/internal/application/query/screen_nickname_personal_data.go`, `src/backend/modules/identity/internal/presentation/facade/screen_nickname.go` — **Bằng chứng (2026-11-28):** domain `nicknamescreen` (`TestBlocked`, `TestBlockedWithoutName` FAIL → PASS), query + read model `NicknamePersonalModel`, facade `Nicknames`, nối vào `identityFacade`
- [x] T016 [US3] Module game: adapter `modcall` tới hai facade (deadline `nickname_moderation_deadline`, ánh xạ lỗi), bộ đếm Valkey — file: `src/backend/modules/game/internal/infrastructure/facade/chat_moderation.go`, `src/backend/modules/game/internal/infrastructure/facade/identity_nickname_screen.go`, `src/backend/modules/game/internal/infrastructure/cache/nickname_attempts.go` — **Bằng chứng (2026-11-28):** `infrastructure/modules/nickname.go` (modcall, fail-closed, `LateNicknameScreens`), `cache/nickname_attempts.go` (ZSET cửa sổ trượt), wiring `module.go` + `compose.go` (`WireIdentity`)
- [x] T017 [US3] Command `ChangeNickname` nhánh tự gõ (thứ tự kiểm tra, facade ngoài `UnitOfWork`, lưu + outbox), metric `eduworld_nickname_checks_total` — file: `src/backend/modules/game/internal/application/command/change_nickname.go` — **Bằng chứng (2026-11-28):** `UpdateProfile` + `typedNickname`, metric `eduworld_nickname_checks_total{result}` + histogram

**Checkpoint**: US3 kiểm độc lập được bằng API.

## Phase 4: User Story 1 — Tự gõ nickname ở màn tạo nhân vật (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [x] T018 [P] [US1] Integration F136-AC-03, F136-AC-04: `oneOf`, `initial`, `game_profile.updated` cùng transaction (TC-GAM-011) — file: `src/backend/modules/game/tests/integration/nickname_free_text_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** `TestNicknameFreeText` FAIL → PASS
- [ ] T019 [P] [US1] GdUnit4 F136-AC-01, F136-AC-02, F136-AC-05, F136-AC-06: `NicknameEditorState` (kiểm sơ bộ, chế độ chip/tự gõ, ánh xạ lỗi) và scene bước Tên (dòng nhắc, chip, lớp 1–3 không gõ) (TC-GAM-011, TC-GAM-013) — file: `src/game/tests/unit/player/test_nickname_editor_state.gd`, `src/game/tests/scene/character/test_character_creation_nickname.gd` — phải FAIL trước khi code — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)

### Implementation cho User Story 1

- [x] T020 [US1] HTTP `updateGameProfile` nhận `oneOf`, mã lỗi mới — file: `src/backend/modules/game/internal/presentation/http/profile.go` — **Bằng chứng (2026-11-28):** handler `updateGameProfile` (`nickname`, thông điệp chung theo `Accept-Language`)
- [ ] T021 [US1] Client `NicknameEditorState` + `ProfileService.update_nickname_text()` — file: `src/game/scripts/player/nickname_editor_state.gd`, `src/game/scripts/player/profile_service.gd` — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)
- [ ] T022 [US1] Bước Tên S02: ô gõ + bộ đếm + dòng nhắc + chip + "Gợi ý khác", thông báo lỗi — file: `src/game/scenes/character/character_creation_screen.gd`, `src/game/scripts/player/character_creation.gd` — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)

## Phase 5: User Story 2 — Đổi nickname trong Cài đặt (Priority: P1)

### Tests cho User Story 2

- [x] T023 [P] [US2] Integration F136-AC-08, F136-AC-09: giới hạn tháng kiểm trước facade, `reason = player_change`, `nickname_changes_left`, projection cập nhật (TC-GAM-013) — file: `src/backend/modules/game/tests/integration/nickname_change_limit_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** `TestNicknameChangeLimit`, `TestNicknameProjectionUpdated` FAIL → PASS
- [ ] T024 [P] [US2] Scene F136-AC-07: mục Nickname trong "Hồ sơ của em" (còn lượt, hết lượt với ngày đổi lại được, không đếm ngược) — file: `src/game/tests/scene/settings/test_profile_nickname.gd` — phải FAIL trước khi code — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)

### Implementation cho User Story 2

- [x] T025 [US2] `getGameProfile` trả `nickname_changes_left`, `nickname_change_available_at` — file: `src/backend/modules/game/internal/application/query/get_game_profile.go` — **Bằng chứng (2026-11-28):** `getGameProfile` trả `nickname_changes_left`, `nickname_change_available_at` (read model `NicknameAllowance`)
- [ ] T026 [US2] Mục Nickname trong Cài đặt (dùng `NicknameEditorState`) — file: `src/game/scenes/settings/profile_nickname_panel.gd` — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)

## Phase 6: User Story 4 — Báo cáo nickname (Priority: P2)

### Tests cho User Story 4

- [x] T027 [P] [US4] Chat integration F136-AC-18: `reported_field = nickname`, bản chụp từ `ref.player_profiles` (bỏ qua dữ liệu client), gộp case 24 h (TC-GAM-014) — file: `src/backend/modules/chat/tests/integration/report_nickname_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** `TestReportNicknameSnapshot` FAIL (0 evidence) → PASS
- [ ] T028 [P] [US4] Scene F136-AC-17: lựa chọn "Tên không phù hợp" trên thẻ người chơi ở Hub, bạn bè, chat, kết quả trận (≤ 2 thao tác) (TC-GAM-014) — file: `src/game/tests/scene/social/test_report_nickname.gd` — phải FAIL trước khi code — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)

### Implementation cho User Story 4

- [x] T029 [US4] `CreateReport` nhận `reported_field`, chụp nickname vào bằng chứng; `getModerationCase` trả `reported_nickname` — file: `src/backend/modules/chat/internal/application/command/report_create.go`, `src/backend/modules/chat/internal/application/query/get_moderation_case.go` — **Bằng chứng (2026-11-28):** `CreateReport` + `PlayerNickname` (ref.player_profiles), `getModerationCase` trả `reported_nickname`
- [ ] T030 [US4] Client: lý do "Tên không phù hợp" (icon + chữ) trong menu báo cáo dùng chung — file: `src/game/scenes/social/report_reasons.gd`, `src/game/scripts/chat/report_service.gd` — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)

## Phase 7: User Story 5 — Đặt lại nickname (Priority: P2)

### Tests cho User Story 5

- [x] T031 [P] [US5] Chat integration F136-AC-19: hành động case `nickname_reset` → sanction tức thời + `sanction.applied` + audit; role khác `403` (TC-GAM-014) — file: `src/backend/modules/chat/tests/integration/sanction_nickname_reset_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** `TestSanctionNicknameReset` FAIL (`unknown action`) → PASS
- [x] T032 [P] [US5] Game integration F136-AC-20, F136-AC-21: consumer giao 2 lần → xử lý 1 lần; `resetStudentNickname` scope trường, `404`/`403`, replay, audit; lần chọn lại không tính giới hạn (TC-GAM-014, TC-SVC-003, TC-TEN-001) — file: `src/backend/modules/game/tests/integration/nickname_reset_test.go` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** `TestNicknameResetConsumer`, `TestResetStudentNicknameAuthz` (authztest.Cover CrossTenant + RoleDenied) FAIL → PASS
- [ ] T033 [P] [US5] GdUnit4 F136-AC-23 + Vitest F136-AC-22: chỉ bước Tên sau đặt lại, `ui.player.unnamed`; nút portal + hành động case (TC-GAM-014) — file: `src/game/tests/scene/character/test_nickname_reset_flow.gd`, `src/cms/frontend/apps/portal/src/features/school/students/StudentNicknameReset.test.tsx`, `src/cms/frontend/apps/portal/src/features/moderation/cases/CaseNicknameAction.test.tsx` — phải FAIL trước khi code — **Tiến độ (2026-11-28):** phần portal xong: `StudentNicknameReset.test.tsx`, `CaseNicknameAction.test.tsx` FAIL (4/5) → PASS (5/5); phần Godot `test_nickname_reset_flow.gd` chờ agent Godot

### Implementation cho User Story 5

- [x] T034 [US5] Chat: loại sanction `nickname_reset` + hành động case — file: `src/backend/modules/chat/internal/domain/moderation/sanction.go`, `src/backend/modules/chat/internal/application/command/case_act.go` — **Bằng chứng (2026-11-28):** loại `nickname_reset` + hành động case; migration chat `20270204000136_f136_nickname_reset.sql` (CHECK type/period/action), expirer bỏ qua, không push realtime
- [x] T035 [US5] Game: command `ResetNickname`, consumer `game.sanction_nickname_reset` (đăng ký `module.go`, profile `events`), HTTP `resetStudentNickname` — file: `src/backend/modules/game/internal/application/command/reset_nickname.go`, `src/backend/modules/game/internal/presentation/consumer/sanction_nickname_reset.go`, `src/backend/modules/game/internal/presentation/http/profile.go`, `src/backend/modules/game/module.go` — **Bằng chứng (2026-11-28):** `ResetStudentNickname`, `ResetNicknameBySanction`, consumer `game.sanction_nickname_reset` (worker events), route `resetStudentNickname` (portal staff, Idempotency-Key)
- [ ] T036 [US5] Client: luồng chỉ bước Tên, hiển thị `ui.player.unnamed`; portal: nút "Đặt lại nickname" (F052) và hành động case (F122) — file: `src/game/scripts/player/character_creation.gd`, `src/cms/frontend/apps/portal/src/features/school/students/StudentNicknameReset.tsx`, `src/cms/frontend/apps/portal/src/features/moderation/cases/CaseNicknameAction.tsx` — **Tiến độ (2026-11-28):** phần portal xong (`StudentNicknameReset.tsx`, `useStudentNickname.ts`, `CaseNicknameAction.tsx`); phần Godot (`character_creation.gd`, `ui.player.unnamed`) chờ agent Godot

## Phase 8: User Story 6 — Chất lượng giao diện (Priority: P2)

- [ ] T037 [P] [US6] Test F136-AC-24, F136-AC-25, F136-AC-26, F136-AC-27: pseudo-locale màn mới, lint theme, ma trận khối (bước Tên S02 các trạng thái, mục Nickname), quét hiển thị nickname văn bản thuần (TC-I18N-006, TC-UI-003, TC-CHAT-010) — file: `src/game/tests/scene/test_grade_band_matrix.gd`, `src/game/tests/scene/test_pseudo_locale.gd`, `src/game/tests/unit/test_nickname_plain_text.gd` — phải FAIL trước khi code — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)
- [ ] T038 [US6] Khóa dịch vi/en (`ui.avatar.nickname.*`, `ui.profile.nickname.*`, `ui.player.unnamed`, `ui.report.reason.nickname`, `ui.error.NICKNAME_NOT_ALLOWED`), bố cục `compact` (ô gõ trên bàn phím ảo), sửa tới khi ma trận xanh — file: `src/game/locale/vi.po`, `src/game/locale/en.po`, `src/game/scenes/character/character_creation_screen.gd`, `src/game/scenes/settings/profile_nickname_panel.gd` — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)

## Phase 9: User Story 7 — Riêng tư, tenant, phân quyền, ẩn danh hóa (Priority: P3)

- [x] T039 [P] [US7] Integration F136-AC-28, F136-AC-29, F136-AC-31, F136-AC-32: tenant isolation, `game_player` route matrix, hàm `anonymize_nickname_changes` (SECURITY DEFINER, audit), `check-maxlength` (TC-TEN-001, TC-TEN-011, TC-AUTH-014, TC-GAM-014) — file: `src/backend/modules/game/tests/integration/nickname_privacy_test.go`, `src/backend/modules/game/migrations/` — phải FAIL trước khi code — **Bằng chứng (2026-11-28):** `TestNicknameTenantIsolation`, `TestAnonymizeNicknameHistory`, `TestNicknameKeyBackfill` FAIL → PASS; route matrix `routematrix check`; `check-maxlength` 0 vi phạm
- [x] T040 [US7] `anonymizeUserData` gọi `game.anonymize_nickname_changes` + test kiến trúc facade (`make arch-check`) — file: `src/backend/modules/game/internal/application/command/student_lifecycle.go` — **Bằng chứng (2026-11-28):** `StudentLifecycle.Anonymize` gọi `AnonymizeHistory` + audit `game_profile.nickname_history_anonymized`

## Phase 10: Polish & Cross-Cutting

- [ ] T041 Chụp ảnh trước/sau theo F136-AC-26 (`tools/capture_screens.gd`), xem từng ảnh, ghi bảng màn × khối vào quickstart §5 — file: `specs/136-free-text-nickname/screenshots/`, `specs/136-free-text-nickname/quickstart.md` — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)
- [ ] T042 Chạy quickstart §2 với backend thật (hai client + portal) và lệnh §3; ghi số pass/fail — file: `specs/136-free-text-nickname/quickstart.md` — **Chờ agent Godot** (backend/contract đã xong; fixture contract Godot phải sinh lại bằng `node src/game/tools/gen_fixtures.mjs`)
- [ ] T043 Cập nhật tài liệu sở hữu nếu triển khai phát sinh quyết định mới (CHARACTER_SYSTEM §5.3, MODERATION §2/§5/§7, STUDENT_GAME_UX_CORE S02, STUDENT_GAME_UX_SOCIAL §10, DATABASE_DESIGN `game_profiles`/`sanctions`, EVENT_DESIGN `sanction.applied`), CONFIG_DEFAULTS, CHANGELOG — file: `docs/01-game-design/CHARACTER_SYSTEM.md`, `docs/07-backend/DATABASE_DESIGN.md`, `CHANGELOG.md` — **Tiến độ (2026-11-28):** CHARACTER_SYSTEM §5.3, DATABASE_DESIGN (`game_profiles`, `nickname_changes`, `sanctions`), EVENT_DESIGN (`sanction.applied` consumer game), ADMIN_PORTAL, API_DESIGN, CHANGELOG cập nhật; STUDENT_GAME_UX_* chờ phần Godot
- [ ] T044 Cập nhật dòng F136 trong CODE_PLAN (trạng thái, bằng chứng) và ghi bằng chứng vào tasks.md — file: `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`, `specs/136-free-text-nickname/tasks.md` — **Tiến độ (2026-11-28):** CODE_PLAN F136 = In progress — Godot pending; bằng chứng ghi trong file này

## Dependencies & Execution Order

- F070, F116, F121 Done (F052, F122 cho phần portal). Setup (T001–T006) → Foundational (T007–T009) → US3 (an toàn, bắt buộc trước khi mở nhập tự do) → US1 (MVP) → US2 → US4, US5 (độc lập nhau; US5 cần T034 của chat trước T035 của game) → US6 (chạy lại sau mỗi story có màn mới) → US7 → Polish.
- Trong mỗi module: module nhận (chat, identity) trước module gọi (game). Trong mỗi story: test (FAIL) → code → test pass.

## Definition of Done

> **Bằng chứng gate backend + portal (2026-11-29, sau rebase lên origin/main):** gofmt sạch; `go build ./...`, `go vet -tags integration` (game, chat, identity, cmd) OK; golangci-lint main + arch `--build-tags integration` 0 issues; archgen 742 package 0 vi phạm, routematrix 607 operation 0 lỗi, allowedcallers/facadegen/infragen/sisgen/routegroups `--check`, ewlint, entlint, tenancyguard sạch; unit game/chat/identity/pkg/authz/cmd 99 package ok; integration toàn module (game 3978 s, chat 3514 s, identity 702 s) — 7 test lỗi do máy quá tải/đồng hồ (`TestProfileLifecycleFacade`, `TestNicknameOptionStoreValkey`, `TestPersisterIdempotentAndDLQ`, `TestPresenceFanout`, `TestQuickMessages` deadline; `TestSuggestNicknames` do fixture F136 — đã sửa) chạy lại đều PASS; `TestDailyPlayLimitAtGameLogin` (identity) lỗi theo giờ thực sau 00:00 ICT, không liên quan F136; sau rebase 21 test F136 + safety chat PASS. Specs: check-features/cross-features/portal-catalog/problem-codes/maxlength/enums/validate-schemas/client-authoritative 0 lỗi, oasdiff 0 breaking, docs check-links 0 hỏng. Portal: 2 file test mới FAIL 4/5 → PASS 5/5; `Cases.test.tsx`, `studentsByGrade.test.tsx` + 2 file mới 24/24 PASS; typecheck, ESLint + stylelint, gen:check OK; chưa chạy hết toàn bộ 166 file portal (máy chậm). Chưa chạy: Godot, quickstart với stack thật (T042), `make migrate-lint` (Atlas trong mạng compose dùng chung).

- [ ] Mọi tiêu chí F136-AC-01…F136-AC-32 có test pass (bảng AC → test ở plan.md) và ghi mã test/TC
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (game, chat, identity), `make test-contract`, `make arch-check`, `make specs-lint`, `make specs-breaking`, `make godot-test`, `make godot-export-check`, `make web-lint`, `make web-test` xanh
- [ ] Ảnh ma trận khối đã chụp và xem, bảng ở quickstart §5 (F136-AC-26)
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17); danh mục test chỉ placeholder (R15)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
