# Implementation Plan: F135 — Học sinh tự đăng ký trong game vào tenant chung (bước phụ huynh, mã khôi phục, chống lạm dụng)

**Branch**: `135-game-self-registration` · **Date**: 2026-11-28 · **Spec**: [spec.md](spec.md)

## Summary

Module identity thêm bounded context `selfreg`: use case `RegisterStudent` (write) dùng lại bộ điều hợp Keycloak Admin và thứ tự bước của `CreateCommonTenantStudents` (F010) cho **một** học sinh, cộng bằng chứng đồng ý (`identity.self_registrations`), băm mã khôi phục (`identity.recovery_codes`), giới hạn theo thiết bị/toàn cục đếm trong PostgreSQL, idempotency + audit + outbox (`user.provisioned` `source = self_registration`, `student.placement_changed`). Thêm `RecoverStudentPassword`, `RequestParentPasswordReset`, `ConfirmParentEmail`, `ResetPasswordByParentLink` (token một lần, gửi thư qua hàng đợi email F124), query `GetRegistrationTerms` (văn bản có version từ seed), `GetSelfRegistrationEvidence` cho System Admin, và trường `account_source` + bộ lọc ở các danh sách học sinh/người dùng sẵn có. Godot thêm màn `Register` (5 bước), `RecoveryCodeCard`, `ForgotPassword` gọi REST qua `ApiClient`; sau đăng ký chạy luồng `gameLogin` sẵn có. Portal thêm hai route công khai cho phụ huynh và nhãn/bộ lọc "Tự đăng ký" + khung bằng chứng ở chi tiết học sinh tenant `common`. Realm Keycloak không đổi (`registrationAllowed = false`).

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go (bản của repo) · GDScript Godot 4.x · TypeScript/React (portal) |
| Primary Dependencies | chi, ent + Atlas, oapi-codegen, `src/backend/pkg/{entx,idempotency,outbox,audit,authz,tenancy,httpx,modcall}`, adapter Keycloak Admin của module identity, hàng đợi email F124 (`email_deliveries`, `mail/smtp_sender.go`), argon2id (`golang.org/x/crypto/argon2`); Godot: `ApiClient`, `AuthService` (F061), `UiBuild`, `CompactPolicy` (F125), TTS "Đọc to" (F097); portal: Ant Design 5 + `ops-ui` |
| Storage | PostgreSQL `eduworld_identity` (3 bảng mới), không Valkey cho dữ liệu cần giữ (R9) |
| Testing | Go unit (domain `selfreg`: tuổi/khối, tên đăng nhập, mã khôi phục), integration testcontainers (PostgreSQL + Keycloak), contract test OpenAPI; GdUnit4 (unit service + scene + ma trận khối + ảnh); Vitest + Playwright (portal) |
| Target Platform | Linux container (`core-api`, `worker` profile `jobs`); client Web/Android/iOS (ADR-0101) |
| Project Type | backend module + game client + web portal |
| Performance Goals | `registerStudent` p95 ≤ 1,5 s (gồm Keycloak); không chạm đường `gameLogin` (R30) |
| Constraints | AGENTS R3, R5, R7, R8, R9, R10, R14, R15, R16, R19, R20, R21, R23, R27, R28, R29, R30; ADR-0063, ADR-0068, ADR-0069, ADR-0078, ADR-0086, ADR-0104 |
| Scale/Scope | ≤ `self_registration_global_hourly_limit` đăng ký/giờ; một học sinh mỗi request |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Spec Ready; delta OpenAPI/event/schema là task Phase 1 (T002–T005). Không triển khai mô hình consent chung (ADR-0077 hoãn theo ADR-0082); chỉ bằng chứng xác nhận theo ADR-0104 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Toàn bộ backend trong module identity (sở hữu user, học sinh, tenant); không gọi module khác đồng bộ; module game/learning/chat nhận `user.provisioned`/`student.placement_changed` như học sinh tenant chung. Command dùng `UnitOfWork` + outbox; query (`GetRegistrationTerms`, `GetSelfRegistrationEvidence`, danh sách) chỉ đọc. Keycloak gọi qua adapter có timeout/circuit breaker, **trước** transaction, bù khi lỗi |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Bảng mới có `tenant_id` (= `common`) + soft-delete, mixin `entx`; endpoint công khai không nhận tenant từ client — tenant `common` cố định ở server; không hard-delete; test tenant isolation cho API quản trị |
| IV Server-authoritative | ✅ | Khối/ngày sinh/tên/điều khoản kiểm ở server; thời điểm đồng ý theo đồng hồ server; giới hạn đếm ở server; `Idempotency-Key` + audit cho đăng ký, khôi phục, đặt lại |
| V Xác thực, phân quyền | ✅ | Endpoint công khai không token nhưng có rate limit (như `gameLogin`); API quản trị: `platform.user.manage`, gateway nội bộ; cài đặt `self_registration_enabled` chỉ role platform (R20); không trang đăng ký Keycloak; game vẫn chỉ cho `student` |
| VI An toàn trẻ em | ✅ | R14: không log mật khẩu/mã/ngày sinh/email; R15: không nhánh theo nguồn tài khoản trong chat/tính năng; R16: không thu họ tên, ngày sinh không lộ; R19: ô đồng ý không đánh dấu sẵn; R23/R30: HMAC thiết bị, không IP; mặc định tenant `common` (ADR-0078) giữ nguyên |
| VII Một nguồn sự thật, cấu hình | ✅ | 9 tham số mới đăng ký CONFIG_DEFAULTS (sở hữu: ACCOUNT_PROVISIONING §3.5); chuỗi mới có `maxLength`/`MaxLen` (ADR-0041); FR do PRD cấp (FR-AUTH-033…036) |
| VIII UI tách logic, token | ✅ | Scene Godot chỉ bind `RegistrationService`/`RecoveryService`; portal dùng Admin UI + token; không hardcode màu/cỡ chữ |
| IX Test có bằng chứng | ✅ | Bảng AC → test dưới; test viết trước và FAIL trước khi code; fake Keycloak/SMTP trong unit gắn nhãn TEST DOUBLE, integration dùng Keycloak thật (testcontainers) |

