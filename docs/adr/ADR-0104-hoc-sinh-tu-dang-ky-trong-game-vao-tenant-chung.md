# ADR-0104 — Học sinh tự đăng ký trong game vào tenant chung

> **Trạng thái:** Accepted · một phần supersedes ADR-0063 (mục 2, nguồn tài khoản tenant chung) · **Ngày:** 2026-11-28 · **OQ:** OQ-27 · **Tài liệu:** [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) · [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md) · [MULTITENANCY §8.3](../06-architecture/MULTITENANCY.md#83-tenant-chung) · [CHILD_SAFETY](../10-security/CHILD_SAFETY.md) · [DATA_PRIVACY](../10-security/DATA_PRIVACY.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

FR-AUTH-001, ACCOUNT_PROVISIONING §1 và KEYCLOAK_SSO chốt "không có self-registration": tài khoản học sinh chỉ đến từ SIS của trường hoặc do System Admin tạo/import vào tenant `common` (ADR-0063 mục 2). Đến 2026-11-28 chưa có trường nào tích hợp SIS, và có học sinh muốn chơi thử mà không thuộc trường nào. Owner hỏi (OQ-27, lúc đầu để làm sau), rồi ngày 2026-11-28 chốt các điểm còn mở của OQ-27: xác nhận phụ huynh, mức xã hội, thời hạn, chống tạo tài khoản ảo.

Ràng buộc liên quan: game đăng nhập bằng form trong game qua backend (ADR-0068), không mở trang Keycloak; mặc định an toàn cho tenant `common` (ADR-0078); một người một tài khoản SSO, username không trùng (ADR-0069); không giới hạn đăng nhập theo IP (R30), không lưu IP thô (R23); pháp lý (mô hình consent, DPIA, quy định game cho người dưới 18 tuổi) **không** thuộc version này (ADR-0082, ADR-0077 hoãn).

## Quyết định

1. **Học sinh được tự đăng ký trong game, chỉ vào tenant `common`** [Confirmed — owner 2026-11-28]. Màn "Tạo tài khoản" vẽ bằng Godot (mở từ S01), luôn ngang, dễ dùng cho lớp 1–9, có đọc to, theo luật bố cục điện thoại F125. Tài khoản thuộc School ảo "EduWorld chung" với khối học sinh chọn. Portal và nhân sự **không** có tự đăng ký.
2. **Dữ liệu thu tối thiểu** [Confirmed — owner 2026-11-28]: bắt buộc tên đăng nhập, mật khẩu, khối 1–9, ngày sinh (kiểm hợp lý với khối theo `self_registration_age_margin`; không bao giờ hiển thị cho người chơi khác — R16). **Không** thu họ tên thật. Nickname chọn ở màn tạo nhân vật (ADR-0105).
3. **Xác nhận của phụ huynh trên màn đăng ký** [Confirmed — owner 2026-11-28]: điều khoản ngắn tiếng Việt có version, nút đọc to, ô "Tôi là phụ huynh và đồng ý" (không đánh dấu sẵn). Lưu bằng chứng: thời điểm theo server, phiên bản điều khoản, HMAC `device_id` (không IP thô), nền tảng, phiên bản client. Email phụ huynh **tùy chọn**: có thì gửi link xác nhận (tăng tin cậy, dùng khôi phục mật khẩu); không bắt buộc để chơi.
4. **Khôi phục không cần email** [Confirmed — owner 2026-11-28]: mã khôi phục dùng một lần hiện một lần ở cuối đăng ký (phụ huynh ghi/chụp), chỉ lưu băm; "Quên mật khẩu" bằng tên đăng nhập + mã → mật khẩu mới + mã mới, thu hồi mọi phiên; có email phụ huynh đã xác nhận thì nhận được link đặt lại. Có rate limit, một thông điệp lỗi chung.
5. **Đủ tính năng như học sinh tenant chung** [Confirmed — owner 2026-11-28: "không được phép lược bỏ phần nào"]: chat, bạn bè, club, thi đấu, quest theo cùng một bộ quy tắc (R15) và cùng mặc định tenant `common` (mục 10; không thêm hạn chế, không nới riêng); "Nhà trường giao" giữ trên UI với trạng thái rỗng thân thiện. Sau này gộp vào trường thật bằng saga gộp tài khoản sẵn có, giữ XP/tiến độ. Dọn học sinh không đăng nhập (ADR-0032) áp nguyên.
6. **Chống lạm dụng** [Confirmed — owner 2026-11-28; tham số chốt theo "chốt hết"]: giới hạn số tài khoản mỗi thiết bị (bản cài) mỗi ngày `self_registration_device_daily_limit` và trần toàn cục `self_registration_global_hourly_limit`, đếm trong PostgreSQL (R9); không bao giờ theo một IP (R30); không CAPTCHA (trẻ nhỏ, trợ năng); thông điệp lỗi chung, chỉ báo "Tên đăng nhập đã có người dùng" khi trùng (ADR-0069); tiền tố tên đăng nhập dành riêng `self_registration_reserved_username_prefixes`; mọi thao tác ghi idempotent (`Idempotency-Key`) và có audit; System Admin tắt nhanh bằng `self_registration_enabled`; đăng ký bị chặn trong giờ giới nghiêm như đăng nhập.
7. **Tạo tài khoản qua module identity, không qua Keycloak** [Confirmed — owner "chốt hết"]: realm giữ `registrationAllowed = false`; module identity tạo user bằng Admin API (cùng bộ điều hợp và thứ tự bước của `createCommonTenantStudents`), phát `user.provisioned` với `source = self_registration`. Đăng ký không trả token: client gọi `gameLogin` như mọi lần đăng nhập để đường đăng nhập chỉ có một (R30).
8. **Quản trị** [Confirmed — owner 2026-11-28]: System Admin / nhân sự platform xem, lọc (nhãn "Tự đăng ký"), xem bằng chứng (email che), khóa, đặt lại mật khẩu (kèm mã khôi phục mới) học sinh tự đăng ký trên màn tenant `common` sẵn có. Phụ huynh muốn dừng: liên hệ hỗ trợ, System Admin khóa tài khoản (không có API rút đồng ý tự phục vụ trong version này).
9. **Rủi ro pháp lý được ghi nhận, không giải quyết ở đây** [Confirmed — owner 2026-11-28]: đồng ý bằng một ô đánh dấu không xác minh được người đánh dấu là phụ huynh, không xác minh tuổi, và có thể chưa đáp ứng yêu cầu về sự đồng ý của cha mẹ/người giám hộ khi xử lý dữ liệu cá nhân của trẻ em theo pháp luật bảo vệ dữ liệu cá nhân của Việt Nam (Nghị định 13/2023/NĐ-CP, Luật Bảo vệ dữ liệu cá nhân) và quy định về trò chơi điện tử cho người dưới 18 tuổi. Đây là biện pháp sản phẩm/kỹ thuật owner chọn để mở dùng thử, **không** phải mô hình consent pháp lý (ADR-0077 vẫn hoãn theo ADR-0082). **Khuyến nghị:** pháp chế rà soát văn bản điều khoản và cách xác nhận trước khi mở rộng ra ngoài pilot hoặc quảng bá rộng; nếu pháp chế yêu cầu xác minh mạnh hơn (ví dụ email phụ huynh bắt buộc có xác nhận, hoặc xác minh qua kênh khác), thay bằng ADR mới.
10. **Mặc định xã hội của tenant `common` bật** [Confirmed — owner 2026-11-28] (thay một phần ADR-0078 mục 3): owner yêu cầu học sinh tự đăng ký có đủ tính năng, "không được phép lược bỏ phần nào", nên trong tenant `common` **kết bạn, DM và World chat bật mặc định** cho mọi học sinh của tenant (không riêng học sinh tự đăng ký, giữ R15). System Admin giữ công tắc sự cố `putCommonTenantSocialOverride` tắt từng mục (`direct_enabled`, `world_enabled`, `friends_enabled`) cho một tenant `common`, có lý do + audit. Không đổi: Area chỉ quick + emote, mọi tin qua pipeline kiểm duyệt, bước PII/grooming luôn chạy (ADR-0078 mục 1), giờ giới nghiêm platform, chặn/tắt tiếng/báo cáo, Platform Safety Team kiểm duyệt. Dòng override đã lưu giữ nguyên nghĩa (mỗi cờ là trạng thái hiệu lực; chỉ nghĩa của "chưa có dòng" đổi).

## Hệ quả

- (+) Học sinh ngoài trường đối tác chơi được ngay; dữ liệu học tập/game giữ được khi trường tham gia sau.
- (+) Không mở bề mặt tấn công của Keycloak (không trang đăng ký); đường đăng nhập không đổi.
- (+) Dữ liệu tối thiểu: không họ tên, ngày sinh/email mã hóa cột, không IP.
- (−) Tenant `common` có thể lớn nhanh và có tài khoản bỏ dở; giảm bằng giới hạn thiết bị và dọn học sinh không đăng nhập.
- (−) Không có `school_admin` cho các học sinh này: Platform Safety Team và System Admin gánh kiểm duyệt và hỗ trợ (đã là mô hình của tenant `common`).
- (−) Rủi ro pháp lý ở mục 9 còn mở cho tới khi pháp chế rà soát.
- Việc phải làm theo (cùng MR): PRD FR-AUTH-001/017/021 sửa, FR-AUTH-033…036 mới; ACCOUNT_PROVISIONING §1, §3.5; KEYCLOAK_SSO; MULTITENANCY §8.3; CHILD_SAFETY §3.6; DATA_PRIVACY §2; STUDENT_GAME_UX_CORE S01; PRODUCT_VISION; GLOSSARY; CONFIG_DEFAULTS; OPEN_QUESTIONS OQ-27 Answered; feature spec [F135](../../specs/135-game-self-registration/spec.md).

## Phương án đã loại

1. **Bật trang đăng ký của Keycloak** (`registrationAllowed = true`): mở trình duyệt trái ADR-0068, khó kiểm giới hạn theo thiết bị và bằng chứng phụ huynh, mở endpoint công khai trên IdP.
2. **Email phụ huynh bắt buộc có xác nhận trước khi chơi:** owner loại vì nhiều phụ huynh nông thôn không có email.
3. **Tài khoản dùng thử N ngày rồi khóa:** owner chọn lâu dài trong tenant `common` rồi gộp khi trường tham gia.
4. **Hạn chế xã hội riêng cho học sinh tự đăng ký** (chỉ câu nhanh, không club): owner chọn đủ tính năng như học sinh tenant chung; an toàn dựa vào mặc định tenant `common` (ADR-0078) và một bộ quy tắc chat.
5. **Giới hạn theo IP hoặc CAPTCHA:** trái R30 (NAT trường học); CAPTCHA khó với trẻ lớp 1–3 và người dùng trợ năng.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-28 | Tạo ADR (Accepted), đóng OQ-27 | Owner 2026-11-28; F135 |
| 2026-11-28 | Thêm mục 10: tenant `common` bật mặc định kết bạn, DM, World chat (thay một phần ADR-0078 mục 3); công tắc sự cố của System Admin | Owner 2026-11-28 |
