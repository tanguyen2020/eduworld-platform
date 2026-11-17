# Specification Quality Checklist: F100 — Party: tạo/tham gia/mời/kick/chuyển leader/giải tán, một party active, giới hạn

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization, tenant isolation, idempotency/audit (nếu có thao tác ghi), lỗi chính
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Mâu thuẫn tài liệu đã chọn hướng và cần sửa ở task Polish: mã lỗi block (PARTY_SYSTEM §14 vs API_DESIGN §5); rate lời mời đã chốt 50/ngày (RB-405, CONFIG_DEFAULTS C-3); channel `party` "cùng transaction" (PARTY_SYSTEM §2) không khả thi giữa hai DB (ADR-0037); `club_id` study group của PARTY_SYSTEM §4 lỗi thời theo ADR-0084.
- FR-PTY-003 ghi "Class Group" nhưng enum `party_type` không có; feature dùng `visibility = class`.
- Giới hạn phụ của PARTY_SYSTEM §10 chưa có tên CONFIG_DEFAULTS — task Polish đăng ký.
