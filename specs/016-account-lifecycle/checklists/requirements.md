# Specification Quality Checklist: F016 — Vòng đời tài khoản: khóa, thu hồi truy cập, dọn học sinh không đăng nhập, ẩn danh hóa

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
- [ ] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành

## Notes

- Đã chốt (Technical Architect 2026-11-16): khi dọn giữ user Keycloak `enabled`, gỡ membership tenant/Organization, thu hồi phiên; đăng nhập lại → khôi phục (FR-AUTH-024, AC-AUTH-018). ACCOUNT_PROVISIONING §6b, TC-AUTH-020 sửa theo. Status Ready.
- Thứ tự saga dọn [Proposed] (module tham gia trước, identity sau) khác ACCOUNT_PROVISIONING §6b — cần cập nhật tài liệu cùng PR.
- Permission `tenant.user.lock` [Proposed] chưa có trong catalog ADMIN_PORTAL §2.2.
