# Feature Specification: F011 — Đồng bộ SIS bằng CSV và provisioning tài khoản

**Feature Branch**: `011-sis-sync-provisioning` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `identity` · **Mốc**: M-C1 · **Phụ thuộc**: F010
**Input**: Nhận dữ liệu trường/lớp/học sinh/enrollment/trạng thái bằng file CSV (cột = tên trường DTO của EduWorld SIS Contract v1), validate → staging → reconciliation → apply, tạo tài khoản Keycloak cho học sinh, định danh duy nhất, login sync. FR nguồn: FR-SIS-002, FR-SIS-003 (phần `csv_import`), FR-SIS-005…FR-SIS-008, FR-ORG-018, FR-AUTH-005, FR-AUTH-014, FR-AUTH-031.

## Clarifications

### Session 2026-11-16

- Q: `contract_v1` có phải làm ở M-C1? → A: Đã chốt (Technical Architect 2026-11-16): F011 chỉ gồm `csv_import` + provisioning (M-C1). Provider API `contract_v1` (pull full/delta, `lookupStudent`), webhook, conformance kit và `sis-mock` tách sang F017 (M2).
- Q: Danh sách trường của connection lưu mảng hay bảng? → A: Bảng `sis_connection_schools` (ADR-0046, SIS_INTEGRATION §6.1); bỏ cột mảng `school_ids` (delta DATABASE_DESIGN).
- Q: Ai sở hữu khóa/vô hiệu hóa lan sang Keycloak và realtime? → A: F011 đặt trạng thái và phát event khi apply; Keycloak disable, cắt WS, game session do F016.
- Q: Hợp nhất hai tài khoản (`POST /admin/users:merge`)? → A: Thuộc F045; F011 chỉ tạo issue `duplicate_identifier` và hỗ trợ "thêm membership" cho cùng một người.
- Q: Login sync nhánh B (tra cứu chặn ngắn) khi trường chỉ dùng CSV? → A: F011 hiện thực logic nhánh A/B/C sau port `SisLookupPort`; connection `csv_import` không có capability `student_lookup` nên nhánh B rơi về dữ liệu đồng bộ gần nhất (F011-AC-20). Adapter HTTP `lookupStudent` của `contract_v1` do F017 cung cấp.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — School Admin nhập CSV theo contract (Priority: P1)

School Admin (`ad.truong.a`) tải file `schools.csv`, `classes.csv`, `students.csv`, `enrollments.csv` (cột = tên trường DTO), xem dry-run diff rồi áp dụng (US-SIS-001).

**Why this priority**: Trường chưa có API dùng CSV; là đường duy nhất M-C1 có học sinh thật.

**Independent Test**: Upload bộ CSV trường demo với `dry_run=true` → diff; áp dụng → có trường/lớp/học sinh/enrollment, `sync_run` `succeeded`, event `student.synced`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F011-AC-01 | KHI School Admin gửi import CSV kèm `dry_run=true`, module identity PHẢI validate bằng JSON Schema trích từ contract, trả diff (create/update/noop/deactivate) và không thay đổi dữ liệu nghiệp vụ | FR-SIS-003 · [SIS_INTEGRATION §4.3](../../docs/09-identity/SIS_INTEGRATION.md) | TC-AUTH-011 |
| F011-AC-02 | NẾU bản ghi thiếu trường bắt buộc, sai kiểu hoặc sai enum, THÌ module identity PHẢI từ chối bản ghi đó với `SIS_CONTRACT_VIOLATION` (số dòng, cột) và, ở chế độ CSV, không áp dụng file chứa dòng lỗi (all-or-nothing theo file) | FR-SIS-003 · FR-SIS-006 | TC-AUTH-011 |
| F011-AC-03 | KHI bản ghi có cột ngoài contract (kể cả giáo viên, phân công giảng dạy), module identity PHẢI bỏ qua, không lưu và tăng metric trường lạ | FR-SIS-002 · ADR-0021 | TC-SIS-107 |
| F011-AC-04 | KHI áp dụng, module identity PHẢI apply theo thứ tự school → class → student → enrollment, lô 500 bản ghi idempotent theo `raw_hash`, phát `school.synced`, `class.synced`, `student.synced`, `enrollment.changed` qua outbox | FR-SIS-002 · [SIS_INTEGRATION §6.2](../../docs/09-identity/SIS_INTEGRATION.md) | TC-SVC-008 |
| F011-AC-05 | KHI import lại cùng file, module identity PHẢI cho 0 bản ghi `update` | FR-SIS-003 | — |
| F011-AC-06 | HỆ THỐNG PHẢI chỉ cho School Admin có `tenant.sis.import` trong scope trường gọi import; file vượt `csv_import_max` PHẢI bị từ chối `422 VALIDATION_FAILED`; mọi import PHẢI có `Idempotency-Key` và audit `actor = system:sis-sync:<connection_id>` cho từng thay đổi (không ghi PII đầy đủ) | FR-SIS-006 · FR-SIS-007 | — |
| F011-AC-07 | NẾU School Admin tenant A gọi `getSisImport` của tenant B hoặc trường khác cùng tenant, THÌ HỆ THỐNG PHẢI trả `404` | NFR-SEC-006 | TC-TEN-001 |

