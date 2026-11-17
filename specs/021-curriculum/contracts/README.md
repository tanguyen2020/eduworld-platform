# Contracts: F021 — Curriculum: cấu trúc, LO, DAG tiên quyết, version, mapping, import/export, seed GDPT

**Spec**: [../spec.md](../spec.md) · **Plan**: [../plan.md](../plan.md)

## 1. API (OpenAPI)

Spec: `specs/openapi/eduworld-api.v1.yaml` (`x-service: learning`), tách sang `specs/openapi/modules/learning.v1.yaml` bằng `scripts/openapi/split-public.mjs`.

| operationId | Method + path | Spec | Permission | Trạng thái |
|---|---|---|---|---|
| `listCmsCurricula` | `GET /api/v1/cms/learning/curricula` | eduworld-api | `curriculum.read` | Có sẵn |
| `createCurriculum` | `POST /api/v1/cms/learning/curricula` | eduworld-api | `curriculum.write` | Cần sửa (schema `CurriculumInput`) |
| `getCmsCurriculum` | `GET /api/v1/cms/learning/curricula/{curriculum_id}` | eduworld-api | `curriculum.read` | Cần sửa (schema `Curriculum`) |
| `updateCurriculum` | `PATCH /api/v1/cms/learning/curricula/{curriculum_id}` | eduworld-api | `curriculum.write` | Cần sửa |
| `listCmsSubjects` · `createSubject` · `updateSubject` | `GET/POST /cms/learning/subjects`, `PATCH /cms/learning/subjects/{subject_id}` | eduworld-api | `curriculum.read` / `curriculum.write` (chỉ platform) | Có sẵn |
| `listCmsTopics` · `createTopic` · `updateTopic` | `GET/POST /cms/learning/topics`, `PATCH /cms/learning/topics/{topic_id}` | eduworld-api | `curriculum.read` / `curriculum.write` | Cần sửa (`curriculum_version_id`, `code`) |
| `listCmsObjectives` · `createObjective` · `updateObjective` | `GET/POST /cms/learning/objectives`, `PATCH /cms/learning/objectives/{objective_id}` | eduworld-api | `curriculum.read` / `curriculum.write` | Cần sửa (query `curriculum_version_id`, `textbook_section_id`) |
| `listCurriculumVersions` · `createCurriculumVersion` | `GET/POST /cms/learning/curricula/{curriculum_id}/versions` | eduworld-api | `curriculum.read` / `curriculum.write` | Cần thêm |
| `getCurriculumVersion` · `updateCurriculumVersionDraft` | `GET/PATCH /cms/learning/curricula/{curriculum_id}/versions/{version}` | eduworld-api | `curriculum.read` / `curriculum.write` | Cần thêm |
| `transitionCurriculumVersion` | `POST /cms/learning/curricula/{curriculum_id}/versions/{version}:transition` | eduworld-api | `curriculum.write` (submit_review) / `curriculum.publish` (approve, reject, archive) | Cần thêm |
| `publishCurriculumVersion` | `POST /cms/learning/curricula/{curriculum_id}/versions/{version}:publish` | eduworld-api | `curriculum.publish` | Cần thêm |
| `exportCurriculumVersion` | `GET /cms/learning/curricula/{curriculum_id}/versions/{version}:export?format=yaml\|csv` | eduworld-api | `curriculum.read` | Cần thêm |
| `getCurriculumVersionLoMappings` · `putCurriculumVersionLoMappings` | `GET/PUT /cms/learning/curricula/{curriculum_id}/versions/{version}/lo-mappings` | eduworld-api | `curriculum.read` / `curriculum.write` | Cần thêm |
| `deleteCurriculum` · `deleteTopic` · `deleteObjective` | `DELETE` trên path tương ứng | eduworld-api | `curriculum.write` | Cần thêm |
| `listCmsCompetencies` · `createCompetency` · `updateCompetency` | `GET/POST /cms/learning/competencies`, `PATCH /cms/learning/competencies/{competency_id}` | eduworld-api | `curriculum.read` / `curriculum.write` | Cần thêm |
| `listTextbookSeries` · `createTextbookSeries` · `listTextbooks` · `createTextbook` · `listTextbookSections` · `createTextbookSection` · `updateTextbookSection` · `putTextbookSectionObjectives` | `/cms/learning/textbook-series`, `/cms/learning/textbooks`, `/cms/learning/textbook-sections` (+ `/{section_id}`, `/{section_id}/objectives`) | eduworld-api | `curriculum.read` / `curriculum.write` (platform) | Cần thêm |
| `createContentImport` · `getContentImport` | `POST /cms/learning/imports`, `GET /cms/learning/imports/{import_id}` | eduworld-api | `curriculum.write` (kind curriculum) | Cần sửa (`kind` thêm `curriculum_yaml`, `curriculum_csv`) |
| `listCurricula` · `listSubjects` · `listTopics` · `listLearningObjectives` | `GET /api/v1/learning/curricula`, `/learning/subjects`, `/learning/topics`, `/learning/objectives` | eduworld-api | principal đã đăng nhập trong tenant | Có sẵn |
| `getLearningObjective` | `GET /api/v1/learning/objectives/{objective_id}` | eduworld-api | principal đã đăng nhập trong tenant | Cần thêm |

