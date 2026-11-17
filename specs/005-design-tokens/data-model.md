# Data Model: F005 — Design tokens game/admin/shared và pipeline sinh theme

**Spec**: [spec.md](spec.md) · **Plan**: [plan.md](plan.md) · Nguồn sự thật bảng: [DATABASE_DESIGN](../../docs/07-backend/DATABASE_DESIGN.md)

## 1. Entity

Không áp dụng — F005 không có dữ liệu bền vững trong database; token là file JSON trong Git.

Cấu trúc token để tham chiếu khi code (W3C Design Tokens):

| Thuộc tính | Bắt buộc | Ghi chú |
|---|---|---|
| `$value` | có | literal chỉ ở tầng primitive; tầng semantic/component dùng alias `{path.to.token}` |
| `$type` | có | `color`, `dimension`, `fontFamily`, `fontWeight`, `duration`, `number`, `shadow`, `gradient` |
| `$description` | có | mô tả mục đích |
| `$deprecated` | khi bỏ | chuỗi lý do + token thay thế; giữ ≥ 1 phiên bản |
| `$extensions.eduworld.contrastWith` | token chữ | danh sách token nền cần kiểm, mức `normal`/`large` |
| `$extensions.eduworld.variants` | màu nghiệp vụ | `light`, `dark`, `game` |

Nhóm token tối thiểu:

| Bộ | Nhóm | Ví dụ tên token |
|---|---|---|
| shared | brand | `color.brand.primary`, `color.brand.secondary` |
| shared | domain | `color.subject.math`, `color.subject.vietnamese`, `color.subject.english`, `color.subject.science`; `color.rarity.common…legendary`; `color.content_status.draft`, `in_review`, `approved`, `published`, `archived`, `rejected` |
| game | semantic/component | `color.surface.panel`, `color.text.primary`, `color.feedback.correct`, `color.feedback.incorrect`, `component.button.primary.*`, `component.hint_button.*`, `component.voice_caption.*`, `text.expansion_allowance`, `text.secondary_language.*` |
| game | density | `density.<band>.font.base`, `density.<band>.button.min_height` |
| game | art3d | `zone.<subject>.primary`, `toon.steps`, `outline.width`, `env.<zone>.*`, `vfx.<kind>.gradient` |
| admin | seed/semantic/mode | `antd.colorPrimary`, `antd.fontFamily`, `antd.fontSize`, `antd.borderRadius`, `antd.controlHeight`, `status.*`, `chart.categorical.1…8`, `math.font-scale`, `banner.*` |
