# Specification Quality Checklist: F041 — Giao bài của nhà trường: assignment, `ResultRelease`, gia hạn/miễn, `my-assignments`

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

- Mâu thuẫn tài liệu đã ghi (không chặn spec): mã lỗi Academic Manager ngoài môn (đã chốt `PERMISSION_DENIED`, RB-113); `assignments.result_visibility` (DATABASE_DESIGN §2.14) vs `feedback_timing`/`result_release` (spec); event `assignment.closed`/`submission_updated` chỉ có ở SCHOOL_LMS §12.
- Đối tượng `study_group` phụ thuộc quyết định ở F040 US4; F041 dùng port nên spec vẫn Ready.
