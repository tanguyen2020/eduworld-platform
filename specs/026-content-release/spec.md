# Feature Specification: F026 — Phát hành nội dung: publish/rollback/unpublish, snapshot, manifest, ghim version, nơi sử dụng

**Feature Branch**: `026-content-release` · **Created**: 2026-11-16 · **Status**: Done · **Module**: module `learning` (+ consumer manifest trong module `game`) · **Mốc**: M-C2 · **Phụ thuộc**: F025
**Input**: Mô hình hai lớp Version + Release pointer cho mọi `LearningContentType` (ADR-0015, ADR-0043): publish ngay/hẹn giờ, rollback đổi con trỏ, unpublish, archive, snapshot bất biến (Valkey/CDN, không answer key), content manifest theo tenant có `min_client_version`, đọc theo version đã ghim, báo cáo nơi sử dụng. FR nguồn: FR-CNT-004, FR-CNT-008…012, FR-EXR-015.

## Clarifications

### Session 2026-11-16

- Q: Manifest do ai dựng? → A: Bảng `game.content_manifests` và `getContentManifest` thuộc module `game` (DATABASE_DESIGN §3.13, F014). F026 phát `content.*` đủ trường và thêm consumer `game.manifest_on_learning_content` gộp mục nội dung học tập vào manifest.
- Q: Hẹn giờ publish lưu ở đâu khi `content_releases` là append-only? → A: Bảng mới `content.scheduled_publications`; dòng `content_releases` chỉ ghi khi con trỏ thực sự đổi.
- Q: "Nơi sử dụng" gồm tham chiếu từ quest/challenge (module game)? → A: F026 trả tham chiếu trong module `learning` (question set, lesson, activity, bài giao, exam session); tham chiếu từ nội dung game do F078 bổ sung (projection) [Confirmed].

### Session 2026-11-20 (triển khai)

- Q: Game biết tenant đích của item platform thế nào khi không đọc được DB learning? → A: Payload `content.published/rolled_back/unpublished` mang `tenant_ids` (tenant sở hữu hoặc mọi tenant được gán lúc phát, đọc qua `tenancy.Bypass(platform_job)`); gán mới sau khi publish chưa cập nhật manifest (theo dõi ở F045/F078) [Confirmed].
- Q: Khóa hàng khi publish song song? → A: Optimistic theo `version` của item/version (`409 CONTENT_VERSION_CONFLICT`); job hẹn giờ dùng `version` của lịch hẹn để hai worker không áp dụng trùng [Confirmed].
- Q: Rollback/unpublish ai được làm? → A: rollback `content.release.rollback` (System Admin, liveops theo catalog role); unpublish `learning_content.publish`.
- Q: Nội dung riêng tenant lên CDN? → A: Không; chỉ nội dung platform không phải đề kiểm tra (`cdn_eligible`) lên `ew-media-public`; còn lại phục vụ qua API [Confirmed].
- Q: `active_pinned_sessions`, nơi sử dụng ngoài question set? → A: 0 và chỉ question set cho tới F030/F041/F042 [Confirmed].

### Session 2026-11-20 (owner chốt)

- OQ F054-3 (nơi dùng từ quest/challenge/thi đấu, FR-CNT-008) → **Chốt:** module learning giữ projection tham chiếu ngoài (`content.external_usages`, theo version) dựng từ event: module game đưa `content_refs[]` (loại, id, version nội dung học tập) vào payload `content.published` của cấu hình game (F078; schema owner F071), module competition làm tương tự cho cấu hình thi đấu đã publish (F108). `getContentUsages` thêm `ref_type` `quest`, `minigame`, `challenge`, `competition` đọc từ projection này; không gọi đồng bộ module khác (R29). Task ở F078 (gồm phía learning) và F108.
- Route/consumer/job → đã nối binary 2026-11-20.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Xuất bản ngay một version đã duyệt (Priority: P1)

Người có `learning_content.publish` xuất bản version `approved`; học sinh bắt đầu lượt mới nhận version này; version cũ vẫn xem/rollback được. US-CMS-001.

**Why this priority**: Kết thúc Content Loop M-C2 (soạn → duyệt → publish → học sinh làm).

