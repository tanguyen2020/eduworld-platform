# ADR-0105 — Nickname tự gõ qua kiểm duyệt server (giữ gợi ý sinh tự động)

> **Trạng thái:** Accepted · thay quy tắc "không nhập tự do" của [CHARACTER_SYSTEM §5.3](../01-game-design/CHARACTER_SYSTEM.md#53-nickname) và phần nickname của F070-AC-10 · **Ngày:** 2026-11-28 · **OQ:** — · **Tài liệu:** [CHARACTER_SYSTEM](../01-game-design/CHARACTER_SYSTEM.md) · [MODERATION](../03-social-multiplayer/MODERATION.md) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

CHARACTER_SYSTEM §5.3 (Confirmed, 2026-11-16) chỉ cho học sinh chọn nickname từ danh sách sinh tự động (tính từ + con vật + số, ví dụ "Cú Mèo Chăm Chỉ 27") để tránh lộ tên thật và ngôn từ xấu; F070 hiện thực bằng `suggestNicknames` + `nickname_option_id` và từ chối mọi chuỗi tự do (F070-AC-10). MODERATION §2 đã dự phòng "nickname tự đặt (nếu cho phép)".

Ngày 2026-11-28 owner quyết định: học sinh 6–15 tuổi không thích tên sinh sẵn — cho **tự gõ** nickname, giữ nút "Gợi ý" làm trợ giúp tùy chọn (không bỏ tính năng nào); 3–32 ký tự, chữ tiếng Việt có dấu, chữ số, khoảng trắng; duy nhất trong tenant; giữ giới hạn đổi 1 lần/tháng và quyền đặt lại của `school_admin`/`safety_moderator`; người chơi báo cáo được nickname; kiểm duyệt ở server bằng cùng chuẩn hóa + danh mục từ nhạy cảm của chat, chặn số điện thoại/email/URL/tài khoản mạng xã hội, từ chối bằng thông điệp chung không nêu từ khớp; kết quả client không được tin; nhắc "Đừng dùng tên thật của em"; áp cho màn tạo nhân vật và màn đổi tên. Triển khai: [F136](../../specs/136-free-text-nickname/spec.md).

## Quyết định

1. **Hai cách đặt nickname, cùng một bộ quy tắc cho mọi khối (R15):** gõ tự do hoặc chọn gợi ý sinh từ bộ từ đã duyệt (`suggestNicknames`, `nickname_suggestion_ttl` giữ nguyên). `updateGameProfile` nhận đúng một trong `nickname` hoặc `nickname_option_id`. Học sinh lớp 1–3 vẫn hoàn tất bằng chạm gợi ý, không bắt buộc gõ. [Confirmed — owner 2026-11-28]
2. **Định dạng (module game, thuần domain):** chuẩn hóa NFC, bỏ khoảng trắng đầu/cuối, gộp khoảng trắng; chỉ chữ Latin kể cả chữ tiếng Việt có dấu dựng sẵn, chữ số, khoảng trắng đơn; ≥ 1 chữ cái; tối đa `nickname_max_digits` chữ số; độ dài `nickname_min_length`–32 điểm mã. [Confirmed — owner 2026-11-28 (3–32, bộ ký tự) · giới hạn chữ số và chặn chữ hệ khác: Confirmed — owner "chốt hết"]
3. **Duy nhất trong tenant, không phân biệt hoa thường:** khóa so trùng `nickname_key` (chữ thường + NFC + khoảng trắng đã gộp), unique partial `(tenant_id, nickname_key) WHERE deleted_at IS NULL`; kiểm trùng **sau** kiểm duyệt để không dò được từ bị chặn. [Confirmed — owner "chốt hết"]
4. **Kiểm duyệt dùng chung pipeline chat:** module game gọi facade `moderateDisplayText` của module chat (`context = nickname`) — cùng chuẩn hóa tiếng Việt, danh mục CMS platform + tenant, công tắc hiệu lực (ADR-0030) và bước PII/grooming luôn chạy (ADR-0078). Không hardcode từ ở bất kỳ đâu; không trả từ khớp, vị trí hay danh mục. [Confirmed — owner 2026-11-28]
5. **Chặn tên thật và tên trường của chính học sinh:** facade mới `screenNicknamePersonalData` của module identity chỉ trả `allowed`/`blocked` (bằng họ tên, chứa ≥ 2 từ của họ tên, hoặc chứa tên trường — so không dấu); module game không bao giờ nhận họ tên (R13, R16). [Confirmed — owner "chốt hết"]
6. **Từ chối an toàn:** mọi lý do nội dung → một mã `422 NICKNAME_NOT_ALLOWED` với một thông điệp chung thân thiện; kiểm duyệt hoặc identity lỗi/quá `nickname_moderation_deadline` → `503`, không lưu (fail-closed); giới hạn `nickname_attempt_rate` lần gửi/học sinh → `429`; chỉ đo metric, không tự phạt trẻ; không ghi chuỗi bị từ chối vào log/audit/telemetry (R14). [Confirmed — owner "chốt hết"]
7. **Báo cáo và đặt lại:** báo cáo nickname qua `createReport` (`context.reported_field = nickname`), module chat chụp nickname từ projection phía server. `safety_moderator` đặt lại bằng sanction tức thời `nickname_reset` (event `sanction.applied`, module game tiêu thụ); `school_admin`/`tenant_admin`/System Admin đặt lại bằng `resetStudentNickname` (permission tenant `lms.student.nickname_reset`, `reason` + `Idempotency-Key`, audit). Sau đặt lại nickname = `NULL`, người khác thấy chuỗi trung tính, học sinh chỉ chọn lại ở bước Tên, lần chọn đó không tính giới hạn tháng. Danh mục đổi về sau **không** tự đặt lại nickname đã được chấp nhận. [Confirmed — owner 2026-11-28 (báo cáo, quyền đặt lại) · cơ chế: Confirmed — owner "chốt hết"]
8. **Ẩn danh hóa:** nickname tự gõ có thể chứa dữ liệu cá nhân nên ẩn danh hóa thay cả nickname trong lịch sử `game.nickname_changes` (append-only) qua hàm `SECURITY DEFINER` của migration; không xóa vật lý (ADR-0070). [Confirmed — owner "chốt hết"]
9. **Hiển thị:** mọi nơi hiển thị nickname là văn bản thuần (không BBCode/markup); tên thật, trường, khối không hiển thị cho người chơi khác (R16, không đổi). [Confirmed]

## Hệ quả

- (+) Học sinh tự chọn tên mình thích; gợi ý vẫn có cho em không muốn gõ, nên không mất tính năng nào.
- (+) Một pipeline, một danh mục cho mọi văn bản học sinh nhập (chat, tên party/đội, nickname) — R15, R33.
- (−) Rủi ro tên không phù hợp lọt danh mục tăng so với danh sách sinh sẵn: bù bằng PII luôn chạy, chặn họ tên/tên trường, báo cáo + đặt lại, chuỗi trung tính khi đặt lại.
- (−) `updateGameProfile` tự gõ thêm hai lời gọi facade trong tiến trình (p95 mục tiêu ≤ 300 ms) và phụ thuộc module chat/identity sẵn sàng (fail-closed → học sinh dùng gợi ý khi lỗi).
- (−) Việc phải làm theo (cùng MR với ADR này): CHARACTER_SYSTEM §5.3, MODERATION §2/§5/§7, STUDENT_GAME_UX_CORE S02, CONFIG_DEFAULTS (tham số mới), PRD FR-GAME-005/FR-GAME-019; khi triển khai: contract game/chat/identity, DATABASE_DESIGN, EVENT_DESIGN (`sanction.applied`), DATA_PRIVACY (ẩn danh lịch sử nickname).

## Phương án đã loại

1. **Giữ chỉ danh sách sinh sẵn:** owner không chọn — học sinh không thích tên sinh sẵn.
2. **Bỏ gợi ý, chỉ gõ tự do:** trái yêu cầu "không bỏ tính năng nào"; lớp 1–3 cần cách không gõ phím.
3. **Kiểm duyệt nickname bằng bộ lọc riêng trong module game:** tạo danh mục thứ hai, lệch chuẩn hóa với chat, vi phạm một nguồn sở hữu (R33) và dễ hardcode từ (R15).
4. **Duyệt tay mọi nickname trước khi hiển thị (hàng đợi):** chậm, tăng tải Safety Team; báo cáo + đặt lại đủ cho phần lọt.
5. **Fail-open khi kiểm duyệt lỗi:** không chấp nhận được với dữ liệu trẻ em hiển thị công khai.
6. **Trả mã lỗi chi tiết (độ dài, ký tự, từ cấm):** giúp dò danh mục; chỉ kiểm sơ bộ định dạng ở client để thân thiện.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-28 | Tạo ADR (Accepted theo quyết định owner) | Owner 2026-11-28; F136 |
