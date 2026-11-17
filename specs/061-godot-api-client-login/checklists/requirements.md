# Specification Quality Checklist: F061 — `ApiClient`, đăng nhập game trong form, chọn tenant, takeover, buộc cập nhật, đăng xuất

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-16 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md — các chi tiết đó ở plan.md
- [x] Tập trung vào giá trị cho người dùng và nhà trường
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng (FR-AUTH-002, 003, 008, 009, 017, 023, 027, FR-GAME-008, FR-GAME-014)
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (AC-07, AC-11), tenant isolation (AC-15, AC-16, AC-19), idempotency (AC-16, AC-17), lỗi chính (AC-05…09, AC-14, AC-27, AC-28), giới hạn độ dài (AC-03)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (theo ADR-0083 cho D15; D16 theo ADR-0068; kiểm tài liệu ở task T004)

## Notes

- Takeover không giữ mật khẩu (ADR-0083, research R1); idempotency/audit thao tác ghi do backend (F013/F014).
- 2026-11-16: thêm User Story 6 (tự vào lại khi phiên còn hạn), viết lại F061-AC-04, 12, 15, 18, 22, 23, 25, 26 theo ADR-0086 (thay ADR-0064 mục 1, 2, 4, 5); `game_mobile_background_relogin` bỏ.
