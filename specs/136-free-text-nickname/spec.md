# Feature Specification: F136 — Nickname tự gõ qua kiểm duyệt (giữ "Gợi ý"), báo cáo và đặt lại nickname

**Feature Branch**: `136-free-text-nickname` · **Created**: 2026-11-28 · **Status**: Ready · **Module**: module `game` (+ module `chat`, module `identity`, `src/game/`, `src/cms/frontend/apps/portal`) · **Mốc**: M5 · **Phụ thuộc**: F070, F116, F121 (và F052 màn học sinh của portal, F122 portal kiểm duyệt, F125 chế độ `compact`, F114 phong cách tầng)
**Input**: Học sinh lớp 1–9 không thích nickname sinh tự động; owner (2026-11-28) cho học sinh **tự gõ** nickname ở màn tạo nhân vật (S02) và màn đổi tên, **giữ** nút "Gợi ý" (gợi ý sinh từ bộ từ đã duyệt) làm trợ giúp tùy chọn. Nickname tự gõ phải qua kiểm duyệt phía server cùng chuẩn hóa và danh mục từ nhạy cảm của chat (F116), bước PII luôn chạy, chặn tên thật/tên trường của chính học sinh; người chơi báo cáo được nickname; `school_admin` và `safety_moderator` đặt lại được. Quyết định: [ADR-0105](../../docs/adr/ADR-0105-nickname-tu-go-qua-kiem-duyet.md). Quy tắc sở hữu: [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname). FR nguồn: FR-GAME-005, FR-GAME-019, FR-MOD-002.

## Clarifications

### Session 2026-11-28 (quyết định owner — [Confirmed — owner 2026-11-28])

- Q: Học sinh có được tự gõ nickname? → A: Có, ở màn tạo nhân vật (S02) và màn đổi tên; thay quy tắc "không nhập tự do" của CHARACTER_SYSTEM §5.3 và F070-AC-10 (ADR-0105). [Confirmed — owner 2026-11-28]
- Q: Bỏ gợi ý sinh tự động? → A: Không. Giữ nút "Gợi ý"/"Gợi ý khác" (`suggestNicknames`, `nickname_option_id`) làm trợ giúp tùy chọn — không bỏ tính năng nào. [Confirmed — owner 2026-11-28]
- Q: Độ dài và ký tự cho phép? → A: 3–32 ký tự; chữ cái tiếng Việt có dấu, chữ số, khoảng trắng. Duy nhất trong tenant; giới hạn đổi 1 lần/tháng giữ nguyên (`nickname_change_monthly_limit`); `school_admin`/`safety_moderator` đặt lại giữ nguyên; người chơi báo cáo được nickname. [Confirmed — owner 2026-11-28]
- Q: Kiểm duyệt nickname thế nào? → A: Ở server, cùng bước chuẩn hóa và danh mục từ nhạy cảm của chat (F116; không hardcode từ — R15), cộng chặn số điện thoại, email, URL, tài khoản mạng xã hội (bước PII luôn chạy, MODERATION §3.3); từ chối bằng một thông điệp chung thân thiện, không bao giờ nêu từ đã khớp (R15). Kết quả kiểm tra phía client không được tin (R5). [Confirmed — owner 2026-11-28]
- Q: Nhắc nhở trên màn? → A: Dòng gợi ý "Đừng dùng tên thật của em"; tên thật/trường không bao giờ hiển thị cho người khác (R16). [Confirmed — owner 2026-11-28]
- Q: Học sinh tự đăng ký (F135)? → A: Sau đăng ký và đăng nhập, `setup_required = true` đưa vào S02 như mọi học sinh, chọn nickname ở đây (không có ô nickname trên màn đăng ký). [Confirmed — owner 2026-11-28]

### Session 2026-11-28 (speckit.clarify — [Confirmed — owner "chốt hết"]: phương án an toàn nhất)