---

### User Story 2 — Provisioning tài khoản học sinh và định danh duy nhất (Priority: P1)

Học sinh mới từ SIS có tài khoản Keycloak local (role `student`, member đúng một Organization), ExternalIdentity `(issuer, subject)` và liên kết SIS nguồn; username/email trùng không bao giờ tạo tài khoản thứ hai.

**Why this priority**: Không có tài khoản thì học sinh không đăng nhập được (F013).

**Independent Test**: Apply học sinh mới `hs.thdemo.000123` → Keycloak user tồn tại, `external_identities` và `sis_identity_links` active, event `user.provisioned`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F011-AC-08 | KHI apply học sinh mới chưa có link, module identity PHẢI tạo User `pending`, phát `student.provisioning_requested`; worker PHẢI tạo Keycloak user (không gửi email học sinh), gán role `student`, thêm member Organization của tenant, ghi `external_identities(issuer, subject)` và `sis_identity_links` khóa `(sis_connection_id, student_external_id)`, phát `user.provisioned` | FR-AUTH-005 · [ACCOUNT_PROVISIONING §2](../../docs/09-identity/ACCOUNT_PROVISIONING.md) | TC-AUTH-030 |
| F011-AC-09 | NẾU Keycloak trả 409 khi tạo user, THÌ worker PHẢI tìm theo attribute `eduworld_user_id`; khớp thì liên kết, không khớp thì tạo issue `duplicate_identifier` | FR-AUTH-031 · AC-AUTH-026 | TC-AUTH-074 |
| F011-AC-10 | HỆ THỐNG PHẢI coi `username`/`email` duy nhất toàn hệ thống sau chuẩn hóa trim, NFC, chữ thường; bản ghi SIS trùng với tài khoản **người khác** (kể cả tenant khác, tài khoản soft-delete chưa ẩn danh hóa) PHẢI không được tạo/ghi đè và tạo issue `duplicate_identifier` | FR-AUTH-031 · ADR-0069 | TC-AUTH-073 |
| F011-AC-11 | KHI issue `duplicate_identifier` được xác nhận là cùng một người, module identity PHẢI thêm membership tenant mới vào tài khoản sẵn có, không tạo tài khoản thứ hai, ghi audit | FR-AUTH-031 · AC-AUTH-026 | TC-AUTH-080 |
| F011-AC-12 | HỆ THỐNG PHẢI khóa định danh ngoài bằng `(issuer, subject)` với `issuer` thuộc allowlist cấu hình; không dùng email/tên hiển thị/`preferred_username` làm khóa | FR-AUTH-005 · FR-AUTH-014 | TC-AUTH-009 |
| F011-AC-13 | KHI hai SIS gửi cùng `student_external_id` cho hai học sinh khác nhau, module identity PHẢI tạo hai bản ghi và hai link riêng theo khóa nguồn | FR-AUTH-005 · ADR-0046 | TC-AUTH-030 |

---

### User Story 3 — Xử lý reconciliation issue (Priority: P1)

School Admin xem các bản ghi xung đột (trùng, thiếu lớp, chuyển trường cần xác nhận…) và xử lý (US-SIS-002).

**Why this priority**: Dữ liệu sai trường đưa học sinh vào sai tenant; phải có người duyệt.

