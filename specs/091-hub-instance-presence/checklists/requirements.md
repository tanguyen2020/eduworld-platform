# Specification Quality Checklist: F091 — Hub instance, presence, đồng bộ vị trí

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

- Module sở hữu hub khác CODE_PLAN (module `game`) và mô tả định tuyến AsyncAPI; spec theo tài liệu sở hữu (research R1) — cần Architect xác nhận khi review và sửa CODE_PLAN dòng F091.
- Số "1,2" (hệ số dung sai tốc độ) và "20 vi phạm/phút", "90%" là quy tắc thuật toán ở REALTIME_ARCHITECTURE §6 / MULTIPLAYER_GAMEPLAY §6, chưa có tên tham số; giữ link nguồn.
- `audit_hub_membership` chưa có trong CONFIG_DEFAULTS — đăng ký ở Polish.
- F091 là feature đầu tiên của module competition: task Setup gồm skeleton + projection identity.