- Q: Body của `updateGameProfile`? → A: `GameProfileUpdate` nhận **hoặc** `nickname` (chuỗi tự gõ, `maxLength: 32`) **hoặc** `nickname_option_id` (gợi ý) — `oneOf`, gửi cả hai → `422 VALIDATION_FAILED`. Mọi lý do từ chối nội dung (sai ký tự, quá ngắn/dài, quá nhiều chữ số, từ nhạy cảm, PII, tên thật/tên trường) trả **một** mã `422 NICKNAME_NOT_ALLOWED` với thông điệp chung — không có mã con, không có vị trí, không echo chuỗi (tránh dò danh mục). [Confirmed — owner "chốt hết"]
- Q: Chuẩn hóa và tập ký tự cụ thể? → A: Unicode NFC, bỏ khoảng trắng đầu/cuối, gộp nhiều khoảng trắng thành một; chỉ chữ Latin kể cả chữ tiếng Việt dựng sẵn (`đ`/`Đ`, nguyên âm có dấu), chữ số 0–9 và khoảng trắng đơn; ≥ 1 chữ cái; tối đa `nickname_max_digits` (4) chữ số; dài `nickname_min_length` (3)…32 điểm mã sau chuẩn hóa. Không emoji, không ký tự đặc biệt, không chữ hệ khác (tránh giả dạng ký tự giống nhau). [Confirmed — owner "chốt hết"]
- Q: "Duy nhất trong tenant" có phân biệt hoa thường? → A: Không phân biệt: thêm cột `nickname_key` (chữ thường + NFC + khoảng trắng đã gộp) với unique partial `(tenant_id, nickname_key) WHERE deleted_at IS NULL`; giữ index cũ `(tenant_id, nickname)` (expand, backfill cho nickname hiện có). Lý do: "An An" và "an an" dễ bị dùng để giả danh. [Confirmed — owner "chốt hết"]
- Q: Thứ tự kiểm tra? → A: định dạng (trong module game) → kiểm duyệt (facade chat `moderateDisplayText`, `context = nickname`) → dữ liệu cá nhân (facade identity `screenNicknamePersonalData`) → trùng (`409 NICKNAME_UNAVAILABLE`). Trùng chỉ được báo sau khi qua kiểm duyệt, để không dùng thông báo "đã có người dùng" dò được từ bị chặn. [Confirmed — owner "chốt hết"]
- Q: Chặn tên thật/tên trường của chính học sinh bằng cách nào khi module game không có họ tên (R13, R16)? → A: Operation facade mới của identity `screenNicknamePersonalData` (`x-allowed-callers: [game]`): nhận `student_id` + nickname, trả **chỉ** `allowed`/`blocked`; `blocked` khi nickname (không dấu, chữ thường) bằng họ tên của chính học sinh, chứa ≥ 2 từ của họ tên đó, hoặc chứa tên trường của học sinh. Không trả họ tên hay tên trường. Học sinh tự đăng ký không có họ tên → chỉ kiểm tên trường. [Confirmed — owner "chốt hết"]
- Q: Kiểm duyệt hoặc identity lỗi/hết hạn? → A: **Fail-closed**: `503 UNAVAILABLE`, không lưu, không coi là `allowed`; client mời thử lại hoặc chọn tên gợi ý (đường gợi ý không gọi kiểm duyệt nên vẫn dùng được). Deadline `nickname_moderation_deadline` (1 s) cho mỗi lần gọi facade. [Confirmed — owner "chốt hết"]
- Q: Chống dò danh mục bằng thử nhiều lần? → A: `nickname_attempt_rate` = 10 lần/10 phút/học sinh cho mọi lần gửi nickname tự gõ (kể cả bị từ chối) → `429 RATE_LIMITED` + `Retry-After`. Chỉ đếm và đo metric, **không** tự phạt trẻ (MODERATION §5 tinh thần giáo dục). [Confirmed — owner "chốt hết"]
- Q: Có màn đổi tên trong client chưa? → A: Chưa có (F070 chỉ có API). F136 thêm mục "Nickname" trong Cài đặt → "Hồ sơ của em" (S15): hiện nickname hiện tại, ô gõ + chip gợi ý, số lần đổi còn lại trong tháng, ngày đổi lại được (từ `retry_after`). [Confirmed — owner "chốt hết"]
- Q: Báo cáo nickname dùng operation nào? → A: `createReport` sẵn có, `target_type = user`, `reason = inappropriate_content`, thêm `context.reported_field = nickname` (enum một giá trị). Module chat chụp nickname **hiện tại** từ projection `ref.player_profiles` vào bằng chứng của case, không nhận nickname từ client. Client có lựa chọn "Tên không phù hợp" (icon + chữ) trên thẻ người chơi. [Confirmed — owner "chốt hết"]
- Q: `safety_moderator` đặt lại nickname bằng gì? → A: Loại sanction mới `nickname_reset` (tức thời, không thời hạn) trong module chat, phát `sanction.applied` (schema owner F116, thêm giá trị enum); module game tiêu thụ → nickname = `NULL`, `setup_required = true`, lần vào sau học sinh chọn lại (chỉ bước Tên của S02), lần chọn đó không tính vào giới hạn tháng. [Confirmed — owner "chốt hết"]
- Q: `school_admin` đặt lại nickname bằng gì? → A: Operation public mới của module game `resetStudentNickname` (`POST /api/v1/tenant/students/{student_id}/game-profile/nickname:reset`), permission tenant mới `lms.student.nickname_reset` (School Admin trong trường, Tenant Admin; System Admin qua tenant đích kể cả `common`), `reason` + `Idempotency-Key` bắt buộc, audit; nút "Đặt lại nickname" trên trang học sinh của portal (F052). [Confirmed — owner "chốt hết"]
- Q: Trong lúc nickname = `NULL` sau đặt lại, người khác thấy gì? → A: Chuỗi trung tính `ui.player.unnamed` ("Bạn mới"/"New friend") — không hiện nickname cũ, không username. [Confirmed — owner "chốt hết"]
- Q: Ẩn danh hóa? → A: Nickname tự gõ có thể chứa dữ liệu cá nhân, nên `anonymizeUserData` thay nickname ở `game.game_profiles` **và** `old_nickname`/`new_nickname` ở `game.nickname_changes` (append-only) qua hàm `SECURITY DEFINER` viết tay trong migration, có audit (sửa quyết định "giữ nguyên lịch sử" của F070). Không xóa vật lý (ADR-0070). [Confirmed — owner "chốt hết"]
- Q: Ghi log/audit chuỗi bị từ chối? → A: Không ghi chuỗi bị từ chối ở bất kỳ mức log nào, không gửi telemetry; audit chỉ ghi lần đổi thành công (giá trị cũ/mới) và mã kết quả của lần bị từ chối (không có chuỗi) — R14. [Confirmed — owner "chốt hết"]
- Q: Lớp 1–3 có phải gõ phím? → A: Không bắt buộc: chạm một chip gợi ý vẫn hoàn tất S02 (giữ AC-UX-0201); ô gõ là tùy chọn ở mọi khối, cùng một bộ quy tắc kiểm duyệt (R15). [Confirmed — owner "chốt hết"]

### Session 2026-11-28 (speckit.implement backend + portal — [Confirmed — owner "chốt hết"]: chi tiết triển khai, phương án an toàn nhất)

