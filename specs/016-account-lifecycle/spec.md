# Feature Specification: F016 — Vòng đời tài khoản: khóa, thu hồi truy cập, dọn học sinh không đăng nhập, ẩn danh hóa

**Feature Branch**: `016-account-lifecycle` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `identity` (+ phần nhận saga ở module `game`, module `learning`) · **Mốc**: M-C3 · **Phụ thuộc**: F011, F014, F015
**Input**: Khóa/mở khóa/vô hiệu hóa tài khoản và thu hồi truy cập ở REST, realtime, Keycloak trong giới hạn thời gian; lan truyền trạng thái từ SIS; worker dọn (soft-delete) học sinh lâu không vào game có báo trước, loại trừ, ngưỡng duyệt, khôi phục; ẩn danh hóa theo retention/yêu cầu, không xóa vật lý. FR nguồn: FR-AUTH-010, FR-AUTH-024, FR-SIS-008, FR-ORG-014, NFR-PRIV-002, NFR-PRIV-003.

## Clarifications

### Session 2026-11-16

- Q: School Admin khóa học sinh bằng endpoint/permission nào (US-AUTH-003)? → A: `lockUser` hiện chỉ ở `/admin/*` (System Admin). Thêm `POST /tenant/users/{user_id}:lock|:unlock` với permission mới `tenant.user.lock` (gán cho `tenant_admin`, `school_admin` scope trường) [Proposed]; delta catalog ADMIN_PORTAL §2.2.
- Q: Saga dọn kiểm "không có game session active / không đang làm bài kiểm tra / không có case mở" thế nào khi identity không đọc DB module khác? → A: [Proposed] Gọi `cleanupStudentData` của module tham gia **trước** khi soft-delete ở identity; module trả `status = blocked` kèm `blocked_reason` (`active_game_session`, `active_match`, `open_exam_attempt`, `open_moderation_case` — đã có trong `StudentLifecycleResult`) thì saga bù (`restoreStudentData` cho bước đã `done`) và tính học sinh vào `skipped_active`. Khác thứ tự trong ACCOUNT_PROVISIONING §6b (identity trước) — cần cập nhật tài liệu.
- Q: Học sinh bị dọn đăng nhập lại để khôi phục, trong khi ACCOUNT_PROVISIONING §6b cũ yêu cầu `enabled=false` trên Keycloak (password grant sẽ thất bại)? → A: Đã chốt (Technical Architect 2026-11-16): khi dọn **giữ user Keycloak `enabled`** (và realm role `student`), gỡ membership tenant (Keycloak Organization của tenant + membership trong identity), thu hồi mọi phiên Keycloak và refresh token, soft-delete hồ sơ (ADR-0028, ADR-0032). Học sinh đăng nhập lại → password grant thành công, module identity (client duy nhất có password grant) quyết định khôi phục hay từ chối (FR-AUTH-024, AC-AUTH-018). ACCOUNT_PROVISIONING §6b và TC-AUTH-020 được sửa theo hướng này.
- Q: Ẩn danh hóa có xóa user Keycloak (IDENTITY_MAPPING §5, DATA_PRIVACY §4)? → A: Không xóa vật lý (ADR-0070, DATA_PRIVACY §3): vô hiệu hóa và ẩn danh hóa user Keycloak; ExternalIdentity giữ dòng (chỉ chứa UUID).

### Session 2026-11-17