**Independent Test**: Import học sinh không có lớp → issue `missing_class`; resolve issue sau khi bổ sung lớp → issue `resolved`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F011-AC-14 | KHI reconciler gặp tình huống trong conflict matrix (`possible_duplicate`, `transfer_confirmation`, `missing_class`, `class_school_mismatch`, `orphan_enrollment`, `multi_school_enrollment`, `school_province_missing`, `unmapped_school`, `cleaned_up_student_in_sis`, `duplicate_identifier`), module identity PHẢI tạo issue, bỏ qua bản ghi và phát `sis.reconciliation_issue_raised` | FR-SIS-005 · [SIS_INTEGRATION §6.3](../../docs/09-identity/SIS_INTEGRATION.md) | TC-ORG-106 |
| F011-AC-15 | KHI học sinh không có enrollment active sau apply, module identity PHẢI không tự gán lớp và tạo issue `missing_class` | FR-SIS-005 · FR-ORG-018 | TC-SIS-103 |
| F011-AC-16 | KHI chuyển trường giữa hai SIS cùng tenant `unit` với `national_student_id` hash trùng, module identity PHẢI chuyển link sang khóa nguồn mới, đánh dấu link cũ `superseded`, giữ nguyên User; khác tenant PHẢI tạo `transfer_confirmation` | FR-SIS-005 · ADR-0046 | TC-AUTH-035 |
| F011-AC-17 | KHI người có quyền resolve issue, module identity PHẢI lưu `resolved_by`, `resolved_at`, ghi audit; người ngoài scope trường PHẢI nhận `404` | FR-SIS-005 | — |

---

### User Story 4 — Đồng bộ khi đăng nhập (login sync) (Priority: P1)

Khi học sinh đăng nhập (F013 gọi), hệ thống bảo đảm có đủ trường/lớp/enrollment: đủ & mới thì không gọi SIS; thiếu thì tra cứu chặn ngắn; cũ thì làm mới nền.

**Why this priority**: Đăng nhập lần đầu phải hiển thị đúng trường/lớp mà không biến SIS thành điểm nghẽn.

**Independent Test**: Với fake `SisLookupPort` ghi số lời gọi (chỉ trong test): nhánh A 0 lời gọi; nhánh B gọi đúng connection; connection `csv_import` → không gọi, dùng dữ liệu gần nhất; nhánh C tạo một job `login_refresh`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F011-AC-18 | TRONG KHI học sinh có bản ghi đồng bộ đủ trường/lớp/enrollment và `last_synced_at` mới hơn `sis_login_refresh_after_hours`, module identity PHẢI không gọi SIS | FR-ORG-018 · ADR-0061 | TC-SIS-101 |
| F011-AC-19 | KHI thiếu trường/lớp/enrollment và connection nguồn có capability `student_lookup`, module identity PHẢI gọi `SisLookupPort.Lookup` của đúng connection (adapter `contract_v1` do F017) với timeout `sis_login_sync_timeout_ms`, qua validator + reconciliation + apply trước khi trả kết quả; vẫn thiếu lớp PHẢI trả kết quả `NO_ACTIVE_ENROLLMENT` | FR-ORG-018 · FR-SIS-005 | TC-SIS-102 |
| F011-AC-20 | NẾU lookup timeout, lỗi 5xx, connection không có `student_lookup` hoặc vượt `sis_login_sync_concurrency`, THÌ module identity PHẢI dùng dữ liệu đồng bộ gần nhất nếu đủ, nếu không trả `NO_ACTIVE_ENROLLMENT` kèm thông điệp thử lại, không retry đồng bộ | FR-ORG-018 · ADR-0045 | TC-SIS-104 |
| F011-AC-21 | KHI dữ liệu cũ hơn ngưỡng, module identity PHẢI tiếp tục đăng nhập ngay và xếp đúng một job `login_refresh` vào `IDENTITY_SIS_JOBS` (gộp trùng theo `Nats-Msg-Id` trong 10 phút) | FR-ORG-018 | TC-SIS-105 |
| F011-AC-22 | KHI 200 học sinh cùng nhánh B trên một connection, module identity PHẢI giới hạn lookup đồng thời theo `sis_login_sync_concurrency`, mở circuit breaker khi SIS lỗi và tăng `eduworld_sis_login_sync_total{result="throttled"}` | FR-ORG-018 · NFR-AVAIL-005 | TC-SIS-106 |

---

### User Story 5 — Trạng thái tài khoản từ SIS (Priority: P2)

SIS báo nghỉ học/ra trường/đình chỉ/chuyển đi; trạng thái EduWorld cập nhật trong một chu kỳ đồng bộ.

**Why this priority**: Học sinh nghỉ học không được tiếp tục vào hệ thống.