### AC → test

| AC | Test |
|---|---|
| F135-AC-01, AC-02, AC-06 | `src/game/tests/scene/auth/test_register_flow.gd`, `src/game/tests/unit/auth/test_registration_service.gd` |
| F135-AC-03, AC-04, AC-05 | `src/backend/modules/identity/tests/integration/self_registration_test.go::TestRegisterStudent{Creates,Outbox,CompensatesKeycloak}` |
| F135-AC-07, AC-08, AC-09 | `src/backend/modules/identity/internal/domain/selfreg/*_test.go`, integration `TestRegisterStudentValidation`, `TestRegisterStudentUsernameTaken`, `TestRegisterStudentCurfew` |
| F135-AC-10…AC-15 | integration `TestSelfRegistrationEvidence`, `TestParentEmailConfirm`; `tests/scene/auth/test_parent_step.gd`; portal `ParentConfirmPage.test.tsx` |
| F135-AC-16…AC-21 | integration `TestRecoverStudentPassword`, `TestParentPasswordReset`; domain `recovery_code_test.go`; `tests/scene/auth/test_recovery_code_card.gd`, `test_forgot_password.gd`; portal `ParentResetPasswordPage.test.tsx` |
| F135-AC-22…AC-26 | integration `TestSelfRegistrationLimits`, `TestSelfRegistrationIdempotency`, `TestSelfRegistrationSwitch`; `deploy/apisix` route test; `TestPublicAuthRoutesNoBodyLog` |
| F135-AC-27, AC-28 | integration chéo module `tests/e2e/self_registered_parity_test.go` (cùng bộ quyền với học sinh `platform_common`); `tests/scene/hub/test_school_tasks_empty.gd` |
| F135-AC-29, AC-30 | integration `TestSelfRegisteredMergeKeepsProgress` (saga F045), `TestSelfRegisteredCleanupAndAnonymize` |
| F135-AC-31…AC-33 | integration `TestAccountSourceFilter`, `TestSelfRegistrationEvidenceAuthz`, `TestResetPasswordIssuesRecoveryCode`; portal Vitest `CommonStudentsList.test.tsx`; Playwright `platform/self-registered.spec.ts` |
| F135-AC-34…AC-36 | `tests/scene/test_grade_band_matrix.gd` (màn mới), `tests/scene/test_pseudo_locale.gd`, `tools/capture_screens.gd`, quickstart §5 |
| F135-AC-37, AC-38 | `tests/unit/auth/test_registration_privacy.gd` (quét log/`user://`), `tests/unit/auth/test_auth_boundaries.gd` |

