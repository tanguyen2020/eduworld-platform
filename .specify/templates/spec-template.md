# Feature Specification: F[NNN] — [Tên tính năng]

**Feature Branch**: `[NNN-slug]` · **Created**: [YYYY-MM-DD] · **Status**: Draft · **Module**: `[module (identity/learning/game/competition/chat/realtime) hoặc web/godot/pkg]` · **Mốc**: [M…] · **Phụ thuộc**: [F…, hoặc —]
**Input**: [Mô tả ngắn và FR nguồn trong PRD]

<!--
Chuẩn Spec Kit + EARS (ADR-0082). spec.md trả lời CÁI GÌ và VÌ SAO, không mô tả cách code.
- Mã FR/US chỉ từ docs/00-overview/PRODUCT_REQUIREMENTS.md. Không tự tạo mã FR.
- Mỗi user story độc lập, có ưu tiên P1/P2/P3, có Independent Test.
- Tiêu chí chấp nhận viết EARS, mã F[NNN]-AC-[NN], có Nguồn và TC (mã TC có thật trong TEST_STRATEGY hoặc "—").
- Tham số theo TÊN trong CONFIG_DEFAULTS; contract theo operationId/schema trong specs/.
- Điểm chưa rõ: đánh dấu [NEEDS CLARIFICATION: câu hỏi] và liệt kê ở mục Clarifications. Spec chỉ Ready khi không còn marker.
Mẫu EARS:
  HỆ THỐNG PHẢI <hành vi>.                                  (luôn đúng)
  KHI <sự kiện>, HỆ THỐNG PHẢI <hành vi>.                    (theo sự kiện)
  TRONG KHI <trạng thái>, HỆ THỐNG PHẢI <hành vi>.           (theo trạng thái)
  NẾU <điều kiện bất thường>, THÌ HỆ THỐNG PHẢI <hành vi>.   (ngoại lệ)
  Ở NƠI <tùy chọn được bật>, HỆ THỐNG PHẢI <hành vi>.        (tùy chọn)
-->

## Clarifications

### Session [YYYY-MM-DD]

- Q: [câu hỏi] → A: [trả lời, nguồn]

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — [Tiêu đề] (Priority: P1)

[Hành trình của người dùng bằng lời thường; mã US trong PRD nếu có.]

**Why this priority**: [Giá trị, vì sao trước]

**Independent Test**: [Cách kiểm riêng story này và giá trị nó mang lại]

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F[NNN]-AC-01 | KHI …, HỆ THỐNG PHẢI … | FR-… · `docs/…` §… | TC-… |

---

### User Story 2 — [Tiêu đề] (Priority: P2)

…

### Edge Cases

- NẾU …, THÌ HỆ THỐNG PHẢI … (ghi mã AC nếu đã có ở bảng trên)

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-… | … | `docs/…` §… |

### Key Entities

- **[Entity]**: [ý nghĩa, thuộc tính chính, quan hệ — không mô tả cách lưu]

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `…` | … |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| … | NFR-… · AGENTS R… · ADR-… |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: [Kết quả đo được, không phụ thuộc công nghệ]

## Ngoài phạm vi

- [Việc không làm ở feature này, feature nào làm, hoặc "deferred — ADR-0082"]

## Assumptions

- [Giả định; nhãn [Proposed]/[TBD] kèm OQ nếu có]