- Q: Mã lỗi khi vượt phạm vi trường/môn của UserRole? → A: `403 PERMISSION_DENIED`; `FORBIDDEN_SCOPE` không phát và sẽ bị gỡ khỏi `ProblemCode` ([API_DESIGN §4.1](../../docs/07-backend/API_DESIGN.md), RB-113).
- Q: Ai khôi phục học sinh từ Thùng rác tenant? → A: Chỉ Tenant Admin (`tenant.trash.read/restore`, [ADMIN_PORTAL §3.8](../../docs/04-admin-portal/ADMIN_PORTAL.md), RB-112); School Admin không có Thùng rác.
- Q: Consumer thu hồi phiên game/kết thúc session thuộc feature nào khi F013/F014 (M-C1) làm trước F016 (M-C3)? → A: Đã chốt theo [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý): payload schema `user.access_revoked`, `user.status_changed`, `student.cleaned_up` là contract-first (F013 T045, F016 duyệt nội dung); F013/F014 hiện thực consumer và test bằng fixture (F013-AC-29, F014-AC-24); F016 phát event đúng schema và chịu kiểm end-to-end (F016-AC-24).
- Q: Học sinh quên mật khẩu (kể cả tenant `common`) thì ai đặt lại, mật khẩu giao thế nào? → A (Technical Architect 2026-11-17): `resetStudentPassword` thuộc F016 — School Admin/Tenant Admin/System Admin (permission `lms.student.password_reset`), mật khẩu sinh ngẫu nhiên trả một lần, thu hồi phiên game (`password_changed`), phát `user.access_revoked` (`reason = password_changed`, không tắt user); quy tắc ở [ACCOUNT_PROVISIONING §6d](../../docs/09-identity/ACCOUNT_PROVISIONING.md#6d-đặt-lại-mật-khẩu-học-sinh) (F016-AC-25…27).

### Session 2026-11-19 (triển khai — câu hỏi mở, mặc định [Proposed])

- OQ F016-1: Lý do/nguồn của mỗi lần đổi trạng thái lưu ở bảng append-only `identity.user_status_history` (`source`: staff, sis, cleanup, restore, anonymize, retention) thay vì thêm cột `users.status_reason` [Proposed].
- OQ F016-2: Facade `cleanupStudentData`/`restoreStudentData`/`anonymizeUserData` của module game và learning trả `nothing_to_do` vì chưa có dữ liệu riêng của học sinh (game chỉ trả `blocked: active_game_session` khi có session active). Feature thêm dữ liệu (F030+, F070+) mở rộng các operation này; competition/chat tham gia saga ở F100/F115.
- OQ F016-3: Gỡ/thêm membership Keycloak Organization khi dọn/khôi phục đi qua `tenant_membership.changed` (worker membership F010), không gọi Keycloak trực tiếp từ saga [Proposed].
- OQ F016-4: Bảng của F016 (saga, dọn, loại trừ, yêu cầu dữ liệu, erasure log, lịch sử trạng thái) viết bằng SQL tay trong migration (có RLS) thay vì ent schema; các use case đọc của dọn/yêu cầu dữ liệu (preview, danh sách lần chạy, danh sách yêu cầu) nằm ở command, đọc trong platform/tenant scope [Proposed].
- OQ F016-5: `Idempotency-Key` của mỗi bước saga là UUIDv7 tất định (thời điểm bắt đầu saga + hash bước); TTL idempotency của facade giới hạn lần thử lại muộn [Proposed].
- OQ F016-6: `resetStudentPassword` thu hồi phiên game trong transaction riêng sau khi ghi event [Proposed].
- OQ F016-7: Binary chưa nối route/consumer/job của F016 (OQ F010-2). F016-AC-24 đã kiểm từng chặng (payload đọc từ outbox đúng schema và đưa vào consumer thật; consumer F013/F014 có test fixture); kiểm end-to-end core-api + worker làm khi nối binary.
  - 2026-11-20 [Proposed]: binary đã nối theo OQ F010-2 (specs/010-tenant-org-structure/spec.md, phiên "nối binary"): `module.go` của module khai báo route/relay/consumer/job, `cmd/internal/wiring.Compose` ghép module.
- OQ F016-8: "Còn hợp lệ trong SIS" khi khôi phục lúc đăng nhập = link SIS active của user trỏ tới bản ghi `sis.synced_students` `record_status = active`; user không có link SIS (tenant chung, tài khoản tạo tay) coi là hợp lệ [Proposed].
- OQ F016-9: Tenant Admin khôi phục từ Thùng rác không kiểm SIS (người dùng quyết định) [Proposed].
- OQ F016-10: Realm đặt `editUsernameAllowed: false` nên khi ẩn danh hóa, user Keycloak bị tắt và xóa email/họ tên/thuộc tính nhưng giữ username; username trong `identity.users` đổi thành `anon.<user_id>`. Cần quyết định có bật sửa username hay đặt quy ước username không chứa tên thật.
- OQ F016-11: `createDataRequest` do System Admin (MFA) ghi nhận được coi là đã duyệt và chạy saga ngay (không có bước duyệt riêng trong contract); `request_type = export` trả `422` vì xuất dữ liệu ngoài phạm vi (ADR-0082). Ánh xạ trạng thái: pending→received, approved→in_progress, executed→completed.
- OQ F016-12: Job `retention-anonymizer` tính `student_data_retention` (365 ngày, CONFIG_DEFAULTS) từ lần đổi sang `deactivated` gần nhất trong lịch sử trạng thái, không có lịch sử thì dùng `users.updated_at`; mỗi lần chạy tối đa 100 học sinh [Proposed].
- OQ F016-13: Khi khôi phục lúc đăng nhập, identity được khôi phục trước; nếu facade một module tạm lỗi, đăng nhập vẫn thành công, saga thử lại bước đó và module tự khôi phục khi nhận `student.restored` (ACCOUNT_PROVISIONING §6b.1).

### Session 2026-11-20 (bổ sung sau F057 — [Proposed], chờ Technical Architect duyệt)

- Q: Hủy lần dọn đang `awaiting_approval` (OQ F057-2, Clarification 2026-11-16 của F057)? → A: [Proposed] Thêm `cancelStudentCleanupRun` (`POST /admin/student-cleanup/runs/{run_id}:cancel`, `platform.student_cleanup.approve`, `Idempotency-Key`, body `{reason}` 3..500 ký tự, `204`; run không ở `awaiting_approval` → `409 CONFLICT`, không có → `404`). Run chuyển `cancelled` (`finished_at` = lúc hủy), audit `student_cleanup.cancelled` kèm lý do; không học sinh nào bị dọn; lần chạy job sau tạo lần dọn mới theo cấu hình hiện hành. `StudentCleanupRun.status` thêm `cancelled`; CHECK của `platform.student_cleanup_runs` mở rộng bằng migration `20261127000001_f016_cleanup_run_cancel.sql` (expand). Portal dùng chung action `action.platform_admin.system_settings.approve_cleanup` (ADR-0090) với `approveStudentCleanupRun`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Khóa, mở khóa, vô hiệu hóa và thu hồi truy cập (Priority: P1)

School Admin khóa học sinh đã nghỉ học (US-AUTH-003); học sinh không vào được hệ thống, phiên đang mở bị cắt; mở khóa thì đăng nhập lại được.

**Why this priority**: An toàn tài khoản; AC-AUTH-006 là tiêu chí P1.

**Independent Test**: `ad.truong.a` khóa `hs.binh.3a` đang chơi → REST `403 ACCOUNT_LOCKED` ≤ 60 s, game session kết thúc `account_locked`, Keycloak `enabled=false` ≤ 1 phút.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F016-AC-01 | KHI người có quyền khóa tài khoản (kèm `reason`, `Idempotency-Key`), module identity PHẢI đặt `locked` và ghi audit, phát `user.status_changed` + `user.access_revoked` trong cùng transaction | FR-AUTH-010 · US-AUTH-003 · [ACCOUNT_PROVISIONING §6](../../docs/09-identity/ACCOUNT_PROVISIONING.md) | TC-AUTH-006 |
| F016-AC-02 | KHI tài khoản bị khóa/vô hiệu hóa, mọi module PHẢI trả `403 ACCOUNT_LOCKED`/`403 ACCOUNT_DEACTIVATED` cho request REST mới trong ≤ 60 giây | FR-AUTH-010 · AC-AUTH-006 | TC-AUTH-006 |
| F016-AC-03 | KHI nhận `user.access_revoked` (trừ `reason = password_changed`, F016-AC-26), worker identity PHẢI đặt user Keycloak `enabled=false` và thu hồi mọi phiên trong ≤ 1 phút (retry có backoff, reconciliation bảo đảm hội tụ); kết thúc game session (F014-AC-24) và thu hồi phiên game (F013-AC-29) do consumer của F014/F013, kiểm end-to-end ở F016-AC-24 | FR-AUTH-010 · ADR-0031 | TC-AUTH-006 |
| F016-AC-04 | KHI mở khóa, module identity PHẢI đặt `active`, phát `user.access_restored`, worker PHẢI bật lại user Keycloak; học sinh đăng nhập lại được | FR-AUTH-010 | — |
| F016-AC-05 | NẾU School Admin khóa người ngoài trường mình, người dùng tenant khác, hoặc nhân sự, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED`/`404`; thiếu `reason` PHẢI trả `422 VALIDATION_FAILED` | FR-AUTH-010 · NFR-SEC-006 | TC-TEN-003 |
| F016-AC-06 | HỆ THỐNG PHẢI hiển thị cho học sinh lý do khóa ở dạng trung tính; chi tiết lý do chỉ nhân sự trong scope xem được | FR-AUTH-010 · AGENTS R16 | — |
| F016-AC-25 | KHI School Admin của trường học sinh, Tenant Admin của tenant hoặc System Admin (tenant đích qua `X-Tenant-Id`, gồm tenant `common`) gọi `resetStudentPassword` kèm `reason` và `Idempotency-Key`, module identity PHẢI sinh mật khẩu ngẫu nhiên, đặt qua Keycloak Admin API với `temporary` theo khối (6–9 `true`, 1–5 `false` [Proposed]), trả đúng một lần trong `initial_password` với `Cache-Control: no-store`, không lưu/log mật khẩu, ghi audit; replay PHẢI không có `initial_password` và có `password_redacted = true` | FR-AUTH-017 · AC-AUTH-028 · AGENTS R14 · [ACCOUNT_PROVISIONING §6d](../../docs/09-identity/ACCOUNT_PROVISIONING.md#6d-đặt-lại-mật-khẩu-học-sinh) | TC-AUTH-087 |
| F016-AC-26 | KHI đặt lại mật khẩu học sinh thành công, module identity PHẢI thu hồi mọi `game_auth_sessions` của học sinh với `revoke_reason = password_changed` và phiên Keycloak theo `sid`, phát `user.access_revoked` (`reason = password_changed`) trong cùng transaction; consumer PHẢI không đặt Keycloak `enabled=false` và không đổi `users.status`; refresh/resume kế tiếp PHẢI nhận `401 REFRESH_TOKEN_INVALID` và kết nối realtime PHẢI đóng `4403` sau `session.session.ended` (`password_changed`) | FR-AUTH-008 · AC-AUTH-028 · ADR-0086 · [ACCOUNT_PROVISIONING §6d](../../docs/09-identity/ACCOUNT_PROVISIONING.md#6d-đặt-lại-mật-khẩu-học-sinh) | TC-AUTH-087 |
| F016-AC-27 | NẾU người gọi là học sinh, hoặc School Admin gọi cho học sinh trường khác/tenant khác, THÌ HỆ THỐNG PHẢI trả `403`/`404` và không đổi mật khẩu; thiếu `reason` PHẢI trả `422 VALIDATION_FAILED` | FR-AUTH-017 · NFR-SEC-006 · [ACCOUNT_PROVISIONING §6d](../../docs/09-identity/ACCOUNT_PROVISIONING.md#6d-đặt-lại-mật-khẩu-học-sinh) | TC-TEN-001 |

---

### User Story 2 — Lan truyền trạng thái từ SIS (Priority: P1)

SIS báo đình chỉ/nghỉ học/chuyển đi; tài khoản bị chặn và phiên hiện tại bị thu hồi trong một chu kỳ đồng bộ.

**Why this priority**: SIS là nguồn sự thật trạng thái học sinh; FR-SIS-008 P1.

**Independent Test**: Import `status=transferred_out` cho học sinh đang chơi → phiên hiện tại bị thu hồi ≤ 60 s, lần tạo game session sau bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F016-AC-07 | KHI apply SIS (F011) phát `user.access_revoked` do `suspended`/`inactive`/`graduated`/`transferred_out`, HỆ THỐNG PHẢI áp cùng chuỗi thu hồi như US1 (cache, Keycloak, game session) và hoàn tất trong một chu kỳ đồng bộ | FR-SIS-008 · FR-AUTH-010 | TC-AUTH-036 |
| F016-AC-08 | KHI học sinh `transferred_out` đăng nhập hoặc tạo game session tiếp theo, HỆ THỐNG PHẢI từ chối (`NO_ACTIVE_ENROLLMENT` hoặc `ACCOUNT_NOT_PROVISIONED` theo membership) | FR-SIS-008 · AC-AUTH-013 | TC-AUTH-036 |

---

### User Story 3 — Dọn học sinh không đăng nhập game (Priority: P2)

Worker hằng ngày soft-delete học sinh quá số ngày cấu hình không vào game; nhà trường thấy danh sách "sắp bị dọn", loại trừ hoặc gia hạn; lần dọn lớn cần System Admin duyệt.

**Why this priority**: FR-AUTH-024 P2; giảm dữ liệu cá nhân đang xử lý tích cực.

**Independent Test**: Tenant cấu hình 90 ngày; HS A `last_login_at` 91 ngày, HS B chưa từng đăng nhập (`provisioned_at` 95 ngày), HS C loại trừ, HS D đang có session → chỉ A, B bị soft-delete; không câu `DELETE` nào chạy.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F016-AC-09 | KHI worker chạy theo lịch, HỆ THỐNG PHẢI chọn học sinh có `COALESCE(last_login_at, provisioned_at) + số ngày hiệu lực < now()` (số ngày hiệu lực từ cài đặt tenant/platform, `0`/`null` = tắt), không bị loại trừ còn hạn | FR-AUTH-024 · ADR-0032 | TC-AUTH-020 |
| F016-AC-10 | KHI học sinh vào cửa sổ báo trước, worker PHẢI phát `student.cleanup_scheduled` (theo lô) và học sinh PHẢI xuất hiện trong `previewTenantStudentCleanup` với ngày dự kiến dọn | FR-AUTH-024 | — |
| F016-AC-11 | KHI Tenant Admin/School Admin có `tenant.student_cleanup.exclude` loại trừ hoặc gia hạn (bắt buộc `reason`), module identity PHẢI lưu loại trừ, ghi audit và lần chạy kế tiếp PHẢI bỏ qua học sinh đó | FR-AUTH-024 | — |
| F016-AC-12 | NẾU số học sinh sẽ dọn trong một lần chạy vượt ngưỡng duyệt của `inactive_student_cleanup_days`, THÌ worker PHẢI dừng ở `awaiting_approval`, phát cảnh báo `StudentCleanupLargeBatch` và chỉ dọn sau khi System Admin có `platform.student_cleanup.approve` xác nhận | FR-AUTH-024 · ADR-0032 | TC-AUTH-021 |
| F016-AC-13 | KHI dọn một học sinh, module identity PHẢI chạy saga (`Idempotency-Key = <run_id>:<student_id>`): gọi `cleanupStudentData` của các module tham gia, nếu mọi bước `done` thì soft-delete User/Student/membership với `delete_reason = inactive_cleanup`, gỡ user khỏi Keycloak Organization của tenant, thu hồi mọi phiên Keycloak + refresh token nhưng **giữ user Keycloak `enabled`** (không đặt `enabled=false`), phát `student.cleaned_up`; request bằng token cũ PHẢI bị từ chối trong ≤ 60 giây; không bước nào xóa vật lý | FR-AUTH-024 · AC-AUTH-018 · ADR-0028 · ADR-0036 · [ACCOUNT_PROVISIONING §6b](../../docs/09-identity/ACCOUNT_PROVISIONING.md) | TC-AUTH-020 |
| F016-AC-14 | NẾU một module trả `blocked` (đang có session, trận, bài kiểm tra hoặc case mở), THÌ saga PHẢI bù các bước đã `done` bằng `restoreStudentData`, giữ học sinh nguyên trạng và tính vào `skipped_active` | FR-AUTH-024 | TC-SVC-009 |
| F016-AC-15 | NẾU bước gọi một module tạm lỗi, THÌ saga PHẢI thử lại có backoff, không chạy bước nào hai lần (idempotent), và cảnh báo `SagaStuck` khi kẹt quá ngưỡng | FR-AUTH-024 · ADR-0042 | TC-SVC-009 |
| F016-AC-16 | HỆ THỐNG PHẢI cho `previewTenantStudentCleanup`, `createStudentCleanupExclusions`, `deleteStudentCleanupExclusions` chỉ trả/ghi dữ liệu của tenant phiên (School Admin: trường mình); tenant khác `404` | FR-AUTH-024 · NFR-SEC-006 | TC-TEN-001 |

---

### User Story 4 — Khôi phục học sinh bị dọn (Priority: P2)

Học sinh bị dọn quay lại và vẫn hợp lệ trong SIS được khôi phục đủ dữ liệu cũ; SIS sync không tự khôi phục.

**Why this priority**: Dọn nhầm qua kỳ nghỉ không được làm mất dữ liệu học tập.

**Independent Test**: Học sinh bị dọn đăng nhập lại (còn active trong SIS) → `student.restored`, dữ liệu game/học tập dùng lại; SIS full sync trước đó chỉ tạo issue.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F016-AC-17 | KHI `(iss, sub)` trỏ tới học sinh soft-delete với `inactive_cleanup` và bản ghi SIS còn `active` có enrollment active (tenant chung: không cần SIS), module identity (Keycloak xác thực thành công vì user vẫn `enabled`) PHẢI không trả `ACCOUNT_NOT_PROVISIONED` mà chạy saga khôi phục (identity bỏ `deleted_at`, thêm lại membership tenant và membership Keycloak Organization, `restoreStudentData` các service) và phát `student.restored` | FR-AUTH-024 · AC-AUTH-018 | TC-AUTH-022 |
| F016-AC-18 | NẾU bản ghi SIS không còn hợp lệ, THÌ module identity PHẢI trả `403 ACCOUNT_DEACTIVATED` và không khôi phục | FR-AUTH-024 | — |
| F016-AC-19 | KHI Tenant Admin (`tenant.trash.restore`; School Admin và role tenant khác không có Thùng rác) khôi phục học sinh từ Thùng rác tenant (`restoreTenantTrashItem`, `resource_type = student`), module identity PHẢI chạy cùng saga khôi phục và ghi audit; School Admin gọi PHẢI nhận `403 PERMISSION_DENIED` | FR-AUTH-024 · ADR-0028 | TC-TEN-015 |

---

### User Story 5 — Ẩn danh hóa theo retention và yêu cầu (Priority: P3)

Hết thời gian lưu giữ hoặc có yêu cầu qua nhà trường, dữ liệu cá nhân được ẩn danh hóa ở mọi module; không xóa vật lý.

**Why this priority**: FR-ORG-014 P5, NFR-PRIV-003; cần trước production nhưng không chặn M-C3.

**Independent Test**: Tạo yêu cầu ẩn danh hóa cho học sinh đã `deactivated` → saga hoàn tất ở identity, learning, game; `status = anonymized`, username `anon.<user_id>`, nhật ký erasure có dòng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F016-AC-20 | KHI yêu cầu ẩn danh hóa được duyệt hoặc học sinh `deactivated` quá thời hạn lưu giữ theo DATA_PRIVACY §2, module identity PHẢI chạy saga `user_anonymize`: xóa trắng PII (`display_name`, `email`, `full_name`, `student_code`, `date_of_birth_enc`, `national_id_hash`), đặt username `anon.<user_id>`, `status = anonymized`, vô hiệu hóa và ẩn danh hóa user Keycloak, gọi `anonymizeUserData` các service, ghi nhật ký erasure, phát `user.anonymized` | FR-ORG-014 · NFR-PRIV-003 · ADR-0070 | — |
| F016-AC-21 | HỆ THỐNG PHẢI không xóa vật lý bất kỳ dòng nào khi ẩn danh hóa; yêu cầu chỉ hoàn tất khi mọi module trả `done` | FR-ORG-014 · ADR-0028 | TC-TEN-016 |
| F016-AC-22 | HỆ THỐNG PHẢI chỉ cho `platform.data_request.write` tạo/xem yêu cầu dữ liệu (`X-Tenant-Id` = tenant đích khi ghi) và ghi audit với mã yêu cầu | FR-ORG-014 · ADR-0027 | TC-TEN-004 |
| F016-AC-24 | KHI khóa thật (US1), SIS phát thu hồi (US2) hoặc dọn thật (US3) một học sinh đang có game session và phiên game, HỆ THỐNG PHẢI end-to-end: phát `user.status_changed`/`user.access_revoked`/`student.cleaned_up` đúng payload schema contract-first (F013 T045), module game kết thúc session với `account_locked`/`cleanup` (consumer F014-AC-24), module identity thu hồi phiên game và refresh kế tiếp trả `401 REFRESH_TOKEN_INVALID` (consumer F013-AC-29), trong ≤ 1 phút | FR-AUTH-010 · FR-AUTH-024 · ADR-0031 · ADR-0086 · [CODE_PLAN §4.1](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md#41-phụ-thuộc-ẩn-và-đề-xuất-xử-lý) | TC-AUTH-006 |
| F016-AC-23 | HỆ THỐNG PHẢI giữ event và projection của feature chỉ gồm định danh và trạng thái tối thiểu (không họ tên, ngày sinh, mã học sinh) | NFR-PRIV-002 · AGENTS R14 | — |

### Edge Cases

- NẾU học sinh vừa bị dọn vừa được SIS gửi lại, THÌ không tự khôi phục; issue `cleaned_up_student_in_sis` (F011).
- NẾU Keycloak lỗi khi gỡ membership Organization/thu hồi phiên lúc dọn, THÌ hồ sơ vẫn ở trạng thái soft-delete (identity từ chối mọi token), worker thử lại có backoff; không bao giờ chuyển sang `enabled=false`.
- NẾU Keycloak lỗi khi thu hồi, THÌ REST vẫn chặn nhờ trạng thái DB + cache ≤ 60 s; job reconciliation đồng bộ lại Keycloak.
- NẾU khóa và dọn cùng lúc, THÌ trạng thái `locked` được giữ; dọn bỏ qua tài khoản `locked`.
- NẾU chat/module competition chưa triển khai (trước M4), THÌ saga chỉ gồm module đã được nối facade; module mới phải hiện thực operation facade trước khi được thêm vào saga.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-010 | Khóa/vô hiệu hóa, chặn REST, cắt realtime qua `user.access_revoked`, Keycloak disable | [ACCOUNT_PROVISIONING §6](../../docs/09-identity/ACCOUNT_PROVISIONING.md) |
| FR-AUTH-024 | Worker dọn học sinh (giữ Keycloak `enabled`, gỡ membership, thu hồi phiên), báo trước, loại trừ, ngưỡng duyệt, khôi phục khi đăng nhập lại | [ACCOUNT_PROVISIONING §6b](../../docs/09-identity/ACCOUNT_PROVISIONING.md) |
| FR-SIS-008 | Lan truyền khóa/vô hiệu hóa từ SIS trong một chu kỳ | [SIS_INTEGRATION §4.4](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-ORG-014 | Nghĩa vụ xóa = ẩn danh hóa; không xóa vật lý | [DATA_PRIVACY §4](../../docs/10-security/DATA_PRIVACY.md) |
| NFR-PRIV-002 | Data minimization ở event/projection | [DATA_PRIVACY §1](../../docs/10-security/DATA_PRIVACY.md) |
| NFR-PRIV-003 | Retention job tự động ẩn danh hóa | [DATA_PRIVACY §2](../../docs/10-security/DATA_PRIVACY.md) |

### Key Entities

- **UserStatus**: `pending`/`active`/`locked`/`deactivated`/`anonymized` với lý do và thời điểm.
- **CleanupExclusion**: loại trừ/gia hạn học sinh khỏi đợt dọn, có hạn và lý do.
- **StudentCleanupRun**: một lần chạy dọn của tenant: chế độ, trạng thái, số ứng viên/đã dọn/loại trừ/bỏ qua, người duyệt.
- **SagaRun, SagaStep**: điều phối thao tác liên module (dọn, khôi phục, ẩn danh hóa) với tiến độ từng bước.
- **DeletionRequest, ErasureLog**: yêu cầu ẩn danh hóa và nhật ký thực hiện (áp lại sau restore backup).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `inactive_student_cleanup_days` | Số ngày dọn, báo trước, giờ chạy, ngưỡng duyệt |
| `principal_cache`, `principal_cache_ttl` | Độ trễ thu hồi quyền ở REST |
| `idempotency_record_ttl` | Khóa idempotency cho lock/unlock |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Chỉ soft-delete/ẩn danh hóa; không `DELETE` | AGENTS R21, R24 · ADR-0028 · ADR-0070 |
| Số ngày dọn từ cài đặt platform/tenant, không hardcode/env | AGENTS R24, R33 |
| Gọi module khác chỉ qua facade (`src/backend/pkg/modcall`), idempotent | AGENTS R29 · ADR-0074 |
| Không log PII; event tối thiểu | AGENTS R14 · NFR-PRIV-002 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Tài khoản bị khóa không thực hiện được request REST nào sau 60 giây và mất kết nối realtime sau 5 giây (khi có realtime).
- **SC-002**: 0 học sinh bị loại trừ hoặc đang chơi bị dọn trong bộ kịch bản kiểm thử.
- **SC-003**: 100% học sinh bị dọn đủ điều kiện quay lại được khôi phục với dữ liệu học tập/game không đổi.
- **SC-004**: 0 câu `DELETE`/`TRUNCATE` được thực thi bởi feature (kiểm bằng log truy vấn trong test).

## Ngoài phạm vi

- Phát trạng thái từ dữ liệu SIS → F011; thùng rác liên tenant và saga clone → F045.
- Đóng WebSocket theo `user.access_revoked` → F090; rời party/channel → F100, F115 (cung cấp `cleanupStudentData`/`restoreStudentData` của competition/chat).
- Màn khóa tài khoản, "Học sinh sắp bị dọn", duyệt lần dọn trên portal → F052, F057.
- Xuất dữ liệu học sinh; đánh giá pháp lý ẩn danh hóa — deferred (ADR-0082).

## Assumptions

- Tên tham số thời hạn lưu giữ (học sinh deactivated, login events) chưa có trong CONFIG_DEFAULTS; giá trị lấy theo DATA_PRIVACY §2 — task đăng ký.
- Engine saga (`platform.saga_runs`, `saga_steps`) do feature đầu tiên cần nó tạo (F016 hoặc F045), feature kia dùng lại.
- Permission `tenant.user.lock` [Proposed] chờ cập nhật ADMIN_PORTAL §2.2.
