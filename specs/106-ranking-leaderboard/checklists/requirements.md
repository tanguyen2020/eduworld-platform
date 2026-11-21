# Specification Quality Checklist: F106 — Ranking Glicko-2, tier, season và 12 bảng xếp hạng

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

- FR-RNK-003 (PRD) chỉ yêu cầu ẩn số rating với lớp 1–5; COMPETITION_SYSTEM §8.2 và `tiers` ẩn với mọi khối — spec theo tài liệu sở hữu (chặt hơn).
- Tên đã chốt (RB-412): `rating_changes`, `leaderboard_id`, key Valkey `ew:lb:<tenant>:<leaderboard_id>:…` (prefix thật `ew:competition:`) theo VALKEY_DESIGN; vị trí quanh mình ±3 (RB-415).
- Spec nhắc Valkey/PostgreSQL vì FR-LDB-002 là yêu cầu nghiệp vụ về nơi lưu.
- ADR-0095 (2026-11-21, WP-S): bỏ scope `class`, metric `class_goal`, bảng "Mục tiêu lớp", "Lớp em tuần này", "Nhiệm vụ của lớp"; thay bằng `grade_goal` (mục tiêu tập thể của khối trong trường, không tên, ẩn ô < 5 học sinh); FR-LDB-001 còn 12 loại bảng; quyền nhân sự `lms.student.read`. "Lớp 1–3/4–5/6–9" trong spec là khối.
