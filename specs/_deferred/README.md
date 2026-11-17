# specs/_deferred — contract không thuộc version này

> Trạng thái: Approved · Owner: Backend Architect · Cập nhật: 2026-11-15 · Liên quan: [ADR-0082](../../docs/adr/ADR-0082-spec-driven-development-va-noi-dung-deferred.md) · [docs/99-deferred](../../docs/99-deferred/README.md)

> [!WARNING]
> Không sinh code, không lint contract, không triển khai từ thư mục này. Script trong `scripts/specs/` và `scripts/openapi/` không quét `specs/_deferred/`.

| File | Nội dung |
|---|---|
| `schemas/activities/activity-deferred-common.schema.json` | Phần chung của các activity deferred (`deferredActivityCommon`, `deferredEvaluation`, `deferredFeedback`, `deferredCompletionRule`, `deferredActivityType`) — tách khỏi `specs/schemas/activities/activity-base.schema.json` ngày 2026-11-15; vẫn `$ref` các kiểu cơ bản (`uuid`, `i18n*`, `mathDisplay`…) của `activity-base` |
| `schemas/activities/written-response.schema.json` | Cấu hình activity tự luận |
| `schemas/activities/speaking.schema.json` | Cấu hình activity bài nói |
| `schemas/activities/project.schema.json` | Cấu hình activity dự án |
| `schemas/activities/rubric.schema.json` | Rubric chấm tay |
| `examples/activity.written-response.math-g7-equation.json` | Ví dụ activity tự luận |
| `examples/rubric.literature-g7-essay.json` | Ví dụ rubric bài văn lớp 7 |

Kiểm tra thủ công (ngày 2026-11-15, không chạy trong CI): 6 schema compile Ajv strict cùng `specs/schemas`, 2 example hợp lệ.
