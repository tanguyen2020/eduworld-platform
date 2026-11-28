# Tasks: F135 — Học sinh tự đăng ký trong game vào tenant chung (bước phụ huynh, mã khôi phục, chống lạm dụng)

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

Quy ước: test viết trước và phải FAIL trước khi code của story đó; fake/fixture gắn nhãn TEST DOUBLE (R17); backend dưới `src/backend/modules/identity/`, client dưới `src/game/`, portal dưới `src/cms/frontend/apps/portal/`.

## Phase 1: Setup

- [ ] T001 Kiểm tiền đề: adapter Keycloak Admin + thứ tự bước của `createCommonTenantStudents` (F010), hàng đợi email (F124), token một lần (F012), saga gộp (F045), form S01 (F061); thiếu thì dừng và làm feature đó trước — file: `src/backend/modules/identity/`, `src/game/scenes/auth/`
- [ ] T002 [P] Delta OpenAPI D1, D6: 7 operation mới, schema, `account_source`, ví dụ; tách bundle, `make gen MOD=identity` — file: `specs/openapi/modules/identity.v1.yaml`, `specs/openapi/eduworld-api.v1.yaml`
- [ ] T003 [P] Mã lỗi D2 (`REGISTRATION_DISABLED`, `REGISTRATION_LIMIT_REACHED`, `REGISTRATION_BUSY`, `RECOVERY_INVALID`, `LINK_EXPIRED`) — file: `docs/07-backend/API_DESIGN.md`, `scripts/specs/check-problem-codes.mjs` (dữ liệu)
- [ ] T004 [P] Delta event D3 (`user.provisioned.source` thêm `self_registration`) và cài đặt platform D4 — file: `specs/events/payloads/identity/user.provisioned.v1.schema.json`, `docs/07-backend/EVENT_DESIGN.md`, schema cài đặt platform
- [ ] T005 [P] Route APISIX công khai + rate limit + tắt log body (D5); khóa HMAC thiết bị `EW_IDENTITY_DEVICE_HASH_KEY` trong `.env.example` và SOPS — file: `deploy/apisix/`, `deploy/compose/.env.example`, `deploy/swarm/`
- [ ] T006 [P] Khóa dịch `ui.register.*`, `ui.recovery.*`, `ui.error.<CODE>` (vi/en), mẫu thư `parent_email_confirm`, `parent_password_reset` (vi/en), văn bản điều khoản version đầu tiên (vi/en) để owner duyệt — file: `src/game/locale/{vi,en}.po`, `src/backend/modules/identity/internal/infrastructure/mail/templates/`, `src/backend/modules/identity/seed/registration_terms/`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T007 Ent schema `self_registrations`, `recovery_codes`, `recovery_attempts`, `parent_link_tokens` (mixin `entx`, `MaxLen`), cột `identity.users.account_source` + backfill; `make migrate-diff`, RLS/GRANT viết tay — file: `src/backend/modules/identity/internal/infrastructure/persistence/ent/schema/`, `src/backend/modules/identity/migrations/`
- [ ] T008 Domain `selfreg` (AgeGradeRule, UsernamePolicy, RecoveryCode, ConsentEvidence, RegistrationLimits) + unit test — file: `src/backend/modules/identity/internal/domain/selfreg/`
- [ ] T009 Port + repo ent (write/read) và `DeviceHasher` có version khóa — file: `src/backend/modules/identity/internal/application/port/`, `src/backend/modules/identity/internal/infrastructure/persistence/`
- [ ] T010 Đăng ký route + handler khung trong `module.go` (binary `core-api`), `authz` cho API quản trị — file: `src/backend/modules/identity/module.go`, `src/backend/modules/identity/internal/presentation/http/selfreg_handler.go`
- [ ] T011 [P] Godot: `RegistrationService`, `RecoveryService` khung + fixture contract sinh từ example (TEST DOUBLE) — file: `src/game/scripts/authentication/`, `src/game/tools/gen_fixtures.mjs`, `src/game/tests/fixtures/contract/`