**Independent Test**: Version `approved` (F025) → `publishContentVersion` → con trỏ đổi, một `content_releases`, snapshot không có đáp án, outbox `content.published`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F026-AC-01 | KHI gọi `publishContentVersion` không có `publish_at` cho version `approved`, module learning PHẢI chạy lại `PublishReadinessGuard` (F025), dựng snapshot, rồi trong một transaction đặt version `published` + `published_at`, đổi `current_version_id`, ghi `content_releases` (`publish`), audit `content.published`, outbox `content.published`; trả `200 ContentRelease`. | FR-CNT-009 · [CONTENT_MODEL §5, §5.4](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001 |
| F026-AC-02 | NẾU version không ở `approved` THÌ module learning PHẢI trả `422 INVALID_STATE_TRANSITION`; NẾU có tham chiếu con chưa `published` THÌ PHẢI trả `409 CONTENT_NOT_PUBLISHED` liệt kê item; guard lỗi trả `AUDIO_REQUIRED`/`TRANSLATION_INCOMPLETE`/`MEDIA_NOT_READY`. | [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-010 |
| F026-AC-03 | KHI publish lặp cùng `Idempotency-Key`, module learning PHẢI trả response đã lưu và chỉ có một release; NẾU `If-Match` lệch, THÌ PHẢI trả `409 CONTENT_VERSION_CONFLICT`. | R10 · AC-EXR-003 | TC-CNT-003 |
| F026-AC-04 | HỆ THỐNG PHẢI từ chối mọi thay đổi nội dung của version `published`/`archived` bằng `409 CONTENT_NOT_EDITABLE` ở use case và trigger DB. | FR-CNT-004 · ADR-0015 | TC-CMS-002 |
| F026-AC-05 | KHI tạo version mới từ version đã publish, module learning PHẢI tạo draft với `based_on_version_id`; NẾU item đã có một draft/in_review mở, THÌ PHẢI trả `409 CONFLICT`. | FR-CNT-004 · [CONTENT_MODEL §5](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-002 |
| F026-AC-06 | KHI publish version mới, module learning PHẢI giữ version published trước đó ở trạng thái `published` (không còn là con trỏ). | ADR-0043 | TC-CMS-003 |
| F026-AC-07 | NẾU người gọi thiếu permission, thuộc tenant khác (→ `404`), hoặc là người dùng tenant thao tác nội dung platform đã gán (→ `403 PERMISSION_DENIED`), THÌ module learning PHẢI từ chối và ghi audit `authz.denied` khi 403. | R7 · R8 · ADR-0029 | TC-TEN-001 |

---

### User Story 2 — Rollback và gỡ xuất bản (Priority: P1)

Phát hiện lỗi sau publish, người có quyền trỏ con trỏ về version cũ hoặc gỡ xuất bản; học sinh đang làm không bị ngắt.

**Why this priority**: Giảm thiệt hại khi nội dung lỗi mà không phá lịch sử học tập.

**Independent Test**: Publish v4 rồi v5 → rollback về v4 có lý do → con trỏ = v4, không có version mới, `content_releases` ghi `rollback`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F026-AC-08 | KHI gọi `rollbackContent` với `to_version` là version `published` của cùng item, khác con trỏ và có `reason`, module learning PHẢI chỉ đổi `current_version_id`, không tạo version, ghi `content_releases` (`rollback`), audit `content.rolled_back`, outbox `content.rolled_back`. | FR-CNT-009 · AC-CNT-002 | TC-CNT-002 |
| F026-AC-09 | NẾU `to_version` là version `archived`, chưa publish, của item khác hoặc đang là con trỏ, THÌ module learning PHẢI trả `422 ROLLBACK_TARGET_INVALID` (use case và trigger `check_release_target`). | [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-002 |
| F026-AC-10 | KHI gọi `unpublishContent` có `reason`, module learning PHẢI đặt con trỏ `NULL`, ghi `content_releases` (`unpublish`), outbox `content.unpublished`; lượt mới sau đó nhận `409 CONTENT_NOT_PUBLISHED`. | FR-CNT-009 | TC-CMS-003 |
| F026-AC-11 | HỆ THỐNG PHẢI yêu cầu `content.release.rollback` cho rollback và `learning_content.publish` cho unpublish; thiếu `reason` → `422 VALIDATION_FAILED`; `reason` > 500 ký tự → `422 VALIDATION_FAILED`. | [ADMIN_PORTAL §6](../../docs/04-admin-portal/ADMIN_PORTAL.md) · R28 | TC-TEN-024 |

---

### User Story 3 — Snapshot bất biến và đọc theo version đã ghim (Priority: P1)

Mỗi version publish có snapshot không đáp án; mọi đơn vị học đã ghim đọc đúng version ghim, lượt mới đọc theo con trỏ.

**Why this priority**: Bảo đảm ADR-0016 và ADR-0043 cho F030 (attempt) và client.

**Independent Test**: Ghim v3 cho một đơn vị học, publish v4, rollback, unpublish → đọc theo `version_id` ghim luôn ra v3; quét snapshot không có trường đáp án.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F026-AC-12 | HỆ THỐNG PHẢI dựng snapshot không chứa `answer_key`, `scoring_rule`, `accepted_answers`, `correct_option_id(s)`, `explanation`, `options[].feedback`, `misconception_tag`, `hints`, `min_response_ms`. | FR-QB-003 · ADR-0016 · AC-EXR-001 | TC-CNT-006 |
| F026-AC-13 | HỆ THỐNG PHẢI không đưa lên CDN công khai snapshot của câu `exam_only`, blueprint đề kiểm tra và nội dung riêng tenant; nội dung riêng tenant chỉ phục vụ bằng signed URL hoặc qua API có kiểm quyền. | FR-CNT-012 · ADR-0044 · R31 | TC-CNT-006 |
| F026-AC-14 | KHI cache snapshot trên Valkey trống hoặc Valkey không khả dụng, module learning PHẢI đọc version từ PostgreSQL, trả cùng nội dung và nạp lại cache khi Valkey có lại. | [CONTENT_MODEL §5.4](../../docs/02-learning/CONTENT_MODEL.md) · R9 | TC-CNT-006 |
| F026-AC-15 | KHI một đơn vị học đã ghim `version_id`, `VersionReader` PHẢI trả đúng version đó kể cả sau publish, rollback hoặc unpublish; KHI bắt đầu đơn vị mới, `ReleaseResolver` PHẢI trả version theo con trỏ tại thời điểm bắt đầu. | FR-CNT-011 · AC-CNT-001 | TC-CNT-001 |
| F026-AC-16 | KHI nhận `content.published`/`content.rolled_back`/`content.unpublished`, consumer PHẢI cập nhật `ew:learning:current:<tenant_id>:<item_type>:<item_id>` cho tenant sở hữu hoặc mọi tenant được gán (platform chưa gán dùng `_platform`), idempotent theo `processed_events`. | ADR-0043 mục 6 · [VALKEY_DESIGN](../../docs/07-backend/VALKEY_DESIGN.md) | TC-SVC-003 |

---

### User Story 4 — Hẹn giờ xuất bản (Priority: P2)

Người xuất bản chọn giờ publish (ICT) cho version đã duyệt; worker áp dụng đúng giờ; có thể hủy trước giờ.

**Why this priority**: Phát hành nội dung theo lịch học; không chặn pilot.

**Independent Test**: Hẹn `publish_at` = now + 6 phút với đồng hồ giả → `202`, đến giờ con trỏ đổi, `content.published.scheduled = true`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F026-AC-17 | KHI `publish_at` ≥ hiện tại + `scheduled_publish_min_lead`, module learning PHẢI trả `202 ContentRelease` (`status = scheduled`), ghi lịch hẹn, audit `content.publish_scheduled`, outbox `content.publish_scheduled`; NẾU sớm hơn, THÌ PHẢI trả `422 VALIDATION_FAILED`. | FR-CNT-009 | TC-CNT-004 |
| F026-AC-18 | KHI đến `publish_at`, worker PHẢI chạy lại guard và áp dụng publish trong ±1 phút (`content.published.scheduled = true`); NẾU guard lỗi, THÌ PHẢI đánh dấu lịch `failed` với lý do và không đổi con trỏ; hai instance worker không áp dụng trùng. | FR-CNT-009 | TC-CNT-004 |
| F026-AC-19 | KHI gọi hủy một lịch `scheduled` trước giờ chạy, module learning PHẢI đặt `cancelled`, ghi audit; NẾU lịch đã áp dụng, THÌ PHẢI trả `422 INVALID_STATE_TRANSITION`. | [ADMIN_PORTAL §3.11](../../docs/04-admin-portal/ADMIN_PORTAL.md) | TC-CNT-004 |

---

### User Story 5 — Lịch sử, xem version chỉ đọc, so sánh (Priority: P2)

Biên tập viên/reviewer xem danh sách version, dòng thời gian release, nội dung từng version (chỉ đọc) và diff hai version.

**Why this priority**: Reviewer cần diff để duyệt; người xuất bản cần lịch sử để quyết định rollback.

**Independent Test**: Item có v1–v3 → `listContentVersions`, `getContentVersion` (v1 archived), `diffContentVersions` v3 với v2 bằng tài khoản có/không có `learning_content.review`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F026-AC-20 | KHI gọi `listContentVersions`, module learning PHẢI trả version mới nhất trước, `current_version`, `is_current`, `change_note`, người tạo/duyệt/xuất bản và `active_pinned_sessions`; `getContentVersion` PHẢI trả nội dung chỉ đọc của mọi version kể cả `archived`. | FR-CNT-010 · AC-CNT-003 | TC-CMS-002 |
| F026-AC-21 | KHI gọi `diffContentVersions`, module learning PHẢI trả thay đổi theo JSON pointer (`added`/`removed`/`changed`); NẾU người gọi thiếu `learning_content.review`, THÌ PHẢI loại các pointer thuộc `answer_key`, `scoring_rule`, `options[].feedback`, `hints`. | FR-CNT-010 · [ADMIN_PORTAL §3.11](../../docs/04-admin-portal/ADMIN_PORTAL.md) | — |
| F026-AC-22 | KHI gọi `listContentReleases`, module learning PHẢI trả mọi publish/rollback/unpublish (mới nhất trước) với `from_version`, `to_version`, `reason`, `released_by`, `released_at`. | FR-CNT-009 | TC-CMS-004 |

---

### User Story 6 — Content manifest theo tenant (Priority: P2)

Game client tải manifest của tenant (ETag/304) khi khởi động và khi vào màn mới; manifest có nội dung học tập tenant sở hữu hoặc được gán, kèm `min_client_version` theo `activity_type`.

**Why this priority**: Client cache theo `(item_id, version)` và nhận nội dung mới ≤ `publish_to_visible_max`.

**Independent Test**: Publish activity platform đã gán cho `th-demo-a` → manifest `th-demo-a` tăng `manifest_version`, mục có `activity_type` + `min_client_version`; `pgd-demo` (không gán) không thấy.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F026-AC-23 | KHI nhận `content.published`/`rolled_back`/`unpublished` của module learning, module game PHẢI dựng lại manifest của tenant sở hữu hoặc mọi tenant được gán, tăng `manifest_version`, đổi `etag`; `getContentManifest` với `If-None-Match` trùng PHẢI trả `304`. | FR-CNT-012 · [CONTENT_MODEL §5.3](../../docs/02-learning/CONTENT_MODEL.md) | TC-CNT-005 |
| F026-AC-24 | HỆ THỐNG PHẢI đưa nội dung mới vào manifest trong `publish_to_visible_max` (p95, đo từ `released_at` tới `generated_at`). | FR-CNT-012 | TC-CNT-005 |
| F026-AC-25 | Ở NƠI mục manifest có `item_type = activity`, module game PHẢI ghi `activity_type` và `min_client_version` của loại đó (lấy từ Activity Registry qua payload event). | FR-EXR-015 · [ACTIVITY_ENGINE §12](../../docs/02-learning/ACTIVITY_ENGINE.md) | TC-ACT-004 |
| F026-AC-26 | HỆ THỐNG PHẢI không có trong manifest của một tenant mục thuộc tenant khác hoặc mục platform chưa gán cho tenant đó. | R8 · ADR-0029 | TC-TEN-019 |

---

### User Story 7 — Nơi sử dụng và lưu trữ version (Priority: P3)

Trước khi archive, biên tập viên xem nội dung đang được tham chiếu ở đâu; archive ẩn version khỏi lựa chọn mới.

**Why this priority**: Tránh archive nhầm nội dung đang dùng; không chặn pilot.

**Independent Test**: Câu hỏi nằm trong question set và bài giao → `getContentUsages` liệt kê; archive version con trỏ bị từ chối, version cũ archive được.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F026-AC-27 | KHI gọi `getContentUsages`, module learning PHẢI trả các tham chiếu trong phạm vi tenant người gọi (question set, lesson, activity, bài giao, exam session) theo version, kèm số lượt đang ghim. | FR-CNT-008 | — |
| F026-AC-28 | KHI `archive` (qua `transitionContentVersion`) một version `published` không phải con trỏ, module learning PHẢI chuyển `archived`, audit `content.archived` có lý do, outbox `content.version_archived`; NẾU là con trỏ hiện hành, THÌ PHẢI trả `422 INVALID_STATE_TRANSITION`. | FR-CNT-003 · [CONTENT_MODEL §5](../../docs/02-learning/CONTENT_MODEL.md) | TC-CMS-001 |

### Edge Cases

- Hai yêu cầu publish khác key cùng lúc cho hai version của một item: khóa hàng item (`SELECT … FOR UPDATE`), người sau `409 CONTENT_VERSION_CONFLICT` (F026-AC-03).
- Snapshot dựng xong nhưng transaction đổi con trỏ thất bại: snapshot mồ côi vô hại (bất biến theo version), không phát event.
- Rollback khi có lịch hẹn đang chờ cho item: lịch vẫn giữ; khi tới giờ guard kiểm tra lại và áp dụng (log cảnh báo).
- Unpublish nội dung đang được bài giao `follow_latest` tham chiếu: bài giao xử lý theo F041, F026 chỉ phát event.
- Platform item gán cho nhiều tenant: một event → nhiều manifest (F026-AC-23).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CNT-004 | Bất biến version published (use case + trigger), version mới `based_on` | [CONTENT_MODEL §5](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-008 | `getContentUsages` (tham chiếu trong module learning) trước archive | [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-009 | Publish ngay/hẹn giờ, rollback, unpublish, lịch sử release | [CONTENT_MODEL §5](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-010 | Xem version chỉ đọc, diff (preview ở F027) | [CONTENT_MODEL §5.1](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-011 | `ReleaseResolver` + `VersionReader` cho mọi đơn vị ghim (attempt dùng ở F030) | [CONTENT_MODEL §5.2](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-CNT-012 | Snapshot Valkey/CDN, con trỏ cache, manifest tenant | [CONTENT_MODEL §5.3, §5.4](../../docs/02-learning/CONTENT_MODEL.md) |
| FR-EXR-015 | `min_client_version` theo `activity_type` trong manifest | [ACTIVITY_ENGINE §12](../../docs/02-learning/ACTIVITY_ENGINE.md) |

### Key Entities

- **ContentItem**: định danh item với con trỏ release hiện hành và số version mới nhất.
- **ContentVersion**: bản nội dung; bất biến khi published; khóa snapshot + hash.
- **ContentRelease**: lịch sử đổi con trỏ (publish/rollback/unpublish, lý do, người làm).
- **ScheduledPublication**: lịch hẹn publish của một version (scheduled/applied/cancelled/failed).
- **Snapshot**: bản dựng bất biến của một version, không đáp án.
- **ContentManifest** (module game): danh sách item → version của tenant, `manifest_version`, ETag.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `scheduled_publish_min_lead` | Khoảng tối thiểu từ hiện tại tới `publish_at` |
| `publish_to_visible_max` | SLO từ publish đến manifest |
| `assets_per_version_max` | Số media tối đa một snapshot tham chiếu |
| `idempotency_record_ttl` | Giữ kết quả `Idempotency-Key` |

TTL trượt của snapshot Valkey (ADR-0043 mục 6) chưa có tên trong CONFIG_DEFAULTS → task Polish đăng ký `content_snapshot_cache_ttl`.

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Valkey chỉ là cache; mất Valkey không mất dữ liệu | ADR-0004 · R9 |
| Không đưa đề kiểm tra/answer key lên CDN | R31 · ADR-0044 |
| Gọi chéo module chỉ qua event | R13 · R29 · ADR-0042 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: 95% lần publish xuất hiện trong manifest tenant trong `publish_to_visible_max`.
- **SC-002**: 0 snapshot/CDN object chứa trường đáp án (quét tự động mỗi build).
- **SC-003**: 100% rollback không tạo version mới và có lý do trong lịch sử release.
- **SC-004**: Học sinh đang làm không bao giờ nhận version khác version đã ghim (kiểm bằng test pinning qua publish/rollback/unpublish).

## Ngoài phạm vi

- Workflow duyệt, guard audio/ngôn ngữ → F025; preview → F027.
- Ghim version trong attempt (`attempts.source_version_id`) → F030; bài giao → F041; exam session → F042; lesson session → F024.
- Endpoint `getContentManifest`, bảng `game.content_manifests`, mục nội dung game → F014/F078; WS `content.manifest.updated` → F090.
- Tham chiếu từ quest/challenge trong báo cáo nơi sử dụng → F078.
- Màn hình lịch sử/rollback → F054; client cache theo manifest → F063.

## Assumptions

- Consumer manifest trong module game phụ thuộc F014 Done (bảng + endpoint) → đề xuất thêm F014 vào phụ thuộc của F026.
- `min_client_version` của mỗi `activity_type` khai báo trong metadata plugin Activity Registry (F030) và được đưa vào payload `content.published` [Confirmed]; F026 làm trước F030 nên dùng bảng metadata tĩnh cho `quiz`/`interactive_exercise` tới khi registry có.
- Nội dung riêng tenant lên CDN qua bucket `ew-media-private` + signed URL; nội dung platform không nhạy cảm qua `ew-media-public` (ADR-0044).
