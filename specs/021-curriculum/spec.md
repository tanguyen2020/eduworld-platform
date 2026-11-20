# Feature Specification: F021 — Curriculum: cấu trúc, LO, DAG tiên quyết, version, mapping, import/export, seed GDPT

**Feature Branch**: `021-curriculum` · **Created**: 2026-11-16 · **Status**: Ready · **Module**: module `learning` · **Mốc**: M-C2 · **Phụ thuộc**: F020
**Input**: Khung chương trình (Curriculum → CurriculumVersion → Môn × Khối → Topic → LO) có version bất biến sau publish, đồ thị LO tiên quyết, mapping LO giữa version, mapping bộ sách, import/export và seed GDPT 2018 Toán 1–5. FR nguồn: FR-CUR-001…012, FR-CMS-002.

## Clarifications

### Session 2026-11-16

- Q: OpenAPI coi "curriculum" là môn × khối có version, CURRICULUM_MODEL tách Curriculum/CurriculumVersion/SubjectGrade — theo bên nào? → A: Theo tài liệu sở hữu [CURRICULUM_MODEL §2–§3](../../docs/02-learning/CURRICULUM_MODEL.md#2-cấu-trúc-phân-cấp) và DATABASE_DESIGN §2.4; schema OpenAPI `Curriculum`/`CurriculumInput` sửa ở Phase 2 (contracts §6).
- Q: Workflow CurriculumVersion dùng endpoint chung `content/{content_type}`? → A: Không — `LearningContentType` không có `curriculum`; dùng endpoint riêng dưới `/cms/learning/curricula/{curriculum_id}/versions/…` (Cần thêm), cùng state machine `contentmodel.Lifecycle` của F020.
- Q: Seed GDPT 2018 để trống `yccd_ref` nhưng version `GDPT2018` không được publish khi thiếu → môi trường dev dùng gì? → A: Seed dev/pilot-trước-rà-soát nhập vào curriculum mã `GDPT2018_SAMPLE` (không áp quy tắc `yccd_ref`) và publish được; curriculum `GDPT2018` chỉ publish sau khi EdTech rà soát điền `yccd_ref` [Confirmed] ([CURRICULUM_MODEL §7](../../docs/02-learning/CURRICULUM_MODEL.md#7-seed-ví-dụ--gdpt-2018-toán-lớp-15-confirmed-oq-08)).
- Q: `school.settings.curriculum_version_id` do ai lưu? → A: module identity (cài đặt trường, F015); module learning đọc qua projection `ref.schools` [Confirmed].

### Session 2026-11-19 (triển khai)

- Q: Câu trả lời 2026-11-16 về `ref.schools` mâu thuẫn plan/data-model/contracts (đọc `lms.school_curriculum_settings`, owner F040)? → A: Theo plan/data-model (mới hơn, khớp CURRICULUM_MODEL §5 và DATABASE_DESIGN §2.16): đọc `lms.school_curriculum_settings` cùng module. Bảng do F040 tạo; khi chưa có bảng (kiểm `to_regclass`) hoặc trường chưa cài đặt thì dùng version published có hiệu lực chứa ngày hiện tại. Chọn dòng của trường theo khối (dòng có `grade_level` ưu tiên hơn dòng `NULL`), mới nhất trước; chưa lọc theo năm học vì module learning chưa có projection năm học (OQ F021-5) [Confirmed].
- Q: Bảng con của version (H) hiển thị cho tenant theo assignment nào? → A: Theo assignment của curriculum: mọi bảng `curriculum.*` thuộc version (`curriculum_versions`, `subject_grades`, `topics`, `learning_objectives`, `competencies`, `lo_version_mappings`) có thêm cột `curriculum_id` để interceptor `entx.HybridContent{ResourceIDColumn: curriculum_id}` và policy RLS so với `infra.tenant_resource_assignments.resource_id` (OQ F021-1) [Confirmed].
- Q: `competency_ids`, `prerequisite_ids` (`uuid[]`) và `subjects.grade_levels` (`smallint[]`) lưu thế nào khi ent không có kiểu mảng PostgreSQL? → A: `jsonb` mảng với CHECK `jsonb_array_length ≤ 20` (grade_levels: 1–9 phần tử, giá trị 1–9) và index GIN `jsonb_path_ops` cho `prerequisite_ids`; DATABASE_DESIGN cập nhật (OQ F021-2) [Confirmed].
- Q: Cặp môn × khối được tạo qua đâu khi contracts không có endpoint riêng? → A: `updateCurriculumVersionDraft` nhận `subject_grades[]` (thay cả danh sách của version draft, không gỡ được cặp còn topic) và `createTopic` tự tạo cặp còn thiếu; import tạo theo file (OQ F021-3) [Confirmed].
- Q: `createContentImport` cần `asset_id` của media (F027 chưa có)? → A: Port `AssetReader` đọc file theo `asset_id`; adapter thật đến với F027 (presign/object storage), test dùng test double gắn nhãn; seed đọc file cục bộ (`source_ref`). Job `queued` chạy bằng `ImportWorker.RunPending` (profile `jobs` của worker, nối vào binary cùng F020-3) (OQ F021-4) [Confirmed].
- Q: Version `rejected` quay lại `draft` bằng thao tác nào khi contract chỉ có `submit_review/approve/reject/archive`? → A: Thêm `revise` (`rejected|approved → draft`, `curriculum.write`) theo `contentmodel.Lifecycle` (OQ F021-6) [Confirmed].
- Q: `textbook_section_objectives` PK `(section_id, learning_objective_id)` nhưng bảng G cần soft-delete? → A: Khóa thay thế `section_objective_id` + UNIQUE `(section_id, objective_id) WHERE deleted_at IS NULL`; gán lại = soft-delete dòng cũ, thêm dòng mới (OQ F021-7) [Confirmed].
- Q: PATCH (merge-patch) dùng schema `*Input` có `required`? → A: Thêm schema `CurriculumPatch`, `CurriculumVersionPatch`, `TopicPatch`, `LearningObjectivePatch`, `SubjectPatch`, `CompetencyPatch`, `TextbookSectionPatch` (`minProperties: 1`); mã curriculum/môn/competency không đổi được sau khi tạo [Confirmed].
- Q: `content.import_jobs` loại H? → A: Loại T* (`entx.TenantOptional` + cột `owner_scope`): job không phải tài nguyên gán cho tenant; job platform (`tenant_id` NULL) ghi dưới PlatformScope (OQ F021-9) [Confirmed].
- Q: `createContentImport` thuộc action portal `question_bank.import` (`learning_content.import`)? → A: Giữ `x-portal-action` hiện có; quyền nghiệp vụ theo `kind`: `curriculum_yaml/curriculum_csv` cần `curriculum.write`, loại khác trả 422 tới F024 [Confirmed].
- Q: File CSV mang thông tin curriculum/version ở đâu? → A: Mỗi dòng một LO với cột `curriculum_code`, `curriculum_name_vi`, `version_label` lặp lại (phải giống nhau), `subject_code`, `grade_level`, `topic_code`, `topic_name`, `parent_topic_code`, `objective_code`, `description`, `bloom_level`, `yccd_ref`, `prerequisite_codes` (`|`); CSV không mang competency (chỉ YAML) — mô tả trong `specs/schemas/curriculum-import.schema.json` [Confirmed].
- Q: Nhân sự platform không có `platform.cross_tenant.read` (vd. `content_admin`) đọc CMS curriculum với scope nào? → A: `PlatformScope` của httpx (chỉ curriculum platform); System Admin xem curriculum của một tenant bằng `X-Tenant-Id`. Không dùng `scope.ForRead` (cần quyền liên tenant) cho CMS curriculum [Confirmed].
- Q: `/learning/*` (đọc công khai) kiểm quyền gì khi học sinh không có permission? → A: Principal hợp lệ có tenant phiên (`TenantIdHeader`); người không thuộc tenant bị httpx trả `403 TENANT_MISMATCH`; chỉ version `published`, không field biên tập [Confirmed].
- Q: Audit `authz.denied` khi từ chối trong transaction của use case (ghi curriculum platform)? → A: Ghi bằng context tách khỏi transaction (giữ principal + scope) vì transaction bị rollback [Confirmed].
- Q: Cache Valkey của cây curriculum (plan: `ew:learning:cur:tree:…`) có trong F021? → A: Chưa — read model đọc thẳng DB (version published bất biến nên có thể cache sau, khi có số liệu tải) (OQ F021-10) [Confirmed].
- Q: Seed duyệt version thế nào khi người duyệt phải khác tác giả? → A: Seed là job hệ thống (`SystemPlatformScope("curriculum-seed")`): `submit_review`/`approve` ghi `content_transitions` với `actor_id` = UUID rỗng, publish qua cùng hàm của use case (release, audit, event) [Confirmed].
- Q: Test integration đặt ở `tests/integration/curriculum/`? → A: Đặt trong package `tests/integration` (file `curriculum_*_test.go`) để dùng chung `TestMain`/cluster và fixture tenant; command/query trong package `command`/`query` phẳng, HTTP trong `internal/presentation/http` như module identity.
- Q: Kiểm thử `tools/routematrix` báo lỗi ở `tests/contract/platform_ops_auth_test.go` (F020, `authztest.Cover` với biến)? → A: Facade operation không thuộc ma trận public; bỏ lời gọi `Cover`, vòng lặp vẫn là test role-denied của facade.

### Session 2026-11-20 (follow-up OQ F053)

- Q: Portal kéo-thả cây chủ đề/LO cần `If-Match` từ danh sách? → A: `Topic.row_version`, `LearningObjective.row_version` (readOnly, = phiên bản dòng của ETag `W/"<row_version>"`) trả trong mọi list/get topic và LO (F053-AC-27, AC-28); test `TestF053Followups`.

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Soạn cấu trúc chương trình (Priority: P1)

Content Admin (`cm.noidung`) tạo curriculum platform, version nháp, cặp môn × khối, cây chủ đề, LO có mã chuẩn, `yccd_ref`, năng lực; Academic Manager tạo curriculum riêng của tenant.

**Why this priority**: Mọi câu hỏi/bài học phải gắn LO của một curriculum version (FR-CNT-002).

**Independent Test**: Gọi API CMS tạo đủ cây cho Toán lớp 3 rồi đọc lại; thử sai mã LO, sai quyền, khác tenant.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F021-AC-01 | KHI người có `curriculum.write` tạo curriculum (mã `UPPER_SNAKE`, duy nhất trong phạm vi sở hữu) kèm `Idempotency-Key`, HỆ THỐNG PHẢI tạo curriculum và version 1 ở `draft` | FR-CUR-001 · FR-CUR-005 · FR-CMS-002 | — |
| F021-AC-02 | HỆ THỐNG PHẢI cho phép nhiều curriculum và nhiều version của một curriculum tồn tại song song, mỗi version có `version_label` dạng `MAJOR.MINOR` và khoảng hiệu lực riêng | FR-CUR-002 · [CURRICULUM_MODEL §3.1](../../docs/02-learning/CURRICULUM_MODEL.md#31-curriculum--curriculumversion) | — |
| F021-AC-03 | KHI lưu LO, HỆ THỐNG PHẢI kiểm mã theo định dạng `<SUBJECT>.G<grade>.<STRAND>.<SKILL>[.<n>]` và duy nhất trong curriculum version; vi phạm PHẢI trả `422 VALIDATION_FAILED` chỉ rõ field | FR-CUR-003 · [CURRICULUM_MODEL §4](../../docs/02-learning/CURRICULUM_MODEL.md#4-mã-learning-objective-chuẩn-confirmed) | — |
| F021-AC-04 | KHI lưu LO có `yccd_ref` dài hơn 64 ký tự hoặc `competency_ids` không thuộc danh mục competency của cùng version, HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` | FR-CUR-009 | TC-LRN-018 |
| F021-AC-05 | KHI tạo topic, HỆ THỐNG PHẢI yêu cầu topic cha (nếu có) cùng version, môn và khối, và chặn vòng lặp cây với `422 VALIDATION_FAILED` | FR-CUR-001 · [CURRICULUM_MODEL §3.2](../../docs/02-learning/CURRICULUM_MODEL.md#32-topic) | — |
| F021-AC-06 | NẾU người gọi không có `curriculum.write` (vd. `school_admin`), hoặc người dùng tenant ghi môn học (danh mục platform) hay curriculum platform, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` và ghi audit `authz.denied` | [ADMIN_PORTAL §2.2](../../docs/04-admin-portal/ADMIN_PORTAL.md#22-permission-catalog) · AC-CMS-001 | — |
| F021-AC-07 | TRONG KHI một curriculum version không ở `draft`, HỆ THỐNG PHẢI từ chối mọi thay đổi topic, LO, competency, môn × khối của version đó với `409 CONTENT_NOT_EDITABLE` | ADR-0015 | TC-CMS-002 |
| F021-AC-08 | HỆ THỐNG PHẢI chỉ trả curriculum của tenant phiên hoặc curriculum platform được gán; curriculum riêng tenant A PHẢI trả `404 NOT_FOUND` với người dùng tenant B | AC-ORG-001 · FR-CNT-005 | TC-TEN-001 |
| F021-AC-09 | NẾU `PATCH` thiếu `If-Match`, THÌ HỆ THỐNG PHẢI trả `428 PRECONDITION_REQUIRED`; NẾU ETag lệch, THÌ PHẢI trả `412 VERSION_MISMATCH` | [API_DESIGN §7](../../docs/07-backend/API_DESIGN.md#7-optimistic-concurrency-cmslms) | — |
| F021-AC-10 | KHI xóa curriculum, topic hoặc LO thuộc version `draft`, HỆ THỐNG PHẢI soft-delete, đưa vào Thùng rác (`resource_type = curriculum`) và ghi audit; curriculum đã có version published PHẢI chỉ được archive | FR-CMS-014 · ADR-0028 | TC-TEN-015 |

---

### User Story 2 — Đồ thị LO tiên quyết (Priority: P1)

Người soạn khai báo LO tiên quyết; hệ thống chặn chu trình để gợi ý ôn tập và bài chẩn đoán dùng được.

**Why this priority**: FR-CUR-007 là P2 và là đầu vào của F032.

**Independent Test**: Tạo A→B→C, thử thêm C→A, kiểm lỗi trả đường đi chu trình.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F021-AC-11 | KHI lưu `prerequisite_ids`, HỆ THỐNG PHẢI yêu cầu mọi LO tiên quyết cùng curriculum version, không quá 20 phần tử và không tự tham chiếu | FR-CUR-007 | — |
| F021-AC-12 | NẾU thay đổi tạo chu trình trong đồ thị tiên quyết, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` với `errors[].code = PREREQUISITE_CYCLE` và danh sách mã LO trên chu trình, không lưu thay đổi | FR-CUR-007 · [CURRICULUM_MODEL §10](../../docs/02-learning/CURRICULUM_MODEL.md#10-kiểm-thử) | — |

---

### User Story 3 — Duyệt và publish CurriculumVersion (Priority: P1)

Version nháp đi qua gửi duyệt → duyệt → publish; sau publish bất biến; muốn sửa thì tạo version mới.

**Why this priority**: Nội dung chỉ publish được khi LO thuộc version đã publish.

**Independent Test**: Đi hết workflow cho version Toán 3; thử sửa sau publish; thử publish `GDPT2018` thiếu `yccd_ref`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F021-AC-13 | KHI chuyển trạng thái version (`submit_review`, `approve`, `reject`, `archive`), HỆ THỐNG PHẢI tuân state machine, yêu cầu người duyệt khác tác giả, `reject` bắt buộc ghi chú, ghi `content.content_transitions` và audit; chuyển sai PHẢI trả `422 INVALID_STATE_TRANSITION` | FR-CUR-005 · [CONTENT_MODEL §6](../../docs/02-learning/CONTENT_MODEL.md#6-workflow-trạng-thái) | TC-CMS-001 |
| F021-AC-14 | NẾU publish version của curriculum `GDPT2018` khi còn LO thiếu `yccd_ref`, THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` liệt kê mã LO thiếu | FR-CUR-009 · [CURRICULUM_MODEL §3.3](../../docs/02-learning/CURRICULUM_MODEL.md#33-learningobjective) | TC-LRN-018 |
| F021-AC-15 | KHI publish version `approved` với `Idempotency-Key`, HỆ THỐNG PHẢI trong một transaction chuyển version sang `published`, đặt `current_version_id` của curriculum, ghi `content.content_releases` (`item_type = curriculum`), audit và outbox `curriculum.version_published`; gọi lại cùng key PHẢI trả kết quả cũ | FR-CUR-002 · ADR-0043 · R10 | TC-CMS-003 |
| F021-AC-16 | HỆ THỐNG PHẢI chặn mọi UPDATE nội dung của version `published` ở cả use case và trigger DB (`409 CONTENT_NOT_EDITABLE`) | ADR-0015 · R11 | TC-CMS-002 |
| F021-AC-17 | KHI tạo version mới từ một version, HỆ THỐNG PHẢI sao sâu môn × khối, topic, LO, competency sang version `draft` mới (ID mới, giữ `code` và `stable_key`, `supersedes_version_id` trỏ version nguồn) và từ chối nếu item đã có một version `draft`/`in_review` mở (`409 CONFLICT`) | FR-CUR-002 · [CURRICULUM_MODEL §5](../../docs/02-learning/CURRICULUM_MODEL.md#5-đa-chương-trình--di-chuyển-phiên-bản) | TC-CMS-004 |
| F021-AC-18 | NẾU người không có `curriculum.publish` gọi approve/reject/publish, THÌ HỆ THỐNG PHẢI trả `403 PERMISSION_DENIED` | [ADMIN_PORTAL §2.3](../../docs/04-admin-portal/ADMIN_PORTAL.md#23-role--permission) | — |

---

### User Story 4 — Đọc cây curriculum cho game và portal (Priority: P2)

Học sinh và nhân sự đọc cây môn → chủ đề → LO của version đang áp dụng cho trường mình.

**Why this priority**: Game render khu học tập, LMS lọc nội dung theo LO.

**Independent Test**: `hs.an.3a` gọi `listCurricula`, `listTopics`, `listLearningObjectives`; trường có/không cấu hình version mặc định.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F021-AC-19 | KHI người dùng gọi `listCurricula`, HỆ THỐNG PHẢI trả curriculum đã publish khả dụng cho tenant và version đang áp dụng cho trường của người dùng theo cấu hình version mặc định năm học; không có cấu hình thì PHẢI dùng version published có khoảng hiệu lực chứa ngày hiện tại | FR-CUR-006 · FR-CUR-008 | TC-LRN-001 |
| F021-AC-20 | HỆ THỐNG PHẢI chỉ trả version `published` qua `/learning/*` và không trả metadata biên tập (tác giả, người duyệt, ghi chú version, trạng thái nháp) | FR-CUR-006 · [CURRICULUM_MODEL §9](../../docs/02-learning/CURRICULUM_MODEL.md#9-api-tóm-tắt) | — |
| F021-AC-21 | KHI tra LO theo ID của một version cũ (đã có version mới), HỆ THỐNG PHẢI vẫn trả đúng LO đó để lịch sử attempt/learning event hiển thị đúng | FR-CUR-002 · [CURRICULUM_MODEL §5](../../docs/02-learning/CURRICULUM_MODEL.md#5-đa-chương-trình--di-chuyển-phiên-bản) | TC-LRN-006 |

---

### User Story 5 — Import/export và seed GDPT 2018 Toán 1–5 (Priority: P2)

Content Admin nhập cây chương trình từ YAML/CSV, xuất lại để rà soát; seed dev/pilot chạy qua cùng use case.

**Why this priority**: Đội nội dung cần nhập nhanh; seed không được viết SQL tay (FR-CUR-010).

**Independent Test**: Chạy seed hai lần; import file lỗi ở dry-run; export rồi import lại.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F021-AC-22 | KHI gọi `createContentImport` với `kind` curriculum (`curriculum_yaml`/`curriculum_csv`) và `Idempotency-Key`, HỆ THỐNG PHẢI tạo job `202`, kiểm toàn bộ file bằng cùng validator của CMS, với `dry_run` không ghi gì, ngược lại tạo version `draft`, và trả lỗi theo dòng (`row`, `field`, `code`) qua `getContentImport` | FR-CUR-010 | — |
| F021-AC-23 | HỆ THỐNG PHẢI nạp seed GDPT 2018 Toán lớp 1–5 bằng đúng use case import (không SQL thủ công); chạy lại `make seed` PHẢI không tạo bản ghi trùng | FR-CUR-004 · [CURRICULUM_MODEL §7](../../docs/02-learning/CURRICULUM_MODEL.md#7-seed-ví-dụ--gdpt-2018-toán-lớp-15-confirmed-oq-08) | — |
| F021-AC-24 | KHI xuất một version (`yaml`/`csv`), HỆ THỐNG PHẢI trả file mà import lại cho cấu trúc tương đương (cùng mã topic, mã LO, tiên quyết, `yccd_ref`) | FR-CUR-010 | — |
| F021-AC-25 | NẾU file import vượt `csv_import_max` hoặc sai định dạng, THÌ job PHẢI kết thúc `failed` với lỗi rõ ràng và không ghi dữ liệu | FR-CUR-010 · NFR-MAINT-005 | — |

---

### User Story 6 — Mapping LO giữa hai version (Priority: P3)

Khi publish version thay thế, người soạn khai báo LO cũ ánh xạ sang LO mới để mastery được chiếu sang.

**Why this priority**: FR-CUR-011 là P3, cần khi chương trình đổi giữa năm học.

**Independent Test**: Tạo version 2 từ version 1, đổi một LO (split), thiếu mapping → publish bị chặn.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F021-AC-26 | NẾU publish version có `supersedes_version_id` mà còn LO của version cũ chưa có mapping (`same`, `split`, `merge`, `replaces`, `removed`), THÌ HỆ THỐNG PHẢI trả `422 VALIDATION_FAILED` liệt kê LO thiếu | FR-CUR-011 | — |
| F021-AC-27 | KHI version thay thế được publish, HỆ THỐNG PHẢI đưa bảng mapping vào payload `curriculum.version_published` cho consumer mastery và không sửa learning event gốc nào | FR-CUR-011 · [CURRICULUM_MODEL §5](../../docs/02-learning/CURRICULUM_MODEL.md#5-đa-chương-trình--di-chuyển-phiên-bản) | — |

---

### User Story 7 — Mapping bộ sách giáo khoa → LO (Priority: P3)

Content Admin khai báo bộ sách, tập, bài và ánh xạ bài ↔ LO để người biên tập và Academic Manager lọc theo bài sách.

**Why this priority**: FR-CUR-012 P3, [TBD] OQ-08.

**Independent Test**: Tạo bộ sách "Cánh Diều" Toán 3 tập 1, bài 12 gắn 2 LO; lọc LO theo bài.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F021-AC-28 | KHI Content Admin tạo bộ sách, tập sách, bài sách và gán bài ↔ LO (n–n), HỆ THỐNG PHẢI chỉ lưu metadata (mã, tên, nhà xuất bản, chương, số bài, tiêu đề, trang) và không có trường lưu nguyên văn sách | FR-CUR-012 · [CURRICULUM_MODEL §6](../../docs/02-learning/CURRICULUM_MODEL.md#6-mapping-bộ-sách-giáo-khoa-confirmed-tbd--oq-08) | — |
| F021-AC-29 | KHI lọc LO theo `textbook_section_id`, HỆ THỐNG PHẢI trả đúng các LO đã gán cho bài sách đó trong phạm vi tenant thấy được | FR-CUR-012 | — |

### Edge Cases

- NẾU xóa LO đang là tiên quyết của LO khác trong version draft, THÌ HỆ THỐNG PHẢI từ chối `422 VALIDATION_FAILED` liệt kê LO phụ thuộc (F021-AC-11).
- NẾU LO đã được nội dung tham chiếu, THÌ chỉ được sửa trong version draft mới, không xóa ở version đã publish (F021-AC-16).
- NẾU hai version cùng curriculum có khoảng hiệu lực chồng nhau cho cùng môn × khối, THÌ HỆ THỐNG PHẢI cảnh báo khi submit review (không chặn) để trường chọn version mặc định (F021-AC-19).
- NẾU tenant đã gán curriculum platform rồi bị gỡ gán, THÌ `listCurricula` không còn trả curriculum đó; LO cũ vẫn tra được cho lịch sử (F021-AC-21).

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-CUR-001 | Cấu trúc Curriculum → Version → Môn × Khối → Topic → LO (cấp 6–9 do F024/F030 làm) | [CURRICULUM_MODEL §2](../../docs/02-learning/CURRICULUM_MODEL.md#2-cấu-trúc-phân-cấp) |
| FR-CUR-002 | Nhiều curriculum, nhiều version song song, version bất biến | [CURRICULUM_MODEL §3.1, §5](../../docs/02-learning/CURRICULUM_MODEL.md#31-curriculum--curriculumversion) |
| FR-CUR-003 | Mã LO chuẩn, `stable_key` | [CURRICULUM_MODEL §4](../../docs/02-learning/CURRICULUM_MODEL.md#4-mã-learning-objective-chuẩn-confirmed) |
| FR-CUR-004 | Seed GDPT 2018 Toán 1–5 qua import | [CURRICULUM_MODEL §7](../../docs/02-learning/CURRICULUM_MODEL.md#7-seed-ví-dụ--gdpt-2018-toán-lớp-15-confirmed-oq-08) |
| FR-CUR-005 | CRUD CMS + workflow CurriculumVersion | [CURRICULUM_MODEL §9](../../docs/02-learning/CURRICULUM_MODEL.md#9-api-tóm-tắt) |
| FR-CUR-006 | API đọc cây cho client | [CURRICULUM_MODEL §9](../../docs/02-learning/CURRICULUM_MODEL.md#9-api-tóm-tắt) |
| FR-CUR-007 | DAG tiên quyết, phát hiện chu trình | [CURRICULUM_MODEL §3.3](../../docs/02-learning/CURRICULUM_MODEL.md#33-learningobjective) |
| FR-CUR-008 | Đọc version mặc định theo năm học của trường (cấu hình ở identity) | [CURRICULUM_MODEL §5](../../docs/02-learning/CURRICULUM_MODEL.md#5-đa-chương-trình--di-chuyển-phiên-bản) |
| FR-CUR-009 | `yccd_ref`, `competency_ids`, danh mục competency, chặn publish `GDPT2018` thiếu `yccd_ref` | [CURRICULUM_MODEL §3.3–§3.4](../../docs/02-learning/CURRICULUM_MODEL.md#34-competency-năng-lực-phẩm-chất-confirmed) |
| FR-CUR-010 | Import/export YAML/CSV có validate, Idempotency-Key, audit | [CURRICULUM_MODEL §9](../../docs/02-learning/CURRICULUM_MODEL.md#9-api-tóm-tắt) |
| FR-CUR-011 | Bảng mapping LO khi publish version thay thế (chiếu mastery ở F031) | [CURRICULUM_MODEL §5](../../docs/02-learning/CURRICULUM_MODEL.md#5-đa-chương-trình--di-chuyển-phiên-bản) |
| FR-CUR-012 | Bộ sách → tập → bài ↔ LO, không sao chép nguyên văn | [CURRICULUM_MODEL §6](../../docs/02-learning/CURRICULUM_MODEL.md#6-mapping-bộ-sách-giáo-khoa-confirmed-tbd--oq-08) |
| FR-CMS-002 | Backend của Curriculum Management (UI ở F053) | [ADMIN_PORTAL §3.1](../../docs/04-admin-portal/ADMIN_PORTAL.md#31-curriculum-management-master-prompt-81) |

### Key Entities

- **Curriculum**: chương trình (mã, tên, phạm vi sở hữu), con trỏ version hiện hành.
- **CurriculumVersion**: bản chương trình (nhãn `MAJOR.MINOR`, trạng thái, hiệu lực, version bị thay thế).
- **Subject**: môn học — danh mục platform dùng chung.
- **SubjectGrade**: cặp môn × khối trong một version.
- **Topic**: cây chủ đề trong một môn × khối của version.
- **LearningObjective**: đơn vị đo mastery; mã chuẩn, `stable_key`, Bloom, `yccd_ref`, năng lực, tiên quyết.
- **Competency**: năng lực/phẩm chất của version.
- **LoVersionMapping**: ánh xạ LO giữa hai version.
- **TextbookSeries / Textbook / TextbookSection**: bộ sách, tập, bài và liên kết bài ↔ LO.
- **ImportJob**: job nhập file có báo cáo lỗi theo dòng.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `csv_import_max` | Kích thước tối đa file import |
| `request_body_max` | Giới hạn body của API CMS |
| `idempotency_record_ttl` | Thời hạn khóa idempotency cho tạo/publish/import |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Version published bất biến; sửa = version mới | ADR-0015 · AGENTS R11 |
| Mọi chuỗi có giới hạn (`yccd_ref` ≤ 64, mã ≤ 64) | NFR-MAINT-005 · ADR-0041 |
| Tenant isolation 100% endpoint | NFR-SEC-006 · ADR-0005 |
| Idempotency + audit cho publish/import | NFR-SEC-004 · AGENTS R10 |
| Không sao chép nguyên văn sách giáo khoa | [CURRICULUM_MODEL §6](../../docs/02-learning/CURRICULUM_MODEL.md#6-mapping-bộ-sách-giáo-khoa-confirmed-tbd--oq-08) |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Seed Toán lớp 1–5 (16 LO mẫu) nạp đầy đủ trong một lệnh và chạy lại không sinh trùng.
- **SC-002**: 100% thao tác sửa version đã publish bị từ chối; 0 learning event bị sửa khi publish version mới.
- **SC-003**: 100% chu trình tiên quyết bị phát hiện trong bộ test (gồm chu trình dài ≥ 5 LO).
- **SC-004**: Export rồi import lại một version cho cấu trúc giống hệt (so sánh mã topic/LO, tiên quyết, `yccd_ref`).

## Ngoài phạm vi

- UI cây curriculum, editor LO, so sánh version — F053.
- Lưu và sửa cấu hình version mặc định của trường — F015 (identity); F021 chỉ đọc.
- Chiếu mastery theo mapping — F031 (tiêu thụ `curriculum.version_published`).
- LearningUnit, Lesson, LearningActivity (cấp 6–8) — F024, F030.
- Media upload cho file import qua portal — F027 (`presignMediaUpload`); seed chạy bằng file cục bộ.
- Rà soát nội dung chương trình theo văn bản chính thức (điền `yccd_ref` thật) — EdTech Specialist, ngoài phạm vi code (OQ-08).

## Assumptions

- Mã curriculum duy nhất trong phạm vi sở hữu (platform; từng tenant) để không lộ sự tồn tại curriculum của tenant khác qua `409` [Confirmed].
- Tên topic và mô tả LO lưu theo ngôn ngữ của curriculum (một bản), khớp `TopicInput.name`/`LearningObjectiveInput.description` của OpenAPI; tên curriculum/môn là `LearningName` [Confirmed].
- Seed dùng mã `GDPT2018_SAMPLE`; file seed ở `src/backend/modules/learning/seed/curriculum/gdpt2018_math_g1_5.yaml` (thay đường dẫn cũ `backend/migrations/...` trong CURRICULUM_MODEL §7) [Confirmed].
- Cấu hình version mặc định của trường đến module learning qua projection `ref.schools` (cột mới + event của F015) [Confirmed].