**Checkpoint**: nền tảng sẵn sàng.

## Phase 3: User Story 1 — Tự tạo tài khoản và vào game (Priority: P1) 🎯 MVP

### Tests cho User Story 1

- [ ] T012 [P] [US1] Integration F135-AC-03, F135-AC-04, F135-AC-05: tạo đủ bản ghi + Keycloak thật, outbox `user.provisioned` `source = self_registration` + `student.placement_changed` không PII, bù Keycloak khi transaction lỗi (TC-AUTH-102, TC-AUTH-106) — file: `src/backend/modules/identity/tests/integration/self_registration_test.go` — phải FAIL trước khi code
- [ ] T013 [P] [US1] Unit + integration F135-AC-07, F135-AC-08, F135-AC-09: biên tuổi/khối, tiền tố dành riêng, trường lạ, trùng tên (khác hoa thường, tenant khác, soft-delete), giới nghiêm (TC-AUTH-103, TC-AUTH-092) — file: `src/backend/modules/identity/internal/domain/selfreg/age_grade_rule_test.go`, `src/backend/modules/identity/tests/integration/self_registration_validation_test.go` — phải FAIL trước khi code
- [ ] T014 [P] [US1] Scene F135-AC-01, F135-AC-02, F135-AC-06: nút theo `enabled`, đủ 5 bước không ô họ tên, sau `201` gọi `gameLogin` và xóa mật khẩu khỏi RAM (TC-AUTH-102) — file: `src/game/tests/scene/auth/test_register_flow.gd`, `src/game/tests/unit/auth/test_registration_service.gd` — phải FAIL trước khi code

### Implementation cho User Story 1

- [ ] T015 [US1] Command `RegisterStudent` theo luồng plan.md (Keycloak trước transaction, bù, outbox, audit) + query `GetRegistrationTerms` — file: `src/backend/modules/identity/internal/application/command/selfreg/register_student.go`, `src/backend/modules/identity/internal/application/query/selfreg/get_registration_terms.go`
- [ ] T016 [US1] Handler `registerStudent`, `getRegistrationTerms` (`no-store`, Problem Details) — file: `src/backend/modules/identity/internal/presentation/http/selfreg_handler.go`
- [ ] T017 [US1] Godot màn `Register` (bước khối, ngày sinh, tài khoản, phụ huynh — US2 điền nội dung bước), nút "Tạo tài khoản" trên S01, nối `AuthService.login` — file: `src/game/scenes/auth/Register.tscn`, `src/game/scenes/auth/register*.gd`, `src/game/scenes/auth/login_form.gd`

**Checkpoint**: US1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — Bước phụ huynh, bằng chứng, email tùy chọn (Priority: P1)

### Tests cho User Story 2

- [ ] T018 [P] [US2] Integration F135-AC-11, F135-AC-12, F135-AC-13, F135-AC-14, F135-AC-15: bằng chứng (server time, version, HMAC, không IP/UA), email mã hóa + thư xếp hàng, link một lần/hết hạn `410`, giới hạn thư/địa chỉ, version điều khoản cũ `422` (TC-AUTH-104) — file: `src/backend/modules/identity/tests/integration/self_registration_evidence_test.go` — phải FAIL trước khi code
- [ ] T019 [P] [US2] Scene F135-AC-10: ô đồng ý không đánh dấu sẵn, nút mờ, "Đọc to", gửi `terms_version` (TC-AUTH-104) — file: `src/game/tests/scene/auth/test_parent_step.gd` — phải FAIL trước khi code
- [ ] T020 [P] [US2] Vitest trang công khai xác nhận email (thành công, `410`) — file: `src/cms/frontend/apps/portal/src/public/parent/ParentConfirmPage.test.tsx` — phải FAIL trước khi code