**Independent Test**: Import `status=suspended` cho `hs.chi.3b` → `users.status = locked`, phát `user.status_changed` và `user.access_revoked`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F011-AC-23 | KHI SIS báo `inactive`/`graduated`, module identity PHẢI đặt `deactivated`, đóng enrollment; báo `suspended` PHẢI đặt `locked`; báo `transferred_out` PHẢI đóng enrollment; mỗi thay đổi phát `user.status_changed` và `user.access_revoked` trong cùng transaction | FR-SIS-008 · FR-SIS-002 | TC-AUTH-036 |
| F011-AC-24 | KHI bản ghi biến mất trong lần import CSV đầy đủ của trường, module identity PHẢI đánh dấu `missing_in_source`, không deactivate; sau hai lần liên tiếp vượt 5% PHẢI tạo `deactivation_review` | FR-SIS-005 · [SIS_INTEGRATION §5](../../docs/09-identity/SIS_INTEGRATION.md) | — |
| F011-AC-25 | KHI học sinh đã bị dọn (`inactive_cleanup`) xuất hiện lại trong sync, module identity PHẢI không tự khôi phục và tạo issue `cleaned_up_student_in_sis` | FR-SIS-005 · ADR-0032 | — |

---

### User Story 6 — Connection `csv_import` và ranh giới truy cập SIS (Priority: P2)

System Admin tạo connection loại `csv_import` cho tenant và gán trường; mỗi trường chỉ một connection active; API SIS chỉ có trên gateway nội bộ.

**Why this priority**: Import CSV cần connection để gắn SyncRun và khóa nguồn (ADR-0046); ranh giới gateway là yêu cầu bảo mật.

**Independent Test**: `sa.admin` tạo connection `csv_import` cho `th-demo-a`, gán TH Demo A → OK; gán lại cho connection thứ hai → `409`; gọi `/api/v1/sis/connections` qua gateway public → không có route.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F011-AC-26 | KHI người có `platform.sis_connection.manage` gọi `createSisConnection` với `provider_kind = csv_import` (`X-Tenant-Id` = tenant đích, `Idempotency-Key`) rồi `setSisConnectionSchools`, module identity PHẢI tạo connection không cần `config`/`secret_ref` và ghi audit; NẾU trường đã có connection active khác THÌ PHẢI trả `409 SIS_SCHOOL_ALREADY_CONNECTED`; người thiếu quyền PHẢI nhận `403`, tenant khác `404` | FR-SIS-006 · ADR-0046 · [SIS_INTEGRATION §6.1](../../docs/09-identity/SIS_INTEGRATION.md) | TC-TEN-001 |
| F011-AC-27 | HỆ THỐNG PHẢI chỉ mở route `/api/v1/sis/*` trên gateway nội bộ, không trả `secret_ref`/credential ở API nào, và game client không có đường gọi SIS | FR-SIS-007 · ADR-0009 | — |

### Edge Cases

