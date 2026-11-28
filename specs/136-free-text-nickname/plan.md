# Implementation Plan: F136 — Nickname tự gõ qua kiểm duyệt (giữ "Gợi ý"), báo cáo và đặt lại nickname

**Branch**: `136-free-text-nickname` · **Date**: 2026-11-28 · **Spec**: [spec.md](spec.md)

## Summary

Module game mở rộng use case `ChangeNickname` (F070) nhận thêm `nickname` tự gõ: domain `profile.Nickname` (thuần) chuẩn hóa và kiểm định dạng, sinh `nickname_key`; application gọi hai port mới — `NicknameModeration` (adapter facade chat `moderateDisplayText`, `context = nickname`) và `NicknamePersonalDataScreen` (adapter facade identity `screenNicknamePersonalData`) — với deadline `nickname_moderation_deadline`, fail-closed; kiểm trùng bằng unique partial `(tenant_id, nickname_key)`; giới hạn lần thử bằng bộ đếm Valkey. Đặt lại nickname có hai lối vào cùng một command `ResetNickname`: consumer `sanction.applied` (`nickname_reset`, do Safety Team áp trong module chat) và operation public `resetStudentNickname` cho nhà trường. Module chat thêm `context = nickname` cho `moderateDisplayText`, `context.reported_field = nickname` cho `createReport` (bản chụp từ `ref.player_profiles`), sanction `nickname_reset` và hành động case tương ứng. Module identity thêm facade `screenNicknamePersonalData`. Godot: bước Tên của S02 có ô gõ + chip gợi ý, mục Nickname trong Cài đặt, "Tên không phù hợp" trong báo cáo, hiển thị `ui.player.unnamed`. Portal: nút "Đặt lại nickname" trên trang học sinh (F052) và hành động trên case (F122).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go (bản của repo), GDScript Godot 4.x static typing, TypeScript/React (portal) |
| Primary Dependencies | ent + Atlas, oapi-codegen, `src/backend/pkg/{entx,tenancy,authz,idempotency,outbox,modcall,audit,httpx}`; Godot `ApiClient`, `ProfileService` (F070), `ReportService` (F119), `UiBuild`, `CompactPolicy` (F125); portal `ops-ui`, `apiClient` |
| Storage | `eduworld_game`: `game.game_profiles` (+ `nickname_key`), `game.nickname_changes` (append-only); `eduworld_chat`: `chat.sanctions` (loại mới), `chat.reports.context`, `moderation_evidence`; Valkey `ew:game:nick:attempts:<student_id>` |
| Testing | Go unit (domain `Nickname`, command), integration testcontainers (PostgreSQL thật, global filter, unique index đồng thời), facade test bên gọi/bên nhận (`make test-contract`), consumer idempotent; GdUnit4 unit/scene + ma trận khối; Vitest + Playwright portal |
| Target Platform | Backend `core-api` + `worker` (profile `events` cho consumer); client Web, Android, iOS (ADR-0101); portal |
| Project Type | backend modular monolith + game client + web |
| Performance Goals | `updateGameProfile` tự gõ p95 ≤ 300 ms (SC-003); facade trong tiến trình, deadline 1 s |
| Constraints | AGENTS R1, R5, R7, R8, R10, R13, R14, R15, R16, R19, R21, R23, R27, R28, R29, R32; ADR-0020, ADR-0030, ADR-0041, ADR-0070, ADR-0078, ADR-0105 |
| Scale/Scope | ≤ 1 nickname/học sinh/tenant; ≤ `nickname_attempt_rate` lần thử/học sinh; lưu lượng nhỏ (đổi tên hiếm) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec Ready; delta contract (game public, chat public + internal, identity internal, event payload, permission catalog) là task Phase 1 trước code. Không chạm nội dung hoãn (ADR-0082) |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Nickname thuộc module game; kiểm duyệt nằm ở module chat, dữ liệu họ tên ở module identity → chỉ qua facade (`x-allowed-callers: [game]`) gọi **trước** transaction ghi (không gọi facade trong `UnitOfWork`); đặt lại từ Safety Team qua event `sanction.applied`. Command `ChangeNickname`/`ResetNickname` dùng `UnitOfWork` + outbox; query không ghi |
| III Cô lập tenant, an toàn dữ liệu | ✅ | `nickname_key` unique partial theo tenant `WHERE deleted_at IS NULL`; RLS + interceptor; test isolation; `nickname_changes` append-only, ẩn danh hóa bằng hàm `SECURITY DEFINER` viết tay, không hard-delete |
| IV Server-authoritative | ✅ | Mọi quyết định (định dạng, kiểm duyệt, trùng, giới hạn) ở server; client chỉ kiểm sơ bộ để hiển thị; bản chụp nickname báo cáo lấy từ server; `resetStudentNickname` có `Idempotency-Key` + audit, consumer idempotent theo `sanction_id` |
| V Xác thực, phân quyền | ✅ | `updateGameProfile` qua `game_player`; `resetStudentNickname` `authz.Require(lms.student.nickname_reset)` + scope trường; sanction `nickname_reset` chỉ `safety_moderator`/`system_admin` (permission `moderation.sanction.*` là platform — R20); permission mới không cấp cho role học sinh |
| VI An toàn trẻ em | ✅ | R15: cùng pipeline + danh mục CMS của chat, không hardcode từ, không trả từ khớp; PII luôn chạy (ADR-0078); R16: chặn họ tên/tên trường của chính em, facade không trả dữ liệu; R14: không log chuỗi bị từ chối; R19: không đếm ngược, không phạt tự động khi thử nhiều lần |
| VII Một nguồn sự thật, cấu hình | ✅ | Tham số mới `nickname_min_length`, `nickname_max_digits`, `nickname_attempt_rate`, `nickname_moderation_deadline` đăng ký CONFIG_DEFAULTS (owner CHARACTER_SYSTEM §5.3); chuỗi mới có `maxLength`/`MaxLen`; FR từ PRD |
| VIII UI tách logic, token | ✅ | Scene chỉ bind state của `NicknameEditorState`; style từ Theme/token (F114), `compact` (F125); portal dùng `ops-ui` |
| IX Test có bằng chứng | ✅ | Mỗi F136-AC có test (bảng dưới), viết trước và FAIL trước khi code; facade fake gắn nhãn TEST DOUBLE; fixture danh mục chỉ placeholder |