## Project Structure

### Documentation (feature này)

```text
specs/135-game-self-registration/
├── spec.md · plan.md · data-model.md · quickstart.md · tasks.md
├── contracts/README.md
├── checklists/requirements.md
└── screenshots/                     # ảnh ma trận khối (T046)
```

### Source Code (repository)

```text
src/backend/modules/identity/
├── internal/domain/selfreg/              # AgeGradeRule, UsernamePolicy, RecoveryCode (sinh, định dạng, băm), ConsentEvidence, DeviceHasher
├── internal/application/command/selfreg/ # register_student.go, recover_password.go, request_parent_reset.go, confirm_parent_email.go, reset_by_parent_link.go
├── internal/application/query/selfreg/   # get_registration_terms.go, get_self_registration_evidence.go
├── internal/application/port/            # write: SelfRegistrationRepo, RecoveryCodeRepo, ParentLinkTokenRepo; RegistrationCounter; KeycloakUsers (có sẵn)
├── internal/infrastructure/persistence/ent/schema/{self_registration,recovery_code,parent_link_token}.go
├── internal/infrastructure/mail/templates/{vi,en}/{parent_email_confirm,parent_password_reset}.tmpl
├── internal/presentation/http/selfreg_handler.go   # public routes + admin evidence
├── seed/registration_terms/<version>.{vi,en}.md
├── migrations/<ts>_f135_self_registration.sql      # Atlas + RLS/GRANT viết tay
└── tests/integration/self_registration_test.go
src/game/
├── scenes/auth/Register.tscn + register.gd          # 5 bước, thanh tiến độ, ✕ Đóng
├── scenes/auth/{register_grade_step,register_birthdate_step,register_account_step,register_parent_step}.gd
├── scenes/auth/recovery_code_card.gd · forgot_password.gd
├── scripts/authentication/{registration_service.gd,recovery_service.gd}
├── scenes/auth/login_form.gd                        # SỬA: nút "Tạo tài khoản", "Quên mật khẩu?" mở rộng
└── locale/{vi,en}.po                                # ui.register.*, ui.recovery.*, ui.error.*
src/cms/frontend/apps/portal/
├── src/public/parent/{ParentConfirmPage,ParentResetPasswordPage}.tsx   # route công khai, không đăng nhập
└── src/workspaces/platform/students/                # nhãn + bộ lọc "Tự đăng ký", khung bằng chứng
deploy/apisix/                                       # route công khai + limit-count theo thiết bị/tên
```

**Structure Decision**: Một bounded context mới trong module identity vì mọi dữ liệu (user, học sinh, tenant `common`, Keycloak) thuộc identity; không cần facade mới. Godot thêm màn trong `scenes/auth/` cạnh form đăng nhập F061; portal thêm route công khai cạnh `/login`, `/reset-password` của F051 (cùng shell công khai, không nạp chunk workspace).

## Phase 0: Research

