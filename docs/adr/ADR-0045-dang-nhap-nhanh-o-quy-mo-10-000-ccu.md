# ADR-0045 — Đăng nhập nhanh ở quy mô 10.000 CCU

> **Trạng thái:** Accepted · một phần superseded by ADR-0072 · **Ngày:** 2026-11-15 · **OQ:** OQ-03 · **Tài liệu:** [KEYCLOAK_SSO](../09-identity/KEYCLOAK_SSO.md), [DEPLOYMENT](../11-operations/DEPLOYMENT.md), [TEST_STRATEGY](../12-project/TEST_STRATEGY.md) · [Chỉ mục ADR](README.md)

> [!WARNING]
> **Một phần đã được thay bởi [ADR-0072](ADR-0072-bo-vai-tro-giao-vien-hoc-sinh-tu-hoc-kiem-duyet-tap-trung.md).** Không còn hiệu lực: "mã lớp ngắn hạn do giáo viên mở" trong cách đăng nhập nhanh (mục 4); phương án đăng nhập nhanh cho lớp 1–3 được đề xuất lại ở ADR-0076 (Proposed). **Còn hiệu lực:** năng lực đăng nhập 10.000 CCU và các mục còn lại theo chú thích hiện có.

> [!WARNING]
> **Mục 3 (phiên dài, refresh token lưu trên thiết bị) đã được thay bởi ADR-0064; mục 4 (đăng nhập nhanh) đã được thay bởi ADR-0063** (2026-11-15): QR lớp + PIN, thẻ QR, mật khẩu hình ảnh, Device Grant, SSO của trường không thuộc Release 1.0. **Còn hiệu lực:** mục tiêu tải, SLO, phiên dài + refresh token, năng lực Keycloak, biên/backend, kiểm chứng.

## Bối cảnh

Stakeholder xác nhận mục tiêu **10.000 CCU** và lo ngại đăng nhập bằng username/password sẽ chậm. Chậm chủ yếu do băm mật khẩu ở Keycloak (tốn CPU) cộng với "bão đăng nhập" khi cả trường vào học cùng lúc.

## Quyết định

1. **Mục tiêu tải [Confirmed]:** 10.000 CCU; bão đăng nhập 3.000 lượt/10 phút, đỉnh 50 lượt đăng nhập mật khẩu/giây; 500 refresh token/giây.
2. **SLO [Confirmed]:** đăng nhập mật khẩu (bấm nút → vào hub) p95 ≤ 3 s, trong đó Keycloak p95 ≤ 800 ms; refresh p95 ≤ 200 ms; `POST /auth/session` + `POST /game/sessions` p95 ≤ 300 ms.
3. **Ít phải nhập mật khẩu:** client `eduworld-game` dùng **refresh token rotation**, SSO idle 7 ngày, tối đa 30 ngày [Confirmed]; an toàn nhờ một thiết bị (ADR-0031), thu hồi khi takeover/khóa, đăng xuất trên máy dùng chung. Lần mở game sau chỉ refresh.
4. **Cách đăng nhập nhanh [Confirmed, chọn theo trường]:** SSO qua IdP của trường/đơn vị (Google/Microsoft); mã QR (mã lớp ngắn hạn do giáo viên mở kèm PIN, hoặc thẻ QR cá nhân) qua authenticator tùy biến; Device Authorization Grant cho máy phòng tin học; mật khẩu hình ảnh cho lớp 1–3 (UX đăng nhập, không liên quan chính sách chat).
5. **Keycloak đủ năng lực:** cluster ≥ 3 node 4 vCPU/8 GB [Confirmed], **scale theo lịch học** (ADR-0024, không autoscale theo tải); persistent user sessions (Keycloak 26) + cache Infinispan; băm mật khẩu argon2id/pbkdf2 benchmark ≤ 80 ms/lần, rehash khi đăng nhập; theme đăng nhập qua CDN; brute-force detection theo tài khoản, không khóa theo IP NAT của trường.
6. **Biên và backend:** APISIX rate limit `/auth` theo tenant và thiết bị, `429` + `Retry-After`, client thử lại có jitter; **hàng chờ đăng nhập** ("Đang vào lớp…"); backend cache JWKS, `principals:resolve` và tenant (60 s); bootstrap trả manifest/hồ sơ từ cache; tạo game session không gọi đồng bộ nhiều service; **warm-up** trước khung giờ cao điểm theo lịch học.
7. **Kiểm chứng:** k6 (luồng game) + keycloak-benchmark (luồng đăng nhập) trên staging tương đương production; không tuyên bố đạt 10.000 CCU khi chưa có kết quả benchmark.

## Hệ quả

(+) Phần lớn lượt vào game chỉ là refresh token; bão đăng nhập được hấp thụ bằng năng lực chuẩn bị trước + hàng chờ. (−) Phiên dài đòi hỏi đăng xuất/thu hồi tốt; authenticator tùy biến (QR, mật khẩu hình ảnh) phải phát triển, bảo trì khi nâng cấp Keycloak và kiểm tra bảo mật.

## Lịch sử thay đổi

| Ngày | Thay đổi | Nguồn |
|---|---|---|
| 2026-11-15 | Accepted (10.000 CCU) | Stakeholder |
| 2026-11-15 | Autoscale Keycloak đổi thành scale theo lịch học | ADR-0024 |
| 2026-11-15 | Một phần superseded by ADR-0072 (bỏ vai trò giáo viên) | Technical Architect |
