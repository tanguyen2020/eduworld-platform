# ADR-0085 — Dọn học sinh không đăng nhập: giữ user Keycloak, gỡ membership

> **Trạng thái:** Accepted · **Ngày:** 2026-11-16 · **OQ:** — · **Thay một phần:** ADR-0032 (mục 3, "vô hiệu hóa user Keycloak") · **Tài liệu:** [ACCOUNT_PROVISIONING](../09-identity/ACCOUNT_PROVISIONING.md) · [specs/016-account-lifecycle](../../specs/016-account-lifecycle/spec.md) · [Chỉ mục ADR](README.md)

## Bối cảnh

[ADR-0032](ADR-0032-worker-don-tai-khoan-hoc-sinh-khong-dang-nhap-game.md) mục 3 yêu cầu vô hiệu hóa user Keycloak khi dọn học sinh không đăng nhập, trong khi FR-AUTH-024, AC-AUTH-018 và TC-AUTH-022 yêu cầu **tự khôi phục khi học sinh đăng nhập lại**. Với tài khoản local, user Keycloak bị vô hiệu hóa thì không đăng nhập lại được, nên hai yêu cầu không cùng đạt (câu hỏi chặn của feature F016).

## Quyết định

1. Khi dọn: **giữ user Keycloak ở trạng thái enabled**; gỡ membership tenant (Keycloak Organization và `identity` membership), thu hồi mọi phiên/refresh token, soft-delete hồ sơ (User/Student/GameProfile theo ADR-0028), rời party.
2. Học sinh đăng nhập lại: xác thực Keycloak vẫn thành công; identity-service nhận diện tài khoản đã dọn, khôi phục hồ sơ và membership theo dữ liệu SIS hiện hành (FR-AUTH-024). Không còn trong SIS → từ chối như tài khoản chưa được cấp (`ACCOUNT_NOT_PROVISIONED`).
3. Các phần còn lại của ADR-0032 (cấu hình, báo trước, loại trừ, ngưỡng xác nhận) giữ nguyên.

## Hệ quả

(+) Khôi phục tự động khi đăng nhập lại; không cần thao tác thủ công của nhà trường. (−) User Keycloak của học sinh đã dọn vẫn tồn tại; an toàn vì không có membership nên không vào được tenant nào.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-16 | Tạo ADR | Technical Architect |
