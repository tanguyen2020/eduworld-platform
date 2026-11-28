# Specification Quality Checklist: F136 — Nickname tự gõ qua kiểm duyệt (giữ "Gợi ý"), báo cáo và đặt lại nickname

**Purpose**: Kiểm chất lượng spec trước khi lập kế hoạch (`/speckit.plan`) · **Created**: 2026-11-28 · **Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Không có chi tiết hiện thực (ngôn ngữ, framework, tên bảng) trong spec.md ngoài tên contract/khóa dịch mà client và module khác tiêu thụ — chi tiết ở plan.md
- [x] Tập trung vào giá trị cho học sinh, nhà trường và Safety Team
- [x] Mọi mục bắt buộc đã điền

## Requirement Completeness

- [x] Không còn marker [NEEDS CLARIFICATION]
- [x] Mọi FR chính của feature (theo CODE_PLAN) có mặt và có mã PRD đúng: FR-GAME-005, FR-GAME-019, FR-MOD-002
- [x] Mọi tiêu chí viết theo EARS, kiểm thử được, không mơ hồ
- [x] Có tiêu chí cho authorization (F136-AC-21, AC-29), tenant isolation (AC-28), idempotency/audit (AC-20, AC-21), lỗi chính (AC-05, AC-14, AC-15)
- [x] Success criteria đo được và không phụ thuộc công nghệ
- [x] Edge case đã liệt kê
- [x] Phạm vi và "Ngoài phạm vi" rõ ràng; không có nội dung hoãn
- [x] Phụ thuộc và giả định đã ghi

## Feature Readiness

- [x] Mỗi user story có Independent Test
- [x] Tham số dùng tên CONFIG_DEFAULTS, không chép giá trị (tham số mới đăng ký cùng MR)
- [x] Không mâu thuẫn tài liệu sở hữu và ADR hiện hành (ADR-0105 thay quy tắc "không nhập tự do" của CHARACTER_SYSTEM §5.3 và F070-AC-10)

## Notes

- Kiểm duyệt nickname dùng lại facade `moderateDisplayText` của F116 (thêm `context = nickname`, caller `game`) thay vì tạo pipeline riêng, để một danh mục và một chuẩn hóa áp cho mọi văn bản học sinh nhập (R15, R33).
- Chặn họ tên/tên trường cần dữ liệu của module identity → facade mới `screenNicknamePersonalData` chỉ trả quyết định (R13, R16).
- Đặt lại từ Safety Team đi qua sanction + event (`sanction.applied`, owner F116) để quyết định moderation có một nguồn; đặt lại của nhà trường là API module game.
- F070 Clarifications "`game.nickname_changes` giữ nguyên khi ẩn danh (nickname sinh từ bộ từ đã duyệt)" không còn đúng khi có nhập tự do → F136-AC-31.
