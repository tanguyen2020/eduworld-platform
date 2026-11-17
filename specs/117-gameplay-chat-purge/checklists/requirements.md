# Specification Quality Checklist: F117 — Chat gameplay luôn có (`match`/`team`/`party`) và purge chat gameplay

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

- Thứ tự mốc: purge cần report/case (F121, M5) nhưng F117 ở M4b → job fail-closed (F117-AC-19) — đã chốt RB-001.
- Event kết thúc trận là `match.completed` (RB-507, CHAT_SYSTEM §4 đã sửa). Payload `match.started` hiện chỉ có `match_id` → cần delta ở F105.
- PRD FR-PTY-001 ghi "Party Chat văn bản tự do: P5" mâu thuẫn FR-CHAT-009 (P4, Confirmed) → theo FR-CHAT-009.
- TC-CHAT-018 ghi "schema chat-policy v4", schema hiện hành là v5.
