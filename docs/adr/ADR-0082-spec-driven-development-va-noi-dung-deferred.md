# ADR-0082 — Spec-Driven Development (Spec Kit + EARS) cho AI coding; tách nội dung không làm vào thư mục deferred

> **Trạng thái:** Accepted · **Ngày:** 2026-11-15 · **OQ:** OQ-12, OQ-24 · **Bổ sung:** ADR-0010, ADR-0072 · **Tài liệu:** [Constitution](../../.specify/memory/constitution.md) · [CODE_PLAN](../12-project/CODE_PLAN_CMS_LMS_FIRST.md) · [AGENTS.md](../../AGENTS.md) · [docs/README](../README.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

Code của EduWorld sẽ được viết chủ yếu bởi AI coding agent. Bộ tài liệu hiện có (PRD, tài liệu domain, ADR, contract trong `specs/`) đầy đủ nhưng phân tán: để làm một tính năng, agent phải tự gom FR, quy tắc, tham số, contract và test từ nhiều nơi, dễ sót hoặc hiểu sai. Một số nội dung không làm trong version này (kỹ năng viết/nói, pháp lý) vẫn nằm lẫn trong cây tài liệu và specs, dễ bị triển khai nhầm. Ngoài ra nhiều tài liệu domain có bảng FR cục bộ dùng lại mã FR của PRD với nghĩa khác.

## Quyết định

### 1. Mô hình SDD: Spec Kit + EARS [Confirmed]

Bố cục theo **chuẩn GitHub Spec Kit**:

| Thành phần | Vị trí |
|---|---|
| Constitution (9 điều khoản + câu hỏi gate; chi tiết quy tắc do AGENTS.md §2 sở hữu) | `.specify/memory/constitution.md` |
| Template | `.specify/templates/{spec,plan,tasks,checklist,data-model,contracts,quickstart}-template.md` |
| Feature spec | `specs/<NNN>-<slug>/` (cạnh các thư mục contract `specs/openapi`, `specs/asyncapi`, `specs/schemas`, `specs/events`) |
| Danh mục feature, thứ tự triển khai, trạng thái | [CODE_PLAN_CMS_LMS_FIRST](../12-project/CODE_PLAN_CMS_LMS_FIRST.md) §4–§6 |
| Lệnh cho AI agent | `.claude/commands/speckit.{specify,clarify,plan,tasks,analyze,implement}.md` |
| Kiểm tra tự động | `scripts/specs/check-features.mjs` |

Mỗi thư mục feature gồm:

- `spec.md` — **cái gì & vì sao**: user story có ưu tiên (P1/P2/P3) và *Independent Test*; **tiêu chí chấp nhận viết theo EARS** mã `F<NNN>-AC-<NN>` ("HỆ THỐNG PHẢI …", "KHI … , HỆ THỐNG PHẢI …", "TRONG KHI …", "NẾU … THÌ …", "Ở NƠI …"); Functional Requirements (mã PRD); Key Entities; tham số theo tên CONFIG_DEFAULTS; Success Criteria đo được; Ngoài phạm vi; Clarifications và `[NEEDS CLARIFICATION]`.
- `plan.md` — **làm thế nào**: Technical Context, **Constitution Check** (gate), Project Structure, thiết kế theo tầng, tích hợp, bảo mật/tenant/idempotency/audit, quan sát, Complexity Tracking. Kèm `research.md` (khi có quyết định cần nghiên cứu), `data-model.md`, `contracts/README.md` (liệt kê contract dùng và delta, không chép contract), `quickstart.md` (cách chạy và kiểm, bằng chứng nghiệm thu).
- `checklists/requirements.md` — kiểm chất lượng spec trước khi lập kế hoạch.
- `tasks.md` — task `T001 [P] [USn] … — file: …` theo phase: Setup → Foundational → từng user story (test viết trước, phải FAIL trước khi code) → Polish; Dependencies & Execution Order; Definition of Done.

Quy trình cho AI agent: `/speckit.specify` → `/speckit.clarify` → `/speckit.plan` → `/speckit.tasks` → `/speckit.analyze` → `/speckit.implement`; chỉ code feature `Ready` có mọi phụ thuộc `Done`; báo cáo theo AGENTS §5.

### 2. Sở hữu nội dung (bổ sung R33)

| Loại nội dung | Nơi sở hữu |
|---|---|
| Yêu cầu chức năng (FR, US) | [PRODUCT_REQUIREMENTS](../00-overview/PRODUCT_REQUIREMENTS.md) — **nguồn duy nhất của mã FR** |
| Quy tắc nghiệp vụ, thiết kế domain | Tài liệu domain trong `docs/` |
| Tham số mặc định | [CONFIG_DEFAULTS](../00-overview/CONFIG_DEFAULTS.md) + tài liệu sở hữu |
| Contract API/event/schema | `specs/openapi`, `specs/asyncapi`, `specs/schemas`, `specs/events` |
| Tiêu chí chấp nhận thực thi (EARS) và kế hoạch triển khai của một tính năng | `specs/<NNN>-<slug>/` |
| Danh mục, thứ tự và trạng thái feature | [CODE_PLAN_CMS_LMS_FIRST](../12-project/CODE_PLAN_CMS_LMS_FIRST.md) |
| Test case | [TEST_STRATEGY](../12-project/TEST_STRATEGY.md) |

- Tiêu chí EARS phải ghi nguồn (mã FR/AC và mục tài liệu domain), dùng **tên tham số** chứ không chép giá trị. Mâu thuẫn giữa feature spec và tài liệu sở hữu là lỗi phải sửa trong cùng merge request.
- **Mã FR chỉ do PRD cấp.** Tài liệu domain không có bảng FR cục bộ với mã riêng; khi cần chi tiết, tài liệu domain dùng đúng mã FR của PRD hoặc mã AC riêng của domain.

### 3. Nội dung không làm trong version này → thư mục deferred [Confirmed]

- **Không làm:** kỹ năng viết/nói (`written_response`, `speaking`, `project`, rubric, hàng đợi chấm, AI chấm bài — ADR-0053) và **pháp lý** (rà soát theo luật bảo vệ dữ liệu cá nhân, mô hình consent, DPIA, quy định game cho người dưới 18 tuổi — ADR-0077, OQ-12, OQ-24).
- Tài liệu thiết kế của các nội dung này chuyển sang `docs/99-deferred/`; schema/example chuyển sang `specs/_deferred/`. Hai thư mục này **không** thuộc phạm vi codegen, lint contract, feature spec hay AI coding; `AGENTS.md` cấm agent đọc để triển khai.
- Các nguyên tắc **kỹ thuật** về bảo vệ dữ liệu vẫn áp dụng và ở lại tài liệu chính: tối thiểu hóa dữ liệu, không log PII, định danh giả trong analytics, mã hóa, soft-delete và ẩn danh hóa, cô lập tenant.
- Mở lại một nội dung deferred phải có ADR mới, chuyển tài liệu về cây chính và viết feature spec.

### 4. Tham số mặc định

Các tham số đăng ký trong đợt sửa ngày 2026-11-15 (ADR-0072 và EDU_FIT_REVIEW) được Technical Architect duyệt, chuyển nhãn [Confirmed].

## Hệ quả

(+) AI agent có một điểm vào rõ ràng cho từng tính năng, có tiêu chí kiểm thử được và thứ tự task; giảm hiểu sai và triển khai nhầm phạm vi. Truy vết FR → feature → TC tường minh. (−) Thêm khoảng 70 feature spec phải bảo trì cùng tài liệu domain; mọi thay đổi quy tắc phải cập nhật feature spec liên quan trong cùng merge request. Rủi ro pháp lý khi chạy với dữ liệu trẻ em thật được chấp nhận ở version này (ghi trong RISKS).

## Phương án đã loại

- **Kiro (requirements/design/tasks) thuần** và **Spec Kit với Given/When/Then:** không chọn; EARS ngắn gọn, máy dễ kiểm và hợp với yêu cầu phản ứng của hệ thống.
- **Xóa hẳn nội dung không làm:** không chọn để giữ thiết kế cho phiên bản sau.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Tạo ADR | Technical Architect |
| 2026-11-16 | Bố cục theo chuẩn GitHub Spec Kit: `.specify/`, `specs/<NNN>-<slug>/`, danh mục trong CODE_PLAN, lệnh `/speckit.*` | Technical Architect |
