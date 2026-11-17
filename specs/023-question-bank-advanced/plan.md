# Implementation Plan: F023 — Question Bank nâng cao: short_answer, image_based, interactive, câu tham số hóa

**Branch**: `023-question-bank-advanced` · **Date**: 2026-11-16 · **Spec**: [spec.md](spec.md)

## Summary

Đăng ký thêm ba `TypeHandler` (`short_answer`, `image_based`, `interactive`) vào registry của F022 và một lớp `ParamExpander` dùng chung cho `single_choice`/`numeric`. Thành phần mới đều ở domain thuần: `geometry` (điểm trong vùng), `widget` (registry widget + mô phỏng phát lại + validator), `expr` (lexer/parser/evaluator an toàn, AST giới hạn độ sâu), `params` (sinh giá trị theo seed với rejection sampling). Không thêm bảng; dùng cột `render_payload`/`answer_key` của F022. Thêm endpoint đọc `listQuestionWidgets` cho editor portal.

## Technical Context

| Mục | Giá trị |
|---|---|
| Language/Version | Go ≥ 1.24 |
| Primary Dependencies | registry F022, `santhosh-tekuri/jsonschema/v6`, `cockroachdb/apd/v3` (evaluator decimal), `math/rand/v2` (PCG) |
| Storage | PostgreSQL `eduworld_learning` (bảng của F022, không đổi cột) |
| Testing | Bảng test, property test, fuzz test (`go test -fuzz`) cho `expr` và replay widget |
| Target Platform | Linux container |
| Project Type | backend module |
| Performance Goals | Chấm `interactive` (replay ≤ 500 thao tác) < 5 ms p99; sinh tham số < 2 ms p99 (benchmark) |
| Constraints | ADR-0016; AGENTS R5, R6; QUESTION_BANK §3.11, §5 |
| Scale/Scope | Giới hạn độ dài theo schema (ADR-0041) |

## Constitution Check

*GATE: phải đạt trước Phase 0 và kiểm lại sau Phase 1.* Câu hỏi gate ở [constitution](../../.specify/memory/constitution.md).

| Điều | Kết quả | Ghi chú |
|---|---|---|
| I Spec trước, code sau | ✅ | Delta `question.schema.json` (giới hạn mảng, schema widget), `listQuestionWidgets` ở Phase 2 |
| II Ranh giới module, Clean Architecture + CQRS | ✅ | Toàn bộ logic mới ở domain; handler chỉ đọc registry |
| III Cô lập tenant, an toàn dữ liệu | ✅ | Dùng lại repository + scope của F022 |
| IV Server-authoritative | ✅ | Replay action log; evaluator không eval tổng quát; DTO không có vùng/biểu thức |
| V Xác thực, phân quyền | ✅ | `learning_content.read/write` như F022 |
| VI An toàn trẻ em | ✅ | Không có dữ liệu học sinh; không log response ở INFO |
| VII Một nguồn sự thật, cấu hình | ⚠️ | Số lần thử sinh tham số là hằng code mới — đăng ký CONFIG_DEFAULTS ở T034 |
| VIII UI tách logic, token | ✅ | Không có UI |
| IX Test có bằng chứng | ✅ | Mỗi F023-AC có task test; fuzz test cho evaluator |

## Project Structure

### Documentation (feature này)

```text
specs/023-question-bank-advanced/
├── spec.md
├── plan.md
├── data-model.md
├── quickstart.md
├── contracts/README.md
├── checklists/requirements.md
└── tasks.md
```

### Source Code (repository)

```text
modules/learning/
├── internal/domain/question/
│   ├── types/{short_answer.go,image_based.go,interactive.go}
│   ├── normalize/{levenshtein.go}                 # fuzzy, dùng chung chuẩn hóa F022
│   ├── geometry/{circle.go,rect.go,polygon.go}
│   ├── widget/{registry.go,balance_scale_v1.go}   # Widget{Code, Version, ConfigSchema, Replay(config, log) (state, error), Validators map}
│   ├── expr/{lexer.go,parser.go,eval.go}          # AST giới hạn độ sâu 32, chỉ toán tử cho phép
│   └── params/{spec.go,sampler.go,expand.go}
├── internal/application/query/question/list_widgets.go
├── internal/presentation/http/public/cms_question_widgets.go   # listQuestionWidgets
└── tests/{unit,integration,contract}/question/advanced/
```

**Structure Decision**: Mở rộng registry F022 (không sửa scorer lõi); widget và evaluator tách package để fuzz test độc lập.

## Phase 0: Research

Không cần research.md: thuật toán even-odd, Levenshtein và parser Pratt là chuẩn; lựa chọn decimal/PRNG đã ghi ở [F022 research](../022-question-bank-core/research.md).

## Phase 1: Design & Contracts

- **Data model** → [data-model.md](data-model.md)
- **Contracts** → [contracts/README.md](contracts/README.md)
- **Quickstart** (cách chạy và kiểm) → [quickstart.md](quickstart.md)

### Module và tầng

| Tầng | Thành phần | Ghi chú |
|---|---|---|
| domain | `ShortAnswerHandler`, `ImageBasedHandler`, `InteractiveHandler`, `geometry.Contains`, `widget.Registry`, `expr.Parse/Eval`, `params.Sample(seed)`, `params.Expand(version, values)` | Thuần Go |
| application | Query `ListQuestionWidgets`; `Delivery.Render/Grade` của F022 gọi `params.Expand` khi version có `params` | `learning_content.read` |
| infrastructure | Không mới (dùng repo F022) | |
| presentation | Handler `listQuestionWidgets` | |

### Tích hợp và phụ thuộc

- F022 (registry, chuẩn hóa, `Delivery`), F021 (mã môn để chặn fuzzy). Bên dùng: F030 (lưu `presentation.params`, cờ `integrity_mismatch` → `assessment.integrity_signals`), F063 (scene widget).

### Bảo mật, tenant, idempotency, audit

- Không endpoint ghi mới; tạo/sửa câu đi qua endpoint F022 (Idempotency-Key, If-Match, audit như F022).
- Evaluator: giới hạn độ dài biểu thức 255, độ sâu AST, số mũ ≤ 10, chặn tràn số (decimal context có giới hạn chữ số).

### Quan sát

- Metric `eduworld_qb_interactive_integrity_mismatch_total{widget}`, `eduworld_qb_param_sampling_failures_total`; span `learning.qb.grade` (thuộc tính `type`).

## Complexity Tracking

| Vi phạm | Vì sao cần | Phương án đơn giản hơn bị loại vì |
|---|---|---|
| VII: hằng số số lần thử sinh tham số chưa có trong CONFIG_DEFAULTS | Cần giới hạn để kiểm khả thi khi lưu và tránh vòng lặp vô hạn khi sinh | Không giới hạn thì có thể treo request; đăng ký tham số ở T034 trong cùng PR |