- NẾU `province_code` cũ hoặc thiếu ở School, THÌ xử lý theo F010 (ánh xạ kế thừa / `pending_review`) — TC-ORG-103, TC-ORG-104.
- NẾU grade SIS khác grade lớp, THÌ dùng grade của lớp và tạo `grade_mismatch`.
- NẾU admin sửa tay trường dữ liệu SIS sở hữu, THÌ lần sync sau ghi đè và cảnh báo.
- NẾU học sinh có enrollment active ở hai trường, THÌ không apply enrollment thứ hai, issue `multi_school_enrollment`.
- NẾU khóa đăng nhập trùng giữa hai connection, THÌ không tự chọn; issue `ambiguous_login`.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-SIS-002 | Đồng bộ School/Class/Student/Enrollment/Status; không giáo viên | [SIS_INTEGRATION §2.3](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-003 | Chỉ phần `csv_import` (cột = tên trường DTO, validator JSON Schema trích từ contract); phần `contract_v1` → F017 | [SIS_INTEGRATION §4](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-005 | Reconciliation issue, conflict matrix, resolve | [SIS_INTEGRATION §6](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-006 | Lỗi theo bản ghi, SyncRun, audit cho import CSV (retry pull HTTP → F017) | [SIS_INTEGRATION §7–§8](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-007 | Godot không gọi SIS; `/api/v1/sis` chỉ gateway nội bộ | [SIS_INTEGRATION §9](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-SIS-008 | Apply trạng thái từ SIS và phát event trong một chu kỳ (lan truyền tới Keycloak/realtime: F016) | [IDENTITY_MAPPING §5](../../docs/09-identity/IDENTITY_MAPPING.md) |
| FR-ORG-018 | Login sync nhánh A/B/C sau port `SisLookupPort` (adapter `contract_v1` → F017) | [SIS_INTEGRATION §4.5](../../docs/09-identity/SIS_INTEGRATION.md) |
| FR-AUTH-005 | `(issuer, subject)` là khóa định danh ngoài | [IDENTITY_MAPPING §1](../../docs/09-identity/IDENTITY_MAPPING.md) |
| FR-AUTH-014 | Allowlist nhiều issuer | [KEYCLOAK_SSO §11](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-031 | Username/email duy nhất, một người một tài khoản, `duplicate_identifier` | [ACCOUNT_PROVISIONING §1](../../docs/09-identity/ACCOUNT_PROVISIONING.md) |

### Key Entities

- **SISConnection**: kết nối SIS của một tenant; F011 chỉ dùng loại `csv_import` (loại `contract_v1` với `config`, `secret_ref`, lịch, watermark → F017); phục vụ tập trường (mỗi trường một connection active).
- **SisImport, SyncRun, StagingRecord, SyncRecordError**: một lần nhập/đồng bộ, bản ghi trung gian đã chuẩn hóa, lỗi theo bản ghi.
- **SyncedStudent, StudentLoginKey, SISLink**: bản ghi SIS theo khóa nguồn, khóa đăng nhập, ánh xạ school/class/enrollment.
- **ExternalIdentity, SisIdentityLink**: `(issuer, subject)` → User; User ↔ bản ghi SIS nguồn.
- **ReconciliationIssue**: xung đột cần người xử lý, loại, gợi ý, trạng thái.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `sis_login_sync_timeout_ms` | Timeout lookup nhánh B |
| `sis_login_refresh_after_hours` | Ngưỡng nhánh C |
| `sis_login_sync_concurrency` | Giới hạn lookup đồng thời/connection |
| `sis_staging_retention` | Ẩn danh hóa staging |
| `csv_import_max` | Kích thước file |
| `identifier_uniqueness` | Quy tắc định danh duy nhất |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không gọi SIS trên đường đăng nhập ngoài nhánh B; không retry đồng bộ | ADR-0045 · ADR-0061 · AGENTS R30 |
| Không log PII (tên, ngày sinh) ở INFO; audit chỉ ghi diff trường | AGENTS R14 · NFR-PRIV-002 |
| File CSV mã hóa at-rest, không log nội dung | NFR-SEC-002 · AGENTS R3, R4 |
| Mọi chuỗi có giới hạn; chỉ soft-delete | AGENTS R21, R28 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Import bộ CSV 1.000 học sinh của một trường hoàn tất, học sinh đăng nhập được, trong ≤ 10 phút.
- **SC-002**: 0 tài khoản trùng username/email (không phân biệt hoa thường) sau mọi kịch bản import trùng.
- **SC-003**: Login sync nhánh A không tạo request SIS nào (đo bằng mock ghi số request).
- **SC-004**: Trạng thái `suspended`/`inactive` trong file CSV có hiệu lực ở EduWorld ngay khi import áp dụng xong.

## Ngoài phạm vi

- Tạo tenant/trường thủ công, chuyển lớp/trường ở mức domain → F010.
- Keycloak disable/logout, cắt WebSocket, kết thúc game session khi khóa → F016; khóa thủ công → F016.
- Gọi login sync trong luồng `/auth/game/login` → F013.
- Hợp nhất tài khoản (`/admin/users:merge`) → F045.
- **SIS `contract_v1` → F017**: provider API pull theo EduWorld SIS Contract (FR-SIS-001, FR-SIS-003 phần `contract_v1`), full/delta sync theo lịch và webhook (FR-SIS-004), retry/backoff/`auth_error`/`contract_error` của pull, `triggerSisSync`, adapter `lookupStudent` cho login sync nhánh B, cấu hình connection `contract_v1` (`config`, `secret_ref`), conformance kit + `sis-mock` (FR-SIS-009).
- JIT qua IdP trường, on-demand lookup theo `idp_subject` — mở rộng sau (ADR-0063).
- Màn hình SIS trên portal → F052.

## Assumptions

- DTO trong `specs/sis/eduworld-sis-contract.v1.yaml` có `province_code`, `moet_code`, `school_level` (đã có); F011 chỉ dùng `components.schemas` làm định nghĩa cột CSV và validator.
- Ngưỡng 5%, lô 500 [Proposed] theo SIS_INTEGRATION; cần đăng ký CONFIG_DEFAULTS nếu muốn cấu hình được (task Polish).
- School Admin truy cập `/api/v1/sis/reconciliation-issues` qua gateway nội bộ theo API_DESIGN §9.15 [Proposed].
