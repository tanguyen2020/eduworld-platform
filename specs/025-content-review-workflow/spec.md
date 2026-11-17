# Feature Specification: F025 — Workflow duyệt nội dung, song ngữ, lồng tiếng `g1_3`

**Feature Branch**: `025-content-review-workflow` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C2 · **Phụ thuộc**: F022
**Input**: Workflow Draft → Review → Approved (+ Rejected → Draft) cho mọi `LearningContentType`, hàng đợi duyệt theo lane, kiểm tra ngôn ngữ nội dung (ADR-0056) và audio đọc đề cho nhóm `g1_3`. FR nguồn: FR-CNT-003, FR-CNT-018, FR-CNT-019, FR-CMS-006, FR-CMS-007, FR-CMS-008, NFR-I18N-002.

## Clarifications

### Session 2026-11-16

- Q: Thao tác publish/rollback/unpublish/archive thuộc feature nào? → A: F026 (endpoint riêng theo ADR-0043); F025 chỉ làm `submit_review`, `approve`, `reject`, `revise` và cung cấp bộ kiểm tra (guard) dùng lại khi publish.
- Q: Mã lỗi khi reviewer = author? → A: `403 PERMISSION_DENIED` [Proposed] (không có mã riêng trong `ProblemCode`; CONTENT_MODEL §6 chỉ nêu quy tắc).
- Q: Trả nội dung rejected về draft bằng gì? → A: action `revise` trên `transitionContentVersion` (delta contract, CONTENT_MODEL §6 `rejected → draft`, `approved → draft`).
- Q: `self_approve_allowed` có làm không? → A: Không — tham số chưa đăng ký CONFIG_DEFAULTS; reviewer ≠ author luôn áp dụng.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Gửi duyệt, duyệt, từ chối nội dung (Priority: P1)

Tác giả (Content Admin với dữ liệu platform; `tenant_content_editor`/`academic_manager` với dữ liệu tenant) gửi duyệt một version draft; reviewer cùng lane, khác tác giả, duyệt hoặc từ chối kèm lý do; tác giả sửa lại. US-CMS-001.

**Why this priority**: Không có duyệt thì không có nội dung `approved` để publish (F026); là bước đầu của Content Loop M-C2.

