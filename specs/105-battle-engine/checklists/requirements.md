# Specification Quality Checklist: F105 — Battle engine server-authoritative và 8 chế độ thi đấu

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

- Feature chạm hai module: phần module learning (`createQuestionPlan`, `scorePlanAnswer`, `revealPlanItem`) làm trước (F022 đã ghi "endpoint làm ở F105").
- Lệch tài liệu: COMPETITION_SYSTEM §3.1 "quá hạn → câu còn lại tính 0" vs MULTIPLAYER_GAMEPLAY §9.2 "Thua do abandon, đối thủ thắng" (tài liệu sở hữu grace) — spec theo MULTIPLAYER_GAMEPLAY, kết thúc trận ngay.
- REALTIME_ARCHITECTURE §7.1 tạo plan "khi tạo match", `learning.internal.v1.yaml` ghi "khi trận chuyển sang countdown" — spec theo contract (countdown).
- ADR-0095 (2026-11-21, WP-S): thi đấu/thử thách lớp → thi đấu khối/thử thách khối (`grade_challenge`, FR-CMP-001); Timed Quiz lớp 1–3 trả `grade_goal` thay `class_goal`; quyền nhân sự `lms.student.read`.