Không cần `research.md`: mẫu tạo học sinh tenant chung (F010, ACCOUNT_PROVISIONING §3.4), token một lần (F012 `forgotPortalPassword`), hàng đợi email (F124), bảo vệ đăng nhập (F013) đã có; quyết định còn lại chốt ở Clarifications 2026-11-28.

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `selfreg.AgeGradeRule`, `UsernamePolicy`, `RecoveryCode`, `ConsentEvidence`, `RegistrationLimits` | Thuần Go, không I/O |
| application (write) | `RegisterStudent`, `RecoverStudentPassword`, `RequestParentPasswordReset`, `ConfirmParentEmail`, `ResetPasswordByParentLink`; mở rộng `ResetStudentPassword` (cấp mã mới khi là tài khoản tự đăng ký) | `UnitOfWork` + outbox + audit; Keycloak trước transaction, bù khi lỗi |
| application (read) | `GetRegistrationTerms`, `GetSelfRegistrationEvidence`; `account_source` trong query danh sách | Không ghi DB |
| infrastructure | ent repo, adapter Keycloak Admin (có sẵn), hàng đợi email (có sẵn), HMAC thiết bị (khóa xoay vòng từ secret) | Timeout, retry chỉ cho thao tác idempotent, circuit breaker |
| presentation | HTTP handler public + admin; đăng ký route trong `module.go` (binary `core-api`) | `authz.Require(platform.user.manage)` ở API quản trị |

### Luồng `RegisterStudent`

1. Kiểm `self_registration_enabled`, giờ giới nghiêm, `Idempotency-Key` (replay → kết quả đã lưu, không mã).
2. Kiểm dữ liệu (domain): khối, ngày sinh với `self_registration_age_margin`, tên đăng nhập + tiền tố dành riêng, `terms_version` hiệu lực, ô đồng ý.
3. Đếm giới hạn thiết bị/toàn cục từ PostgreSQL (`self_registrations` theo `device_id_hash`, `created_at`).
4. Kiểm trùng tên đăng nhập (SSO + `identity.users`, ADR-0069) → `409`.
5. Sinh `user_id` (UUIDv7), tạo user Keycloak (attribute `eduworld_user_id`, role `student`, member Organization `common`, mật khẩu `temporary = false`); `409` từ Keycloak → tìm theo attribute (idempotent).
6. Transaction: `identity.users`, `org.students`, ExternalIdentity, membership, `self_registrations`, `recovery_codes` (băm), `parent_link_tokens` + `email_deliveries` (nếu có email), audit, outbox. Lỗi → bù: `enabled = false` user Keycloak, trả `503`.
7. Trả `201` (`recovery_code` một lần, `Cache-Control: no-store`).

### Bảo mật, tenant, idempotency, audit

- Endpoint công khai: không token, không `X-Tenant-Id`; APISIX `limit-count` theo `device_id` (body) và theo tên đăng nhập như `gameLogin`; không log body (APISIX + backend + trace, FR-AUTH-003).
- HMAC thiết bị dùng khóa riêng `EW_IDENTITY_DEVICE_HASH_KEY` có version (xoay vòng, R3/R4); đếm giới hạn so khớp theo mọi version khóa còn hiệu lực trong ngày.
- Mã khôi phục, token link: chỉ băm; so sánh thời gian hằng; `401 RECOVERY_INVALID` có độ trễ tối thiểu cố định.
- Audit: `user.self_registered`, `user.self_registration_rejected` (lý do mã, không dữ liệu), `user.password_recovered`, `parent_email.confirmed`, `self_registration.evidence_viewed`, `platform_settings.changed` (công tắc).

### Quan sát

- Metric `eduworld_self_registration_total{result}` (`created`, `invalid`, `username_taken`, `device_limit`, `busy`, `disabled`, `curfew`, `error`), `eduworld_password_recovery_total{method,result}`, `eduworld_parent_email_total{kind,result}`; alert khi `created` vượt 3× trung bình 7 ngày (MONITORING, T042). Không nhãn chứa tên/thiết bị.

## Complexity Tracking

> Không có vi phạm.

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