### Implementation cho User Story 2

- [ ] T021 [US2] Lưu bằng chứng + email (mã hóa, HMAC) trong `RegisterStudent`; command `ConfirmParentEmail`; mẫu thư qua hàng đợi F124 — file: `src/backend/modules/identity/internal/application/command/selfreg/confirm_parent_email.go`, `src/backend/modules/identity/internal/infrastructure/mail/templates/`
- [ ] T022 [US2] Bước phụ huynh trong Godot (văn bản thuần, "Đọc to", ô email tùy chọn) — file: `src/game/scenes/auth/register_parent_step.gd`
- [ ] T023 [US2] Trang công khai `/phu-huynh/xac-nhan` (shell công khai, không nạp workspace) — file: `src/cms/frontend/apps/portal/src/public/parent/ParentConfirmPage.tsx`, router công khai

## Phase 5: User Story 3 — Mã khôi phục và "Quên mật khẩu" (Priority: P1)

### Tests cho User Story 3

- [ ] T024 [P] [US3] Integration F135-AC-17, F135-AC-18, F135-AC-19, F135-AC-20: băm argon2id, replay không mã, khôi phục đổi mật khẩu + mã mới + thu hồi phiên + `user.access_revoked`, `401` chung thời gian không phân biệt, `429` không khóa đăng nhập, `202` luôn, link phụ huynh đặt lại (TC-AUTH-105) — file: `src/backend/modules/identity/tests/integration/password_recovery_test.go` — phải FAIL trước khi code
- [ ] T025 [P] [US3] Scene F135-AC-16, F135-AC-21: màn mã (nhóm 4, "Đã ghi lại mã" bắt buộc, không lưu đĩa), "Quên mật khẩu?" ba lựa chọn, xóa ô sau mỗi lần gửi (TC-AUTH-105) — file: `src/game/tests/scene/auth/test_recovery_code_card.gd`, `src/game/tests/scene/auth/test_forgot_password.gd` — phải FAIL trước khi code
- [ ] T026 [P] [US3] Vitest trang công khai đặt lại mật khẩu qua link — file: `src/cms/frontend/apps/portal/src/public/parent/ParentResetPasswordPage.test.tsx` — phải FAIL trước khi code

### Implementation cho User Story 3

- [ ] T027 [US3] Commands `RecoverStudentPassword`, `RequestParentPasswordReset`, `ResetPasswordByParentLink` + handler — file: `src/backend/modules/identity/internal/application/command/selfreg/`, `src/backend/modules/identity/internal/presentation/http/selfreg_handler.go`
- [ ] T028 [US3] Godot `RecoveryCodeCard`, `ForgotPassword` + `RecoveryService` — file: `src/game/scenes/auth/recovery_code_card.gd`, `src/game/scenes/auth/forgot_password.gd`, `src/game/scripts/authentication/recovery_service.gd`
- [ ] T029 [US3] Trang `/phu-huynh/dat-lai-mat-khau` — file: `src/cms/frontend/apps/portal/src/public/parent/ParentResetPasswordPage.tsx`

## Phase 6: User Story 4 — Chống lạm dụng, idempotency, công tắc (Priority: P2)

### Tests cho User Story 4

- [ ] T030 [P] [US4] Integration F135-AC-22, F135-AC-23, F135-AC-24, F135-AC-25: giới hạn thiết bị từ PostgreSQL (không IP), trần giờ, idempotency (replay/khác body), audit + metric, công tắc (role tenant `403`) (TC-AUTH-106) — file: `src/backend/modules/identity/tests/integration/self_registration_limits_test.go` — phải FAIL trước khi code
- [ ] T031 [P] [US4] Test F135-AC-26: route công khai không cần token/`X-Tenant-Id`, rate limit APISIX, không log body (TC-AUTH-100) — file: `deploy/apisix/tests/`, `src/backend/modules/identity/tests/integration/public_auth_routes_log_test.go` — phải FAIL trước khi code