### AC → test

| AC | Test |
|---|---|
| F136-AC-01, AC-02, AC-05, AC-06 | `src/game/tests/unit/player/test_nickname_editor_state.gd`, `src/game/tests/scene/character/test_character_creation_nickname.gd` |
| F136-AC-03, AC-04 | `src/backend/modules/game/internal/application/command/change_nickname_test.go`, integration `TestNicknameFreeText` |
| F136-AC-07 | `src/game/tests/scene/settings/test_profile_nickname.gd` |
| F136-AC-08, AC-09 | integration `TestNicknameChangeLimit`, `TestNicknameProjectionUpdated` |
| F136-AC-10 | `src/backend/modules/game/internal/domain/profile/nickname_test.go` (bảng chuỗi mẫu) |
| F136-AC-11, AC-12, AC-14 | `change_nickname_test.go` (fake facade TEST DOUBLE: blocked, lỗi, timeout); facade test `make test-contract` (chat `moderateDisplayText` context `nickname`, identity `screenNicknamePersonalData`) |
| F136-AC-13 | integration `TestNicknameKeyUniqueConcurrent` |
| F136-AC-15 | integration `TestNicknameAttemptRate` |
| F136-AC-16, AC-30 | integration `TestNicknameRejectNoLeak` (response + log capture) |
| F136-AC-17 | `src/game/tests/scene/social/test_report_nickname.gd` |
| F136-AC-18 | chat integration `TestReportNicknameSnapshot` |
| F136-AC-19 | chat integration `TestSanctionNicknameReset` |
| F136-AC-20 | game integration `TestNicknameResetConsumer` (giao 2 lần) |
| F136-AC-21 | game integration `TestResetStudentNicknameAuthz` (scope, `403`, `404`, replay, audit) |
| F136-AC-22 | portal Vitest `StudentNicknameReset.test.tsx`, `CaseNicknameAction.test.tsx`; Playwright `students/nickname-reset.spec.ts` |
| F136-AC-23 | `src/game/tests/scene/character/test_nickname_reset_flow.gd` |
| F136-AC-24 | `src/game/tests/scene/test_pseudo_locale.gd` (màn mới), `tools/extract_pot.gd` |
| F136-AC-25, AC-26 | `tests/scene/test_grade_band_matrix.gd` (màn mới), `tools/lint_theme_overrides.gd`, `tools/capture_screens.gd`, quickstart §5 |
| F136-AC-27 | `src/game/tests/unit/test_nickname_plain_text.gd` (quét mã: nơi hiển thị nickname không bật BBCode) |
| F136-AC-28 | integration `TestNicknameTenantIsolation` |
| F136-AC-29 | `TestOperationsMatchSpec` + route matrix `game_player` (TC-AUTH-014), `make arch-check` |
| F136-AC-31 | integration `TestAnonymizeNicknameHistory` |
| F136-AC-32 | `make specs-lint` (`check-maxlength`), `make migrate-lint MOD=game` |