**Independent Test**: Tạo câu hỏi draft (F022) → `submit_review` → reviewer khác `approve` → version `approved`, có `content_transitions`, audit và event; reviewer = author bị từ chối.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F025-AC-01 | KHI tác giả gọi `submit_review` cho version `draft` hợp lệ, module learning PHẢI chuyển version sang `in_review`, tạo một review task theo lane, ghi `content_transitions`, audit `content.submitted` và outbox `content.version_submitted` trong cùng transaction. | FR-CNT-003 · [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001 |
| F025-AC-02 | NẾU version không qua JSON Schema của loại nội dung hoặc thiếu metadata bắt buộc khi `submit_review`, THÌ module learning PHẢI trả `422 VALIDATION_FAILED` liệt kê JSON pointer lỗi và giữ trạng thái `draft`. | FR-CNT-003 · [CONTENT_MODEL §3, §6](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001 |
| F025-AC-03 | NẾU version tham chiếu media có trạng thái khác `ready` khi `submit_review`, THÌ module learning PHẢI trả `422 MEDIA_NOT_READY` kèm danh sách `asset_id`. | [CONTENT_MODEL §7](../../docs/02-learning/CONTENT_MODEL.md) · ADR-0044 | TC-CNT-008 |
| F025-AC-04 | KHI reviewer có `learning_content.review` (activity: `learning_content.activity.review`) trong đúng scope gọi `approve` cho version `in_review`, module learning PHẢI chuyển sang `approved`, ghi `reviewer_id`, đóng review task, audit `content.approved`, outbox `content.version_approved`. | FR-CNT-003 · FR-CMS-008 | TC-CMS-001 |
| F025-AC-05 | NẾU người gọi `approve` là `author_id` của version, THÌ module learning PHẢI từ chối `403 PERMISSION_DENIED` và ghi audit `authz.denied`. | [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) · [ADMIN_PORTAL §4](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-001 |
| F025-AC-06 | KHI reviewer gọi `reject` có `note` (1–2.000 ký tự), module learning PHẢI chuyển version sang `rejected`, audit `content.rejected` kèm lý do, outbox `content.version_rejected`; NẾU thiếu `note`, THÌ PHẢI trả `422 VALIDATION_FAILED`. | FR-CNT-003 | TC-CMS-004 |
| F025-AC-07 | KHI tác giả gọi `revise` trên version `rejected` hoặc `approved` (chưa publish), module learning PHẢI đưa version về `draft` và ghi `content_transitions`. | [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001 |
| F025-AC-08 | NẾU action không hợp lệ với trạng thái hiện tại (ví dụ `approve` trên `draft`, mọi action trên `published`), THÌ module learning PHẢI trả `422 INVALID_STATE_TRANSITION` và không đổi dữ liệu. | FR-CNT-003 · FR-CMS-007 | TC-CMS-002 |
| F025-AC-09 | HỆ THỐNG PHẢI yêu cầu `If-Match` và `Idempotency-Key` cho `transitionContentVersion`: thiếu `If-Match` → `428 PRECONDITION_REQUIRED`, lệch → `412 VERSION_MISMATCH`; cùng key + cùng body → trả lại response đã lưu, không tạo transition thứ hai. | [API_DESIGN §6, §7](../../docs/07-backend/API_DESIGN.md) · R10 | TC-CMS-004 |
| F025-AC-10 | NẾU người dùng tenant gọi transition trên nội dung của tenant khác hoặc nội dung platform (kể cả đã gán), THÌ module learning PHẢI trả `404 NOT_FOUND` (tenant khác) hoặc `403 PERMISSION_DENIED` (platform đã gán, chỉ đọc). | R8 · ADR-0029 · AC-CMS-002 | TC-TEN-001 |
| F025-AC-11 | HỆ THỐNG PHẢI không sửa attempt, evaluation hay learning event nào khi chuyển trạng thái workflow. | FR-CMS-007 · [ADMIN_PORTAL §9](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-LRN-006 |

---

### User Story 2 — Hàng đợi duyệt theo lane (Priority: P1)

Reviewer mở hàng đợi chỉ thấy task của lane và tenant mình; System Admin lọc nhiều tenant. US-CMS-002.

**Why this priority**: Reviewer cần tìm việc; cô lập lane/tenant là ràng buộc bảo mật.

**Independent Test**: Tạo task ở lane `tenant` của hai tenant và lane `platform_learning`; gọi `listContentReviewTasks` bằng ba tài khoản, so tập kết quả.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F025-AC-12 | KHI reviewer tenant gọi `listContentReviewTasks`, module learning PHẢI chỉ trả task `review_lane = tenant` của tenant phiên (lọc thêm scope trường nếu UserRole scope `school`) và bỏ qua `tenant_ids`. | FR-CMS-008 · [ADMIN_PORTAL §4](../../docs/04-admin-portal/ADMIN_PORTAL.md) · AC-CMS-002 | TC-CMS-TEN-01 |
| F025-AC-13 | KHI reviewer platform (`content_reviewer`) gọi, module learning PHẢI chỉ trả task lane `platform_learning`; KHI `system_admin` gửi `tenant_ids`, PHẢI trả task các tenant đó kèm `tenant_id` và ghi audit `cross_tenant.accessed`. | ADR-0027 · R20 | TC-TEN-018 |
| F025-AC-14 | NẾU người gọi thiếu `learning_content.review`, THÌ module learning PHẢI trả `403 PERMISSION_DENIED`. | R7 · [ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CMS-AUTHZ-01 |

---

### User Story 3 — Audio bắt buộc cho nội dung lớp 1–3 (Priority: P1)

Nội dung có `grade_levels` giao {1, 2, 3} phải có audio đọc đề/hướng dẫn trước khi gửi duyệt và publish; câu đọc to làm lộ đáp án khai báo miễn trừ và reviewer xác nhận. US-EXR-003.

**Why this priority**: Học sinh lớp 1–3 chưa đọc thạo; là điều kiện publish (FR-CNT-019) của pilot Toán lớp 1–5.

**Independent Test**: Câu hỏi `grade_levels = [2]` thiếu `stem.audio_ref` → `submit_review` bị chặn; thêm `audio_exempt_reason` và reviewer xác nhận → đi tiếp.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F025-AC-15 | TRONG KHI version có `grade_levels` giao {1, 2, 3}, module learning PHẢI chặn `submit_review` bằng `422 AUDIO_REQUIRED` liệt kê field thiếu khi thiếu `stem.audio_ref` (câu hỏi), `instructions_audio_ref` (activity) hoặc `audio_ref` của bước `worked_example` (lesson) bản `vi`. | FR-CNT-019 · [CONTENT_MODEL §8.1](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-010 |
| F025-AC-16 | Ở NƠI version khai báo `audio_exempt_reason` (≤ 255 ký tự), module learning PHẢI bỏ yêu cầu audio của đề đó khi gửi duyệt và PHẢI yêu cầu `approve` có `audio_exemption_acknowledged = true`; thiếu cờ → `422 VALIDATION_FAILED`. | FR-CNT-019 · [CONTENT_MODEL §8.1](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-010 |
| F025-AC-17 | HỆ THỐNG PHẢI cung cấp cùng bộ kiểm tra audio/ngôn ngữ (`PublishReadinessGuard`) để F026 chạy lại khi publish và trả cùng mã lỗi. | FR-CNT-019 · FR-CNT-018 | TC-CNT-010 |

---

### User Story 4 — Ngôn ngữ nội dung và trạng thái bản dịch (Priority: P2)

Mỗi version có `content_language`; bản theo `content_language` là bắt buộc, bản còn lại tùy chọn và được theo dõi `translation_status`; học sinh nhận bản dự phòng có nhãn.

**Why this priority**: Trường song ngữ cần biết nội dung nào thiếu dịch; không chặn pilot tiếng Việt.

**Independent Test**: Lưu version `content_language = vi` không có `en` → `translation_status = partial`, vẫn gửi duyệt được; xóa bản `vi` của một trường bắt buộc → `TRANSLATION_INCOMPLETE`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F025-AC-18 | NẾU một trường văn bản học tập bắt buộc thiếu bản theo `content_language` khi `submit_review`, THÌ module learning PHẢI trả `422 TRANSLATION_INCOMPLETE` với `errors[]` là JSON pointer còn thiếu. | FR-CNT-018 · NFR-I18N-002 | — |
| F025-AC-19 | KHI lưu draft, module learning PHẢI tính `translation_status` (`complete`/`partial`/`outdated`) theo [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md), lưu cùng version và trả trong `ContentVersion.translation_status`; `partial`/`outdated` chỉ cảnh báo, không chặn. | FR-CNT-018 | TC-I18N-002 |
| F025-AC-20 | KHI phục vụ nội dung học tập cho học sinh với `Accept-Language` không có bản dịch, module learning PHẢI trả bản `content_language` kèm header `Content-Language` thực tế để client gắn nhãn "Chưa có bản tiếng Anh". | NFR-I18N-002 · ADR-0056 | TC-I18N-003 |
| F025-AC-21 | HỆ THỐNG PHẢI chuẩn hóa Unicode NFC mọi văn bản tiếng Việt khi lưu và từ chối chuỗi vượt `maxLength` bằng `422 VALIDATION_FAILED` chỉ rõ trường. | [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md) · R28 | TC-TEN-024 |

---

### User Story 5 — Xuất/nhập file dịch (Priority: P3)

Người dịch có `learning_content.translate` xuất trường cần dịch ra XLIFF 2.1/CSV, nhập lại thành version draft mới.

**Why this priority**: Phục vụ trường song ngữ; sau pilot.

**Independent Test**: Export 2 câu hỏi `only_missing` → file có khóa `<content_type>/<id>/<version>/<json-pointer>`; sửa bản `en`, import `dry_run` rồi import thật → draft mới, version published không đổi.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F025-AC-22 | KHI gọi `exportLearningTranslations`, module learning PHẢI tạo job `202`, sinh file XLIFF/CSV vào bucket `ew-exports` và trả `download_url` presigned trong `getLearningTranslationJob` khi `succeeded`. | FR-CNT-018 · [ADMIN_PORTAL §1.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F025-AC-23 | KHI gọi `importLearningTranslations` (không `dry_run`), module learning PHẢI tạo version draft mới cho mỗi item khớp khóa, không sửa version `published`, báo dòng không khớp trong `report.errors[]`. | FR-CNT-018 · ADR-0043 | — |
| F025-AC-24 | NẾU người gọi thiếu `learning_content.translate` hoặc item thuộc tenant khác, THÌ module learning PHẢI trả `403 PERMISSION_DENIED` / bỏ qua item và ghi vào `report.errors[]` như không tồn tại. | R7 · R8 · ADR-0005 | TC-TEN-001 |

### Edge Cases

- Hai reviewer cùng `approve` một version: chỉ một thành công, người sau nhận `412 VERSION_MISMATCH` (F025-AC-09).
- Version bị soft-delete (draft/rejected) trong khi có task mở: task chuyển `cancelled`, không xuất hiện ở hàng đợi.
- Nội dung `grade_levels = [3, 4]` vẫn thuộc nhóm `g1_3` (giao khác rỗng) → áp F025-AC-15.
- Môn Tiếng Anh (`content_language = en`) lớp 1–3: cần audio `en` cho phần tiếng Anh và `vi` cho lời dẫn (F025-AC-15).
- Lựa chọn dạng chữ thiếu audio: chỉ cảnh báo (`warnings[]`), không chặn.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CNT-003 | `submit_review`, `approve`, `reject`, `revise`; trạng thái `archive` và publish ở F026 | [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-018 | `content_language`, `translation_status`, `TRANSLATION_INCOMPLETE`, xuất/nhập file dịch (nội dung học tập) | [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-019 | Kiểm tra audio `g1_3`, miễn trừ được duyệt (`AUDIO_REQUIRED`) | [CONTENT_MODEL §8.1](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CMS-006 | Phần workflow + audit trail CMS (`content_transitions`, audit); versioning/publishing ở F026, preview ở F027 | [ADMIN_PORTAL §4, §7](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-007 | Workflow không chạm lịch sử bài đã làm | [ADMIN_PORTAL §9](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| FR-CMS-008 | Lane `tenant`: `tenant_content_editor` soạn, `tenant_content_reviewer` duyệt | [ADMIN_PORTAL §2.3, §3.5](../../docs/04-admin-portal/ADMIN_PORTAL.md) |
| NFR-I18N-002 | Phần nội dung học tập: bản `content_language` bắt buộc, dịch tùy chọn, fallback có nhãn | [CONTENT_MODEL §8](../../docs/02-learning/CONTENT_MODEL.md) |

### Key Entities

- **ContentVersion** (đã có từ F020/F022): thêm `translation_status`, `translation_meta`, `audio_exempt_reason`, `audio_exempt_approved_by`.
- **ReviewTask**: việc duyệt một version theo `review_lane` (`tenant` | `platform_learning`), tenant, người được giao, bình luận theo field.
- **ContentTransition**: bản ghi append-only mỗi lần đổi trạng thái (từ → tới, người làm, ghi chú).
- **TranslationJob**: job xuất/nhập file dịch, báo cáo dòng.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `learning_translation_required` | Tenant bắt buộc bản dịch thứ hai hay không (mặc định chỉ `content_language`) |
| `idempotency_record_ttl` | Thời gian giữ kết quả `Idempotency-Key` |
| `request_body_max` | Giới hạn body transition/import request |

SLA cảnh báo task quá hạn ([ADMIN_PORTAL §4](../../docs/04-admin-portal/ADMIN_PORTAL.md)) chưa có tên trong CONFIG_DEFAULTS → task Polish đăng ký `content_review_sla_days`.

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không log nội dung câu hỏi/đáp án ở INFO | AGENTS R14 |
| Mọi chuỗi có `MaxLen`/`maxLength` | ADR-0041 · R28 |
| Command handler (write context, `UnitOfWork`), outbox cùng transaction | ADR-0038 · ADR-0042 · ADR-0087 · R26 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 100% version chuyển `approved` có reviewer khác tác giả (kiểm bằng truy vấn đối soát `content_transitions`).
- **SC-002**: 0 câu hỏi `g1_3` vào `in_review` khi thiếu audio mà không có miễn trừ được xác nhận.
- **SC-003**: Reviewer tenant không bao giờ thấy task của tenant khác hoặc lane platform (test tự động duyệt mọi tổ hợp tài khoản demo).
- **SC-004**: Mỗi lần đổi trạng thái có đúng một dòng `content_transitions` và một audit record.

## Ngoài phạm vi

- Publish, rollback, unpublish, archive, hẹn giờ, snapshot → F026.
- Preview, media pipeline, công thức SVG → F027.
- Kiểm tra song ngữ bắt buộc `vi` + `en` của nội dung game, audio thoại NPC/quest → module game (F078).
- Màn hình hàng đợi duyệt, editor song ngữ → F054, F053.
- Bulk approve: portal gọi lần lượt; không có endpoint hàng loạt.

## Assumptions

- `review_lane` dùng `tenant` | `platform_learning` theo ADMIN_PORTAL §3.5; enum trong OpenAPI (`school`/`organization`) được sửa ở Phase 2 (D8).
- Tên event theo EVENT_DESIGN §4.2 (`content.version_submitted/approved/rejected`), không theo CONTENT_MODEL §6 (D11).
- Engine TTS chưa chọn [TBD]; audio là media thường do F027 xử lý; F025 chỉ kiểm tra tham chiếu và trạng thái `ready`.
- Media status đọc từ `content.media_assets` (F027) → đề xuất thêm F027 vào phụ thuộc của F025 (OQ-23 [Proposed] cho phạm vi hỗ trợ `g1_3`).