- Q: `oneOf` của `GameProfileUpdate` viết thế nào để không breaking? → A: Thêm thuộc tính `nickname` (`minLength: 1`, `maxLength: 32`) + `not: {required: [nickname, nickname_option_id]}`; server trả `422 VALIDATION_FAILED` khi có cả hai hoặc `nickname: null`. `oasdiff` 0 breaking. [Confirmed — owner "chốt hết"]
- Q: Bộ đếm `nickname_attempt_rate` đếm từ đâu? → A: Mọi lần gửi `nickname` tự gõ được đếm **trước** mọi kiểm tra (kể cả lần sai định dạng), rồi định dạng → giới hạn tháng → kiểm duyệt → dữ liệu cá nhân → trùng. Valkey lỗi → `503` (fail-closed, đường gợi ý vẫn dùng được). Gõ lại đúng nickname hiện tại → `200` không đổi gì, không gọi facade. [Confirmed — owner "chốt hết"]
- Q: "Chữ Latin" gồm chữ nào? → A: Chỉ `A–Z`, `a–z` và chữ tiếng Việt dựng sẵn (gồm `đ`/`Đ`); chữ Latin khác (`ñ`, `ü`…), chữ hệ khác, emoji, ký tự vô hình, dấu kết hợp không dựng được đều bị chặn (chống giả dạng). [Confirmed — owner "chốt hết"]
- Q: Backfill `nickname_key` cho dữ liệu cũ khi có khóa trùng? → A: Migration expand: thêm cột + unique partial, backfill `lower(normalize(gộp khoảng trắng))`; nếu hai dòng sống cùng tenant ra cùng khóa (chỉ có thể với dòng cũ khác dạng Unicode), dòng cũ nhất giữ khóa, dòng còn lại giữ nguyên nickname với `nickname_key = NULL` tới lần đổi sau — không xóa, không đổi tên ai; index cũ `lower(nickname)` giữ nguyên. [Confirmed — owner "chốt hết"]
- Q: Lịch sử đặt lại ghi gì khi cột `new_nickname` là NOT NULL (F070)? → A: `reason = reset`, `new_nickname = ''` (không đổi ràng buộc bảng append-only); thêm cột nullable `source_ref_id` = `sanction_id` để consumer áp **một lần mỗi sanction** (kể cả giao lại sau khi học sinh đã chọn tên mới); `changed_by` = `issued_by` của sanction (`uuid` 0 khi hệ thống), người đặt lại ở portal. [Confirmed — owner "chốt hết"]
- Q: Sanction `nickname_reset` ở các đường khác của module chat? → A: Có trong enum `SanctionType` (response và `createSanction` — cùng ngữ nghĩa tức thời); không đẩy `chat.sanction.applied` realtime (không đổi AsyncAPI), không vào cache sanction, job `sanction-expirer` bỏ qua (không có `sanction.lifted`), thu hồi → `409` (đã kết thúc); CHECK `chat.sanctions` nới cho loại mới (`ends_at = starts_at`, `scope = all`). [Confirmed — owner "chốt hết"]
- Q: `context.reported_field` hợp lệ khi nào? → A: Chỉ `nickname` với `target_type = user`; khác → `422`. Bằng chứng nickname được lưu thành một dòng `moderation_evidence` kể cả khi báo cáo không kèm tin nhắn. [Confirmed — owner "chốt hết"]
- Q: Facade `screenNicknamePersonalData` so với gì? → A: `org.students.full_name`, `identity.users.display_name` và tên trường (`name` + bản dịch); tên trường so cả phần riêng sau khi bỏ tiền tố loại trường ("Trường", "Tiểu học", "THCS"…), phần riêng quá ngắn (1 từ < 4 chữ) chỉ so tên đầy đủ. [Confirmed — owner "chốt hết"]
- Q: Audit lần từ chối? → A: `game_profile.nickname_refused` chỉ có mã kết quả (`format`/`moderation`/`personal`/`taken`), best effort; lần đổi thành công `game_profile.nickname_changed` (cũ/mới, nguồn), đặt lại `game_profile.nickname_reset` (nickname cũ, lý do, nguồn), ẩn danh lịch sử `game_profile.nickname_history_anonymized`. [Confirmed — owner "chốt hết"]
- **Câu hỏi mở F136-1** [Proposed]: F136-AC-22 yêu cầu nút "Đặt lại nickname" của portal hiện nickname hiện tại, nhưng danh sách học sinh (`listTenantGradeStudents`, module identity) không có nickname (nickname thuộc module game). Tạm thời hộp thoại nêu tên học sinh + giải thích tác dụng (component nhận `nickname` tùy chọn); không thêm trường contract ngoài phạm vi (R34). Cần owner chọn: thêm `nickname` vào một operation đọc của module game cho trang học sinh, hoặc bỏ yêu cầu hiển thị.
- Q: `nickname_changes_left` có trong response của `updateGameProfile`? → A: Chỉ `getGameProfile` (trường tùy chọn trong `GameProfile`); client đọc lại hồ sơ sau khi đổi. [Confirmed — owner "chốt hết"]

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tự gõ nickname ở màn tạo nhân vật, vẫn có "Gợi ý" (Priority: P1) 🎯 MVP

Minh (lớp 5) tạo nhân vật lần đầu, đến bước Tên gõ "Minh Siêu Nhân" thì thấy dòng "Đừng dùng tên thật của em", sửa thành "Siêu Nhân Sao Băng" và vào Quảng trường Tia Sáng. Lan (lớp 2) không gõ phím, chạm một tên gợi ý như trước.

**Why this priority**: Yêu cầu chính của owner (trẻ 6–15 tuổi không thích tên sinh sẵn); mọi học sinh mới đều qua bước này (FR-GAME-005, FR-GAME-015).