## Project Structure

### Documentation (feature này)

```text
specs/136-free-text-nickname/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
├── checklists/requirements.md
└── screenshots/                     # ảnh ma trận khối (T041)
```

### Source Code (repository)

```text
src/backend/modules/game/
├── ent/schema/gameprofile.go                        # SỬA: nickname_key MaxLen(32), index partial
├── migrations/<ts>_f136_nickname_key.sql           # expand + backfill; hàm SECURITY DEFINER anonymize_nickname_changes (viết tay)
├── internal/domain/profile/nickname.go              # MỚI: chuẩn hóa, kiểm định dạng, NicknameKey (thuần)
├── internal/application/port/nickname_screen.go     # MỚI: NicknameModeration, NicknamePersonalDataScreen, AttemptCounter
├── internal/application/command/change_nickname.go  # SỬA: nhánh nickname tự gõ, thứ tự kiểm tra, fail-closed
├── internal/application/command/reset_nickname.go   # MỚI: dùng chung cho consumer + API nhà trường
├── internal/infrastructure/facade/{chat_moderation.go,identity_nickname_screen.go}  # adapter modcall
├── internal/infrastructure/cache/nickname_attempts.go
├── internal/presentation/http/profile.go            # SỬA: oneOf, mã lỗi; MỚI resetStudentNickname
└── internal/presentation/consumer/sanction_nickname_reset.go
src/backend/modules/chat/
├── internal/application/command/{report_create.go,case_act.go}   # SỬA: reported_field nickname, hành động nickname_reset
├── internal/domain/moderation/sanction.go           # SỬA: loại nickname_reset (tức thời)
└── internal/presentation/facade/moderate_display_text.go          # SỬA: context nickname
src/backend/modules/identity/
├── internal/application/query/screen_nickname_personal_data.go    # MỚI (read context)
└── internal/presentation/facade/screen_nickname.go
src/backend/pkg/authz/catalog.go                     # SỬA: lms.student.nickname_reset (tenant)
src/game/
├── scripts/player/nickname_editor_state.gd           # MỚI: thuần — chuỗi, kiểm sơ bộ, chế độ chip/tự gõ, lỗi
├── scripts/player/profile_service.gd                 # SỬA: update_nickname_text(), mã lỗi mới
├── scenes/character/character_creation_screen.gd     # SỬA: bước Tên có ô gõ + chip + gợi ý
├── scenes/settings/profile_nickname_panel.gd         # MỚI: mục Nickname trong "Hồ sơ của em"
├── scenes/social/report_reasons.gd                   # SỬA: "Tên không phù hợp"
├── locale/vi.po, en.po                               # ui.avatar.nickname.*, ui.profile.nickname.*, ui.player.unnamed …
└── tests/{unit/player, scene/character, scene/settings, scene/social}/
src/cms/frontend/apps/portal/src/
├── features/school/students/StudentNicknameReset.tsx # MỚI
└── features/moderation/cases/CaseNicknameAction.tsx  # MỚI
```

