# Specification Quality Checklist: F028 — Import Word/MathType (`lessons_docx`) qua Pandoc

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-27 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực trong spec.md ngoài ràng buộc owner đã chốt (Pandoc, image `eduworld/media-tools`)
- [x] Tập trung vào giá trị cho người soạn nội dung
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] FR chính (FR-QB-011 phần Word/MathType) có mặt và có mã PRD đúng
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được
- [x] Có tiêu chí cho authorization, tenant isolation, idempotency/audit, lỗi chính
- [x] Success criteria đo được
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung deferred
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (ADR-0055 §4: SVG render trên portal)

## Notes

- F028-AC-16 cần bộ file Word thật (bản quyền, ngoài repo) — đo ở quickstart §2.6 trước khi mở cho người soạn.