**Independent Test**: `hs.an.3a` (chưa có nickname) mở S02 → gõ tên hợp lệ → `updateGameProfile {nickname}` `200`, `game_profile.updated`; học sinh khác chạm chip gợi ý → `{nickname_option_id}` `200`; gửi cả hai trường → `422 VALIDATION_FAILED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F136-AC-01 | KHI học sinh đến bước Tên của S02, game client PHẢI hiện ô gõ nickname (bộ đếm ký tự tới 32, bàn phím ảo tiếng Việt trên điện thoại/Web), dòng gợi ý cố định "Đừng dùng tên thật của em", các chip gợi ý từ `suggestNicknames` và nút "Gợi ý khác"; học sinh mọi khối PHẢI hoàn tất được bước này chỉ bằng chạm một chip (không bắt buộc gõ phím) | FR-GAME-005 · FR-GAME-015 · [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname) · AC-UX-0201 | TC-GAM-011 |
| F136-AC-02 | KHI học sinh gõ, game client PHẢI kiểm tra sơ bộ (độ dài `nickname_min_length`–32 sau chuẩn hóa, tập ký tự cho phép, tối đa `nickname_max_digits` chữ số) chỉ để hiện gợi ý thân thiện và làm mờ nút "Tiếp" khi chưa hợp lệ; game client PHẢI không có danh mục từ hay logic kiểm duyệt nào và PHẢI coi mọi quyết định cuối cùng là của server | FR-GAME-019 · AGENTS R1 · AGENTS R5 · AGENTS R15 | TC-GAM-013 |
| F136-AC-03 | KHI học sinh xác nhận, game client PHẢI gửi `updateGameProfile` với đúng một trong hai: `nickname` (chuỗi đã gõ) hoặc `nickname_option_id` (chip đã chọn, khi học sinh không sửa chuỗi của chip); NẾU body có cả hai hoặc không có trường nào về nickname mà có `nickname: null`, THÌ module game PHẢI trả `422 VALIDATION_FAILED` | FR-GAME-005 · [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname) | TC-GAM-013 |
| F136-AC-04 | KHI nickname hợp lệ (US3), module game PHẢI lưu nickname đã chuẩn hóa và `nickname_key`, ghi `game.nickname_changes` (`reason = initial` cho lần đặt đầu), phát `game_profile.updated` trong cùng transaction (outbox) và trả `GameProfile`; game client PHẢI cho Mây chào bằng nickname mới rồi vào Main Hub như F070 | FR-GAME-005 · FR-GAME-015 · AGENTS R10 · AGENTS R26 | TC-GAM-011 |
| F136-AC-05 | NẾU server trả `422 NICKNAME_NOT_ALLOWED`, THÌ game client PHẢI hiện một thông điệp chung thân thiện ("Tên này chưa dùng được, em thử tên khác hoặc chọn tên gợi ý nhé"), giữ chuỗi trong ô để em sửa và làm nổi nút "Gợi ý khác"; `409 NICKNAME_UNAVAILABLE` PHẢI hiện "Tên này đã có bạn dùng rồi" kèm gợi ý mới; `429 RATE_LIMITED` PHẢI hiện "Em nghỉ một chút rồi thử lại nhé" và khóa nút tới `Retry-After`; `503 UNAVAILABLE` PHẢI hiện "Chưa kiểm tra được tên lúc này, em thử lại hoặc chọn tên gợi ý" — không câu nào nêu lý do cụ thể hay từ bị chặn | FR-GAME-019 · AGENTS R15 · [STUDENT_GAME_UX_CORE S02](../../docs/05-ux/STUDENT_GAME_UX_CORE.md#s02--tạo-avatar-lần-đầu) | TC-GAM-013 |
| F136-AC-06 | KHI học sinh chạm một chip gợi ý rồi sửa chuỗi trong ô, game client PHẢI chuyển sang gửi `nickname` tự gõ (không gửi `nickname_option_id` của chip đó); đường gợi ý (`suggestNicknames` + `nickname_option_id`, TTL `nickname_suggestion_ttl`) PHẢI giữ nguyên hành vi F070-AC-07…F070-AC-09 | FR-GAME-005 · [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname) | TC-GAM-011 |

---

### User Story 2 — Đổi nickname trong Cài đặt (Priority: P1)

Dũng (lớp 7) vào Cài đặt → Hồ sơ của em → Nickname, gõ tên mới; tháng này em đã đổi rồi nên thấy "Em đổi lại được từ 01/12".

**Why this priority**: Owner yêu cầu áp dụng cho cả màn đổi tên; client hiện chưa có màn này (F070 chỉ có API).

**Independent Test**: `hs.dung.7a` đổi nickname lần đầu trong tháng → `200`, lịch sử `player_change`; lần hai → `409 NICKNAME_CHANGE_LIMIT` + `retry_after`; người chơi khác trong Hub thấy tên mới sau `game_profile.updated`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F136-AC-07 | KHI học sinh mở Cài đặt → "Hồ sơ của em" → "Nickname", game client PHẢI hiện nickname hiện tại, ô gõ + chip gợi ý + "Gợi ý khác" giống S02, dòng "Đừng dùng tên thật của em" và số lần đổi còn lại trong tháng; KHI đã hết lượt, PHẢI khóa ô với ngày đổi lại được (từ `retry_after`, ngày theo locale, không đếm ngược giờ/phút) | FR-GAME-005 · AGENTS R19 · [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname) | TC-GAM-013 |
| F136-AC-08 | KHI học sinh đổi nickname sau lần đặt đầu, module game PHẢI áp cùng kiểm tra của US3, tính vào `nickname_change_monthly_limit` (tháng dương lịch giờ Asia/Ho_Chi_Minh; trường chỉ siết thêm), ghi `reason = player_change`; NẾU vượt giới hạn, THÌ PHẢI trả `409 NICKNAME_CHANGE_LIMIT` kèm `retry_after` **trước** khi gọi kiểm duyệt; lần đặt đầu và lần chọn lại sau đặt lại (US5) PHẢI không tính | FR-GAME-005 · F070-AC-11 | TC-GAM-013 |
| F136-AC-09 | KHI đổi nickname thành công, module game PHẢI phát `game_profile.updated` để `ref.player_profiles` của competition và chat cập nhật; game client của người chơi khác PHẢI hiện tên mới ở thẻ người chơi, nhãn trên đầu nhân vật và danh sách bạn bè sau khi nhận cập nhật, không cần đăng nhập lại | FR-GAME-005 · [CHARACTER_SYSTEM §8](../../docs/01-game-design/CHARACTER_SYSTEM.md#8-đồng-bộ-appearance-trong-multiplayer) | TC-GAM-011 |

---

### User Story 3 — Server kiểm tra và kiểm duyệt nickname tự gõ (Priority: P1)

Một học sinh gõ "Minh 0912 345 678", rồi "zalo minhxinh", rồi một từ trong danh mục nhạy cảm: mỗi lần chỉ thấy cùng một câu nhẹ nhàng; không lần nào lưu được và em không biết từ nào bị chặn.

**Why this priority**: Điều kiện an toàn bắt buộc để mở nhập tự do (R15, R16, constitution VI); không có thì không được phát hành US1/US2.

**Independent Test**: Integration module game với facade chat/identity fake có nhãn TEST DOUBLE và facade thật (contract test): bảng chuỗi mẫu (placeholder danh mục test, không từ thật) → kỳ vọng `200`/`422`/`409`/`429`/`503`; kiểm response không chứa từ khớp, vị trí, danh mục.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F136-AC-10 | KHI nhận `nickname` tự gõ, module game PHẢI chuẩn hóa (NFC, bỏ khoảng trắng đầu/cuối, gộp khoảng trắng) rồi kiểm: độ dài `nickname_min_length`–32 điểm mã, chỉ chữ Latin kể cả chữ tiếng Việt có dấu, chữ số và khoảng trắng đơn, ≥ 1 chữ cái, ≤ `nickname_max_digits` chữ số; NẾU vi phạm, THÌ PHẢI trả `422 NICKNAME_NOT_ALLOWED` với thông điệp chung theo `Accept-Language`, không echo chuỗi | FR-GAME-005 · FR-GAME-019 · ADR-0041 | TC-GAM-013 |
| F136-AC-11 | KHI chuỗi qua bước định dạng, module game PHẢI gọi facade `moderateDisplayText` của module chat (`context = nickname`, `TenantScope` của phiên, deadline `nickname_moderation_deadline`) — cùng chuẩn hóa tiếng Việt, danh mục hiệu lực platform + tenant, công tắc hiệu lực và bước PII luôn chạy (số điện thoại kể cả viết bằng chữ/tách khoảng, email, URL/tên miền, tài khoản/ứng dụng mạng xã hội, `@handle`); NẾU `decision = blocked`, THÌ PHẢI trả `422 NICKNAME_NOT_ALLOWED` | FR-GAME-019 · AGENTS R15 · AGENTS R29 · [MODERATION §2, §3.3](../../docs/03-social-multiplayer/MODERATION.md#33-bước-4--pii-detector-stepspii-luôn-chạy) · ADR-0078 | TC-GAM-013, TC-CHAT-010 |
| F136-AC-12 | KHI chuỗi qua kiểm duyệt, module game PHẢI gọi facade `screenNicknamePersonalData` của module identity; module identity PHẢI trả `blocked` khi nickname (không dấu, chữ thường) bằng họ tên của chính học sinh, chứa ≥ 2 từ của họ tên đó hoặc chứa tên trường của học sinh, và PHẢI chỉ trả `allowed`/`blocked` (không họ tên, không tên trường); NẾU `blocked`, THÌ module game PHẢI trả `422 NICKNAME_NOT_ALLOWED` | FR-GAME-019 · AGENTS R16 · AGENTS R13 · AGENTS R29 | TC-GAM-013 |
| F136-AC-13 | Module game PHẢI kiểm trùng theo `nickname_key` (chữ thường, NFC, khoảng trắng đã gộp) trong tenant **sau** khi qua kiểm duyệt và kiểm dữ liệu cá nhân, và PHẢI dựa vào unique partial `(tenant_id, nickname_key) WHERE deleted_at IS NULL`; NẾU trùng (kể cả hai request đồng thời — chỉ một thắng), THÌ PHẢI trả `409 NICKNAME_UNAVAILABLE` | FR-GAME-005 · AGENTS R21 · AGENTS R27 | TC-GAM-013 |
| F136-AC-14 | NẾU facade chat hoặc identity lỗi, trả `503` hoặc quá `nickname_moderation_deadline`, THÌ module game PHẢI trả `503 UNAVAILABLE`, PHẢI không lưu nickname và PHẢI không coi kết quả là `allowed` (fail-closed); đường `nickname_option_id` PHẢI vẫn hoạt động vì gợi ý sinh từ bộ từ đã duyệt | FR-GAME-019 · AGENTS R18 · AGENTS R29 | TC-GAM-013 |
| F136-AC-15 | Module game PHẢI giới hạn mọi lần gửi `nickname` tự gõ theo `nickname_attempt_rate` cho mỗi học sinh (đếm cả lần bị từ chối); NẾU vượt, THÌ PHẢI trả `429 RATE_LIMITED` + `Retry-After` trước khi gọi facade; PHẢI ghi metric `eduworld_nickname_checks_total{result}` (không nhãn người dùng) và PHẢI không tự áp sanction hay cảnh cáo học sinh | FR-GAME-019 · AGENTS R19 · [MODERATION §5](../../docs/03-social-multiplayer/MODERATION.md#5-report-flow) | TC-GAM-013 |
| F136-AC-16 | Mọi response từ chối nickname PHẢI không chứa từ đã khớp, vị trí khớp, danh mục, loại PII hay họ tên/tên trường; ngoài `updateGameProfile` PHẢI không có endpoint nào nhận nickname tự gõ; module game PHẢI không tin bất kỳ cờ "đã kiểm tra" nào từ client | FR-GAME-019 · AGENTS R5 · AGENTS R15 · AGENTS R16 | TC-GAM-013 |

---

### User Story 4 — Báo cáo nickname không phù hợp (Priority: P2)

Bình thấy một bạn trong Quảng trường có nickname khó chịu, chạm vào nhân vật → "Báo cáo" → "Tên không phù hợp" → "Cảm ơn em, đội an toàn EduWorld sẽ xem xét".

**Why this priority**: Lớp bảo vệ thứ hai khi danh mục bỏ sót (owner yêu cầu); dùng hàng đợi moderation sẵn có của F121.

**Independent Test**: `hs.binh.3a` báo cáo `hs.an.3a` với `context.reported_field = nickname` → `201`; case của Safety Team có bản chụp nickname do server lấy từ `ref.player_profiles`; client gửi kèm nickname giả trong `note` → bằng chứng vẫn là nickname thật trên server.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F136-AC-17 | Thẻ người chơi (chạm nhân vật trong Hub S03, hồ sơ bạn bè, danh sách "Người chơi cùng em" của chat, kết quả trận) PHẢI có "Báo cáo" với lựa chọn "Tên không phù hợp" (icon + chữ, ≤ 2 thao tác); KHI chọn, game client PHẢI gửi `createReport` (`target_type = user`, `reason = inappropriate_content`, `context.reported_field = nickname`) rồi hiện lời cảm ơn và gợi ý Chặn như MODERATION §5 | FR-MOD-002 · FR-GAME-019 · [MODERATION §5](../../docs/03-social-multiplayer/MODERATION.md#5-report-flow) | TC-GAM-014 |
| F136-AC-18 | KHI nhận báo cáo có `context.reported_field = nickname`, module chat PHẢI chụp nickname hiện tại của người bị báo cáo từ `ref.player_profiles` vào bằng chứng của case (không nhận nickname từ client), gộp case theo người bị báo cáo trong 24 giờ như F121, và PHẢI không cho người bị báo cáo biết ai đã báo cáo | FR-MOD-002 · AGENTS R5 · AGENTS R16 · [MODERATION §5](../../docs/03-social-multiplayer/MODERATION.md#5-report-flow) | TC-GAM-014 |

---

### User Story 5 — Đặt lại nickname bởi Safety Team và nhà trường (Priority: P2)

Safety Team xem case nickname, chọn "Đặt lại nickname"; lần vào game sau, học sinh thấy "Tên của em chưa phù hợp, em chọn tên mới nhé" và chỉ làm lại bước Tên. Cô School Admin của trường TH Demo A đặt lại nickname một học sinh trường mình trên portal.

**Why this priority**: CHARACTER_SYSTEM §5.3 đã cho `school_admin`/`safety_moderator` đặt lại nhưng chưa có API; nhập tự do làm việc này cần thiết hơn.

**Independent Test**: `safety_moderator` áp `nickname_reset` trên case → `sanction.applied` → game đặt nickname `NULL` (idempotent theo `sanction_id`); `ad.truong.a` gọi `resetStudentNickname` cho học sinh trường mình `200`, trường khác `404`, học sinh tự gọi `403`; lần chọn lại không tính vào giới hạn tháng.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F136-AC-19 | KHI `safety_moderator` chọn hành động "Đặt lại nickname" trên một case, module chat PHẢI tạo sanction `nickname_reset` (tức thời: `ends_at = starts_at`), ghi audit và phát `sanction.applied` (`sanction_type = nickname_reset`) qua outbox; role không phải `safety_moderator`/`system_admin` PHẢI bị từ chối `403` | FR-GAME-019 · FR-MOD-002 · AGENTS R20 · [MODERATION §7](../../docs/03-social-multiplayer/MODERATION.md#7-sanctions) | TC-GAM-014 |
| F136-AC-20 | KHI module game nhận `sanction.applied` loại `nickname_reset`, module game PHẢI (idempotent theo `sanction_id`) đặt `nickname` và `nickname_key` = `NULL`, đặt `setup_required = true`, ghi `game.nickname_changes` (`reason = reset`, `changed_by` = người áp hoặc hệ thống) và phát `game_profile.updated`; lần chọn nickname kế tiếp PHẢI không tính vào `nickname_change_monthly_limit` | FR-GAME-019 · AGENTS R10 · AGENTS R29 | TC-GAM-014, TC-SVC-003 |
| F136-AC-21 | KHI `school_admin` (học sinh thuộc trường trong scope), `tenant_admin` hoặc System Admin (tenant đích qua `X-Tenant-Id`, kể cả `common`) gọi `resetStudentNickname` với `reason` và `Idempotency-Key`, module game PHẢI kiểm permission `lms.student.nickname_reset` bằng `authz.Require`, đặt lại như F136-AC-20 và ghi audit (actor, lý do, nickname cũ); học sinh ngoài scope PHẢI trả `404`, học sinh tự gọi hoặc role thiếu permission PHẢI trả `403`, replay cùng key PHẢI trả kết quả đã lưu | FR-GAME-019 · AGENTS R7 · AGENTS R8 · AGENTS R10 | TC-GAM-014, TC-TEN-001 |
| F136-AC-22 | Trang học sinh của portal (F052) PHẢI có nút "Đặt lại nickname" hiện nickname hiện tại và hộp nhập lý do bắt buộc, chỉ hiện khi người dùng có `lms.student.nickname_reset` (chỉ là hiển thị; backend quyết định); hàng đợi case của Safety Team (F122) PHẢI hiện bản chụp nickname và hành động "Đặt lại nickname" | FR-GAME-019 · AGENTS R1 · AGENTS R32 | TC-GAM-014 |
| F136-AC-23 | KHI `setup_required = true` do nickname bị đặt lại (avatar còn), game client PHẢI chỉ mở bước Tên của S02 với câu thân thiện theo lứa tuổi "Tên của em chưa phù hợp, em chọn tên mới nhé" (MODERATION §7: có lý do ngắn, không nêu ai đặt lại hay ai báo cáo); TRONG KHI nickname = `NULL`, mọi client PHẢI hiển thị người đó bằng chuỗi trung tính `ui.player.unnamed` (không nickname cũ, không username) | FR-GAME-019 · AGENTS R16 · AGENTS R19 | TC-GAM-014 |

---

### User Story 6 — Chất lượng giao diện: song ngữ, token, điện thoại, ma trận khối, văn bản thuần (Priority: P2)

**Why this priority**: Chuẩn giao diện học sinh (F114, F125) và constitution VIII; kiểm sau luồng chính.

**Independent Test**: Ma trận khối cho bước Tên S02 (rỗng, đang gõ, lỗi, hết lượt) và màn Nickname trong Cài đặt × 4 khối × 4 cửa sổ × `vi`/`en`/pseudo; ảnh trước/sau; quét mã hiển thị nickname.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F136-AC-24 | Mọi chuỗi UI mới PHẢI qua khóa `ui.avatar.nickname.*`, `ui.profile.nickname.*`, `ui.player.unnamed`, `ui.report.reason.nickname`, `ui.error.NICKNAME_NOT_ALLOWED` (vi/en, không ghép câu bằng code); thông điệp lỗi server PHẢI theo `Accept-Language`; bố cục PHẢI chịu chuỗi dài hơn 35% (pseudo-locale) không cắt, không tràn | NFR-I18N-001 · NFR-I18N-003 · ADR-0056 | TC-I18N-006 |
| F136-AC-25 | Ô gõ, chip, nút và thông báo PHẢI lấy màu/cỡ chữ/khoảng cách từ Theme sinh từ `design/game-tokens/` theo phong cách tầng F114; trạng thái lỗi PHẢI có icon + chữ, không chỉ màu; TRONG chế độ `compact` (F125), ô gõ PHẢI luôn nằm trên bàn phím ảo (màn cuộn lên khi bàn phím mở), vùng chạm ≥ `density.compact.button.min_height`, chữ ≥ 12 CSS px ở 844×390 và 740×360 | FR-GAME-010 · NFR-A11Y-001 · AGENTS R32 · F125-AC-03 | TC-UI-003 |
| F136-AC-26 | Ma trận khối PHẢI dựng bước Tên S02 (rỗng, đang gõ hợp lệ, lỗi `NICKNAME_NOT_ALLOWED`, `503`, gợi ý) và màn Nickname trong Cài đặt (còn lượt, hết lượt) × 4 khối × cửa sổ 1280×720, 1024×600, 844×390, 740×360 × `vi`/`en`/pseudo và PHẢI pass probe không cắt chữ, không tràn, nút ≥ ngưỡng; ảnh trước/sau PHẢI được chụp ở 1280×720, 844×390, 740×360 (`vi`, `g1_3` và `g6_7`) và ghi bảng màn × khối ở quickstart §5 | FR-GAME-010 · F125-AC-04 · [DESIGN_SYSTEM §9](../../docs/05-ux/DESIGN_SYSTEM.md#9-đảm-bảo-chất-lượng) | TC-UI-003, TC-I18N-006 |
| F136-AC-27 | Game client PHẢI hiển thị mọi nickname (của mình và người khác, ở mọi màn: nhãn trên đầu, thẻ, chat, bạn bè, bảng xếp hạng, boss, party) là văn bản thuần — `Label` hoặc `RichTextLabel` tắt BBCode — và test quét mã PHẢI chặn nơi hiển thị nickname có BBCode bật | FR-GAME-019 · AGENTS R1 · AGENTS R15 | TC-CHAT-010 |

---

### User Story 7 — Riêng tư, cô lập tenant, phân quyền, ẩn danh hóa (Priority: P3)

**Why this priority**: Bắt buộc theo constitution III, V, VI, IX; phần lớn đã có ở F070, chỉ mở rộng cho dữ liệu tự gõ.

**Independent Test**: Test tenant isolation (`hs.dung.7a` tenant khác không ảnh hưởng độ duy nhất của `hs.an.3a`), token nhân sự gọi route người chơi → `403`, quét log khi gửi chuỗi bị chặn, ẩn danh hóa học sinh có lịch sử đổi tên.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F136-AC-28 | Kiểm trùng, đặt lại và lịch sử nickname PHẢI chỉ trong tenant của phiên (`TenantScope`, ent interceptor + RLS); NẾU `X-Tenant-Id` khác tenant của học sinh, THÌ PHẢI trả `403 TENANT_MISMATCH`; nickname giống nhau ở hai tenant khác nhau PHẢI cùng được phép | AGENTS R8 · AGENTS R27 · NFR-SEC-006 | TC-TEN-001, TC-TEN-011 |
| F136-AC-29 | `updateGameProfile` PHẢI chỉ cho học sinh qua middleware `game_player` (token nhân sự → `403 GAME_ACCESS_STUDENT_ONLY`); facade `moderateDisplayText` (`context = nickname`) và `screenNicknamePersonalData` PHẢI chỉ nhận module có trong `x-allowed-callers` (`game`) và PHẢI có facade test cả bên gọi lẫn bên nhận | AGENTS R7 · AGENTS R23 · AGENTS R29 · ADR-0031 | TC-AUTH-014, TC-SVC-005 |
| F136-AC-30 | Hệ thống PHẢI không ghi chuỗi nickname bị từ chối vào log (mọi mức), trace, telemetry hay audit; audit lần đổi thành công PHẢI có giá trị cũ/mới, lần bị từ chối PHẢI chỉ có mã kết quả; log INFO PHẢI không chứa nickname hay họ tên | AGENTS R14 · NFR-PRIV-001 | TC-GAM-013 |
| F136-AC-31 | KHI `anonymizeUserData` chạy cho một học sinh, module game PHẢI thay nickname ở `game.game_profiles` và `old_nickname`/`new_nickname` ở `game.nickname_changes` bằng giá trị ẩn danh (qua hàm `SECURITY DEFINER` của migration, có audit), không xóa vật lý dòng nào | AGENTS R21 · ADR-0070 · NFR-PRIV-001 | TC-GAM-014 |
| F136-AC-32 | Nickname PHẢI không bao giờ được dùng làm khóa đăng nhập, không xuất hiện trong token hay bootstrap của người khác ngoài `PlayerCard`; mọi chuỗi mới trong contract (OpenAPI public + internal, event) PHẢI có `maxLength` (nickname 32, `reason` 500) | AGENTS R16 · AGENTS R28 · ADR-0041 | — |

### Edge Cases

- NẾU học sinh gõ đúng chuỗi của một chip gợi ý mà không chạm chip, THÌ gửi như `nickname` tự gõ (qua kiểm duyệt); gợi ý sinh từ bộ từ đã duyệt nên kết quả `allowed`.
- NẾU danh mục từ nhạy cảm đổi sau khi nickname đã được chấp nhận, THÌ nickname cũ **không** tự bị đặt lại (tránh mất tên hàng loạt); báo cáo + đặt lại (US4, US5) xử lý từng trường hợp.
- NẾU học sinh đang trong trận/party khi bị đặt lại, THÌ chỉ hiển thị `ui.player.unnamed` cho người khác ngay; bước Tên mở ở lần vào game sau (không ngắt trận).
- NẾU hai học sinh cùng tenant gửi "An An" và "an an" cùng lúc, THÌ unique index cho một người, người kia `409`.
- NẾU học sinh chuyển tenant (gộp tài khoản F045) mà nickname đã có người dùng ở tenant đích, THÌ theo F070 (profile đích giữ; nếu trùng → `nickname = NULL`, `setup_required`).
- NẾU công tắc kiểm duyệt tắt bước danh mục (ADR-0030), THÌ bước PII vẫn chạy (ADR-0078) và nickname vẫn phải qua `screenNicknamePersonalData`.
- NẾU học sinh bị `restrict_text`/`mute`, THÌ vẫn đổi nickname được (không phải tin nhắn) nhưng vẫn qua kiểm duyệt; `account_lock` chặn mọi thao tác như cũ.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-GAME-005 | Nickname tự gõ hoặc chọn gợi ý ở S02 và màn đổi tên; duy nhất trong tenant không phân biệt hoa thường; giới hạn đổi/tháng | [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname) |
| FR-GAME-019 | Kiểm tra định dạng, kiểm duyệt qua pipeline chat, chặn tên thật/tên trường, fail-closed, giới hạn lần thử, đặt lại bởi `school_admin`/`safety_moderator` | [CHARACTER_SYSTEM §5.3](../../docs/01-game-design/CHARACTER_SYSTEM.md#53-nickname), [MODERATION §2, §7](../../docs/03-social-multiplayer/MODERATION.md) |
| FR-MOD-002 (phần nickname) | Báo cáo nickname qua `createReport`, bản chụp nickname phía server | [MODERATION §5](../../docs/03-social-multiplayer/MODERATION.md#5-report-flow) |

NFR: NFR-I18N-001, NFR-I18N-003, NFR-A11Y-001, NFR-SEC-006, NFR-PRIV-001.

### Key Entities

- **GameProfile** (có sẵn, F070): thêm `nickname_key` (khóa so trùng).
- **NicknameChange** (có sẵn, append-only): lý do `initial`/`player_change`/`reset`; giá trị được ẩn danh hóa khi ẩn danh học sinh.
- **NicknameAttemptCounter**: bộ đếm lần thử theo học sinh (Valkey, không phải dữ liệu quan trọng — R9).
- **Sanction `nickname_reset`** (module chat): sanction tức thời, nguồn của việc đặt lại bởi Safety Team.
- **Report context `reported_field = nickname`** + bản chụp nickname trong bằng chứng case (module chat).

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `nickname_min_length` | Độ dài tối thiểu sau chuẩn hóa (tối đa 32 cố định theo CHARACTER_SYSTEM §5.3) |
| `nickname_max_digits` | Số chữ số tối đa trong nickname |
| `nickname_attempt_rate` | Giới hạn lần gửi nickname tự gõ mỗi học sinh |
| `nickname_moderation_deadline` | Deadline mỗi lần gọi facade kiểm duyệt/dữ liệu cá nhân |
| `nickname_change_monthly_limit`, `nickname_suggestion_ttl` | Có sẵn (F070), giữ nguyên |
| `compact_short_side`, `compact_density_floors`, `supported_locales` | Giao diện điện thoại, ngôn ngữ |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không logic kiểm duyệt/danh mục ở client; server quyết định | AGENTS R1, R5, R15 |
| Một bộ quy tắc cho mọi khối; không hardcode từ trong code/test/seed (fixture placeholder) | AGENTS R15 · ADR-0020 · ADR-0030 |
| Không hiển thị tên thật/trường; không trả họ tên/tên trường qua facade | AGENTS R16 |
| Gọi module khác chỉ qua facade có kiểu; fail-closed | AGENTS R13, R29 |
| Không log PII/chuỗi bị từ chối | AGENTS R14 |
| Chỉ soft-delete; ẩn danh hóa thay xóa | AGENTS R21 · ADR-0070 |
| Idempotency + audit cho thao tác đặt lại | AGENTS R10 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Học sinh lớp 4–9 đặt được nickname tự gõ hợp lệ ở S02 trong ≤ 60 giây; học sinh lớp 1–3 vẫn hoàn tất S02 không cần gõ phím.
- **SC-002**: 100% chuỗi trong bộ mẫu kiểm thử chứa số điện thoại, email, URL, tài khoản mạng xã hội, từ danh mục (placeholder) hoặc họ tên của chính học sinh bị từ chối; 0 response từ chối chứa từ khớp, vị trí hay danh mục.
- **SC-003**: `updateGameProfile` với nickname tự gõ có p95 ≤ 300 ms ở tải bình thường (gồm hai lần gọi facade trong tiến trình).
- **SC-004**: Safety Team đặt lại một nickname từ case trong ≤ 3 thao tác; học sinh chỉ phải làm lại bước Tên.
- **SC-005**: 100% màn mới pass ma trận khối 4 khối × 4 cửa sổ × 3 ngôn ngữ và có ảnh đã xem ở quickstart §5.

## Ngoài phạm vi

- Màn đăng ký tài khoản (F135) — chỉ dẫn vào S02 của feature này; tên đăng nhập không phải nickname.
- Kiểm duyệt lại hàng loạt nickname cũ khi danh mục đổi; tự động đặt lại nickname theo điểm rủi ro.
- Nickname theo chữ hệ khác (Hán, Thái…), emoji, ký tự trang trí; tên hiển thị khác nhau theo ngôn ngữ.
- Đổi mã lỗi hay quy tắc tên party/đội (giữ `moderateDisplayText` `party_name`/`team_name` như F100).
- Thay đổi chính sách xử phạt khác của MODERATION §7 ngoài thêm `nickname_reset`.

## Assumptions

- F070 (GameProfile, `suggestNicknames`, `updateGameProfile`, `game.nickname_changes`), F116 (pipeline + `moderateDisplayText`), F121 (case, sanction; `sanction.applied` có schema owner F116), F052 (trang học sinh portal) và F122 (hàng đợi case) đã có; F136 mở rộng, không viết lại.
- Module identity có họ tên (`org.students.full_name`) và trường của học sinh; học sinh tự đăng ký (F135) không có họ tên.
- Danh mục từ nhạy cảm trong test chỉ dùng placeholder (R15); test PII dùng số/email giả.
- Phần UI Cài đặt → "Hồ sơ của em" (S15) đã có khung từ F114; F136 thêm mục Nickname.