### Implementation cho User Story 4

- [ ] T032 [US4] `RegistrationLimits` + đếm SQL có điều kiện tenant + soft-delete (có test), công tắc từ cài đặt platform (cache theo F015), metric — file: `src/backend/modules/identity/internal/infrastructure/persistence/registration_counter.go`, `src/backend/modules/identity/internal/application/command/selfreg/register_student.go`

## Phase 7: User Story 5 — Đủ tính năng như học sinh tenant chung, gộp, dọn (Priority: P2)

### Tests cho User Story 5

- [ ] T033 [P] [US5] E2E chéo module F135-AC-27: cùng bộ kiểm quyền (chat policy, kết bạn, club, ghép trận, quest, luyện tập) cho học sinh `self_registration` và `platform_common` → cùng kết quả; quét mã: không nhánh nào đọc `account_source` ngoài màn quản trị (TC-AUTH-102, TC-CHAT-047) — file: `tests/e2e/self_registered_parity_test.go`, `src/backend/tools/archcheck/` — phải FAIL trước khi code
- [ ] T034 [P] [US5] Scene F135-AC-28: "Nhà trường giao" rỗng thân thiện — file: `src/game/tests/scene/hub/test_school_tasks_empty.gd` — phải FAIL trước khi code
- [ ] T035 [P] [US5] Integration F135-AC-29, F135-AC-30: saga gộp giữ XP/vật phẩm/quest/attempts (câu hỏi mở F135-2), dọn + ẩn danh hóa xóa ngày sinh/email/mã (TC-AUTH-107) — file: `src/backend/modules/identity/tests/integration/self_registered_lifecycle_test.go` — phải FAIL trước khi code

### Implementation cho User Story 5

- [ ] T036 [US5] Ẩn danh hóa mở rộng (`self_registrations.parent_email_*`, `recovery_codes` revoke `anonymized`); trạng thái rỗng "Nhà trường giao" nếu thiếu — file: `src/backend/modules/identity/internal/application/command/lifecycle/anonymize_user.go`, `src/game/scenes/ui/hud/`

## Phase 8: User Story 6 — System Admin quản lý (Priority: P2)

### Tests cho User Story 6

- [ ] T037 [P] [US6] Integration F135-AC-31, F135-AC-32, F135-AC-33: `account_source` + lọc + phân trang, tenant isolation (role tenant `404`), bằng chứng (email che, audit mỗi lần xem), `resetStudentPassword` cấp mã mới (TC-AUTH-107) — file: `src/backend/modules/identity/tests/integration/self_registration_admin_test.go` — phải FAIL trước khi code
- [ ] T038 [P] [US6] Vitest + Playwright portal: nhãn/bộ lọc "Tự đăng ký", khung bằng chứng — file: `src/cms/frontend/apps/portal/src/workspaces/platform/students/CommonStudentsList.test.tsx`, `src/cms/frontend/apps/portal/tests/e2e/platform/self-registered.spec.ts` — phải FAIL trước khi code

### Implementation cho User Story 6

- [ ] T039 [US6] Query `GetSelfRegistrationEvidence`, `account_source` trong query danh sách, mở rộng `ResetStudentPassword` — file: `src/backend/modules/identity/internal/application/query/selfreg/get_self_registration_evidence.go`, `src/backend/modules/identity/internal/application/command/account/reset_student_password.go`
- [ ] T040 [US6] Portal: nhãn, bộ lọc, khung bằng chứng ở chi tiết học sinh tenant `common` — file: `src/cms/frontend/apps/portal/src/workspaces/platform/students/`

## Phase 9: User Story 7 — Giao diện lớp 1–9, điện thoại, song ngữ, riêng tư (Priority: P2)

### Tests cho User Story 7

