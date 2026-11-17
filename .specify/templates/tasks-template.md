# Tasks: F[NNN] — [Tên tính năng]

**Input**: [spec.md](spec.md), [plan.md](plan.md), [data-model.md](data-model.md), [contracts/](contracts/README.md), [quickstart.md](quickstart.md)

<!--
Chuẩn Spec Kit (ADR-0082). Định dạng: `- [ ] T001 [P] [US1] Mô tả — file: <đường dẫn>`
- [P]: chạy song song được (khác file, không phụ thuộc task chưa xong).
- [USn]: thuộc user story n trong spec.md (Setup/Foundational/Polish không có nhãn story).
- Test của mỗi story viết TRƯỚC phần code của story và phải FAIL trước khi code (constitution điều IX).
- Mỗi story hoàn thành là một phần tăng trưởng kiểm được độc lập.
- Thứ tự trong story: contract → model/ent → domain → application → infrastructure/presentation → client.
-->

## Phase 1: Setup

- [ ] T001 [Khởi tạo cấu trúc/cấu hình cần cho feature] — file: `…`

## Phase 2: Foundational (chặn mọi user story)

- [ ] T00x [Contract chung: OpenAPI/AsyncAPI/schema/event/spec internal facade] — file: `specs/…` — kiểm: `make specs-lint`, `make gen MOD=<module>`, `make arch-check`
- [ ] T00x [ent schema + migration] — file: `modules/<module>/ent/schema/…` — kiểm: `make migrate-diff MOD=<module>`, `make migrate-lint MOD=<module>`

**Checkpoint**: nền tảng sẵn sàng, các story có thể bắt đầu.

## Phase 3: User Story 1 — [Tiêu đề] (Priority: P1) 🎯 MVP

**Goal**: [story mang lại gì]

**Independent Test**: [như spec.md]

### Tests cho User Story 1

- [ ] T0xx [P] [US1] Test cho F[NNN]-AC-01 … — file: `…_test.go` — phải FAIL trước khi code
- [ ] T0xx [P] [US1] Test tenant isolation + authorization cho endpoint mới — file: `…`

### Implementation cho User Story 1

- [ ] T0xx [P] [US1] [Domain/aggregate] — file: `modules/<module>/internal/domain/…`
- [ ] T0xx [US1] [Command/query handler] — file: `…/internal/application/…`
- [ ] T0xx [US1] [Repository/handler/consumer; đăng ký điểm vào trong `modules/<module>/module.go`] — file: `…`

**Checkpoint**: User Story 1 chạy và kiểm độc lập được.

## Phase 4: User Story 2 — [Tiêu đề] (Priority: P2)

…

## Phase N: Polish & Cross-Cutting

- [ ] T0xx [P] Cập nhật tài liệu sở hữu/CONFIG_DEFAULTS/CHANGELOG nếu thay đổi — file: `docs/…`
- [ ] T0xx Chạy [quickstart.md](quickstart.md) và ghi bằng chứng

## Dependencies & Execution Order

- Setup → Foundational → các user story (theo ưu tiên, story độc lập có thể song song) → Polish.
- [Phụ thuộc riêng giữa task/story]

## Definition of Done

- [ ] Mọi tiêu chí F[NNN]-AC-* có test pass (ghi mã test/TC)
- [ ] `make gen-check`, `make lint`, `make test`, `make test-integration` (và `make web-test`/`make godot-test` nếu có client) xanh
- [ ] Constitution Check trong plan.md vẫn đạt; không mock trong đường chạy thật (R17)
- [ ] Tài liệu sở hữu, CONFIG_DEFAULTS, CHANGELOG cập nhật nếu có thay đổi
- [ ] Trạng thái feature trong [CODE_PLAN](../../docs/12-project/CODE_PLAN_CMS_LMS_FIRST.md) cập nhật
