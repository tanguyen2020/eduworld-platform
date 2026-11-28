# Feature Specification: F135 — Học sinh tự đăng ký trong game vào tenant chung (bước phụ huynh, mã khôi phục, chống lạm dụng)

**Feature Branch**: `135-game-self-registration` · **Created**: 2026-11-28 · **Status**: Ready · **Module**: module `identity` + `src/game/` + `src/cms/frontend/apps/portal` · **Mốc**: M5 · **Phụ thuộc**: F010 (tenant `common`, School ảo, `createCommonTenantStudents`), F012 (mẫu token đặt lại mật khẩu portal), F013 (đăng nhập game, bảo vệ đăng nhập), F016 (khóa, thu hồi phiên, dọn học sinh), F045 (saga gộp tài khoản), F057 (portal workspace D), F061 (form đăng nhập Godot), F124 (gửi email của module identity), F125 (chế độ `compact`), F136 (nickname tự gõ ở S02)
**Input**: Owner (2026-11-28): chưa có trường nào tích hợp SIS, học sinh muốn chơi thử mà không thuộc trường nào. Học sinh tự tạo tài khoản ngay trong game, vào tenant `common`, có đủ tính năng như học sinh tenant chung; phụ huynh xác nhận trên màn đăng ký; khôi phục mật khẩu không cần email. Quyết định ở [ADR-0104](../../docs/adr/ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung.md), đóng OQ-27. FR nguồn: FR-AUTH-001, FR-AUTH-017, FR-AUTH-021, FR-AUTH-033, FR-AUTH-034, FR-AUTH-035, FR-AUTH-036.

## Clarifications

### Session 2026-11-28 (quyết định owner — [Confirmed — owner 2026-11-28])