**Structure Decision**: Giữ nickname trong aggregate `GameProfile` của module game (F070); kiểm duyệt và dữ liệu cá nhân thuộc module khác nên gọi qua facade có kiểu (R29), **trước** khi mở `UnitOfWork` (không giữ transaction qua lời gọi module khác). Đặt lại từ Safety Team đi qua event vì là phản ứng phụ của quyết định moderation (R29: phản ứng phụ dùng event). Một command `ResetNickname` dùng chung cho hai lối vào để quy tắc đặt lại có một nơi.

## Phase 0: Research

Không cần `research.md` — pipeline kiểm duyệt (MODERATION §3), facade `moderateDisplayText` (F116), case/sanction (F121), GameProfile (F070) đã có; các quyết định còn mở đã chốt ở spec Clarifications 2026-11-28.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `profile.Nickname` (chuẩn hóa, kiểm định dạng, `Key()`), `GameProfile.SetNickname/ResetNickname` | Thuần Go, không import hạ tầng |
| application | `command.ChangeNickname` (định dạng → giới hạn tháng → bộ đếm lần thử → kiểm duyệt → dữ liệu cá nhân → `UnitOfWork` lưu + kiểm trùng + outbox), `command.ResetNickname`; port `NicknameModeration`, `NicknamePersonalDataScreen`, `NicknameAttemptCounter` | `authz.Require` đầu use case |
| infrastructure | adapter `modcall` tới `chat/api` và `identity/api`; Valkey counter; ent repo | Deadline, ánh xạ lỗi → `503` |
| presentation | HTTP `updateGameProfile` (oneOf), `resetStudentNickname`; consumer `sanction.applied` (lọc `nickname_reset`) đăng ký trong `module.go`, chạy ở `worker` profile `events` | Problem Details |

### Tích hợp và phụ thuộc

- Module chat (F116, F121): `moderateDisplayText` thêm `context = nickname` và caller `game`; `createReport` thêm `context.reported_field`; sanction `nickname_reset` + hành động case; payload `sanction.applied` thêm giá trị enum (schema owner F116).
- Module identity: facade `screenNicknamePersonalData` đọc `org.students.full_name` + tên trường (read context, không ghi).
- F070: `GameProfile`, `suggestNicknames`, `game_profile.updated`, `anonymizeUserData` (mở rộng ẩn danh lịch sử).
- F052/F122 portal: nút đặt lại, hành động case. F125/F114: giao diện điện thoại, phong cách tầng. F135: học sinh tự đăng ký vào S02 (không thay đổi gì thêm).

### Bảo mật, tenant, idempotency, audit

- `updateGameProfile` giữ hành vi F070 (đổi giá trị, lặp lại cùng chuỗi trả trạng thái hiện tại); `resetStudentNickname` bắt buộc `Idempotency-Key`; consumer theo `processed_events` + `sanction_id`.
- Audit: `game_profile.nickname_changed` (giá trị cũ/mới, actor), `game_profile.nickname_reset` (actor, lý do, nguồn `school_admin`/`moderation`), lần từ chối chỉ mã kết quả; không log chuỗi bị từ chối (R14).
- Bộ đếm lần thử ở Valkey là bảo vệ phụ (mất dữ liệu chỉ nới tạm thời, R9).

### Quan sát

- Metric `eduworld_nickname_checks_total{result="allowed|format|moderation|personal|taken|rate_limited|unavailable"}`, histogram độ trễ facade; span cho từng lời gọi facade; không nhãn người dùng/nickname.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