- [ ] T041 [P] [US7] Ma trận khối + pseudo-locale F135-AC-34, F135-AC-35, F135-AC-36 cho mọi màn mới (TC-UI-003, TC-I18N-006) — file: `src/game/tests/scene/test_grade_band_matrix.gd`, `src/game/tests/scene/test_pseudo_locale.gd` — phải FAIL trước khi code
- [ ] T042 [P] [US7] Unit F135-AC-37, F135-AC-38: quét log/telemetry/`user://` không mật khẩu/mã/ngày sinh/email; không WebView/trang Keycloak (TC-AUTH-100, TC-AUTH-001) — file: `src/game/tests/unit/auth/test_registration_privacy.gd`, `src/game/tests/unit/auth/test_auth_boundaries.gd` — phải FAIL trước khi code

### Implementation cho User Story 7

- [ ] T043 [US7] Bố cục `compact` (F125), "Đọc to" mỗi bước, ✕ Đóng, lỗi cạnh ô có icon + chữ; sửa theo ma trận tới khi xanh — file: `src/game/scenes/auth/register*.gd`, `src/game/scenes/auth/forgot_password.gd`
- [ ] T044 [US7] Alert số đăng ký bất thường + runbook ngắn — file: `deploy/observability/prometheus/rules/identity.yml`, `docs/11-operations/runbooks/`

## Phase 10: Polish & Cross-Cutting

- [ ] T045 Chạy lệnh quickstart §3 (Go identity, specs, Godot, web) và ghi số pass/fail — file: `specs/135-game-self-registration/tasks.md`
- [ ] T046 Chụp ảnh trước/sau theo F135-AC-36, xem từng ảnh, ghi bảng ở quickstart §5 — file: `specs/135-game-self-registration/screenshots/`, `specs/135-game-self-registration/quickstart.md`
- [ ] T047 Chạy quickstart §2 với stack thật (Keycloak, Mailpit) và ghi kết quả — file: `specs/135-game-self-registration/quickstart.md`
- [ ] T048 Owner duyệt văn bản điều khoản version đầu; ghi khuyến nghị pháp chế (câu hỏi mở F135-4) — file: `src/backend/modules/identity/seed/registration_terms/`, `docs/12-project/OPEN_QUESTIONS.md`
- [ ] T049 Cập nhật tài liệu sở hữu nếu triển khai đổi chi tiết (ACCOUNT_PROVISIONING §3.5, KEYCLOAK_SSO, CHILD_SAFETY §3.6, DATA_PRIVACY §2, STUDENT_GAME_UX_CORE S01), CODE_PLAN dòng F135, CHANGELOG — file: `docs/09-identity/ACCOUNT_PROVISIONING.md`, `docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md`, `CHANGELOG.md`

## Dependencies & Execution Order

- F010, F012, F013, F016, F045, F061, F124 có sẵn (T001 chặn mọi story); F136 nên xong trước để S02 có ô gõ nickname (không chặn).
- Setup (T001–T006) → Foundational (T007–T011) → US1 (MVP) → US2, US3 (P1, song song được sau US1) → US4, US5, US6 (P2, độc lập nhau) → US7 (ma trận, quét riêng tư trên mọi màn) → Polish.
- Trong mỗi story: test (FAIL) → code → test pass. T041 chạy lại sau mỗi story có màn mới.

## Definition of Done

- [ ] Mọi tiêu chí F135-AC-01…F135-AC-38 có test pass (bảng AC → test ở plan.md)
- [ ] `make test MOD=identity`, `make test-integration MOD=identity`, `make migrate-lint MOD=identity`, `make specs-lint`, `make godot-test`, `make web-test` xanh
- [ ] Ảnh ma trận khối đã chụp và xem (F135-AC-36); kịch bản quickstart §2 chạy với stack thật
- [ ] Constitution Check vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Văn bản điều khoản được owner duyệt; tài liệu sở hữu, CHANGELOG, CODE_PLAN cập nhật