- Q: Có cho học sinh tự đăng ký không, vào đâu? → A: Có, chỉ trong game (portal và nhân sự không đổi). Tài khoản vào tenant `common`, School ảo "EduWorld chung", khối học sinh chọn. Thay FR-AUTH-001 cũ, câu "Không có self-registration" của [ACCOUNT_PROVISIONING §1](../../docs/09-identity/ACCOUNT_PROVISIONING.md#1-các-con-đường-tạo-tài-khoản) và [KEYCLOAK_SSO](../../docs/09-identity/KEYCLOAK_SSO.md) (phần học sinh). [Confirmed — owner 2026-11-28]
- Q: Màn đăng ký ở đâu, cho ai? → A: Màn trong Godot (luôn ngang), dễ cho lớp 1–9, có đọc to, theo luật bố cục điện thoại F125. [Confirmed — owner 2026-11-28]
- Q: Thu những gì? → A: Bắt buộc tên đăng nhập, mật khẩu, khối (1–9), ngày sinh (kiểm hợp lý với khối, khoảng 6–15 tuổi có biên; không bao giờ hiển thị cho người chơi khác — R16). **Không** thu họ tên thật (tối thiểu hóa dữ liệu). Nickname chọn ở màn tạo nhân vật (F136). [Confirmed — owner 2026-11-28]
- Q: Phụ huynh xác nhận thế nào? → A: Bước phụ huynh ngay trên màn đăng ký: điều khoản ngắn tiếng Việt có đọc to + ô "Tôi là phụ huynh và đồng ý"; lưu bằng chứng (thời điểm, phiên bản điều khoản, hash `device_id` — không IP thô, R23). Email phụ huynh **tùy chọn** (nhiều phụ huynh nông thôn không có): có thì gửi link xác nhận (tăng tin cậy, khôi phục mật khẩu), **không** bắt buộc để chơi. [Confirmed — owner 2026-11-28]
- Q: Quên mật khẩu khi không có email? → A: Mã khôi phục dùng một lần hiện ở cuối đăng ký (phụ huynh ghi/chụp); "Quên mật khẩu" bằng tên đăng nhập + mã → mật khẩu mới; mã lưu băm; có rate limit. [Confirmed — owner 2026-11-28]
- Q: Tính năng của học sinh tự đăng ký? → A: Đầy đủ như học sinh tenant chung hiện có ("không được phép lược bỏ phần nào"): chat, bạn bè, club, thi đấu, quest — cùng một bộ quy tắc (R15). "Nhà trường giao" vẫn có trong UI với trạng thái rỗng thân thiện. Sau này gộp vào trường thật qua saga gộp tài khoản (F045), giữ XP/tiến độ. [Confirmed — owner 2026-11-28]
- Q: Chống tạo tài khoản ảo? → A: Giới hạn số đăng ký theo thiết bị mỗi ngày (bản cài), không bao giờ theo một IP (R30); thông điệp lỗi chung (không lộ tên đăng nhập ngoài "tên đã có người dùng"); tạo user Keycloak qua module identity theo mẫu ACCOUNT_PROVISIONING; event provisioning sẵn có; idempotent (`Idempotency-Key`, R10); audit. [Confirmed — owner 2026-11-28]
- Q: Quản trị? → A: System Admin / nhân sự platform thấy và quản lý học sinh tự đăng ký ở màn sẵn có của tenant `common` (thêm bộ lọc/nhãn); dọn học sinh không đăng nhập (ADR-0032) áp nguyên. [Confirmed — owner 2026-11-28]
- Q: Rủi ro pháp lý? → A: ADR-0104 nêu rủi ro của đồng ý chỉ bằng ô đánh dấu và khuyến nghị pháp chế rà soát; CHILD_SAFETY cập nhật. [Confirmed — owner 2026-11-28]

### Session 2026-11-28 (chi tiết chốt theo lệnh "chốt hết" — phương án an toàn nhất)

- Q: Có bật trang đăng ký của Keycloak (`registrationAllowed`)? → A: **Không.** Realm giữ `registrationAllowed = false`; game không mở trang Keycloak (ADR-0068). Module identity tạo user bằng Admin API (service account sẵn có của provisioning), cùng bộ điều hợp và thứ tự bước của `createCommonTenantStudents`. Không có endpoint công khai nào của Keycloak cho đăng ký. [Confirmed — owner "chốt hết"]
- Q: Sau khi đăng ký có trả token luôn không? → A: Không. `registerStudent` trả tài khoản + mã khôi phục; client giữ mật khẩu trong RAM rồi gọi `gameLogin` như bình thường, để mọi kiểm tra đăng nhập (giới nghiêm, một thiết bị, hàng chờ, phiên bản client) chỉ có một đường (R30). Mật khẩu xóa khỏi RAM ngay sau đó. [Confirmed — owner "chốt hết"]
- Q: Kiểm hợp lý ngày sinh? → A: Năm học bắt đầu `Y` (5/9 giờ ICT). Tuổi theo năm `Y − năm sinh` phải nằm trong `[khối + 5 − 1, khối + 5 + 2]` (`self_registration_age_margin`: nhỏ hơn 1 năm, lớn hơn 2 năm do học muộn/lưu ban) và trong 5–16; ngày tương lai hoặc không tồn tại bị từ chối. Ngày sinh lưu `org.students.date_of_birth_enc` (mã hóa cột như dữ liệu SIS), không vào token, event, log, analytics, không trả cho người chơi khác. [Confirmed — owner "chốt hết"]
- Q: Tên đăng nhập? → A: Cùng mẫu và chuẩn hóa với tài khoản tenant chung (trim, NFC, chữ thường, `^[a-z0-9._-]{3,32}$` ở màn đăng ký — nằm trong mẫu 3–128 chung; không `@`); kiểm trùng trên SSO và `identity.users` kể cả tenant khác và tài khoản soft-delete chưa ẩn danh (ADR-0069); không được bắt đầu bằng tiền tố dành riêng `self_registration_reserved_username_prefixes` (ví dụ `hs.` của username SIS, quy ước nhân sự) để không chiếm trước tên trường sẽ cấp. Màn nhắc "Đừng dùng tên thật của em". Tên đăng nhập không hiển thị cho người chơi khác (R16). [Confirmed — owner "chốt hết"]
- Q: Mật khẩu? → A: Theo password policy của realm (không thêm tham số mới); `temporary = false` vì học sinh tự đặt; client có hai ô (nhập lại) và nút hiện/ẩn; mật khẩu chỉ qua TLS, không log/trace/telemetry (FR-AUTH-003). [Confirmed — owner "chốt hết"]
- Q: Họ tên trong hệ thống? → A: `identity.users.display_name = NULL`, `org.students.full_name = NULL`; portal hiển thị tên đăng nhập + nhãn "Tự đăng ký". [Confirmed — owner "chốt hết"]
- Q: Điều khoản phụ huynh lưu ở đâu? → A: Văn bản có version trong seed của module identity (vi/en, review như mã), phục vụ qua `getRegistrationTerms` (public); đổi văn bản = version mới; bằng chứng ghi `terms_version`. Đọc to dùng TTS sẵn có của client ("Đọc to", F097). [Confirmed — owner "chốt hết"]
- Q: Email phụ huynh lưu thế nào? → A: Mã hóa cột (như ngày sinh) + HMAC để đếm giới hạn gửi; không log; chỉ System Admin thấy dạng che (`a***@g***.com`). Link xác nhận một lần, hạn `parent_email_confirm_ttl`; một địa chỉ nhận tối đa `parent_email_daily_send_limit` email/ngày (anh chị em dùng chung email vẫn được). Email gửi qua hàng đợi email sẵn có của module identity (F124), mẫu vi/en trong file dịch. Trang đích là route công khai của portal (`/phu-huynh/xac-nhan`, `/phu-huynh/dat-lai-mat-khau`), không đăng nhập, không tải mã workspace (ADR-0027). [Confirmed — owner "chốt hết"]
- Q: Mã khôi phục? → A: 12 ký tự Crockford base32 (không ký tự dễ nhầm), hiển thị nhóm 4 (`ABCD-EFGH-JKMN`); băm argon2id; một mã hiệu lực/tài khoản; dùng xong hoặc khi System Admin đặt lại mật khẩu thì mã cũ vô hiệu và cấp mã mới (hiện một lần). Sai mã/tên → `401 RECOVERY_INVALID` chung; quá `recovery_attempt_limit` → `429` (khóa khôi phục, không khóa đăng nhập). [Confirmed — owner "chốt hết"]
- Q: Giới hạn đăng ký? → A: `self_registration_device_daily_limit` (3/thiết bị/ngày ICT, đếm trong PostgreSQL theo HMAC `device_id` — R9) và trần toàn cục `self_registration_global_hourly_limit` (300/giờ) → `429 REGISTRATION_BUSY` + `Retry-After`; không CAPTCHA (trẻ nhỏ, trợ năng); không theo IP. System Admin tắt nhanh bằng `self_registration_enabled` (cài đặt platform, audit). [Confirmed — owner "chốt hết"]
- Q: Đăng ký trong giờ giới nghiêm? → A: Bị từ chối `403 CURFEW_ACTIVE` kèm `reopen_at` như đăng nhập (ADR-0078), không tạo tài khoản. [Confirmed — owner "chốt hết"]
- Q: Mặc định xã hội của tenant `common`? → A: Theo [CHILD_SAFETY §3.6](../../docs/10-security/CHILD_SAFETY.md#36-tenant-common-adr-0078) đã sửa theo quyết định owner 2026-11-28 ([ADR-0104](../../docs/adr/ADR-0104-hoc-sinh-tu-dang-ky-trong-game-vao-tenant-chung.md) mục 10, thay một phần ADR-0078 mục 3): kết bạn, DM, World chat **bật mặc định** cho mọi học sinh tenant `common`; System Admin giữ công tắc sự cố tắt từng mục (lý do + audit); Area chỉ tin nhanh + biểu cảm; chat gameplay luôn có; pipeline kiểm duyệt, PII/grooming, giờ giới nghiêm, chặn/báo cáo không đổi. Học sinh tự đăng ký **không** bị hạn chế thêm và **không** được nới thêm so với học sinh tenant chung khác — "đủ tính năng như học sinh tenant chung". [Confirmed — owner 2026-11-28]
- Q: Phụ huynh muốn rút đồng ý / xóa tài khoản? → A: Không có API tự phục vụ trong F135 (mô hình consent chung vẫn hoãn theo ADR-0077/ADR-0082); điều khoản ghi kênh liên hệ hỗ trợ, System Admin khóa tài khoản bằng luồng khóa sẵn có (F016) có lý do + audit. [Confirmed — owner "chốt hết"]
- Q: Khối thay đổi theo năm học? → A: Không tự đổi; System Admin sửa bằng chức năng sẵn có. Câu hỏi mở F135-1. [Confirmed — owner "chốt hết"]

## User Scenarios & Testing *(bắt buộc)*

### User Story 1 — Tự tạo tài khoản và vào game (Priority: P1) 🎯 MVP

Lan (lớp 4, chưa có trường dùng EduWorld) mở game, bấm "Tạo tài khoản", chọn khối 4 bằng ô hình lớn, chọn ngày sinh bằng ba bánh xe ngày/tháng/năm, đặt tên đăng nhập và mật khẩu; mẹ đọc (hoặc nghe đọc to) điều khoản, đánh dấu đồng ý; Lan được mã khôi phục, mẹ chụp lại; vào game, tạo nhân vật và chọn nickname.

**Why this priority**: Không có bước này thì học sinh ngoài trường đối tác không vào được game (OQ-27).

**Independent Test**: Stack compose: Godot `--device-id` mới → S01 "Tạo tài khoản" → `registerStudent` `201` → `gameLogin` → `createGameSession` → S02; DB: `identity.users` (`student`, `primary_tenant_id = common`, `display_name` NULL), `org.students` School ảo khối 4, `identity.self_registrations` có bằng chứng; outbox `user.provisioned` `source = self_registration`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F135-AC-01 | KHI S01 hiện form đăng nhập và `getRegistrationTerms` trả `enabled = true`, game client PHẢI hiện nút "Tạo tài khoản" mở màn đăng ký; NẾU `enabled = false` hoặc lỗi mạng, THÌ nút PHẢI ẩn (đăng nhập vẫn dùng được) | FR-AUTH-017 · FR-AUTH-033 · [STUDENT_GAME_UX_CORE S01](../../docs/05-ux/STUDENT_GAME_UX_CORE.md#s01--đăng-nhập) | TC-AUTH-102 |
| F135-AC-02 | Màn đăng ký PHẢI gồm theo thứ tự: chọn khối (9 ô 1–9), ngày sinh (chọn ngày/tháng/năm, không gõ tự do), tên đăng nhập (dòng nhắc "Đừng dùng tên thật của em"), mật khẩu + nhập lại (nút hiện/ẩn), bước phụ huynh (US2), và PHẢI không có ô họ tên, trường, số điện thoại, ảnh | FR-AUTH-033 · NFR-PRIV-001 · ADR-0104 | TC-AUTH-102 |
| F135-AC-03 | KHI học sinh gửi đăng ký hợp lệ, module identity PHẢI trong một use case tạo user Keycloak qua Admin API (role `student`, member Organization `common`, `temporary = false`), `identity.users` (`user_type = student`, `primary_tenant_id = common`, `display_name = NULL`), `org.students` (School ảo "EduWorld chung", `grade_level` đã chọn, `full_name = NULL`, `date_of_birth_enc`), ExternalIdentity, membership, bằng chứng `identity.self_registrations` và băm mã khôi phục, rồi trả `201` gồm `user_id`, `username`, `recovery_code` (một lần, `Cache-Control: no-store`) | FR-AUTH-033 · FR-AUTH-021 · [ACCOUNT_PROVISIONING §3.5](../../docs/09-identity/ACCOUNT_PROVISIONING.md#35-học-sinh-tự-đăng-ký-trong-game-adr-0104) | TC-AUTH-102 |
| F135-AC-04 | KHI tạo tài khoản thành công, module identity PHẢI ghi audit `user.self_registered` và outbox cùng transaction `user.provisioned` (`source = self_registration`) và `student.placement_changed`, không đưa ngày sinh, email phụ huynh, `device_id` vào payload | FR-AUTH-033 · AGENTS R10 · AGENTS R14 · [EVENT_DESIGN §4.1](../../docs/07-backend/EVENT_DESIGN.md) | TC-AUTH-102 |
| F135-AC-05 | NẾU một bước sau khi đã tạo user Keycloak thất bại, THÌ module identity PHẢI bù bằng cách vô hiệu hóa (`enabled = false`) user Keycloak vừa tạo và trả `503 UNAVAILABLE`, không để lại tài khoản đăng nhập được mà không có bản ghi `identity.users`; lần gửi lại cùng `Idempotency-Key` PHẢI hoàn tất hoặc trả cùng lỗi | FR-AUTH-033 · AGENTS R10 · AGENTS R18 | TC-AUTH-106 |
| F135-AC-06 | KHI `registerStudent` trả `201`, game client PHẢI hiện màn mã khôi phục (US3) rồi gọi `gameLogin` bằng tên đăng nhập + mật khẩu đang giữ trong RAM, xóa mật khẩu khỏi RAM sau lời gọi, và theo luồng đăng nhập F061 tới `createGameSession` và S02 (nickname theo F136) | FR-AUTH-033 · FR-AUTH-003 · ADR-0068 · ADR-0083 | TC-AUTH-102 |
| F135-AC-07 | NẾU dữ liệu không hợp lệ (khối ngoài 1–9, ngày sinh không tồn tại/tương lai/ngoài `self_registration_age_margin` so với khối, tên đăng nhập sai mẫu hoặc có tiền tố trong `self_registration_reserved_username_prefixes`, mật khẩu không qua password policy, thiếu đồng ý phụ huynh, có trường lạ như `full_name`), THÌ module identity PHẢI trả `422 VALIDATION_FAILED` với `errors[].field` và PHẢI không tạo gì ở Keycloak hay database | FR-AUTH-033 · ADR-0041 · AGENTS R5 | TC-AUTH-103 |
| F135-AC-08 | NẾU tên đăng nhập (sau trim, NFC, chữ thường) trùng tài khoản khác ở bất kỳ tenant nào, kể cả tài khoản soft-delete chưa ẩn danh hóa, THÌ module identity PHẢI trả `409 USERNAME_TAKEN` và game client PHẢI hiện đúng câu "Tên đăng nhập đã có người dùng, em chọn tên khác nhé" mà không nêu tài khoản đó thuộc đâu | FR-AUTH-031 · FR-AUTH-036 · ADR-0069 | TC-AUTH-103 |
| F135-AC-09 | NẾU đăng ký trong giờ giới nghiêm hiệu lực (`platform_curfew`), THÌ module identity PHẢI trả `403 CURFEW_ACTIVE` kèm `reopen_at` và game client PHẢI hiện `CurfewNotice`, không tạo tài khoản | FR-GAME-012 · ADR-0078 | TC-AUTH-092 |

---

### User Story 2 — Bước phụ huynh, bằng chứng đồng ý và email tùy chọn (Priority: P1)

Bố của Minh (lớp 2) không có email: bấm "Đọc to" nghe điều khoản, đánh dấu "Tôi là phụ huynh và đồng ý", bỏ trống ô email. Mẹ của An có email: nhập email, sau đó bấm link trong thư để xác nhận.

**Why this priority**: Owner yêu cầu phụ huynh xác nhận trước khi trẻ vào môi trường có người lạ; bằng chứng phải có từ tài khoản đầu tiên.

**Independent Test**: Đăng ký không email → bằng chứng có `terms_version`, `consented_at`, `device_id_hash`; đăng ký có email → `email_deliveries` có thư xác nhận; bấm link trên portal công khai → `parent_email_confirmed_at`; bấm lại → trang "Link đã được dùng"; chơi được ngay khi chưa xác nhận.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F135-AC-10 | Bước phụ huynh PHẢI hiện văn bản điều khoản lấy từ `getRegistrationTerms` theo ngôn ngữ (mặc định tiếng Việt) với nút "Đọc to", và ô đánh dấu "Tôi là phụ huynh và đồng ý" chưa đánh dấu sẵn; nút "Tạo tài khoản" PHẢI mờ cho tới khi ô được đánh dấu; client PHẢI gửi `parent_consent.terms_version` của văn bản đã hiện | FR-AUTH-034 · AGENTS R19 · ADR-0104 | TC-AUTH-104 |
| F135-AC-11 | KHI tạo tài khoản, module identity PHẢI lưu bằng chứng đồng ý gồm `terms_version` (phải là version đang hiệu lực, không thì `422`), `consented_at` theo đồng hồ server, HMAC `device_id` (khóa xoay vòng như `ip_hash`), nền tảng và `client_version`; PHẢI không lưu IP thô, user-agent thô hay vị trí | FR-AUTH-034 · AGENTS R5 · AGENTS R23 · [DATA_PRIVACY §2](../../docs/10-security/DATA_PRIVACY.md#2-data-inventory) | TC-AUTH-104 |
| F135-AC-12 | Ô email phụ huynh PHẢI tùy chọn; NẾU có email, THÌ module identity PHẢI kiểm định dạng (`maxLength` 254), lưu mã hóa cột kèm HMAC, xếp một thư xác nhận vào hàng đợi email (mẫu vi/en) chứa link một lần có hạn `parent_email_confirm_ttl`, và PHẢI không ghi email vào log, audit chi tiết hay event | FR-AUTH-034 · AGENTS R14 · NFR-PRIV-001 | TC-AUTH-104 |
| F135-AC-13 | KHI phụ huynh mở link xác nhận còn hạn trên trang công khai của portal, module identity (`confirmParentEmail`) PHẢI đặt `parent_email_confirmed_at`, vô hiệu token và trang PHẢI hiện "Cảm ơn phụ huynh đã xác nhận"; NẾU token sai, hết hạn hoặc đã dùng, THÌ PHẢI trả `410 LINK_EXPIRED` với một câu chung, không cho biết tài khoản nào | FR-AUTH-034 · FR-AUTH-035 | TC-AUTH-104 |
| F135-AC-14 | Tài khoản PHẢI chơi được đầy đủ dù email phụ huynh chưa xác nhận hoặc bỏ trống; số thư gửi tới một địa chỉ PHẢI ≤ `parent_email_daily_send_limit` mỗi ngày (vượt thì không gửi, đăng ký vẫn thành công) | FR-AUTH-034 · FR-AUTH-036 | TC-AUTH-104 |
| F135-AC-15 | Văn bản điều khoản PHẢI có version bất biến (đổi nội dung = version mới), PHẢI nêu mục đích dữ liệu thu, việc không thu họ tên, quyền liên hệ hỗ trợ để khóa tài khoản, và game client PHẢI hiển thị nó là văn bản thuần | FR-AUTH-034 · ADR-0104 | TC-AUTH-104 |

---

### User Story 3 — Mã khôi phục và "Quên mật khẩu" (Priority: P1)

Lan quên mật khẩu sau kỳ nghỉ: bấm "Quên mật khẩu?", nhập tên đăng nhập và mã khôi phục mẹ đã chụp, đặt mật khẩu mới, nhận mã mới.

**Why this priority**: Không có trường để đặt lại mật khẩu (FR-AUTH-017 "liên hệ nhà trường" không áp được); thiếu đường khôi phục thì tài khoản mất.

**Independent Test**: `recoverStudentPassword` mã đúng → `200` + mã mới, phiên cũ bị thu hồi; mã sai 5 lần → `429`; replay `registerStudent` → không có `recovery_code`; tài khoản có email đã xác nhận → `requestParentPasswordReset` `202` + thư; trang đặt lại mật khẩu công khai đổi được mật khẩu.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F135-AC-16 | KHI `registerStudent` trả `201`, game client PHẢI hiện màn "Mã khôi phục của em" với mã nhóm 4 ký tự cỡ lớn, câu "Phụ huynh hãy ghi lại hoặc chụp màn hình mã này", nút "Đọc to" và ô "Đã ghi lại mã" phải đánh dấu trước khi đi tiếp; mã PHẢI không lưu xuống đĩa, không vào log/telemetry/crash report | FR-AUTH-035 · AGENTS R14 | TC-AUTH-105 |
| F135-AC-17 | Module identity PHẢI chỉ lưu băm argon2id của mã khôi phục, một mã hiệu lực mỗi tài khoản; replay `registerStudent` cùng `Idempotency-Key` PHẢI trả kết quả đã lưu với `recovery_code_redacted = true`, không có mã | FR-AUTH-035 · AGENTS R10 | TC-AUTH-105 |
| F135-AC-18 | KHI học sinh gửi `recoverStudentPassword` với tên đăng nhập, mã khôi phục đúng và mật khẩu mới hợp lệ, module identity PHẢI đặt mật khẩu qua Keycloak Admin API, vô hiệu mã cũ, tạo mã mới trả **một lần** trong response, thu hồi mọi `game_auth_sessions` + phiên Keycloak và phát `user.access_revoked` (`reason = password_changed`), ghi audit không có mật khẩu/mã | FR-AUTH-035 · [ACCOUNT_PROVISIONING §6d](../../docs/09-identity/ACCOUNT_PROVISIONING.md#6d-đặt-lại-mật-khẩu-học-sinh) · ADR-0086 | TC-AUTH-105 |
| F135-AC-19 | NẾU tên đăng nhập không tồn tại, không phải tài khoản tự đăng ký, mã sai hoặc đã dùng, THÌ module identity PHẢI trả cùng một `401 RECOVERY_INVALID` với thời gian phản hồi không phân biệt được; NẾU vượt `recovery_attempt_limit` theo tên đăng nhập hoặc theo thiết bị, THÌ PHẢI trả `429 RATE_LIMITED` + `Retry-After` và PHẢI không khóa đăng nhập bằng mật khẩu | FR-AUTH-035 · FR-AUTH-028 · AGENTS R30 | TC-AUTH-105 |
| F135-AC-20 | KHI học sinh chọn "Gửi link cho phụ huynh" với tên đăng nhập, module identity (`requestParentPasswordReset`) PHẢI luôn trả `202`; chỉ khi tài khoản tự đăng ký có email phụ huynh đã xác nhận, PHẢI xếp thư có link đặt lại một lần hạn `parent_reset_link_ttl` tới email đó; trang công khai `/phu-huynh/dat-lai-mat-khau` PHẢI đặt mật khẩu mới (`resetPasswordByParentLink`), cấp mã khôi phục mới hiện một lần và thu hồi phiên như F135-AC-18 | FR-AUTH-035 · ADR-0091 | TC-AUTH-105 |
| F135-AC-21 | Form S01 của học sinh tự đăng ký PHẢI có "Quên mật khẩu?" mở lựa chọn "Dùng mã khôi phục" / "Gửi link cho phụ huynh" / "Học sinh của trường: liên hệ nhà trường"; mọi câu báo lỗi khôi phục PHẢI là câu chung, ô mật khẩu và mã PHẢI bị xóa sau mỗi lần gửi | FR-AUTH-017 · FR-AUTH-035 · [KEYCLOAK_SSO](../../docs/09-identity/KEYCLOAK_SSO.md) | TC-AUTH-105 |

---

### User Story 4 — Chống lạm dụng, idempotency, công tắc (Priority: P2)

Một bạn bấm đăng ký liên tục trên cùng máy; một phòng máy 40 máy sau cùng một IP NAT đăng ký cùng lúc; vận hành cần tắt đăng ký khi bị tấn công.

**Why this priority**: Không chặn thì tenant `common` đầy tài khoản ảo; chặn sai (theo IP) thì cả trường không đăng ký được (R30).

**Independent Test**: Cùng `device_id` lần 4 trong ngày → `429 REGISTRATION_LIMIT_REACHED`; 40 `device_id` cùng IP → 40 `201`; đặt trần giờ = 2 trong test → lần 3 `429 REGISTRATION_BUSY`; System Admin tắt → `getRegistrationTerms.enabled = false`, `registerStudent` `403 REGISTRATION_DISABLED`.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F135-AC-22 | NẾU số tài khoản tạo thành công từ cùng HMAC `device_id` trong ngày ICT đã đạt `self_registration_device_daily_limit`, THÌ module identity PHẢI trả `429 REGISTRATION_LIMIT_REACHED` kèm `retry_after` (00:00 ICT hôm sau); bộ đếm PHẢI tính từ PostgreSQL (không chỉ Valkey) và PHẢI không dùng địa chỉ IP | FR-AUTH-036 · AGENTS R9 · AGENTS R30 | TC-AUTH-106 |
| F135-AC-23 | NẾU số đăng ký toàn hệ thống trong 60 phút gần nhất đạt `self_registration_global_hourly_limit`, THÌ module identity PHẢI trả `429 REGISTRATION_BUSY` + `Retry-After` và game client PHẢI hiện "Nhiều bạn đang đăng ký, em thử lại sau ít phút nhé" | FR-AUTH-036 · NFR-AVAIL-001 | TC-AUTH-106 |
| F135-AC-24 | `registerStudent` PHẢI bắt buộc `Idempotency-Key`; cùng key cùng body PHẢI trả kết quả đã lưu (`Idempotent-Replayed`, không có mã khôi phục), cùng key khác body PHẢI trả `409 IDEMPOTENCY_KEY_REUSED`; mỗi lần đăng ký, lần bị từ chối vì giới hạn, mỗi lần khôi phục PHẢI có audit (không có mật khẩu, mã, email, ngày sinh) và metric `eduworld_self_registration_total{result}` | FR-AUTH-036 · AGENTS R10 · AGENTS R14 · AGENTS R18 | TC-AUTH-106 |
| F135-AC-25 | KHI System Admin đặt `self_registration_enabled = false` (cài đặt platform, lý do bắt buộc, audit), `getRegistrationTerms` PHẢI trả `enabled = false` và `registerStudent` PHẢI trả `403 REGISTRATION_DISABLED` trong ≤ thời gian cache cài đặt; luồng khôi phục mật khẩu PHẢI vẫn chạy; role tenant PHẢI không đổi được cài đặt này (`403`) | FR-AUTH-036 · AGENTS R20 | TC-AUTH-106 |
| F135-AC-26 | Mọi endpoint công khai của F135 (`getRegistrationTerms`, `registerStudent`, `recoverStudentPassword`, `requestParentPasswordReset`, `confirmParentEmail`, `resetPasswordByParentLink`) PHẢI không cần token và `X-Tenant-Id`, PHẢI có rate limit theo thiết bị/tên đăng nhập/token ở APISIX và ở backend, PHẢI không log body, và lỗi PHẢI theo Problem Details với thông điệp chung | FR-AUTH-003 · FR-AUTH-036 · AGENTS R7 · AGENTS R14 | TC-AUTH-100 |

---

### User Story 5 — Đủ tính năng như học sinh tenant chung, gộp vào trường sau này (Priority: P2)

Lan vào Main Hub: làm quest, học, thi đấu, vào club, dùng chat như mọi học sinh tenant chung; tab "Nhà trường giao" hiện "Em chưa có bài nhà trường giao. Mình cùng khám phá thế giới nhé!". Năm sau trường của Lan dùng EduWorld: System Admin gộp tài khoản, XP và tiến độ còn nguyên.

**Why this priority**: Owner "không được phép lược bỏ phần nào"; cần chứng minh không có nhánh hạn chế riêng.

**Independent Test**: Học sinh tự đăng ký và học sinh do System Admin tạo cùng khối chạy cùng bộ test quyền (chat policy, kết bạn, club, ghép trận, quest, bài luyện tập) → cùng kết quả; gộp tài khoản bằng saga F045 → XP ledger và attempts thấy ở tài khoản đích.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F135-AC-27 | Học sinh tự đăng ký PHẢI có cùng quyền, policy chat, mặc định xã hội của tenant `common`, giờ giới nghiêm, nội dung và tính năng game/học tập như học sinh tenant chung do System Admin tạo cùng khối; HỆ THỐNG PHẢI không có điều kiện nào đọc nguồn tài khoản (`account_source`) để cấp hay hạn chế tính năng ngoài màn quản trị | FR-AUTH-033 · AGENTS R15 · [CHILD_SAFETY §3.6](../../docs/10-security/CHILD_SAFETY.md#36-tenant-common-adr-0078) | TC-AUTH-102, TC-CHAT-047 |
| F135-AC-28 | KHI học sinh tự đăng ký mở tab "Nhà trường giao", game client PHẢI hiện trạng thái rỗng thân thiện (Mây + câu khuyến khích khám phá) thay vì lỗi, theo trạng thái rỗng chuẩn của S03 | FR-AUTH-033 · FR-LMS-006 · [STUDENT_GAME_UX §7](../../docs/05-ux/STUDENT_GAME_UX.md#7-trạng-thái-chuẩn) | TC-AUTH-102 |
| F135-AC-29 | KHI System Admin gộp tài khoản tự đăng ký vào tài khoản học sinh của trường bằng saga gộp tài khoản sẵn có (F045), HỆ THỐNG PHẢI giữ XP/level, vật phẩm, tiến độ quest và attempts học tập ở tài khoản đích theo [IDENTITY_MAPPING §7](../../docs/09-identity/IDENTITY_MAPPING.md#7-liên-kết-tài-khoản-account-linking), và bằng chứng đồng ý PHẢI giữ cùng tài khoản nguồn (soft-delete, không xóa vật lý) | FR-AUTH-033 · AGENTS R21 · ADR-0028 | TC-AUTH-107 |
| F135-AC-30 | Dọn học sinh không đăng nhập (ADR-0032) PHẢI áp cho học sinh tự đăng ký như học sinh tenant chung khác (cùng tham số, báo trước, khôi phục khi đăng nhập lại), và ẩn danh hóa PHẢI xóa ngày sinh, email phụ huynh, băm mã khôi phục, giữ `terms_version`/`consented_at` | FR-AUTH-024 · FR-AUTH-036 · [DATA_PRIVACY](../../docs/10-security/DATA_PRIVACY.md) | TC-AUTH-107 |

---

### User Story 6 — System Admin quản lý học sinh tự đăng ký (Priority: P2)

System Admin mở danh sách học sinh tenant `common`, lọc "Tự đăng ký", xem bằng chứng đồng ý của một tài khoản bị phụ huynh khiếu nại, khóa tài khoản theo yêu cầu phụ huynh.

**Why this priority**: Owner yêu cầu quản trị được; cần cho xử lý khiếu nại và lạm dụng.

**Independent Test**: Portal workspace D với `sa.platform`: bộ lọc + nhãn; trang chi tiết hiện bằng chứng (email đã che); `ad.truong.a` (role tenant) gọi API → `403`/`404`; khóa/đặt lại mật khẩu dùng nút sẵn có, đặt lại mật khẩu cấp mã khôi phục mới.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F135-AC-31 | Danh sách học sinh tenant `common` của System Admin (`listTenantGradeStudents`, `/admin/users`) PHẢI trả `account_source` (`platform_common`/`self_registration`), hỗ trợ lọc `account_source=self_registration` (phân trang cursor như cũ), và portal PHẢI hiện nhãn "Tự đăng ký" + bộ lọc; role tenant PHẢI không thấy dữ liệu tenant `common` (`404`) | FR-AUTH-036 · AGENTS R8 · AGENTS R20 | TC-AUTH-107 |
| F135-AC-32 | KHI System Admin mở chi tiết học sinh tự đăng ký, `getSelfRegistrationEvidence` (quyền `platform.user.manage`, gateway nội bộ) PHẢI trả `terms_version`, `consented_at`, nền tảng, `client_version`, email phụ huynh đã che và `parent_email_confirmed_at`, không trả HMAC thiết bị hay email rõ; mỗi lần xem PHẢI ghi audit | FR-AUTH-034 · FR-AUTH-036 · AGENTS R14 | TC-AUTH-107 |
| F135-AC-33 | KHI System Admin đặt lại mật khẩu học sinh tự đăng ký bằng `resetStudentPassword` sẵn có, module identity PHẢI cấp thêm mã khôi phục mới (trả một lần cùng mật khẩu tạm) và vô hiệu mã cũ; khóa/mở khóa dùng luồng F016 không đổi | FR-AUTH-035 · FR-AUTH-036 · [ACCOUNT_PROVISIONING §6d](../../docs/09-identity/ACCOUNT_PROVISIONING.md#6d-đặt-lại-mật-khẩu-học-sinh) | TC-AUTH-107 |

---

### User Story 7 — Giao diện thân thiện lớp 1–9, điện thoại, song ngữ, riêng tư (Priority: P2)

**Why this priority**: Người dùng là trẻ 6–15 tuổi và phụ huynh nông thôn; màn phải dễ như S01/S02 (F114, F125).

**Independent Test**: Ma trận khối (`test_grade_band_matrix.gd`) cho mọi màn mới × 4 khối × 1280×720, 1024×600, 844×390, 740×360 × `vi`/`en`/pseudo; ảnh trước/sau; quét log/telemetry không có mật khẩu, mã, ngày sinh, email.

**Acceptance Criteria (EARS)**:

| Mã | Tiêu chí | Nguồn | TC |
|---|---|---|---|
| F135-AC-34 | Màn đăng ký, bước phụ huynh, màn mã khôi phục và "Quên mật khẩu" PHẢI chơi ngang, chia bước có thanh tiến độ (≤ 1 nhóm ô mỗi bước ở `compact`), nút chạm ≥ `density.compact.button.min_height`, chữ ≥ 12 CSS px và vùng chạm ≥ 40 CSS px vật lý ở 844×390 và 740×360, có nút "Đọc to" cho hướng dẫn mỗi bước, mỗi cửa sổ có đúng một nút "✕ Đóng" đỏ (quay về S01, xóa mọi ô) | FR-GAME-010 · NFR-A11Y-001 · F125-AC-03 · F125-AC-06 | TC-UI-003 |
| F135-AC-35 | Mọi chuỗi UI mới PHẢI qua khóa `ui.register.*`, `ui.recovery.*`, `ui.error.<CODE>` (vi/en), lỗi theo field hiện cạnh ô bằng chữ + icon (không chỉ màu), và bố cục PHẢI chịu pseudo-locale dài hơn 35% không cắt chữ | NFR-I18N-001 · NFR-I18N-003 · ADR-0056 | TC-I18N-006 |
| F135-AC-36 | Ma trận khối PHẢI dựng mọi màn mới × 4 khối × 4 cửa sổ × `vi`/`en`/pseudo và pass probe không cắt chữ, không tràn, nút ≥ ngưỡng; ảnh PHẢI được chụp cho mỗi màn ở 844×390, 740×360 (4 khối `vi`) và 1280×720 (`g1_3`, `g8_9`), xem từng ảnh và ghi bảng ở quickstart §5 | FR-GAME-010 · F125-AC-04 | TC-UI-003 |
| F135-AC-37 | game client PHẢI giữ mật khẩu, mã khôi phục, ngày sinh, email phụ huynh chỉ trong RAM của màn, xóa khi rời màn, không ghi `user://`, log, telemetry, crash report; chỉ tên đăng nhập được điền sẵn S01 nếu học sinh chọn "Nhớ tên đăng nhập" | FR-AUTH-003 · AGENTS R14 · ADR-0086 | TC-AUTH-100 |
| F135-AC-38 | Màn đăng ký PHẢI là UI trong game gọi REST qua `ApiClient` (không trình duyệt/WebView, không trang Keycloak), không chứa logic kiểm hợp lệ nào ngoài kiểm để thân thiện (server luôn kiểm lại — R5), và nhân sự dùng tài khoản đăng ký được rồi đăng nhập game vẫn chỉ ra học sinh (`user_type = student`) | FR-AUTH-002 · FR-AUTH-022 · AGENTS R1 · ADR-0068 | TC-AUTH-001 |

### Edge Cases

- NẾU mất mạng sau khi gửi đăng ký, THÌ client gửi lại cùng `Idempotency-Key` khi có mạng; replay không có mã khôi phục → client hiện "Tài khoản đã tạo. Nếu chưa ghi mã khôi phục, phụ huynh nhờ hỗ trợ đặt lại" và vẫn đăng nhập (mật khẩu còn trong RAM). Mã chỉ thấy được ở lần trả đầu (F135-AC-17).
- NẾU `gameLogin` sau đăng ký trả `429 LOGIN_QUEUE_BUSY`, THÌ theo hàng chờ F061; tài khoản đã tạo.
- NẾU học sinh đóng game ở màn mã khôi phục, THÌ mã không hiện lại; khôi phục qua System Admin (F135-AC-33) hoặc email phụ huynh.
- NẾU học sinh đã có tài khoản trường và tự đăng ký thêm, THÌ hai tài khoản tách biệt; gộp do System Admin (F135-AC-29). Hệ thống không đoán trùng người theo ngày sinh.
- NẾU thiết bị là máy dùng chung phòng máy (`shared_device`), THÌ vẫn áp `self_registration_device_daily_limit` theo `device_id` của bản cài/Web; phòng máy nhiều học sinh nên đăng ký trên thiết bị cá nhân hoặc qua tài khoản trường — câu hỏi mở F135-3.
- NẾU tenant `common` bị tắt (`TENANT_ACCESS_DISABLED`), THÌ `registerStudent` trả `403 REGISTRATION_DISABLED`.
- NẾU văn bản điều khoản đổi version khi học sinh đang ở màn, THÌ `422` `errors[0].field = parent_consent.terms_version` → client tải lại điều khoản và yêu cầu đánh dấu lại.

## Requirements *(bắt buộc)*

### Functional Requirements

| FR (PRD) | Phần feature này làm | Tài liệu sở hữu quy tắc |
|---|---|---|
| FR-AUTH-001 | Thêm nguồn tài khoản "tự đăng ký trong game" (chỉ học sinh, chỉ tenant `common`) | [ACCOUNT_PROVISIONING §1, §3.5](../../docs/09-identity/ACCOUNT_PROVISIONING.md) |
| FR-AUTH-017 | Nút "Tạo tài khoản" và "Quên mật khẩu?" mở rộng trên S01 | [STUDENT_GAME_UX_CORE S01](../../docs/05-ux/STUDENT_GAME_UX_CORE.md#s01--đăng-nhập), [KEYCLOAK_SSO](../../docs/09-identity/KEYCLOAK_SSO.md) |
| FR-AUTH-021 | Tenant chung nhận thêm tài khoản tự đăng ký, không JIT khi đăng nhập | [MULTITENANCY §8.3](../../docs/06-architecture/MULTITENANCY.md#83-tenant-chung) |
| FR-AUTH-033 | Màn đăng ký, tạo tài khoản qua identity, event, audit, idempotency | [ACCOUNT_PROVISIONING §3.5](../../docs/09-identity/ACCOUNT_PROVISIONING.md#35-học-sinh-tự-đăng-ký-trong-game-adr-0104) |
| FR-AUTH-034 | Bước phụ huynh, bằng chứng đồng ý, email tùy chọn + xác nhận | [CHILD_SAFETY §3.6](../../docs/10-security/CHILD_SAFETY.md#36-tenant-common-adr-0078), [DATA_PRIVACY §2](../../docs/10-security/DATA_PRIVACY.md#2-data-inventory) |
| FR-AUTH-035 | Mã khôi phục, quên mật khẩu, link phụ huynh | [ACCOUNT_PROVISIONING §3.5, §6d](../../docs/09-identity/ACCOUNT_PROVISIONING.md) |
| FR-AUTH-036 | Giới hạn theo thiết bị/toàn cục, công tắc, quản trị | [ACCOUNT_PROVISIONING §3.5](../../docs/09-identity/ACCOUNT_PROVISIONING.md#35-học-sinh-tự-đăng-ký-trong-game-adr-0104) |

NFR: NFR-PRIV-001, NFR-SAFE-001, NFR-A11Y-001, NFR-I18N-001, NFR-I18N-003, NFR-AVAIL-001.

### Key Entities

- **SelfRegistration** (`identity.self_registrations`): bằng chứng đăng ký và đồng ý của phụ huynh của một học sinh tự đăng ký — phiên bản điều khoản, thời điểm, HMAC thiết bị, nền tảng, email phụ huynh (mã hóa) và thời điểm xác nhận.
- **RecoveryCode** (`identity.recovery_codes`): băm mã khôi phục hiệu lực, thời điểm tạo/dùng/vô hiệu.
- **ParentLinkToken** (`identity.parent_link_tokens`): token một lần (SHA-256) cho xác nhận email và đặt lại mật khẩu qua phụ huynh.
- **RegistrationTerms**: văn bản điều khoản có version (seed module identity).
- **AccountSource**: thuộc tính hiển thị của học sinh (`sis`, `platform_common`, `self_registration`) chỉ dùng cho quản trị.

### Tham số

| Tham số (CONFIG_DEFAULTS) | Dùng để |
|---|---|
| `self_registration_enabled` | Công tắc platform bật/tắt tự đăng ký |
| `self_registration_device_daily_limit` | Số tài khoản tối đa mỗi thiết bị mỗi ngày ICT |
| `self_registration_global_hourly_limit` | Trần đăng ký toàn hệ thống mỗi 60 phút |
| `self_registration_age_margin` | Biên tuổi hợp lý so với khối |
| `self_registration_reserved_username_prefixes` | Tiền tố tên đăng nhập dành riêng |
| `parent_email_confirm_ttl`, `parent_reset_link_ttl`, `parent_email_daily_send_limit` | Link email phụ huynh |
| `recovery_attempt_limit` | Số lần thử mã khôi phục |
| `platform_curfew`, `inactive_student_cleanup_days`, `common_tenant_social_defaults`, `login_rate_limit` | Áp nguyên (không đổi) |

### Ràng buộc phi chức năng

| Ràng buộc | Nguồn |
|---|---|
| Không thu họ tên thật; ngày sinh/email mã hóa cột; không log PII, mật khẩu, mã | AGENTS R14, R16 · NFR-PRIV-001 |
| Không IP thô, không giới hạn theo IP | AGENTS R23, R30 |
| Một bộ quy tắc chat; không hạn chế/nới riêng cho học sinh tự đăng ký | AGENTS R15 · ADR-0020 · ADR-0078 |
| Thao tác ghi idempotent + audit | AGENTS R10 |
| Không có trang đăng ký Keycloak; module identity gọi Admin API qua adapter có timeout/circuit breaker | AGENTS R29 · ADR-0068 |
| Không dark pattern: ô đồng ý không đánh dấu sẵn, không đếm ngược | AGENTS R19 |

## Success Criteria *(bắt buộc)*

### Measurable Outcomes

- **SC-001**: Học sinh lớp 1–3 cùng phụ huynh hoàn tất đăng ký tới S02 trong ≤ 3 phút trên điện thoại tầm trung (kiểm tay 5 cặp phụ huynh–học sinh trong pilot).
- **SC-002**: 0 bản ghi họ tên thật, IP thô, mật khẩu, mã khôi phục bản rõ trong database, log, trace, telemetry của bộ test.
- **SC-003**: 100% học sinh tự đăng ký có bằng chứng đồng ý với `terms_version` hợp lệ.
- **SC-004**: Bộ test quyền cho học sinh tự đăng ký và học sinh tenant chung do System Admin tạo cho cùng kết quả ở 100% kịch bản.
- **SC-005**: 40 thiết bị sau một IP NAT đăng ký đồng thời thành công 100% (không bị giới hạn theo IP).
- **SC-006**: Khôi phục mật khẩu bằng mã thành công trong ≤ 4 thao tác.

## Ngoài phạm vi

- Mô hình consent chung, rút đồng ý tự phục vụ, xác minh danh tính phụ huynh, xác minh tuổi, DPIA (ADR-0077 hoãn theo ADR-0082) — chỉ có bằng chứng xác nhận của F135.
- Đăng ký cho nhân sự, đăng ký trên portal hay trang Keycloak; đăng nhập bằng mạng xã hội; JIT vào tenant chung khi đăng nhập.
- Thay đổi saga gộp tài khoản (F045), dọn học sinh (F016), mặc định xã hội tenant `common` (F120) — chỉ kiểm hành vi (F135-AC-29, AC-30, AC-27).
- Nickname tự gõ và kiểm duyệt nickname → F136. Tự động lên khối mỗi năm học → câu hỏi mở F135-1.

## Assumptions

- F010, F013, F016, F045, F061, F124 đã có (hoặc làm trước theo CODE_PLAN): adapter Keycloak Admin của `createCommonTenantStudents`, hàng đợi email của module identity, saga gộp tài khoản. F136 làm trước hoặc cùng đợt; nếu F136 chưa xong, S02 dùng nickname gợi ý (F070) và F135 vẫn chạy.
- Câu hỏi mở (không chặn Ready): **F135-1** tự đổi khối mỗi năm học cho học sinh tự đăng ký; **F135-2** xác nhận saga gộp tài khoản F045 chạy giữa tenant `common` và tenant trường (ACCOUNT_PROVISIONING §3.4 bước 5 nói lịch sử ở `common` giữ `tenant_id = common`) — T040 kiểm, lệch thì mở việc sửa saga riêng; **F135-3** phòng máy dùng chung (`shared_device`) có cần giới hạn đăng ký riêng; **F135-4** pháp chế rà soát văn bản điều khoản và cách xác nhận bằng ô đánh dấu trước khi mở rộng ra ngoài pilot (ADR-0104).