## 2. Internal API

| operationId | Gọi từ → tới | Spec | `x-allowed-callers` | Trạng thái |
|---|---|---|---|---|
| `listTrash`, `restoreTrashItem`, `executeTenantCloneStep`, `applyTenantAssignments` | identity → learning (`resource_type = curriculum`) | `specs/openapi/modules/learning.internal.v1.yaml` | `identity` | Có sẵn / Cần thêm (F020) |

## 3. Realtime (AsyncAPI)

| Message | Hướng | Trạng thái |
|---|---|---|
| — | Không dùng | — |

## 4. Event (NATS)

| Event | Phát / Tiêu thụ | Payload schema | Trạng thái |
|---|---|---|---|
| `curriculum.version_published` | Phát (learning) → F031, F024 | `specs/events/payloads/learning/curriculum.version_published.v1.schema.json` | Cần thêm |
| — (version mặc định năm học) | Không dùng event: đọc `lms.school_curriculum_settings` cùng module learning (owner F040) | — | — |
| `curriculum.deleted` / `curriculum.restored` | Phát | `resource.deleted.v1` / `resource.restored.v1` | Cần thêm (F020) |

## 5. JSON Schema nội dung

| Schema | Dùng để | Trạng thái |
|---|---|---|
| `specs/schemas/curriculum-import.schema.json` | Định dạng file YAML/JSON import/export curriculum (cây `curriculum → version → subject_grades[] → topics[] → objectives[]`) | Cần thêm |

## 6. Delta cần thực hiện

- Sửa `CurriculumInput`/`Curriculum`: bỏ `subject_id`, `grade_level`, `textbook_series_id` ở cấp curriculum; thêm `owner_scope` (`platform`/`tenant`, D8), `current_version` (object `{version, version_label, status}`), `latest_version_no`; giữ `code`, `name`, `framework`, `tenant_id`.
- Sửa `TopicInput`: thay `curriculum_id` bằng `curriculum_version_id` (bắt buộc), thêm `code` (≤ 64), `sort_order`; `Topic` thêm `code`, `curriculum_version_id`. `LearningObjective`/`LearningObjectiveInput` thêm `stable_key` (≤ 64, readOnly), `sort_order`.
- Thêm schema `CurriculumVersion` (`curriculum_version_id`, `version`, `version_label` pattern `^[0-9]{1,4}\.[0-9]{1,4}$`, `status`, `effective_from/to` date, `supersedes_version_id`, `change_note` ≤ 1000, `author_id`, `reviewer_id`, `published_at`), `CurriculumVersionInput`, `CurriculumTransitionRequest` (`action` enum `submit_review/approve/reject/archive`, `note` ≤ 2000), `LoMappingList` (≤ 5000 phần tử `{from_lo_id, to_lo_id?, relation}`), `Competency`/`CompetencyInput`, `TextbookSeries`/`Textbook`/`TextbookSection` (chuỗi ≤ 255, `page_range` pattern).
- Thêm các operation "Cần thêm" ở §1 (header `TenantIdHeaderOptional`, `IdempotencyKey` cho POST ghi, `IfMatch` cho PATCH/transition/publish; lỗi 403/404/409/412/422/428).
- `ContentImportRequest.kind`: thêm `curriculum_yaml`, `curriculum_csv`. Thêm `errors[].code` mẫu `PREREQUISITE_CYCLE` vào mô tả `ValidationFailed` (không phải `ProblemCode` mới).
- Thêm JSON Schema `curriculum-import.schema.json` + example `specs/examples/curriculum.gdpt2018-sample-math.json`.
- Thêm payload `curriculum.version_published.v1`. Version mặc định năm học đọc từ `lms.school_curriculum_settings` (cùng module learning, owner F040) — không có event cài đặt trường hay cột `ref.schools.curriculum_settings`.
